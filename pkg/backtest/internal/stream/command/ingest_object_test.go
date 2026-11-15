package command_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	bs "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const failedIngestSentence = "Couldn't load data for backtest 'test-backtest': the data service is missing " +
	"its database driver (psycopg). This is a bug in the Foreverbull install, not in your strategy or settings. " +
	"Nothing was saved, so run `fbull backtest create` again once it's fixed." +
	"\nDetails: ModuleNotFoundError: No module named 'psycopg' (postgresql://foreverbull:***@postgres/x)"

func ingestEngineFailure() *engine.MockEngine {
	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Ingest", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("error ingesting: %w",
		status.Error(codes.FailedPrecondition, "the data service is missing its database driver (psycopg)"+
			"\nDetails: ModuleNotFoundError: No module named 'psycopg' (postgresql://foreverbull:s3cr3t@postgres/x)")))
	return ziplineEngine
}

func (test *BacktestCommandTest) ingestWithStore(name string, ziplineEngine engine.Engine, deleteErr error) (*stream.MockMessage, *storage.MockStorage) {
	test.recordClosedEndIfMissing(name)
	store := new(storage.MockStorage)
	store.On("DeleteObject", mock.Anything, storage.IngestionsBucket, test.ingestionObjectID(name)).Run(func(mock.Arguments) {
		// The row is still there: prices are deleted before the backtest record.
		_, getErr := (&repository.Backtest{Conn: test.db}).Get(context.Background(), name)
		test.Require().NoError(getErr)
	}).Return(deleteErr)
	msg := new(stream.MockMessage)
	msg.On("MustGet", stream.DBDep).Return(test.db)
	msg.On("MustGet", stream.StorageDep).Return(store)
	msg.On("Call", mock.Anything, dependency.GetEngineKey).Return(ziplineEngine, nil)
	msg.On("ParsePayload", &bs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.IngestCommand)
		payload.BacktestName = name
	})
	return msg, store
}

func (test *BacktestCommandTest) periodForIngest(name string) {
	err := (&repository.Backtest{Conn: test.db}).UpdatePeriod(context.Background(), name, &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
		Assets:    []*backtest.Backtest_Period_Asset{{Symbol: "AAPL", Enabled: true}},
	})
	test.Require().NoError(err)
}

// A failed ingest removes the id-keyed object. The sentence the user reads
// stays the ingest failure.
func (test *BacktestCommandTest) TestFailedIngestDeletesTheIngestionObject() {
	test.periodForIngest("test-backtest")
	id := test.ingestionObjectID("test-backtest")
	msg, store := test.ingestWithStore("test-backtest", ingestEngineFailure(), nil)

	err := command.Ingest(context.Background(), msg)
	test.Require().EqualError(err, failedIngestSentence)
	test.NotContains(err.Error(), "s3cr3t")
	store.AssertCalled(test.T(), "DeleteObject", mock.Anything, storage.IngestionsBucket, id)

	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), "test-backtest")
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

// When removing that object also fails, the ingest error and the delete
// error are both returned.
func (test *BacktestCommandTest) TestFailedIngestReportsObjectDeleteError() {
	test.periodForIngest("test-backtest")
	id := test.ingestionObjectID("test-backtest")
	del := errors.New("object storage refused the delete")
	msg, store := test.ingestWithStore("test-backtest", ingestEngineFailure(), del)

	err := command.Ingest(context.Background(), msg)
	want := strings.Replace(failedIngestSentence,
		"Nothing was saved, so run `fbull backtest create` again once it's fixed.",
		usercopy.BacktestPricesLeft()+" Run `fbull backtest create` again once it's fixed.", 1)
	test.Require().ErrorIs(err, del)
	test.Require().EqualError(err, want)
	test.NotContains(err.Error(), "Nothing was saved")
	test.NotContains(err.Error(), "object storage refused the delete")
	test.NotContains(err.Error(), "error deleting")
	test.NotContains(err.Error(), "s3cr3t")
	store.AssertCalled(test.T(), "DeleteObject", mock.Anything, storage.IngestionsBucket, id)

	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), "test-backtest")
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

