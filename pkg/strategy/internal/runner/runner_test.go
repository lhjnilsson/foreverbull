package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RunnerTest struct {
	suite.Suite

	store     *MemoryStore
	trading   *FakeTrading
	processor *FakeProcessor
	clock     *FakeClock
	runner    *InProcess

	mon SessionClose
	tue SessionClose
	wed SessionClose
}

type profileRegistry map[string]supplier.Trading

func (p profileRegistry) Get(name string) (supplier.Trading, bool) {
	if p == nil {
		return nil, false
	}
	client, ok := p[name]
	return client, ok
}

func TestRunner(t *testing.T) {
	suite.Run(t, new(RunnerTest))
}

func (test *RunnerTest) SetupTest() {
	test.mon = SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 4},
		Close: time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC),
	}
	test.tue = SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 5},
		Close: time.Date(2024, 3, 5, 16, 0, 0, 0, time.UTC),
	}
	test.wed = SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 6},
		Close: time.Date(2024, 3, 6, 16, 0, 0, 0, time.UTC),
	}

	test.store = NewMemoryStore()
	test.trading = &FakeTrading{Portfolio: &financepb.Portfolio{Cash: 1000}}
	test.processor = &FakeProcessor{
		Orders: []*financepb.Order{{
			Symbol:      "AAPL",
			Amount:      10,
			Type:        financepb.OrderType_ORDER_TYPE_MARKET,
			TimeInForce: financepb.TimeInForce_TIME_IN_FORCE_DAY,
		}},
	}
	test.clock = NewFakeClock(test.mon.Close.Add(-time.Hour), test.mon, test.tue, test.wed)
	test.runner = New(test.store, test.clock, profileRegistry{"alpaca-paper": test.trading}, test.processor).WithPollInterval(5 * time.Millisecond)
	test.store.Put(test.deployment())
}

func (test *RunnerTest) TearDownTest() {
	if test.runner != nil {
		_ = test.runner.Stop(context.Background())
	}
}

func (test *RunnerTest) deployment() *pb.Deployment {
	return &pb.Deployment{
		Id:           "dep-1",
		StrategyId:   "s1",
		StrategyName: "mean-rev",
		VersionId:    "v1",
		Mode:         pb.Deployment_PAPER,
		Backend:      "alpaca-paper",
		CalendarCode: "XNYS",
		Symbols:      []string{"AAPL"},
		ScheduleKind: pb.Deployment_SESSION_CLOSE,
		DryRun:       true,
		Status:       pb.Deployment_RUNNING,
	}
}

func (test *RunnerTest) TestProcessSessionPersistsOrdersWithoutPlaceOrder() {
	ctx := context.Background()
	err := test.runner.ProcessSession(ctx, "dep-1", test.mon)
	test.Require().NoError(err)

	test.Equal(1, test.processor.CallCount())
	test.Equal(1, test.trading.PortfolioCalls())
	test.InDelta(1000, test.processor.Portfolio().GetCash(), 1e-9)
	test.Equal(0, test.trading.PlaceCalls())

	runs, err := test.store.ListRuns(ctx, "", "dep-1")
	test.Require().NoError(err)
	test.Require().Len(runs, 1)
	test.False(runs[0].Skipped)
	test.Nil(runs[0].Error)
	test.Require().Len(runs[0].Orders, 1)
	test.Equal("AAPL", runs[0].Orders[0].Symbol)
	test.Equal(int32(10), runs[0].Orders[0].Amount)

	d, err := test.store.GetDeployment(ctx, "dep-1")
	test.Require().NoError(err)
	test.Equal(int32(4), d.LastSessionDate.Day)
}

func (test *RunnerTest) TestDuplicateTickNoop() {
	ctx := context.Background()
	test.Require().NoError(test.runner.ProcessSession(ctx, "dep-1", test.mon))
	test.Require().NoError(test.runner.ProcessSession(ctx, "dep-1", test.mon))

	test.Equal(1, test.processor.CallCount())
	test.Equal(0, test.trading.PlaceCalls())

	runs, err := test.store.ListRuns(ctx, "", "dep-1")
	test.Require().NoError(err)
	test.Len(runs, 1)
}

