package trading

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// Wrapper fills ISIN from a Directory before the inner Trading is called.
// An empty resolution does not call the inner PlaceOrder.
type Wrapper struct {
	inner supplier.Trading
	dir   Directory
	clock func() time.Time
}

// Option configures a Wrapper.
type Option func(*Wrapper)

// WithClock overrides the time used to resolve aliases.
func WithClock(now func() time.Time) Option {
	return func(w *Wrapper) {
		if now != nil {
			w.clock = now
		}
	}
}

// NewWrapper returns a Trading that resolves identity and then calls inner.
func NewWrapper(inner supplier.Trading, dir Directory, opts ...Option) *Wrapper {
	w := &Wrapper{
		inner: inner,
		dir:   dir,
		clock: time.Now,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// GetPortfolio maps every position back to an ISIN. One unmapped position
// fails the snapshot.
func (w *Wrapper) GetPortfolio(ctx context.Context) (*pb.Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if w == nil || w.inner == nil {
		return nil, fmt.Errorf("trading wrapper has no backend")
	}
	portfolio, err := w.inner.GetPortfolio(ctx)
	if err != nil || portfolio == nil {
		return portfolio, err
	}
	for _, pos := range portfolio.GetPositions() {
		if pos == nil {
			return nil, fmt.Errorf("nil position: %w", supplier.ErrUnmappedPosition)
		}
		if strings.TrimSpace(pos.GetIsin()) != "" {
			continue
		}
		isin, err := w.oneISIN(ctx, pos.GetSymbol(), "position", supplier.ErrUnmappedPosition)
		if err != nil {
			return nil, err
		}
		pos.Isin = isin
	}
	return portfolio, nil
}

// PlaceOrder copies a resolved ISIN onto the order when it is empty.
// Zero or several matches return ErrNotTradable and do not call inner.
func (w *Wrapper) PlaceOrder(ctx context.Context, order *pb.Order) (*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("nil order: %w", supplier.ErrRejected)
	}
	if w == nil || w.inner == nil {
		return nil, fmt.Errorf("trading wrapper has no backend")
	}
	if strings.TrimSpace(order.GetIsin()) == "" {
		isin, err := w.oneISIN(ctx, order.GetSymbol(), "symbol", supplier.ErrNotTradable)
		if err != nil {
			return nil, err
		}
		order.Isin = isin
	}
	return w.inner.PlaceOrder(ctx, order)
}

// GetOrders returns the inner order list.
func (w *Wrapper) GetOrders(ctx context.Context) ([]*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if w == nil || w.inner == nil {
		return nil, fmt.Errorf("trading wrapper has no backend")
	}
	return w.inner.GetOrders(ctx)
}

func (w *Wrapper) oneISIN(ctx context.Context, symbol, kind string, sentinel error) (string, error) {
	symbol = strings.TrimSpace(symbol)
	if w.dir == nil {
		return "", fmt.Errorf("%s %s: %w", kind, symbol, sentinel)
	}
	isins, err := w.dir.Resolve(ctx, symbol, w.now())
	if err != nil {
		return "", err
	}
	if len(isins) != 1 {
		return "", fmt.Errorf("%s %s: %w", kind, symbol, sentinel)
	}
	return isins[0], nil
}

func (w *Wrapper) now() time.Time {
	if w != nil && w.clock != nil {
		return w.clock()
	}
	return time.Now()
}

// ISINGuard is the adapter-side check. An empty ISIN returns ErrNotTradable
// and does not call the inner PlaceOrder.
type ISINGuard struct {
	Inner supplier.Trading
}

// NewISINGuard wraps inner so an empty ISIN fails closed.
func NewISINGuard(inner supplier.Trading) *ISINGuard {
	return &ISINGuard{Inner: inner}
}

// GetPortfolio delegates to the inner Trading.
func (g *ISINGuard) GetPortfolio(ctx context.Context) (*pb.Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g == nil || g.Inner == nil {
		return nil, fmt.Errorf("isin guard has no backend")
	}
	return g.Inner.GetPortfolio(ctx)
}

// PlaceOrder rejects an empty ISIN before the inner PlaceOrder.
func (g *ISINGuard) PlaceOrder(ctx context.Context, order *pb.Order) (*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if order == nil || strings.TrimSpace(order.GetIsin()) == "" {
		return nil, fmt.Errorf("empty isin: %w", supplier.ErrNotTradable)
	}
	if g.Inner == nil {
		return nil, fmt.Errorf("isin guard has no backend")
	}
	return g.Inner.PlaceOrder(ctx, order)
}

// GetOrders delegates to the inner Trading.
func (g *ISINGuard) GetOrders(ctx context.Context) ([]*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g == nil || g.Inner == nil {
		return nil, fmt.Errorf("isin guard has no backend")
	}
	return g.Inner.GetOrders(ctx)
}
