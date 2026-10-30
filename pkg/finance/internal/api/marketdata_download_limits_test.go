package api_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// A symbol repeated in one request is fetched, stored and reported once.
func (suite *MarketdataTest) TestDownloadHistoricalDataSkipsRepeatedSymbols() {
	suite.marketdata.On("GetAsset", mock.Anything, "AAPL").
		Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil).Once()
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", mock.Anything, mock.Anything).Return([]*pb.OHLC{{
		Timestamp: timestamppb.New(time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)),
		Open:      1, High: 2, Low: 0.5, Close: 1.5, Volume: 100,
	}}, nil).Once()

	rsp, err := suite.client.DownloadHistoricalData(context.Background(), &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"AAPL", "AAPL", "AAPL"},
		StartDate: &internal_pb.Date{Year: 2024, Month: 6, Day: 3},
		EndDate:   &internal_pb.Date{Year: 2024, Month: 6, Day: 3},
		Provider:  "yahoo",
	})
	suite.Require().NoError(err)
	msgs := suite.recvAll(rsp)
	suite.Require().Len(msgs, 1)
	suite.Equal("AAPL", msgs[0].GetSymbol())
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_OK, msgs[0].GetStatus(), msgs[0].GetError())
}

// distinctSymbols is n distinct symbols.
func distinctSymbols(n int) []string {
	symbols := make([]string, 0, n)
	for i := range n {
		symbols = append(symbols, fmt.Sprintf("S%05d", i))
	}
	return symbols
}

// download starts a download that fails within a minute instead of at the
// test binary's timeout when the server never ends the stream.
func (suite *MarketdataTest) download(symbols []string) servicePb.Marketdata_DownloadHistoricalDataClient {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	suite.T().Cleanup(cancel)
	rsp, err := suite.client.DownloadHistoricalData(ctx, &servicePb.DownloadHistoricalDataRequest{
		Symbols:   symbols,
		StartDate: &internal_pb.Date{Year: 2024, Month: 6, Day: 3},
		Provider:  "yahoo",
	})
	suite.Require().NoError(err)
	return rsp
}

// requireRefused checks that the download ended with InvalidArgument and
// message, before any provider call and with nothing stored.
func (suite *MarketdataTest) requireRefused(rsp servicePb.Marketdata_DownloadHistoricalDataClient, message string) {
	_, err := rsp.Recv()
	suite.Require().Error(err)
	st, ok := status.FromError(err)
	suite.Require().True(ok)
	suite.Equal(codes.InvalidArgument, st.Code())
	suite.Equal(message, st.Message())
	suite.marketdata.AssertNotCalled(suite.T(), "GetAsset", mock.Anything, mock.Anything)
	suite.marketdata.AssertNotCalled(suite.T(), "GetOHLC", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	var assets, bars int
	suite.Require().NoError(suite.pgx.QueryRow(context.Background(),
		`SELECT (SELECT count(*) FROM asset), (SELECT count(*) FROM ohlc)`).Scan(&assets, &bars))
	suite.Zero(assets, "nothing may be stored for a refused download")
	suite.Zero(bars, "nothing may be stored for a refused download")
}

// acceptAll lets every GetAsset fail, so an accepted download reports each
// symbol once without storing anything.
func (suite *MarketdataTest) acceptAll() {
	suite.marketdata.On("GetAsset", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
}

func (suite *MarketdataTest) TestDownloadSymbolCapIsTenThousand() {
	suite.Equal(10_000, marketdata.MaxDownloadSymbols)
}

const overCapMessage = "one download can carry at most 10,000 distinct symbols, and this one has more. " +
	"Split it into smaller downloads."

// One distinct symbol over the cap is refused before any provider call.
func (suite *MarketdataTest) TestDownloadHistoricalDataRefusesOneOverTheCap() {
	rsp := suite.download(distinctSymbols(marketdata.MaxDownloadSymbols + 1))
	suite.requireRefused(rsp, overCapMessage)
}

// Exactly the cap is allowed, and repeats on top don't count.
func (suite *MarketdataTest) TestDownloadHistoricalDataAllowsExactlyTheCap() {
	symbols := distinctSymbols(marketdata.MaxDownloadSymbols)
	suite.acceptAll()
	rsp := suite.download(append(symbols, symbols[0], symbols[1]))
	suite.Len(suite.recvAll(rsp), marketdata.MaxDownloadSymbols)
}

// More than 20,000 raw symbols is refused by the request's validation rule
// before the service reads the list, even when they are all repeats.
func (suite *MarketdataTest) TestDownloadHistoricalDataRefusesTooLongARawList() {
	symbols := make([]string, 20_001)
	for i := range symbols {
		symbols[i] = "AAPL"
	}
	_, err := suite.download(symbols).Recv()
	suite.Require().Error(err)
	suite.Equal(codes.InvalidArgument, status.Code(err))
	suite.marketdata.AssertNotCalled(suite.T(), "GetAsset", mock.Anything, mock.Anything)
}

// A blank symbol is refused by its position with the invalid-ticker
// sentence, not sent to the provider as a per-symbol STATUS_ERROR.
func (suite *MarketdataTest) TestDownloadHistoricalDataRefusesABlankSymbol() {
	for _, blank := range []string{"", "   "} {
		rsp := suite.download([]string{"AAPL", "MSFT", blank, "AAPL"})
		suite.requireRefused(rsp,
			"symbol 3 of 4 isn't a valid ticker (use 1 to 32 letters, digits, or . ^ = _ -)")
	}
}

// The provider fetch gets the request ctx: when the client cancels the
// download, the fetch in flight sees its ctx end and stops.
func (suite *MarketdataTest) TestDownloadHistoricalDataCancelStopsTheFetch() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	stopped := make(chan error, 1)
	suite.marketdata.On("GetAsset", mock.Anything, "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", mock.Anything, mock.Anything).
		Return(func(fetchCtx context.Context, _ string, _ time.Time, _ *time.Time) ([]*pb.OHLC, error) {
			close(entered)
			select {
			case <-fetchCtx.Done():
				stopped <- fetchCtx.Err()
			case <-time.After(10 * time.Second):
				stopped <- errors.New("the fetch ctx never ended")
			}
			return nil, fetchCtx.Err()
		}).Once()

	rsp, err := suite.client.DownloadHistoricalData(ctx, &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"AAPL"},
		StartDate: &internal_pb.Date{Year: 2024, Month: 6, Day: 3},
		Provider:  "yahoo",
	})
	suite.Require().NoError(err)
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		suite.FailNow("the fetch never started")
	}
	cancel()
	suite.Require().ErrorIs(<-stopped, context.Canceled)
	// The stream ends with the client's cancel, not with a result.
	for {
		if _, err := rsp.Recv(); err != nil {
			suite.Require().NotErrorIs(err, io.EOF, "a cancelled download must not end cleanly")
			break
		}
	}
}