// A canceled ingest whose price delete also fails stays a cancel, so the
// task is stored as CANCELED (the subscriber checks errors.Is(context.Canceled)).
func (test *BacktestCommandTest) TestCanceledIngestDeleteFailureStaysCanceled() {
	test.periodForIngest("test-backtest")
	test.recordClosedEndIfMissing("test-backtest")
	del := errors.New("object storage refused the delete")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng := new(engine.MockEngine)
	eng.On("Ingest", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { cancel() }).
		Return(nil, fmt.Errorf("error ingesting: %w", status.Error(codes.Canceled, "context canceled")))

	err := command.Ingest(ctx, test.priceDeleteMessage("test-backtest", eng, del))
	test.Require().EqualError(err, "Couldn't load data for backtest 'test-backtest': the create was canceled. "+
		"The backtest wasn't saved, but some of its prices couldn't be removed. They don't stop a new create. "+
		"Run `fbull backtest create` again.")
	test.Require().ErrorIs(err, context.Canceled)
	test.Require().NotErrorIs(err, context.DeadlineExceeded)
	test.Require().ErrorIs(err, del)
	test.NotContains(err.Error(), "context canceled")
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), "test-backtest")
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

// A deadline uses its own lead. The deadline stays on the unwrap chain.
func (test *BacktestCommandTest) TestDeadlineIngestDeleteFailureSaysTimedOut() {
	test.periodForIngest("test-backtest")
	test.recordClosedEndIfMissing("test-backtest")
	del := errors.New("object storage refused the delete")
	ctx := &deadlineAfter{Context: context.Background()}
	eng := new(engine.MockEngine)
	eng.On("Ingest", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { ctx.expire() }).
		Return(nil, fmt.Errorf("error ingesting: %w", context.DeadlineExceeded))

	err := command.Ingest(ctx, test.priceDeleteMessage("test-backtest", eng, del))
	test.Require().EqualError(err, "Couldn't load data for backtest 'test-backtest': the create timed out. "+
		"The backtest wasn't saved, but some of its prices couldn't be removed. They don't stop a new create. "+
		"Run `fbull backtest create` again.")
	test.Require().ErrorIs(err, context.DeadlineExceeded)
	test.Require().ErrorIs(err, del)
	test.Require().NotErrorIs(err, context.Canceled)
}

// The row stayed and the prices stayed. The delete-first step is not replaced
// by a claim that the backtest wasn't saved.
func (test *BacktestCommandTest) TestIngestRowAndPriceDeleteFailure() {
	test.periodForIngest("test-backtest")
	test.recordClosedEndIfMissing("test-backtest")
	test.refuseBacktestDelete()
	del := errors.New("object storage refused the delete")
	eng := new(engine.MockEngine)
	eng.On("Ingest", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))

	err := command.Ingest(context.Background(), test.priceDeleteMessage("test-backtest", eng, del))
	test.Require().EqualError(err, "Couldn't load data for backtest 'test-backtest': the data service failed while ingesting prices. "+
		"The backtest was partly saved and couldn't be removed, and some of its stored prices couldn't be removed either. "+
		"Run `fbull backtest delete test-backtest` first. "+
		"Run `fbull backtest create` again once it's fixed."+
		"\nDetails: boom")
	test.Require().ErrorIs(err, del)
	test.NotContains(err.Error(), "wasn't saved")
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), "test-backtest")
	test.Require().NoError(err)
}

// A canceled create whose row also stayed keeps the delete-first step and
// adds that the prices remain.
func (test *BacktestCommandTest) TestCanceledIngestRowAndPriceDeleteFailure() {
	test.periodForIngest("test-backtest")
	test.recordClosedEndIfMissing("test-backtest")
	test.refuseBacktestDelete()
	del := errors.New("object storage refused the delete")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng := new(engine.MockEngine)
	eng.On("Ingest", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { cancel() }).
		Return(nil, fmt.Errorf("error ingesting: %w", context.Canceled))

	err := command.Ingest(ctx, test.priceDeleteMessage("test-backtest", eng, del))
	test.Require().EqualError(err, "Couldn't load data for backtest 'test-backtest': the create was canceled. "+
		"The backtest was partly saved and couldn't be removed, and some of its stored prices couldn't be removed either. "+
		"Run `fbull backtest delete test-backtest` first. "+
		"Run `fbull backtest create` again.")
	test.Require().ErrorIs(err, del)
	test.Require().NotErrorIs(err, context.Canceled)
	test.Require().NotErrorIs(err, context.DeadlineExceeded)
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), "test-backtest")
	test.Require().NoError(err)
}