func (test *RunnerTest) TestDuplicateCreateRunNoop() {
	ctx := context.Background()
	_, err := test.store.CreateRun(ctx, "dep-1", test.mon.Date, nil, false, nil)
	test.Require().NoError(err)

	d := test.deployment()
	d.LastSessionDate = nil
	test.store.Put(d)

	test.Require().NoError(test.runner.ProcessSession(ctx, "dep-1", test.mon))
	test.Equal(1, test.processor.CallCount())

	runs, err := test.store.ListRuns(ctx, "", "dep-1")
	test.Require().NoError(err)
	test.Len(runs, 1)

	got, err := test.store.GetDeployment(ctx, "dep-1")
	test.Require().NoError(err)
	test.Equal(int32(4), got.LastSessionDate.Day)
}

func (test *RunnerTest) TestSkipMissedCloseDoesNotProcess() {
	ctx := context.Background()
	err := test.runner.SkipMissed(ctx, "dep-1", test.mon)
	test.Require().NoError(err)

	test.Equal(0, test.processor.CallCount())
	test.Equal(0, test.trading.PortfolioCalls())
	test.Equal(0, test.trading.PlaceCalls())

	runs, err := test.store.ListRuns(ctx, "", "dep-1")
	test.Require().NoError(err)
	test.Require().Len(runs, 1)
	test.True(runs[0].Skipped)
	test.Equal("skipped_missed_close", runs[0].GetError())

	d, err := test.store.GetDeployment(ctx, "dep-1")
	test.Require().NoError(err)
	test.Equal(int32(4), d.LastSessionDate.Day)
}

func (test *RunnerTest) TestRestartUsesLastSessionDate() {
	ctx := context.Background()
	test.Require().NoError(test.runner.ProcessSession(ctx, "dep-1", test.mon))

	restarted := New(test.store, test.clock, profileRegistry{"alpaca-paper": test.trading}, test.processor)
	test.Require().NoError(restarted.ProcessSession(ctx, "dep-1", test.mon))
	test.Equal(1, test.processor.CallCount())

	test.Require().NoError(restarted.ProcessSession(ctx, "dep-1", test.tue))
	test.Equal(2, test.processor.CallCount())
	test.Equal(0, test.trading.PlaceCalls())
}

func (test *RunnerTest) TestLoopTickThenDuplicateNoop() {
	ctx := context.Background()
	test.Require().NoError(test.runner.Add(ctx, test.deployment()))

	test.clock.Advance(test.mon.Close)

	test.Eventually(func() bool {
		runs, err := test.store.ListRuns(ctx, "", "dep-1")
		return err == nil && len(runs) == 1 && !runs[0].Skipped
	}, time.Second, 10*time.Millisecond)

	test.Equal(1, test.processor.CallCount())
	test.Equal(0, test.trading.PlaceCalls())

	test.clock.Advance(test.mon.Close.Add(time.Second))
	time.Sleep(30 * time.Millisecond)
	test.Equal(1, test.processor.CallCount())
}

func (test *RunnerTest) TestLoopMissedCloseFailsWhenSkipMissedIsOff() {
	ctx := context.Background()
	d := test.deployment()
	d.LastSessionDate = &commonpb.Date{Year: 2024, Month: 3, Day: 1}
	d.SkipMissed = false
	test.store.Put(d)

	test.clock.Advance(test.tue.Close.Add(time.Hour))
	test.Require().NoError(test.runner.Add(ctx, d))

	sentence, err := usercopy.MissedSessionClose("mean-rev", []string{"2024-03-04", "2024-03-05"})
	test.Require().NoError(err)
	test.Eventually(func() bool {
		got, err := test.store.GetDeployment(ctx, "dep-1")
		return err == nil && got.Status == pb.Deployment_ERROR && got.GetError() == sentence
	}, time.Second, 10*time.Millisecond)

	test.Equal(0, test.processor.CallCount())
	test.Equal(0, test.trading.PlaceCalls())

	runs, err := test.store.ListRuns(ctx, "", "dep-1")
	test.Require().NoError(err)
	test.Empty(runs)

	got, err := test.store.GetDeployment(ctx, "dep-1")
	test.Require().NoError(err)
	test.Equal(int32(1), got.LastSessionDate.Day)
	test.Equal(pb.Deployment_ERROR, got.Status)
	test.Require().ErrorIs(missedClose("mean-rev", []string{"2024-03-04", "2024-03-05"}), ErrMissedClose)

	test.clock.Advance(test.wed.Close.Add(time.Hour))
	time.Sleep(40 * time.Millisecond)
	test.Equal(0, test.processor.CallCount())
	got, err = test.store.GetDeployment(ctx, "dep-1")
	test.Require().NoError(err)
	test.Equal(pb.Deployment_ERROR, got.Status)
}

