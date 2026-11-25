package runner

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/stretchr/testify/require"
)

func londonDeployment() *pb.Deployment {
	return &pb.Deployment{
		Id:           "dep-vod",
		StrategyName: "mean-rev",
		Backend:      "alpaca-paper",
		CalendarCode: "XNYS",
		Symbols:      []string{"VOD.L"},
		DryRun:       false,
		Status:       pb.Deployment_RUNNING,
	}
}

func TestRunningLondonDeploymentIsNotTraded(t *testing.T) {
	writeAlpacaProfile(t)
	restore := finance.SetQuoteReader(func(_ context.Context, _, _, symbol string) (finance.Quote, error) {
		if symbol == "VOD.L" {
			return finance.Quote{Exchange: "LSE", Currency: "GBP", Reported: true}, nil
		}
		return finance.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	t.Cleanup(restore)

	store := NewMemoryStore()
	store.Put(londonDeployment())
	trading := &FakeTrading{Portfolio: &financepb.Portfolio{Cash: 1000}}
	processor := &FakeProcessor{Orders: []*financepb.Order{{
		Symbol: "VOD.L", Amount: 10,
	}}}
	mon := SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 4},
		Close: time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC),
	}
	clock := NewFakeClock(mon.Close.Add(-time.Hour), mon)
	runner := New(store, clock, profileRegistry{"alpaca-paper": trading}, processor).WithPollInterval(5 * time.Millisecond)
	t.Cleanup(func() { _ = runner.Stop(context.Background()) })

	require.EqualError(t, runner.Add(context.Background(), londonDeployment()),
		"Couldn't start deployment 'mean-rev': VOD.L is listed outside the US (London, GBP). "+
			"Global markets aren't supported yet. The deployment wasn't started. "+
			"Remove VOD.L from --symbols, then run `fbull strategy deploy mean-rev` again with the same options.")
	clock.Advance(mon.Close)
	time.Sleep(40 * time.Millisecond)

	got, err := store.GetDeployment(context.Background(), "dep-vod")
	require.NoError(t, err)
	require.Equal(t, pb.Deployment_ERROR, got.GetStatus())
	require.Equal(t, "Couldn't start deployment 'mean-rev': VOD.L is listed outside the US (London, GBP). "+
		"Global markets aren't supported yet. The deployment wasn't started. "+
		"Remove VOD.L from --symbols, then run `fbull strategy deploy mean-rev` again with the same options.",
		got.GetError())
	require.Equal(t, 0, processor.CallCount())
	require.Equal(t, 0, trading.PlaceCalls())
	runs, err := store.ListRuns(context.Background(), "", "dep-vod")
	require.NoError(t, err)
	require.Empty(t, runs)
	checked, err := store.MarketsChecked(context.Background(), "dep-vod")
	require.NoError(t, err)
	require.False(t, checked)
}

