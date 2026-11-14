package marketdata

import (
	"context"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
)

// countingPrice records GetOHLC calls. The other methods are unused.
type countingPrice struct {
	calls  int
	symbol string
}

func (c *countingPrice) GetAsset(context.Context, string) (*pb.Asset, error) {
	return nil, nil
}

func (c *countingPrice) SearchAsset(context.Context, string) ([]*pb.Asset, error) {
	return nil, nil
}

func (c *countingPrice) GetIndex(context.Context, string) ([]*pb.Asset, error) {
	return nil, nil
}

func (c *countingPrice) GetOHLC(_ context.Context, symbol string, _ time.Time, _ *time.Time) ([]*pb.OHLC, error) {
	c.calls++
	c.symbol = symbol
	return []*pb.OHLC{{Symbol: symbol}}, nil
}

func TestFetchTradingClientUsesTheConfiguredProvider(t *testing.T) {
	t.Setenv(environment.StooqExchange, "US")
	yahoo := &countingPrice{}
	stooq := &countingPrice{}
	prev := newStooqPriceClient
	newStooqPriceClient = func(newYahoo func() (supplier.Marketdata, error), opts ...StooqOption) (supplier.Marketdata, error) {
		client := &StooqClient{}
		for _, opt := range opts {
			if err := opt(client); err != nil {
				return nil, err
			}
		}
		if client.exchange != "us" {
			t.Errorf("OHLCClient exchange = %q", client.exchange)
		}
		meta, err := newYahoo()
		if err != nil {
			return nil, err
		}
		if meta != yahoo {
			t.Errorf("stooq metadata client is not the yahoo client OHLCClient was given")
		}
		return stooq, nil
	}
	t.Cleanup(func() { newStooqPriceClient = prev })

	start := time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC)
	yahooClient, err := FetchTradingClient("yahoo", "", "", yahoo)
	require.NoError(t, err)
	bars, err := yahooClient.GetOHLC(context.Background(), "AAPL", start, nil)
	require.NoError(t, err)
	require.Equal(t, "AAPL", bars[0].GetSymbol())

	stooqClient, err := FetchTradingClient("stooq", "", "", yahoo)
	require.NoError(t, err)
	bars, err = stooqClient.GetOHLC(context.Background(), "AAPL", start, nil)
	require.NoError(t, err)
	require.Equal(t, "AAPL", bars[0].GetSymbol())

	require.Equal(t, 1, yahoo.calls)
	require.Equal(t, 1, stooq.calls)
}

func TestFetchTradingClientRefusesAMissingOrUnknownProvider(t *testing.T) {
	t.Parallel()
	_, err := FetchTradingClient("", "", "", nil)
	require.EqualError(t, err, usercopy.TradingProviderMissing(""))
	_, err = FetchTradingClient("   ", "", "", nil)
	require.EqualError(t, err, usercopy.TradingProviderMissing(""))
	_, err = FetchTradingClient("binance", "", "", nil)
	require.EqualError(t, err, usercopy.TradingProviderUnknown("binance", ""))
}

func TestFetchTradingClientRefusesStooqWithoutAnExchange(t *testing.T) {
	t.Setenv(environment.StooqExchange, "")
	_, err := FetchTradingClient("stooq", "", "", nil)
	require.EqualError(t, err, usercopy.StooqExchangeMissing)

	for _, exchange := range []string{"L", "DE", "UK", "TO"} {
		t.Setenv(environment.StooqExchange, exchange)
		_, err = FetchTradingClient("stooq", "", "", nil)
		require.EqualError(t, err, usercopy.StooqExchangeUnsupported(exchange), exchange)
	}
}