func (test *RunnerTest) TestLoopOneMissedCloseNamesThatSession() {
	ctx := context.Background()
	d := test.deployment()
	d.LastSessionDate = &commonpb.Date{Year: 2024, Month: 3, Day: 1}
	test.store.Put(d)

	test.clock.Advance(test.mon.Close.Add(time.Hour))
	test.Require().NoError(test.runner.Add(ctx, d))

	sentence, err := usercopy.MissedSessionClose("mean-rev", []string{"2024-03-04"})
	test.Require().NoError(err)
	test.Eventually(func() bool {
		got, err := test.store.GetDeployment(ctx, "dep-1")
		return err == nil && got.Status == pb.Deployment_ERROR && got.GetError() == sentence
	}, time.Second, 10*time.Millisecond)
	test.Equal(0, test.processor.CallCount())
}

func TestLoopFiveMissedClosesNameTheSpan(t *testing.T) {
	t.Parallel()
	sessions := make([]SessionClose, 5)
	for i := range sessions {
		day := 4 + i
		sessions[i] = SessionClose{
			Date:  &commonpb.Date{Year: 2024, Month: 3, Day: int32(day)},
			Close: time.Date(2024, 3, day, 16, 0, 0, 0, time.UTC),
		}
	}
	store := NewMemoryStore()
	processor := &FakeProcessor{}
	clock := NewFakeClock(sessions[0].Close.Add(-time.Hour), sessions...)
	runner := New(store, clock, nil, processor)
	deployment := &pb.Deployment{
		Id:              "dep-1",
		StrategyName:    "mean-rev",
		Status:          pb.Deployment_RUNNING,
		CalendarCode:    "XNYS",
		DryRun:          true,
		LastSessionDate: &commonpb.Date{Year: 2024, Month: 3, Day: 1},
	}
	store.Put(deployment)
	clock.Advance(sessions[4].Close.Add(time.Hour))
	require.NoError(t, runner.Add(context.Background(), deployment))

	sentence, err := usercopy.MissedSessionClose("mean-rev", []string{
		"2024-03-04", "2024-03-05", "2024-03-06", "2024-03-07", "2024-03-08",
	})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		got, err := store.GetDeployment(context.Background(), "dep-1")
		return err == nil && got.GetError() == sentence && got.Status == pb.Deployment_ERROR
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, 0, processor.CallCount())
	runs, err := store.ListRuns(context.Background(), "", "dep-1")
	require.NoError(t, err)
	require.Empty(t, runs)
}

func (test *RunnerTest) TestLoopSkipsMissedCloses() {
	ctx := context.Background()
	d := test.deployment()
	d.SkipMissed = true
	d.LastSessionDate = &commonpb.Date{Year: 2024, Month: 3, Day: 1}
	test.store.Put(d)

	test.clock.Advance(test.tue.Close.Add(time.Hour))
	test.Require().NoError(test.runner.Add(ctx, d))

	test.Eventually(func() bool {
		runs, err := test.store.ListRuns(ctx, "", "dep-1")
		if err != nil {
			return false
		}
		skipped := 0
		for _, run := range runs {
			if run.Skipped {
				skipped++
			}
		}
		return skipped == 2
	}, time.Second, 10*time.Millisecond)

	runs, err := test.store.ListRuns(ctx, "", "dep-1")
	test.Require().NoError(err)
	test.Require().Len(runs, 2)
	for _, run := range runs {
		test.True(run.GetSkipped())
		test.Equal(ErrSkippedMissedClose.Error(), run.GetError())
	}
	test.Equal(int32(4), runs[0].GetSessionDate().GetDay())
	test.Equal(int32(5), runs[1].GetSessionDate().GetDay())

	test.Equal(0, test.processor.CallCount())
	test.Equal(0, test.trading.PlaceCalls())

	test.clock.Advance(test.wed.Close)
	test.Eventually(func() bool {
		return test.processor.CallCount() == 1
	}, time.Second, 10*time.Millisecond)
	test.Equal(0, test.trading.PlaceCalls())
}