func TestAddStoresAPassAndSkipsTheNextCheck(t *testing.T) {
	writeAlpacaProfile(t)
	var calls int
	var provider string
	restore := finance.SetQuoteReader(func(_ context.Context, got, _, _ string) (finance.Quote, error) {
		calls++
		provider = got
		return finance.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	t.Cleanup(restore)

	store := NewMemoryStore()
	deployment := &pb.Deployment{
		Id: "dep-1", StrategyName: "mean-rev", Backend: "alpaca-paper",
		CalendarCode: "XNYS", Symbols: []string{"AAPL"}, DryRun: true, Status: pb.Deployment_RUNNING,
	}
	store.Put(deployment)
	processor := &FakeProcessor{}
	trading := &FakeTrading{Portfolio: &financepb.Portfolio{Cash: 1}}
	mon := SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 4},
		Close: time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC),
	}
	tue := SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 5},
		Close: time.Date(2024, 3, 5, 16, 0, 0, 0, time.UTC),
	}
	clock := NewFakeClock(mon.Close.Add(-time.Hour), mon, tue)
	profiles := profileRegistry{"alpaca-paper": trading}
	runner := New(store, clock, profiles, processor).WithPollInterval(5 * time.Millisecond)

	require.NoError(t, runner.Add(context.Background(), deployment))
	clock.Advance(mon.Close)
	require.Eventually(t, func() bool { return processor.CallCount() == 1 }, time.Second, 10*time.Millisecond)
	checked, err := store.MarketsChecked(context.Background(), "dep-1")
	require.NoError(t, err)
	require.True(t, checked)
	require.Equal(t, 1, calls)
	require.Equal(t, "alpaca", provider)
	require.NoError(t, runner.Stop(context.Background()))

	calls = 0
	restoreFail := finance.SetQuoteReader(func(context.Context, string, string, string) (finance.Quote, error) {
		calls++
		return finance.Quote{}, fmt.Errorf("yahoo timed out")
	})
	t.Cleanup(restoreFail)
	again := New(store, clock, profiles, processor).WithPollInterval(5 * time.Millisecond)
	t.Cleanup(func() { _ = again.Stop(context.Background()) })
	require.NoError(t, again.Add(context.Background(), deployment))
	clock.Advance(tue.Close)
	require.Eventually(t, func() bool { return processor.CallCount() == 2 }, time.Second, 10*time.Millisecond)
	require.Zero(t, calls)
	got, err := store.GetDeployment(context.Background(), "dep-1")
	require.NoError(t, err)
	require.Equal(t, pb.Deployment_RUNNING, got.GetStatus())
}

const unknownSymbolSentence = "Couldn't start deployment 'mean-rev': yahoo doesn't know the symbol NOPE. " +
	"The deployment wasn't started. Check NOPE, or remove it from --symbols, " +
	"then run `fbull strategy deploy mean-rev` again with the same options."

func TestAddUnknownSymbolIsNotAPass(t *testing.T) {
	restore := finance.SetQuoteReader(func(_ context.Context, _, _, symbol string) (finance.Quote, error) {
		if symbol == "NOPE" {
			return finance.Quote{}, finance.ErrUnknownSymbol
		}
		return finance.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	t.Cleanup(restore)

	store := NewMemoryStore()
	deployment := &pb.Deployment{
		Id: "dep-nope", StrategyName: "mean-rev", CalendarCode: "XNYS",
		Symbols: []string{"NOPE"}, DryRun: true, Status: pb.Deployment_RUNNING,
	}
	store.Put(deployment)
	processor := &FakeProcessor{}
	mon := SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 4},
		Close: time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC),
	}
	clock := NewFakeClock(mon.Close.Add(-time.Hour), mon)
	runner := New(store, clock, nil, processor).WithPollInterval(5 * time.Millisecond)
	t.Cleanup(func() { _ = runner.Stop(context.Background()) })

	require.EqualError(t, runner.Add(context.Background(), deployment), unknownSymbolSentence)
	clock.Advance(mon.Close)
	time.Sleep(40 * time.Millisecond)

	got, err := store.GetDeployment(context.Background(), "dep-nope")
	require.NoError(t, err)
	require.Equal(t, pb.Deployment_ERROR, got.GetStatus())
	require.Equal(t, unknownSymbolSentence, got.GetError())
	checked, err := store.MarketsChecked(context.Background(), "dep-nope")
	require.NoError(t, err)
	require.False(t, checked)
	require.Equal(t, 0, processor.CallCount())
}

type orderedStore struct {
	*MemoryStore
	order []string
}

