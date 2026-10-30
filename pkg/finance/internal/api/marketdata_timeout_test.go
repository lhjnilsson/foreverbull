package api_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func june2024Bar() []*pb.OHLC {
	return []*pb.OHLC{{
		Timestamp: timestamppb.New(time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)),
		Open:      1, High: 2, Low: 0.5, Close: 1.5, Volume: 100,
	}}
}

func june2024Request(symbols ...string) *servicePb.DownloadHistoricalDataRequest {
	return &servicePb.DownloadHistoricalDataRequest{
		Symbols:   symbols,
		StartDate: &internal_pb.Date{Year: 2024, Month: 6, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2024, Month: 6, Day: 7},
		// yahoo routes GetOHLC to the injected mock.
		Provider: "yahoo",
	}
}

// The download's time limit stops the work: the symbol whose provider never
// answers is cut off and named in a DeadlineExceeded status with the
// time-limit text, and the symbol
// that finished before the limit is reported and stays stored.
func (suite *MarketdataTest) TestDownloadHistoricalDataStopsAtItsTimeLimit() {
	suite.T().Setenv(environment.FinanceTaskTimeLimit, "2s")
	ctx := context.Background()
	for _, symbol := range []string{"AAPL", "SLOW"} {
		suite.marketdata.On("GetAsset", mock.Anything, symbol).Return(&pb.Asset{Symbol: symbol, Name: symbol}, nil)
	}
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", mock.Anything, mock.Anything).Return(june2024Bar(), nil)
	suite.marketdata.On("GetOHLC", mock.Anything, "SLOW", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			callCtx, _ := args.Get(0).(context.Context)
			<-callCtx.Done()
		}).Return(nil, context.DeadlineExceeded)

	began := time.Now()
	rsp, err := suite.client.DownloadHistoricalData(ctx, june2024Request("AAPL", "SLOW"))
	suite.Require().NoError(err)

	var streamErr error
	reported := make([]string, 0)
	for {
		msg, err := rsp.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				streamErr = err
			}
			break
		}
		suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_OK, msg.GetStatus(), msg.GetError())
		reported = append(reported, msg.GetSymbol())
	}
	suite.Less(time.Since(began), 10*time.Second, "the limit must end the download")
	suite.Equal([]string{"AAPL"}, reported)

	st, ok := status.FromError(streamErr)
	suite.Require().True(ok, "want a gRPC status, got %v", streamErr)
	suite.Equal(codes.DeadlineExceeded, st.Code())
	// The limit has its own text, not the provider-timeout sentence.
	suite.Equal("the download hit its time limit of 2s at SLOW", st.Message())

	ohlc := repository.OHLC{Conn: suite.pgx}
	kept, err := ohlc.List(ctx, "yahoo", []string{"AAPL"}, mustTime("2024-06-07"), 0)
	suite.Require().NoError(err)
	suite.Len(kept, 1, "a symbol finished before the limit stays stored")
	cut, err := ohlc.List(ctx, "yahoo", []string{"SLOW"}, mustTime("2024-06-07"), 0)
	suite.Require().NoError(err)
	suite.Empty(cut)
}

// A provider call that times out while the download still has time fails
// that symbol with the timeout sentence, not the generic download failure.
// The asset lookup is always Yahoo, whatever the price provider.
func (suite *MarketdataTest) TestDownloadHistoricalDataReportsAProviderTimeout() {
	suite.marketdata.On("GetAsset", mock.Anything, "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("error getting bars: %w", context.DeadlineExceeded))
	suite.marketdata.On("GetAsset", mock.Anything, "MSFT").
		Return(nil, fmt.Errorf("error: %w", context.DeadlineExceeded))

	rsp, err := suite.client.DownloadHistoricalData(context.Background(), june2024Request("AAPL", "MSFT"))
	suite.Require().NoError(err)
	got := map[string]string{}
	for _, msg := range suite.recvAll(rsp) {
		suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msg.GetStatus())
		got[msg.GetSymbol()] = msg.GetError()
	}
	suite.Equal(map[string]string{
		"AAPL": "yahoo took too long to respond for AAPL",
		"MSFT": "yahoo took too long to respond for MSFT",
	}, got)
}

// A refetch from Sync that runs out of time is the provider's delay, not
// "couldn't store". Under the symbol lock Alpaca's fetch budget ends before
// the transaction's idle timeout, so this is the error that refetch gives.
func (suite *MarketdataTest) TestDownloadHistoricalDataReportsASyncRefetchTimeout() {
	ctx := context.Background()
	assets := repository.Asset{Conn: suite.pgx}
	ohlc := repository.OHLC{Conn: suite.pgx}
	suite.Require().NoError(assets.Store(ctx, "AAPL", "Apple Inc."))
	// A stored day before the request start makes Sync fetch from that day,
	// outside the span the download fetched first, so it asks the provider.
	suite.Require().NoError(ohlc.Store(ctx, "yahoo", "AAPL", mustTime("2024-05-31"), 1, 2, 0.5, 1.5, 100))

	suite.marketdata.On("GetAsset", mock.Anything, "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", mustTime("2024-06-01"), mock.Anything).
		Return(june2024Bar(), nil).Once()
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", mustTime("2024-05-31"), mock.Anything).
		Return(nil, fmt.Errorf("error getting bars: %w", context.DeadlineExceeded)).Once()

	rsp, err := suite.client.DownloadHistoricalData(ctx, june2024Request("AAPL"))
	suite.Require().NoError(err)
	msgs := suite.recvAll(rsp)
	suite.Require().Len(msgs, 1)
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msgs[0].GetStatus())
	suite.Equal("yahoo took too long to respond for AAPL", msgs[0].GetError())

	kept, err := ohlc.List(ctx, "yahoo", []string{"AAPL"}, mustTime("2024-06-07"), 0)
	suite.Require().NoError(err)
	suite.Len(kept, 1, "the stored day stays as it was")
}