func (test *RunnerTest) TestStartRecoversRunningDeployment() {
	ctx := context.Background()
	test.clock.Advance(test.mon.Close)
	test.Require().NoError(test.runner.Start(ctx))

	test.Eventually(func() bool {
		return test.processor.CallCount() == 1
	}, time.Second, 10*time.Millisecond)
	test.Equal(0, test.trading.PlaceCalls())
}

// downTrading is a profile whose portfolio read fails.
type downTrading struct {
	calls atomic.Int32
	cause error
}

func (d *downTrading) GetPortfolio(context.Context) (*financepb.Portfolio, error) {
	d.calls.Add(1)
	if d.cause != nil {
		return nil, d.cause
	}
	return nil, errors.New("connection refused")
}

func (d *downTrading) PlaceOrder(context.Context, *financepb.Order) (*financepb.Order, error) {
	d.calls.Add(1)
	return nil, errors.New("connection refused")
}

func (d *downTrading) GetOrders(context.Context) ([]*financepb.Order, error) {
	d.calls.Add(1)
	return nil, errors.New("connection refused")
}

func (test *RunnerTest) TestDryRunFailsWhenPortfolioCannotBeRead() {
	cause := errors.New("connection refused")
	down := &downTrading{cause: cause}
	test.runner = New(test.store, test.clock, profileRegistry{"alpaca-paper": down}, test.processor)
	d := test.deployment()
	d.DryRun = true
	d.Backend = "alpaca-paper"
	d.StrategyName = "mean-rev"
	test.store.Put(d)

	err := test.runner.ProcessSession(context.Background(), "dep-1", test.mon)
	test.Require().ErrorIs(err, ErrDryRunPortfolio)
	test.Require().ErrorIs(err, cause)
	sentence := usercopy.DryRunPortfolioUnreadable("mean-rev", "alpaca-paper")
	test.Equal(sentence, err.Error())
	test.NotContains(err.Error(), "connection refused")
	test.Equal(int32(1), down.calls.Load())
	test.Equal(0, test.processor.CallCount())
	test.Equal(0, test.trading.PlaceCalls())

	got, err := test.store.GetDeployment(context.Background(), "dep-1")
	test.Require().NoError(err)
	test.Equal(pb.Deployment_ERROR, got.Status)
	test.Equal(sentence, got.GetError())
	test.Nil(got.LastSessionDate)

	runs, err := test.store.ListRuns(context.Background(), "", "dep-1")
	test.Require().NoError(err)
	test.Empty(runs)
}

func (test *RunnerTest) TestDryRunFailsWhenProfileIsUnknown() {
	test.runner = New(test.store, test.clock, profileRegistry{}, test.processor)
	d := test.deployment()
	d.DryRun = true
	d.Backend = "missing-profile"
	d.StrategyName = ""
	test.store.Put(d)

	err := test.runner.ProcessSession(context.Background(), "dep-1", test.mon)
	test.Require().ErrorIs(err, ErrDryRunPortfolio)
	test.Equal(usercopy.DryRunPortfolioUnreadable("", "missing-profile"), err.Error())
	test.Equal(0, test.processor.CallCount())

	got, err := test.store.GetDeployment(context.Background(), "dep-1")
	test.Require().NoError(err)
	test.Equal(pb.Deployment_ERROR, got.Status)
	test.Equal(usercopy.DryRunPortfolioUnreadable("", "missing-profile"), got.GetError())
}