// A timed-out create whose row also stayed uses the timeout lead and the
// not-removed clause. The deadline is not on the unwrap chain: the name is
// still taken, same as a cancel whose row stayed.
func (test *BacktestCommandTest) TestDeadlineIngestRowAndPriceDeleteFailure() {
	test.periodForIngest("test-backtest")
	test.recordClosedEndIfMissing("test-backtest")
	test.refuseBacktestDelete()
	del := errors.New("object storage refused the delete")
	ctx := &deadlineAfter{Context: context.Background()}
	eng := new(engine.MockEngine)
	eng.On("Ingest", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { ctx.expire() }).
		Return(nil, fmt.Errorf("error ingesting: %w", context.DeadlineExceeded))

	err := command.Ingest(ctx, test.priceDeleteMessage("test-backtest", eng, del))
	test.Require().EqualError(err, "Couldn't load data for backtest 'test-backtest': the create timed out. "+
		"The backtest was partly saved and couldn't be removed, and some of its stored prices couldn't be removed either. "+
		"Run `fbull backtest delete test-backtest` first. "+
		"Run `fbull backtest create` again.")
	test.Require().ErrorIs(err, del)
	test.Require().NotErrorIs(err, context.DeadlineExceeded)
	test.Require().NotErrorIs(err, context.Canceled)
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), "test-backtest")
	test.Require().NoError(err)
}

// A value that is not an engine still deletes the prices before the row.
func (test *BacktestCommandTest) TestIngestCastFailureDeletesPricesBeforeTheRow() {
	test.periodForIngest("test-backtest")
	id := test.ingestionObjectID("test-backtest")
	store := new(storage.MockStorage)
	store.On("DeleteObject", mock.Anything, storage.IngestionsBucket, id).Run(func(mock.Arguments) {
		_, getErr := (&repository.Backtest{Conn: test.db}).Get(context.Background(), "test-backtest")
		test.Require().NoError(getErr)
	}).Return(nil)
	msg := new(stream.MockMessage)
	msg.On("MustGet", stream.DBDep).Return(test.db)
	msg.On("MustGet", stream.StorageDep).Return(store)
	msg.On("Call", mock.Anything, dependency.GetEngineKey).Return("not an engine", nil)
	msg.On("ParsePayload", &bs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		args.Get(0).(*bs.IngestCommand).BacktestName = "test-backtest"
	})

	err := command.Ingest(context.Background(), msg)
	test.Require().EqualError(err, "Couldn't load data for backtest 'test-backtest': the data service failed while ingesting prices. "+
		"Nothing was saved, so run `fbull backtest create` again once it's fixed."+
		"\nDetails: error casting zipline engine")
	store.AssertCalled(test.T(), "DeleteObject", mock.Anything, storage.IngestionsBucket, id)
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), "test-backtest")
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

func (test *BacktestCommandTest) TestMissingSessionsDeleteFailureLeavesPricesText() {
	test.createQuarter("AAA", "BBB")
	test.recordClosedEndIfMissing(quarterBacktest)
	del := errors.New("object storage refused the delete")
	eng := new(engine.MockEngine)
	eng.On("Ingest", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("error ingesting: %w",
		status.Error(codes.DataLoss, "missing prices on trading days: AAA 2024-02-14; BBB 2024-02-20,2024-02-21,2024-03-05"+
			"\nDetails: MissingSessionsError")))
	eng.On("Sessions", mock.Anything, quarterStart, quarterEnd).Return(xnysSessions(quarterEnd), nil)

	err := command.Ingest(context.Background(), test.priceDeleteMessage(quarterBacktest, eng, del))
	test.Require().ErrorIs(err, del)
	test.Contains(err.Error(), usercopy.BacktestPricesLeft())
	test.Contains(err.Error(), "yahoo is missing prices on trading days for 2 symbols")
	test.NotContains(err.Error(), "The backtest wasn't saved.")
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), quarterBacktest)
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

