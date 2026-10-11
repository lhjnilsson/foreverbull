package trading_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const metaISIN = "US30303M1027"

type recordingTrading struct {
	mu        sync.Mutex
	place     int
	orders    []*pb.Order
	portfolio *pb.Portfolio
}

func (r *recordingTrading) GetPortfolio(ctx context.Context) (*pb.Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.portfolio == nil {
		return &pb.Portfolio{}, nil
	}
	return proto.Clone(r.portfolio).(*pb.Portfolio), nil
}

func (r *recordingTrading) PlaceOrder(ctx context.Context, order *pb.Order) (*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.place++
	cloned, _ := proto.Clone(order).(*pb.Order)
	r.orders = append(r.orders, cloned)
	return cloned, nil
}

func (r *recordingTrading) GetOrders(ctx context.Context) ([]*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (r *recordingTrading) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.place
}

func (r *recordingTrading) placed() []*pb.Order {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*pb.Order, len(r.orders))
	copy(out, r.orders)
	return out
}

var (
	_ supplier.Trading = (*trading.Wrapper)(nil)
	_ supplier.Trading = (*trading.ISINGuard)(nil)
	_ supplier.Trading = (*recordingTrading)(nil)
)

func TestSymbolWithTwoAliasesMapsToOneISIN(t *testing.T) {
	fbEnd := time.Date(2022, time.June, 9, 0, 0, 0, 0, time.UTC)
	dir := trading.NewMemoryDirectory(
		trading.SymbolAlias{
			ISIN:   metaISIN,
			Symbol: "FB",
			From:   time.Date(2012, time.May, 18, 0, 0, 0, 0, time.UTC),
			To:     &fbEnd,
		},
		trading.SymbolAlias{
			ISIN:   metaISIN,
			Symbol: "META",
			From:   fbEnd,
		},
		trading.SymbolAlias{
			ISIN:   metaISIN,
			Symbol: "META",
			From:   time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	)
	inner := &recordingTrading{}
	when := time.Date(2021, time.June, 1, 15, 0, 0, 0, time.UTC)
	w := trading.NewWrapper(inner, dir, trading.WithClock(func() time.Time { return when }))

	placed, err := w.PlaceOrder(context.Background(), &pb.Order{Symbol: "FB", Amount: 4})
	require.NoError(t, err)
	require.Equal(t, metaISIN, placed.GetIsin())

	when = time.Date(2024, time.January, 15, 15, 0, 0, 0, time.UTC)
	placed, err = w.PlaceOrder(context.Background(), &pb.Order{Symbol: "META", Amount: 2})
	require.NoError(t, err)
	require.Equal(t, metaISIN, placed.GetIsin())
	require.Equal(t, metaISIN, inner.placed()[0].GetIsin())
	require.Equal(t, metaISIN, inner.placed()[1].GetIsin())

	_, err = w.PlaceOrder(context.Background(), &pb.Order{Symbol: "FB", Amount: 1})
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Equal(t, 2, inner.calls())
}

func TestTwoISINsForOneSymbol(t *testing.T) {
	dir := trading.NewMemoryDirectory(
		trading.SymbolAlias{ISIN: "US1111111111", Symbol: "DUP", From: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)},
		trading.SymbolAlias{ISIN: "US2222222222", Symbol: "DUP", From: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)},
	)
	inner := &recordingTrading{}
	w := trading.NewWrapper(inner, dir)
	_, err := w.PlaceOrder(context.Background(), &pb.Order{Symbol: "DUP", Amount: 1})
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Equal(t, 0, inner.calls())
}

func TestEmptyISINRejectedWithoutInnerPlaceOrder(t *testing.T) {
	inner := &recordingTrading{}
	guard := trading.NewISINGuard(inner)
	_, err := guard.PlaceOrder(context.Background(), &pb.Order{Symbol: "AAPL", Amount: 10})
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Equal(t, 0, inner.calls())

	placed, err := guard.PlaceOrder(context.Background(), &pb.Order{Symbol: "AAPL", Amount: 10, Isin: "US0378331005"})
	require.NoError(t, err)
	require.Equal(t, "US0378331005", placed.GetIsin())
	require.Equal(t, 1, inner.calls())
}

func TestUnmappedPosition(t *testing.T) {
	dir := trading.NewMemoryDirectory(trading.SymbolAlias{
		ISIN:   "US0378331005",
		Symbol: "AAPL",
		From:   time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	})
	inner := &recordingTrading{portfolio: &pb.Portfolio{Positions: []*pb.Position{
		{Symbol: "AAPL", Amount: 3},
	}}}
	w := trading.NewWrapper(inner, dir)
	got, err := w.GetPortfolio(context.Background())
	require.NoError(t, err)
	require.Equal(t, "US0378331005", got.GetPositions()[0].GetIsin())

	inner.portfolio = &pb.Portfolio{Positions: []*pb.Position{
		{Symbol: "AAPL", Amount: 3},
		{Symbol: "NOPE", Amount: 1},
	}}
	got, err = w.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnmappedPosition)
	require.Nil(t, got)
}