func (test *RunnerTest) TestDryRunUsesEmptyPortfolioWhenNoBackend() {
	test.runner = New(test.store, test.clock, profileRegistry{"alpaca-paper": test.trading}, test.processor)
	d := test.deployment()
	d.DryRun = true
	d.Backend = ""
	test.store.Put(d)

	err := test.runner.ProcessSession(context.Background(), "dep-1", test.mon)
	test.Require().NoError(err)
	test.Equal(0, test.trading.PortfolioCalls())
	test.Equal(0, test.trading.PlaceCalls())
	test.Equal(1, test.processor.CallCount())
	got := test.processor.Portfolio()
	test.Require().NotNil(got)
	test.Zero(got.GetCash())
	test.Empty(got.GetPositions())

	runs, err := test.store.ListRuns(context.Background(), "", "dep-1")
	test.Require().NoError(err)
	test.Require().Len(runs, 1)
	test.Nil(runs[0].Error)
	test.Require().Len(runs[0].Orders, 1)
}

func TestProcessSessionNilTrading(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	processor := &FakeProcessor{Orders: []*financepb.Order{{Symbol: "MSFT", Amount: 1}}}
	clock := NewFakeClock(time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC))
	r := New(store, clock, nil, processor)
	store.Put(&pb.Deployment{
		Id:           "dep-1",
		Status:       pb.Deployment_RUNNING,
		CalendarCode: "XNYS",
		Symbols:      []string{"MSFT"},
		DryRun:       true,
	})

	err := r.ProcessSession(context.Background(), "dep-1", SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 4},
		Close: time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, 1, processor.CallCount())
}

func (test *RunnerTest) TestProcessSessionResolvesBackendEachSession() {
	other := &FakeTrading{Portfolio: &financepb.Portfolio{Cash: 2}}
	test.runner = New(test.store, test.clock, profileRegistry{
		"alpaca-paper": test.trading,
		"saxo-sim":     other,
	}, test.processor)

	d := test.deployment()
	d.DryRun = false
	d.Backend = "alpaca-paper"
	test.store.Put(d)

	ctx := context.Background()
	test.Require().NoError(test.runner.ProcessSession(ctx, "dep-1", test.mon))
	test.Equal(1, test.trading.PortfolioCalls())
	test.Equal(1, test.trading.PlaceCalls())
	test.Equal(0, other.PortfolioCalls())
	test.Equal(0, other.PlaceCalls())

	got, err := test.store.GetDeployment(ctx, "dep-1")
	test.Require().NoError(err)
	got.Backend = "saxo-sim"
	test.store.Put(got)

	test.Require().NoError(test.runner.ProcessSession(ctx, "dep-1", test.tue))
	test.Equal(1, test.trading.PortfolioCalls())
	test.Equal(1, test.trading.PlaceCalls())
	test.Equal(1, other.PortfolioCalls())
	test.Equal(1, other.PlaceCalls())
}

func (test *RunnerTest) TestProcessSessionPaperPlaceOrder() {
	d := test.deployment()
	d.DryRun = false
	test.store.Put(d)
	test.processor.Orders = []*financepb.Order{
		{Symbol: "AAPL", Amount: 10},
		{Symbol: "MSFT", Amount: 5},
	}

	ctx := context.Background()
	err := test.runner.ProcessSession(ctx, "dep-1", test.mon)
	test.Require().NoError(err)

	test.Equal(1, test.processor.CallCount())
	test.Equal(len(test.processor.Orders), test.trading.PlaceCalls())
	placed := test.trading.Placed()
	test.Require().Len(placed, 2)
	test.Equal("AAPL", placed[0].Symbol)
	test.Equal(int32(10), placed[0].Amount)
	test.Equal("MSFT", placed[1].Symbol)
	test.Equal(int32(5), placed[1].Amount)

	runs, err := test.store.ListRuns(ctx, "", "dep-1")
	test.Require().NoError(err)
	test.Require().Len(runs, 1)
	test.False(runs[0].Skipped)
	test.Nil(runs[0].Error)
	test.Require().Len(runs[0].Orders, 2)
	test.Equal("AAPL", runs[0].Orders[0].Symbol)
	test.Equal("MSFT", runs[0].Orders[1].Symbol)
}