func (test *BacktestCommandTest) TestUnrecordedCutoffDeleteFailureLeavesPricesText() {
	test.createQuarter("AAA")
	del := errors.New("object storage refused the delete")
	eng := new(engine.MockEngine)

	err := command.Ingest(context.Background(), test.priceDeleteMessage(quarterBacktest, eng, del))
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': foreverbull didn't record which "+
		"trading session it stops at. "+usercopy.BacktestPricesLeft()+" Run `fbull backtest create` again.")
	test.Require().ErrorIs(err, del)
	eng.AssertNotCalled(test.T(), "Ingest", mock.Anything, mock.Anything, mock.Anything)
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), quarterBacktest)
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

func (test *BacktestCommandTest) TestUnusableCutoffDeleteFailureLeavesPricesText() {
	test.createQuarter("AAA")
	_, err := test.db.Exec(context.Background(), `UPDATE backtest SET closed_end = '2099-01-01' WHERE name = $1`, quarterBacktest)
	test.Require().NoError(err)
	del := errors.New("object storage refused the delete")
	eng := new(engine.MockEngine)

	err = command.Ingest(context.Background(), test.priceDeleteMessage(quarterBacktest, eng, del))
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': the recorded session cut-off 2099-01-01 "+
		"is after the backtest ends on 2024-03-31. "+usercopy.BacktestPricesLeft()+" Run `fbull backtest create` again.")
	test.Require().ErrorIs(err, del)
	eng.AssertNotCalled(test.T(), "Ingest", mock.Anything, mock.Anything, mock.Anything)
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), quarterBacktest)
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

// A deadline during an unrecorded cut-off whose row stays does not claim the
// backtest wasn't saved. The deadline stays matchable: this path returns it.
func (test *BacktestCommandTest) TestUnrecordedCutoffDeadlineRowStays() {
	test.createQuarter("AAA")
	test.refuseBacktestDelete()
	del := errors.New("object storage refused the delete")
	ctx := &deadlineAfter{Context: context.Background()}
	eng := new(engine.MockEngine)
	store := new(storage.MockStorage)
	store.On("DeleteObject", mock.Anything, storage.IngestionsBucket, test.ingestionObjectID(quarterBacktest)).Run(func(mock.Arguments) {
		ctx.expire()
	}).Return(del)
	msg := new(stream.MockMessage)
	msg.On("MustGet", stream.DBDep).Return(test.db)
	msg.On("MustGet", stream.StorageDep).Return(store)
	msg.On("Call", mock.Anything, dependency.GetEngineKey).Return(eng, nil)
	msg.On("ParsePayload", &bs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		args.Get(0).(*bs.IngestCommand).BacktestName = quarterBacktest
	})

	err := command.Ingest(ctx, msg)
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': the create timed out. "+
		"The backtest was partly saved and couldn't be removed, and some of its stored prices couldn't be removed either. "+
		"Run `fbull backtest delete oef-2024q1` first. Run `fbull backtest create` again.")
	test.Require().ErrorIs(err, context.DeadlineExceeded)
	test.Require().NotErrorIs(err, context.Canceled)
	test.Require().ErrorIs(err, del)
	test.NotContains(err.Error(), "wasn't saved")
	eng.AssertNotCalled(test.T(), "Ingest", mock.Anything, mock.Anything, mock.Anything)
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), quarterBacktest)
	test.Require().NoError(err)
}

// Missing sessions whose row stays keep the delete-first step, and the prices
// join the not-removed clause.
func (test *BacktestCommandTest) TestMissingSessionsRowAndPriceDeleteFailure() {
	test.createQuarter("AAA", "BBB")
	test.recordClosedEndIfMissing(quarterBacktest)
	test.refuseBacktestDelete()
	del := errors.New("object storage refused the delete")
	eng := new(engine.MockEngine)
	eng.On("Ingest", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("error ingesting: %w",
		status.Error(codes.DataLoss, "missing prices on trading days: AAA 2024-02-14; BBB 2024-02-20,2024-02-21,2024-03-05"+
			"\nDetails: MissingSessionsError")))
	eng.On("Sessions", mock.Anything, quarterStart, quarterEnd).Return(xnysSessions(quarterEnd), nil)

	err := command.Ingest(context.Background(), test.priceDeleteMessage(quarterBacktest, eng, del))
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices on trading "+
		"days for 2 symbols. The backtest was partly saved and couldn't be removed, and some of its stored prices "+
		"couldn't be removed either.\n"+
		"  AAA: 2024-02-14\n"+
		"  BBB: 2024-02-20 to 2024-02-21, 2024-03-05\n"+
		"Run `fbull backtest delete oef-2024q1` first. Try another provider with --provider, or change the "+
		"backtest dates, then run `fbull backtest create` again.")
	test.Require().ErrorIs(err, del)
	test.NotContains(err.Error(), "wasn't saved")
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), quarterBacktest)
	test.Require().NoError(err)
}

