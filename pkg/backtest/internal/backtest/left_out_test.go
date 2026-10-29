package backtest

import (
	"context"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeMembersWithBars is an ETF membership lookup that also knows each
// ISIN's last stored bar, as repository.UniverseMember does.
type fakeMembersWithBars struct {
	fakeMemberSymbols
	aliases  []repository.AliasLastBar
	provider *string
	end      *time.Time
}

func (f fakeMembersWithBars) LastBars(_ context.Context, _ string, provider string, end time.Time) ([]repository.AliasLastBar, error) {
	*f.provider = provider
	*f.end = end
	return f.aliases, nil
}

func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
}

func ptr[T any](v T) *T { return &v }

// A member delisted mid-month stays in universe_member until the next
// month-end list drops it. Its bars end at the delisting, so from the next
// session it is not passed to Process, and the run notice names it.
func TestRunBacktestLeavesOutMemberAfterItsLastBar(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 3, 31, 13, 30, 0, 0, time.UTC)
	after := time.Date(2026, 4, 1, 13, 30, 0, 0, time.UTC)
	later := time.Date(2026, 4, 2, 13, 30, 0, 0, time.UTC)

	client := servicePb.NewMockEngineSessionClient(t)
	pool := worker.NewMockPool(t)
	client.On("RunBacktest", mock.Anything, mock.Anything).
		Return(&servicePb.RunBacktestResponse{NumPeriods: 3}, nil).Once()
	for _, at := range []time.Time{before, after, later} {
		client.On("GetCurrentPeriod", mock.Anything, mock.Anything).Return(periodAt(at), nil).Once()
	}
	client.On("GetCurrentPeriod", mock.Anything, mock.Anything).
		Return(&servicePb.GetCurrentPeriodResponse{IsRunning: false}, nil).Once()
	client.On("PlaceOrdersAndContinue", mock.Anything, mock.Anything).
		Return(&servicePb.PlaceOrdersAndContinueResponse{}, nil).Times(3)

	// ABC's last bar is the 2026-03-31 open (Yahoo stamps 13:30 UTC). FB
	// has no bars of its own: Yahoo stores the ISIN's history under META,
	// and the ISIN's bars run on, so FB stays.
	pool.On("Process", mock.Anything, before, []string{"AAPL", "ABC", "FB"}, mock.Anything).Return(nil, nil).Once()
	pool.On("Process", mock.Anything, after, []string{"AAPL", "FB"}, mock.Anything).Return(nil, nil).Once()
	pool.On("Process", mock.Anything, later, []string{"AAPL", "FB"}, mock.Anything).Return(nil, nil).Once()

	members := []string{"AAPL", "ABC", "FB"}
	provider := ""
	end := time.Time{}
	session := &ZipelineSession{
		client: client,
		symbolsAt: fakeMembersWithBars{
			fakeMemberSymbols: fakeMemberSymbols{byDate: map[string][]string{
				"2026-03-31": members, "2026-04-01": members, "2026-04-02": members,
			}, intervals: etfMembership},
			aliases: []repository.AliasLastBar{
				{Symbol: "AAPL", FromDate: day(2026, 1, 1), LastBar: ptr(time.Date(2026, 4, 30, 13, 30, 0, 0, time.UTC))},
				{Symbol: "ABC", FromDate: day(2026, 1, 1), LastBar: ptr(before)},
				{Symbol: "FB", FromDate: day(2026, 1, 1), LastBar: ptr(time.Date(2026, 4, 30, 13, 30, 0, 0, time.UTC))},
			},
			provider: &provider,
			end:      &end,
		},
		backtests: fakeBacktests{backtest: &pb.Backtest{
			Name: "sp100", Benchmark: "OEF",
			Periods: []*pb.Backtest_Period{{StartDate: date(2026, 1, 1), EndDate: date(2026, 4, 30)}},
		}},
	}

	ch, err := session.RunBacktest(context.Background(), &pb.Execution{Backtest: "sp100"}, pool)
	require.NoError(t, err)
	drain(t, ch)
	require.NoError(t, session.RunFailure())

	require.Equal(t, "yahoo", provider)
	require.Equal(t, day(2026, 5, 1), end)
	require.Equal(t, []string{"1 symbol had no prices after 2026-03-31 and was left out: ABC."}, session.RunNotices())
}