func (test *RunnerTest) TestProcessSessionPlaceOrderErrorStopsSession() {
	d := test.deployment()
	d.DryRun = false
	test.store.Put(d)
	test.processor.Orders = []*financepb.Order{
		{Symbol: "AAPL", Amount: 10},
		{Symbol: "MSFT", Amount: 5},
	}
	test.trading.PlaceErr = errors.New("broker down")
	test.trading.FailAfter = 1

	err := test.runner.ProcessSession(context.Background(), "dep-1", test.mon)
	test.Require().Error(err)
	test.Contains(err.Error(), "place order")
	test.Equal(2, test.trading.PlaceCalls())
	test.Len(test.trading.Placed(), 1)
	test.Equal("AAPL", test.trading.Placed()[0].Symbol)

	runs, err := test.store.ListRuns(context.Background(), "", "dep-1")
	test.Require().NoError(err)
	test.Require().Len(runs, 1)
	test.Require().NotNil(runs[0].Error)
	test.Contains(*runs[0].Error, "place order")
	test.Require().Len(runs[0].Orders, 2)
	test.Equal("AAPL", runs[0].Orders[0].Symbol)
	test.Equal("MSFT", runs[0].Orders[1].Symbol)

	got, err := test.store.GetDeployment(context.Background(), "dep-1")
	test.Require().NoError(err)
	test.Equal(int32(4), got.LastSessionDate.Day)
}

func (test *RunnerTest) TestProcessSessionErrNotTradableStopsBeforeNextOrder() {
	d := test.deployment()
	d.DryRun = false
	test.store.Put(d)
	test.processor.Orders = []*financepb.Order{
		{Symbol: "AAPL", Amount: 10},
		{Symbol: "MSFT", Amount: 5},
	}
	test.trading.PlaceErr = supplier.ErrNotTradable

	err := test.runner.ProcessSession(context.Background(), "dep-1", test.mon)
	test.Require().Error(err)
	test.Require().ErrorIs(err, supplier.ErrNotTradable)
	test.Equal(1, test.trading.PlaceCalls())
	test.Empty(test.trading.Placed())

	runs, err := test.store.ListRuns(context.Background(), "", "dep-1")
	test.Require().NoError(err)
	test.Require().Len(runs, 1)
	test.False(runs[0].Skipped)
	test.Require().NotNil(runs[0].Error)
	test.Contains(*runs[0].Error, "not tradable")
	test.Require().Len(runs[0].Orders, 1)
	test.Equal("AAPL", runs[0].Orders[0].GetSymbol())
	test.Equal(int32(10), runs[0].Orders[0].GetAmount())

	test.Require().NoError(test.runner.ProcessSession(context.Background(), "dep-1", test.mon))
	test.Equal(1, test.trading.PlaceCalls())
	test.Empty(test.trading.Placed())

	got, err := test.store.GetDeployment(context.Background(), "dep-1")
	test.Require().NoError(err)
	test.Equal(int32(4), got.LastSessionDate.Day)
}

func (test *RunnerTest) TestProcessSessionPaperNilTrading() {
	d := test.deployment()
	d.DryRun = false
	test.store.Put(d)
	r := New(test.store, test.clock, nil, test.processor)

	err := r.ProcessSession(context.Background(), "dep-1", test.mon)
	test.Require().Error(err)
	test.Contains(err.Error(), "Trading")
	test.Equal(0, test.trading.PlaceCalls())

	runs, err := test.store.ListRuns(context.Background(), "", "dep-1")
	test.Require().NoError(err)
	test.Require().Len(runs, 1)
	test.Require().NotNil(runs[0].Error)
}

func (test *RunnerTest) TestSkipMissedClosePaperDoesNotPlaceOrder() {
	d := test.deployment()
	d.DryRun = false
	test.store.Put(d)

	err := test.runner.SkipMissed(context.Background(), "dep-1", test.mon)
	test.Require().NoError(err)
	test.Equal(0, test.processor.CallCount())
	test.Equal(0, test.trading.PlaceCalls())
}