func (test *BacktestCommandTest) TestUnrecordedCutoffRowAndPriceDeleteFailure() {
	test.createQuarter("AAA")
	test.refuseBacktestDelete()
	del := errors.New("object storage refused the delete")
	eng := new(engine.MockEngine)

	err := command.Ingest(context.Background(), test.priceDeleteMessage(quarterBacktest, eng, del))
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': foreverbull didn't record which "+
		"trading session it stops at. The backtest was partly saved and couldn't be removed, and some of its stored "+
		"prices couldn't be removed either. Run `fbull backtest delete oef-2024q1` first. Run `fbull backtest create` again.")
	test.Require().ErrorIs(err, del)
	test.NotContains(err.Error(), "wasn't saved")
	eng.AssertNotCalled(test.T(), "Ingest", mock.Anything, mock.Anything, mock.Anything)
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), quarterBacktest)
	test.Require().NoError(err)
}

func (test *BacktestCommandTest) TestUnusableCutoffRowAndPriceDeleteFailure() {
	test.createQuarter("AAA")
	_, err := test.db.Exec(context.Background(), `UPDATE backtest SET closed_end = '2099-01-01' WHERE name = $1`, quarterBacktest)
	test.Require().NoError(err)
	test.refuseBacktestDelete()
	del := errors.New("object storage refused the delete")
	eng := new(engine.MockEngine)

	err = command.Ingest(context.Background(), test.priceDeleteMessage(quarterBacktest, eng, del))
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': the recorded session cut-off 2099-01-01 "+
		"is after the backtest ends on 2024-03-31. The backtest was partly saved and couldn't be removed, and some of "+
		"its stored prices couldn't be removed either. Run `fbull backtest delete oef-2024q1` first. Run `fbull backtest create` again.")
	test.Require().ErrorIs(err, del)
	test.NotContains(err.Error(), "wasn't saved")
	eng.AssertNotCalled(test.T(), "Ingest", mock.Anything, mock.Anything, mock.Anything)
	_, err = (&repository.Backtest{Conn: test.db}).Get(context.Background(), quarterBacktest)
	test.Require().NoError(err)
}

// deadlineAfter reports DeadlineExceeded after expire. The lookup that
// starts the ingest still sees a live context.
type deadlineAfter struct {
	context.Context
	mu sync.Mutex
	on bool
}

func (d *deadlineAfter) expire() {
	d.mu.Lock()
	d.on = true
	d.mu.Unlock()
}

func (d *deadlineAfter) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.on {
		return context.DeadlineExceeded
	}
	return d.Context.Err()
}

func (test *BacktestCommandTest) priceDeleteMessage(name string, eng engine.Engine, deleteErr error) *stream.MockMessage {
	store := new(storage.MockStorage)
	store.On("DeleteObject", mock.Anything, storage.IngestionsBucket, test.ingestionObjectID(name)).Run(func(mock.Arguments) {
		_, getErr := (&repository.Backtest{Conn: test.db}).Get(context.Background(), name)
		test.Require().NoError(getErr)
	}).Return(deleteErr)
	msg := new(stream.MockMessage)
	msg.On("MustGet", stream.DBDep).Return(test.db)
	msg.On("MustGet", stream.StorageDep).Return(store)
	msg.On("Call", mock.Anything, dependency.GetEngineKey).Return(eng, nil)
	msg.On("ParsePayload", &bs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		args.Get(0).(*bs.IngestCommand).BacktestName = name
	})
	return msg
}

// An ingest that finishes keeps the object.
func (test *BacktestCommandTest) TestSuccessfulIngestKeepsTheIngestionObject() {
	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Ingest", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	msg, store := test.ingestWithStore("test-backtest", ziplineEngine, nil)

	err := command.Ingest(context.Background(), msg)
	test.Require().NoError(err)
	store.AssertNotCalled(test.T(), "DeleteObject", mock.Anything, storage.IngestionsBucket, mock.Anything)
}