func (s orderedStore) ListRunning(ctx context.Context) ([]*pb.Deployment, error) {
	all, err := s.MemoryStore.ListRunning(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[string]*pb.Deployment{}
	for _, d := range all {
		byID[d.GetId()] = d
	}
	out := make([]*pb.Deployment, 0, len(s.order))
	for _, id := range s.order {
		if d, ok := byID[id]; ok {
			out = append(out, d)
		}
	}
	return out, nil
}

func TestStartMarksAnUnknownSymbolAndResumesTheOthers(t *testing.T) {
	restore := finance.SetQuoteReader(func(_ context.Context, _, _, symbol string) (finance.Quote, error) {
		if symbol == "NOPE" {
			return finance.Quote{}, finance.ErrUnknownSymbol
		}
		return finance.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	t.Cleanup(restore)

	store := orderedStore{MemoryStore: NewMemoryStore(), order: []string{"dep-nope", "dep-ok"}}
	store.Put(&pb.Deployment{
		Id: "dep-nope", StrategyName: "mean-rev", CalendarCode: "XNYS",
		Symbols: []string{"NOPE"}, DryRun: true, Status: pb.Deployment_RUNNING,
	})
	store.Put(&pb.Deployment{
		Id: "dep-ok", StrategyName: "other", CalendarCode: "XNYS",
		Symbols: []string{"AAPL"}, DryRun: true, Status: pb.Deployment_RUNNING,
	})
	processor := &FakeProcessor{}
	mon := SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 4},
		Close: time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC),
	}
	clock := NewFakeClock(mon.Close.Add(-time.Hour), mon)
	runner := New(store, clock, nil, processor).WithPollInterval(5 * time.Millisecond)
	t.Cleanup(func() { _ = runner.Stop(context.Background()) })

	require.NoError(t, runner.Start(context.Background()))
	got, err := store.GetDeployment(context.Background(), "dep-nope")
	require.NoError(t, err)
	require.Equal(t, pb.Deployment_ERROR, got.GetStatus())
	require.Equal(t, unknownSymbolSentence, got.GetError())

	clock.Advance(mon.Close)
	require.Eventually(t, func() bool { return processor.CallCount() == 1 }, time.Second, 10*time.Millisecond)
	other, err := store.GetDeployment(context.Background(), "dep-ok")
	require.NoError(t, err)
	require.Equal(t, pb.Deployment_RUNNING, other.GetStatus())
}

func TestStartMarksAMissingProfileError(t *testing.T) {
	writeAlpacaProfile(t)
	restore := finance.SetQuoteReader(func(context.Context, string, string, string) (finance.Quote, error) {
		return finance.Quote{}, fmt.Errorf("listing must not be asked when the profile is missing")
	})
	t.Cleanup(restore)

	store := NewMemoryStore()
	store.Put(&pb.Deployment{
		Id: "dep-gone", StrategyName: "mean-rev", Backend: "gone",
		CalendarCode: "XNYS", Symbols: []string{"AAPL"}, DryRun: true, Status: pb.Deployment_RUNNING,
	})
	processor := &FakeProcessor{}
	mon := SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 4},
		Close: time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC),
	}
	clock := NewFakeClock(mon.Close.Add(-time.Hour), mon)
	runner := New(store, clock, nil, processor).WithPollInterval(5 * time.Millisecond)
	t.Cleanup(func() { _ = runner.Stop(context.Background()) })

	require.NoError(t, runner.Start(context.Background()))
	clock.Advance(mon.Close)
	time.Sleep(40 * time.Millisecond)

	got, err := store.GetDeployment(context.Background(), "dep-gone")
	require.NoError(t, err)
	require.Equal(t, pb.Deployment_ERROR, got.GetStatus())
	require.Equal(t, "Couldn't start deployment 'mean-rev': there's no trading profile named 'gone', "+
		"so foreverbull can't check where its symbols are listed. The deployment wasn't started. "+
		"Check the name against the trading profiles set up for foreverbull, then run `fbull strategy deploy mean-rev` again with the same options.",
		got.GetError())
	require.Equal(t, 0, processor.CallCount())
	runs, err := store.ListRuns(context.Background(), "", "dep-gone")
	require.NoError(t, err)
	require.Empty(t, runs)
}

type statusFailStore struct {
	orderedStore
}

func (s statusFailStore) UpdateStatus(ctx context.Context, id string, status pb.Deployment_Status, err error) error {
	if status == pb.Deployment_ERROR {
		return errors.New("status write failed")
	}
	return s.MemoryStore.UpdateStatus(ctx, id, status, err)
}