// The run asks for sessions past the newest stored bar, e.g. a provider that
// has not published the last day yet. Members whose last bar is that newest
// bar are not delisted and stay; only a member whose bars end before it is
// left out and named.
func TestRunBacktestKeepsMembersWhoseLastBarIsTheNewestData(t *testing.T) {
	t.Parallel()

	early := time.Date(2026, 4, 28, 13, 30, 0, 0, time.UTC)
	newest := time.Date(2026, 4, 29, 13, 30, 0, 0, time.UTC)
	past := time.Date(2026, 4, 30, 13, 30, 0, 0, time.UTC)

	client := servicePb.NewMockEngineSessionClient(t)
	pool := worker.NewMockPool(t)
	client.On("RunBacktest", mock.Anything, mock.Anything).
		Return(&servicePb.RunBacktestResponse{NumPeriods: 3}, nil).Once()
	for _, at := range []time.Time{early, newest, past} {
		client.On("GetCurrentPeriod", mock.Anything, mock.Anything).Return(periodAt(at), nil).Once()
	}
	client.On("GetCurrentPeriod", mock.Anything, mock.Anything).
		Return(&servicePb.GetCurrentPeriodResponse{IsRunning: false}, nil).Once()
	client.On("PlaceOrdersAndContinue", mock.Anything, mock.Anything).
		Return(&servicePb.PlaceOrdersAndContinueResponse{}, nil).Times(3)

	pool.On("Process", mock.Anything, early, []string{"AAPL", "ABC", "MSFT"}, mock.Anything).Return(nil, nil).Once()
	pool.On("Process", mock.Anything, newest, []string{"AAPL", "MSFT"}, mock.Anything).Return(nil, nil).Once()
	pool.On("Process", mock.Anything, past, []string{"AAPL", "MSFT"}, mock.Anything).Return(nil, nil).Once()

	members := []string{"AAPL", "ABC", "MSFT"}
	provider := ""
	end := time.Time{}
	session := &ZipelineSession{
		client: client,
		symbolsAt: fakeMembersWithBars{
			fakeMemberSymbols: fakeMemberSymbols{byDate: map[string][]string{
				"2026-04-28": members, "2026-04-29": members, "2026-04-30": members,
			}, intervals: etfMembership},
			aliases: []repository.AliasLastBar{
				{Symbol: "AAPL", FromDate: day(2026, 1, 1), LastBar: ptr(newest)},
				{Symbol: "ABC", FromDate: day(2026, 1, 1), LastBar: ptr(early)},
				{Symbol: "MSFT", FromDate: day(2026, 1, 1), LastBar: ptr(newest)},
			},
			provider: &provider,
			end:      &end,
		},
		backtests: fakeBacktests{backtest: &pb.Backtest{
			Name: "sp100", Benchmark: "OEF",
			Periods: []*pb.Backtest_Period{{StartDate: date(2026, 1, 1), EndDate: date(2026, 4, 30)}},
		}},
	}

	ch, err := session.RunBacktest(context.Background(), &pb.Execution{Backtest: "sp100"}, pool)
	require.NoError(t, err)
	drain(t, ch)
	require.NoError(t, session.RunFailure())

	require.Equal(t, []string{"1 symbol had no prices after 2026-04-28 and was left out: ABC."}, session.RunNotices())
}

func TestWithoutEndedPricesLeavesNothingOutWhenEveryBarEndsTogether(t *testing.T) {
	t.Parallel()

	last := day(2026, 4, 29)
	aliases := []repository.AliasLastBar{
		{Symbol: "AAPL", FromDate: day(2026, 1, 1), LastBar: ptr(last)},
		{Symbol: "MSFT", FromDate: day(2026, 1, 1), LastBar: ptr(last)},
	}
	left := newLeftOut()
	source := withoutEndedPrices(func(context.Context, time.Time) ([]string, error) {
		return []string{"AAPL", "MSFT"}, nil
	}, aliases, left)

	symbols, err := source(context.Background(), day(2026, 4, 30))
	require.NoError(t, err)
	require.Equal(t, []string{"AAPL", "MSFT"}, symbols)
	require.Empty(t, left.notices())
}

func TestRunNoticesEmptyWhenNothingWasLeftOut(t *testing.T) {
	t.Parallel()
	require.Empty(t, (&ZipelineSession{}).RunNotices())
	require.Empty(t, newLeftOut().notices())
}

func TestLeftOutNoticesUseOneLinePerLastPriceDate(t *testing.T) {
	t.Parallel()

	left := newLeftOut()
	left.lastBar["GHI"] = day(2026, 3, 31)
	left.lastBar["ABC"] = day(2026, 3, 31)
	left.lastBar["XYZ"] = day(2026, 4, 15)
	left.lastBar["DEF"] = day(2026, 3, 31)

	require.Equal(t, []string{
		"3 symbols had no prices after 2026-03-31 and were left out: ABC, DEF, GHI.",
		"1 symbol had no prices after 2026-04-15 and was left out: XYZ.",
	}, left.notices())
}

func TestLastBarAtUsesTheAliasThatHoldsTheTickerOnTheDay(t *testing.T) {
	t.Parallel()

	// XYZ was reused: the first ISIN held it until 2026-02-01 and its bars
	// ended in January; the second ISIN holds it after and still trades.
	aliases := []repository.AliasLastBar{
		{Symbol: "XYZ", FromDate: day(2026, 1, 1), ToDate: ptr(day(2026, 2, 1)), LastBar: ptr(day(2026, 1, 20))},
		{Symbol: "XYZ", FromDate: day(2026, 2, 1), LastBar: ptr(day(2026, 4, 30))},
		{Symbol: "NOBARS", FromDate: day(2026, 1, 1)},
	}

	last, ok := lastBarAt(aliases, "XYZ", day(2026, 1, 25))
	require.True(t, ok)
	require.Equal(t, day(2026, 1, 20), last)

	last, ok = lastBarAt(aliases, "XYZ", day(2026, 3, 2))
	require.True(t, ok)
	require.Equal(t, day(2026, 4, 30), last)

	_, ok = lastBarAt(aliases, "NOBARS", day(2026, 3, 2))
	require.False(t, ok, "a member with no bar at all is not left out here; the ingest refuses it")

	_, ok = lastBarAt(aliases, "MISSING", day(2026, 3, 2))
	require.False(t, ok)
}
