package marketdata_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCompositeRoutesOHLCToStooqAndMetadataToYahoo(t *testing.T) {
	t.Parallel()

	ohlc := supplier.NewMockMarketdata(t)
	meta := supplier.NewMockMarketdata(t)

	start := date(t, "2021-01-04")
	end := date(t, "2021-01-06")
	bars := []*pb.OHLC{{Symbol: "AAPL", Open: 1}}
	asset := &pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}
	assets := []*pb.Asset{asset}

	ohlc.On("GetOHLC", "AAPL", start, mock.Anything).Return(bars, nil).Once()
	meta.On("GetAsset", "AAPL").Return(asset, nil).Once()
	meta.On("SearchAsset", "apple").Return(assets, nil).Once()
	meta.On("GetIndex", "^DJI").Return(assets, nil).Once()

	client, err := marketdata.NewComposite(ohlc, meta)
	require.NoError(t, err)

	gotBars, err := client.GetOHLC("AAPL", start, &end)
	require.NoError(t, err)
	require.Equal(t, bars, gotBars)

	gotAsset, err := client.GetAsset("AAPL")
	require.NoError(t, err)
	require.Equal(t, asset, gotAsset)

	gotSearch, err := client.SearchAsset("apple")
	require.NoError(t, err)
	require.Equal(t, assets, gotSearch)

	gotIndex, err := client.GetIndex("^DJI")
	require.NoError(t, err)
	require.Equal(t, assets, gotIndex)

	ohlc.AssertNotCalled(t, "GetAsset", mock.Anything)
	ohlc.AssertNotCalled(t, "SearchAsset", mock.Anything)
	ohlc.AssertNotCalled(t, "GetIndex", mock.Anything)
	meta.AssertNotCalled(t, "GetOHLC", mock.Anything, mock.Anything, mock.Anything)
}

func TestNewCompositeRequiresBothClients(t *testing.T) {
	t.Parallel()

	ohlc := supplier.NewMockMarketdata(t)
	meta := supplier.NewMockMarketdata(t)

	_, err := marketdata.NewComposite(nil, meta)
	require.ErrorContains(t, err, "nil OHLC client")

	_, err = marketdata.NewComposite(ohlc, nil)
	require.ErrorIs(t, err, marketdata.ErrYahooMetadataRequired)
}

func TestNewStooqYahooFailsClosedWhenYahooConstructorFails(t *testing.T) {
	t.Parallel()

	_, err := marketdata.NewStooqYahoo(func() (supplier.Marketdata, error) {
		return nil, errors.New("crumb failed")
	})
	require.ErrorIs(t, err, marketdata.ErrYahooMetadataRequired)
	require.ErrorContains(t, err, "crumb failed")
}

func TestNewStooqYahooFailsClosedWhenYahooConstructorNil(t *testing.T) {
	t.Parallel()

	_, err := marketdata.NewStooqYahoo(nil)
	require.ErrorIs(t, err, marketdata.ErrYahooMetadataRequired)
	require.ErrorContains(t, err, "constructor is nil")
}

func TestNewStooqYahooFailsClosedWhenYahooReturnsNil(t *testing.T) {
	t.Parallel()

	_, err := marketdata.NewStooqYahoo(func() (supplier.Marketdata, error) {
		return nil, nil
	})
	require.ErrorIs(t, err, marketdata.ErrYahooMetadataRequired)
}

func TestNewStooqYahooGetOHLCUsesStooqNotYahoo(t *testing.T) {
	t.Parallel()

	var got stooqReq
	srv := serveStooq(t, readFixture(t, "aapl.us.csv"), http.StatusOK, &got)
	yahoo := supplier.NewMockMarketdata(t)
	yahoo.On("GetAsset", "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil).Once()

	client, err := marketdata.NewStooqYahoo(
		func() (supplier.Marketdata, error) { return yahoo, nil },
		marketdata.WithStooqBaseURL(srv.URL),
	)
	require.NoError(t, err)

	start := date(t, "2021-01-04")
	end := date(t, "2021-01-06")
	bars, err := client.GetOHLC("AAPL", start, &end)
	require.NoError(t, err)
	require.NotEmpty(t, bars)
	require.Equal(t, "AAPL", bars[0].Symbol)
	require.Equal(t, "aapl.us", got.s)
	require.NotContains(t, got.host, "stooq.com")

	asset, err := client.GetAsset("AAPL")
	require.NoError(t, err)
	require.Equal(t, "AAPL", asset.Symbol)

	yahoo.AssertNotCalled(t, "GetOHLC", mock.Anything, mock.Anything, mock.Anything)
}

func TestNewStooqYahooDoesNotCallLiveYahooConstructor(t *testing.T) {
	t.Parallel()

	called := 0
	yahoo := supplier.NewMockMarketdata(t)
	client, err := marketdata.NewStooqYahoo(func() (supplier.Marketdata, error) {
		called++
		return yahoo, nil
	})
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, 1, called)
}