func TestStartReturnsWhenTheErrorStatusCannotBeSaved(t *testing.T) {
	writeAlpacaProfile(t)
	restore := finance.SetQuoteReader(func(context.Context, string, string, string) (finance.Quote, error) {
		return finance.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	t.Cleanup(restore)

	mem := NewMemoryStore()
	store := statusFailStore{orderedStore: orderedStore{MemoryStore: mem, order: []string{"dep-ok", "dep-gone"}}}
	store.Put(&pb.Deployment{
		Id: "dep-ok", StrategyName: "other", CalendarCode: "XNYS",
		Symbols: []string{"AAPL"}, DryRun: true, Status: pb.Deployment_RUNNING,
	})
	require.NoError(t, store.MarkMarketsChecked(context.Background(), "dep-ok"))
	store.Put(&pb.Deployment{
		Id: "dep-gone", StrategyName: "mean-rev", Backend: "gone", CalendarCode: "XNYS",
		Symbols: []string{"AAPL"}, DryRun: true, Status: pb.Deployment_RUNNING,
	})
	processor := &FakeProcessor{}
	mon := SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 4},
		Close: time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC),
	}
	clock := NewFakeClock(mon.Close.Add(-time.Hour), mon)
	runner := New(store, clock, nil, processor).WithPollInterval(5 * time.Millisecond)

	err := runner.Start(context.Background())
	require.Error(t, err)
	got, getErr := store.GetDeployment(context.Background(), "dep-gone")
	require.NoError(t, getErr)
	require.Equal(t, pb.Deployment_RUNNING, got.GetStatus())
	runner.mu.Lock()
	left := len(runner.loops)
	runner.mu.Unlock()
	require.Zero(t, left)
	require.Equal(t, 0, processor.CallCount())
}

func TestAddMarksALookupFailureError(t *testing.T) {
	writeAlpacaProfile(t)
	restore := finance.SetQuoteReader(func(context.Context, string, string, string) (finance.Quote, error) {
		return finance.Quote{}, fmt.Errorf("yahoo timed out")
	})
	t.Cleanup(restore)

	store := NewMemoryStore()
	deployment := &pb.Deployment{
		Id: "dep-1", StrategyName: "mean-rev", Backend: "alpaca-paper",
		CalendarCode: "XNYS", Symbols: []string{"AAPL"}, DryRun: true, Status: pb.Deployment_RUNNING,
	}
	store.Put(deployment)
	processor := &FakeProcessor{}
	mon := SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 4},
		Close: time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC),
	}
	clock := NewFakeClock(mon.Close.Add(-time.Hour), mon)
	runner := New(store, clock, nil, processor).WithPollInterval(5 * time.Millisecond)
	t.Cleanup(func() { _ = runner.Stop(context.Background()) })

	require.EqualError(t, runner.Add(context.Background(), deployment),
		"Couldn't start deployment 'mean-rev': alpaca didn't respond when checking where AAPL is listed. "+
			"The deployment wasn't started. Wait a few minutes, then run `fbull strategy deploy mean-rev` again with the same options.")
	clock.Advance(mon.Close)
	time.Sleep(40 * time.Millisecond)

	got, err := store.GetDeployment(context.Background(), "dep-1")
	require.NoError(t, err)
	require.Equal(t, pb.Deployment_ERROR, got.GetStatus())
	require.Equal(t, "Couldn't start deployment 'mean-rev': alpaca didn't respond when checking where AAPL is listed. "+
		"The deployment wasn't started. Wait a few minutes, then run `fbull strategy deploy mean-rev` again with the same options.",
		got.GetError())
	checked, err := store.MarketsChecked(context.Background(), "dep-1")
	require.NoError(t, err)
	require.False(t, checked)
	require.Equal(t, 0, processor.CallCount())
	runs, err := store.ListRuns(context.Background(), "", "dep-1")
	require.NoError(t, err)
	require.Empty(t, runs)
}
