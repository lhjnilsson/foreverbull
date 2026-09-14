package runner

import (
	"context"
	"testing"
	"time"

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
	test.runner = New(test.store, test.clock, test.trading, test.processor).WithPollInterval(5 * time.Millisecond)
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
		Backend:      "default",
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

	restarted := New(test.store, test.clock, test.trading, test.processor)
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

func (test *RunnerTest) TestLoopSkipsMissedCloses() {
	ctx := context.Background()
	d := test.deployment()
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
