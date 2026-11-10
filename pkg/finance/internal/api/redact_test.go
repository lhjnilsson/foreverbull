package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const crumbURL = "https://query2.finance.yahoo.com/v1/finance/quoteType/AAPL?crumb=s3cr3tCrumb#frag"

func yahooURLError() error {
	return fmt.Errorf("yahoo get asset: %w", &url.Error{
		Op:  "Get",
		URL: crumbURL,
		Err: errors.New("dial tcp: connection refused"),
	})
}

func TestInternalDownloadErrorDoesNotLogCrumb(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	err := internalDownloadError("AAPL", "get asset", yahooURLError())

	require.EqualError(t, err, "couldn't download prices for AAPL. Try again in a few minutes, or use another provider with --provider.")
	assert.Contains(t, buf.String(), "query2.finance.yahoo.com/v1/finance/quoteType/AAPL")
	assert.NotContains(t, buf.String(), "crumb")
	assert.NotContains(t, buf.String(), "s3cr3tCrumb")
}

// sentDownload records the STATUS_ERROR responses a download sends.
type sentDownload struct {
	grpc.ServerStream
	sent []*servicePb.DownloadHistoricalDataResponse
}

func (s *sentDownload) Send(rsp *servicePb.DownloadHistoricalDataResponse) error {
	s.sent = append(s.sent, rsp)
	return nil
}

// A Yahoo *url.Error carries ?crumb=. The download's STATUS_ERROR is what
// backtest create reads, and the price-download log writes the same error.
// Neither may keep the crumb.
func TestDownloadSymbolKeepsYahooCrumbOutOfStatusAndLog(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &sentDownload{}
	md := supplier.NewMockMarketdata(t)
	md.On("GetAsset", mock.Anything, "AAPL").Return(nil, yahooURLError())
	server := NewMarketdataServer(nil, dividendFree{md})

	blocked := server.downloadSymbol(ctx, newDownloadSender(stream, cancel), "AAPL",
		time.Unix(0, 0).UTC(), nil, "yahoo", repository.Asset{}, repository.OHLC{}, md)

	require.Nil(t, blocked)
	require.Len(t, stream.sent, 1)
	msg := stream.sent[0]
	require.Equal(t, servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msg.GetStatus())
	require.Equal(t, downloadFailedError("AAPL").Error(), msg.GetError())
	assertNoCrumb(t, msg.GetError())
	assertNoCrumb(t, buf.String())
	assert.Contains(t, buf.String(), "historical data download failed")
	assert.Contains(t, buf.String(), "query2.finance.yahoo.com/v1/finance/quoteType/AAPL")

	// The status boundary scrubs a Yahoo error that reaches it directly,
	// not only one already replaced with the generic sentence.
	raw := &servicePb.DownloadHistoricalDataResponse{}
	failDownload(raw, yahooURLError())
	require.Equal(t, servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, raw.GetStatus())
	assertNoCrumb(t, raw.GetError())
	assert.Contains(t, raw.GetError(), "query2.finance.yahoo.com/v1/finance/quoteType/AAPL")

	buf.Reset()
	ohlcErr := fmt.Errorf("fail to get OHLC data for symbol AAPL: error making request: %w", &url.Error{
		Op:  "Get",
		URL: "https://query2.finance.yahoo.com/v8/finance/chart/AAPL?crumb=s3cr3tCrumb&interval=1d",
		Err: errors.New("dial tcp: connection refused"),
	})
	reported := askedTickerFailure(ctx, "AAPL", "yahoo", ohlcErr)
	rsp := &servicePb.DownloadHistoricalDataResponse{Symbol: "AAPL"}
	failDownload(rsp, reported)
	require.Equal(t, downloadFailedError("AAPL").Error(), rsp.GetError())
	assertNoCrumb(t, rsp.GetError())
	assertNoCrumb(t, buf.String())
	assert.Contains(t, buf.String(), "historical data download failed")
	assert.Contains(t, buf.String(), "query2.finance.yahoo.com/v8/finance/chart/AAPL")
}

func assertNoCrumb(t *testing.T, text string) {
	t.Helper()
	assert.NotContains(t, text, "crumb=")
	assert.NotContains(t, text, "s3cr3tCrumb")
}

func TestGetAssetErrorDoesNotReturnCrumb(t *testing.T) {
	md := supplier.NewMockMarketdata(t)
	md.On("GetAsset", mock.Anything, "AAPL").Return(nil, yahooURLError())
	server := NewMarketdataServer(nil, dividendFree{md})

	_, err := server.GetAsset(context.Background(), &servicePb.GetAssetRequest{
		Identifier: &servicePb.GetAssetRequest_Symbol{Symbol: "AAPL"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "query2.finance.yahoo.com")
	assert.NotContains(t, err.Error(), "s3cr3tCrumb")
}
