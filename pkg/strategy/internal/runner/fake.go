package runner

import (
	"context"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// FakeClock is a controllable Clock for tests.
type FakeClock struct {
	mu       sync.Mutex
	now      time.Time
	sessions []SessionClose
	waiters  []fakeWaiter
}

type fakeWaiter struct {
	t  time.Time
	ch chan struct{}
}

func NewFakeClock(now time.Time, sessions ...SessionClose) *FakeClock {
	copied := append([]SessionClose(nil), sessions...)
	return &FakeClock{now: now, sessions: copied}
}

var _ Clock = (*FakeClock)(nil)

func (f *FakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *FakeClock) NextClose(_ context.Context, _ string, after *commonpb.Date) (*SessionClose, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i := range f.sessions {
		s := f.sessions[i]
		if after == nil {
			if !s.Close.Before(f.now) {
				cp := s
				return &cp, nil
			}
			continue
		}
		if dateAfter(s.Date, after) {
			cp := s
			return &cp, nil
		}
	}
	return nil, ErrNoSession
}

func (f *FakeClock) WaitUntil(ctx context.Context, t time.Time) error {
	f.mu.Lock()
	if !f.now.Before(t) {
		f.mu.Unlock()
		return nil
	}
	ch := make(chan struct{})
	f.waiters = append(f.waiters, fakeWaiter{t: t, ch: ch})
	f.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}

// Advance sets now and unblocks waiters whose deadline is at or before now.
func (f *FakeClock) Advance(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = now
	rest := f.waiters[:0]
	for _, w := range f.waiters {
		if !now.Before(w.t) {
			close(w.ch)
			continue
		}
		rest = append(rest, w)
	}
	f.waiters = rest
}

// FakeTrading records GetPortfolio and PlaceOrder calls.
type FakeTrading struct {
	mu                sync.Mutex
	Portfolio         *financepb.Portfolio
	GetPortfolioCalls int
	PlaceOrderCalls   int
	GetOrdersCalls    int
	PlacedOrders      []*financepb.Order
	PlaceErr          error
	FailAfter         int
}

var _ supplier.Trading = (*FakeTrading)(nil)

func (f *FakeTrading) GetPortfolio(ctx context.Context) (*financepb.Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.GetPortfolioCalls++
	if f.Portfolio == nil {
		return &financepb.Portfolio{}, nil
	}
	return f.Portfolio, nil
}

func (f *FakeTrading) PlaceOrder(ctx context.Context, order *financepb.Order) (*financepb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.PlaceOrderCalls++
	if f.PlaceErr != nil && (f.FailAfter == 0 || f.PlaceOrderCalls > f.FailAfter) {
		return nil, f.PlaceErr
	}
	f.PlacedOrders = append(f.PlacedOrders, order)
	return order, nil
}

func (f *FakeTrading) GetOrders(ctx context.Context) ([]*financepb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.GetOrdersCalls++
	return nil, nil
}

func (f *FakeTrading) PlaceCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.PlaceOrderCalls
}

func (f *FakeTrading) Placed() []*financepb.Order {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*financepb.Order, len(f.PlacedOrders))
	copy(out, f.PlacedOrders)
	return out
}

func (f *FakeTrading) PortfolioCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.GetPortfolioCalls
}

// FakeProcessor returns canned orders from Process.
type FakeProcessor struct {
	mu            sync.Mutex
	Orders        []*financepb.Order
	Err           error
	Calls         int
	LastTimestamp time.Time
	LastSymbols   []string
}

var _ Processor = (*FakeProcessor)(nil)

func (f *FakeProcessor) Process(_ context.Context, timestamp time.Time, symbols []string, _ *financepb.Portfolio) ([]*financepb.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls++
	f.LastTimestamp = timestamp
	f.LastSymbols = append([]string(nil), symbols...)
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Orders, nil
}

func (f *FakeProcessor) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Calls
}
