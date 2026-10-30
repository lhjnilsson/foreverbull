package api_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
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

// drain reads a download to its end: the responses, and the stream error
// unless it ended with EOF.
func (suite *MarketdataTest) drain(rsp servicePb.Marketdata_DownloadHistoricalDataClient) ([]*servicePb.DownloadHistoricalDataResponse, error) {
	var msgs []*servicePb.DownloadHistoricalDataResponse
	for {
		msg, err := rsp.Recv()
		if errors.Is(err, io.EOF) {
			return msgs, nil
		}
		if err != nil {
			return msgs, err
		}
		msgs = append(msgs, msg)
	}
}

// refusedWait is a provider call whose request slot was refused against
// deadline, wrapped the way net/http returns a transport error.
func refusedWait(deadline time.Time) error {
	return fmt.Errorf("error getting bars: %w", &url.Error{
		Op: "Get", URL: "https://query2.finance.yahoo.com/v8/finance/chart/AAPL",
		Err: &marketdata.WaitRefusedError{Deadline: deadline},
	})
}

// requireNoLimiterText checks that no limiter or x/time/rate text reaches the
// client, whatever sentence it gets instead.
func (suite *MarketdataTest) requireNoLimiterText(text string) {
	for _, raw := range []string{"rate:", "would exceed", "slot", "context deadline exceeded"} {
		suite.NotContains(text, raw)
	}
}

// A provider wait refused because its slot comes after the download's own
// time limit is the limit: no STATUS_ERROR with the provider sentence, and
// the stream ends with the time-limit text for that symbol.
func (suite *MarketdataTest) TestDownloadHistoricalDataTellsAWaitRefusedAtTheLimitAsTheLimit() {
	suite.T().Setenv(environment.FinanceTaskTimeLimit, "1s")
	suite.marketdata.On("GetAsset", mock.Anything, "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", mock.Anything, mock.Anything).Return(
		func(ctx context.Context, _ string, _ time.Time, _ *time.Time) ([]*pb.OHLC, error) {
			deadline, ok := ctx.Deadline()
			suite.Require().True(ok, "the download's ctx carries its limit")
			return nil, refusedWait(deadline)
		})

	rsp, err := suite.client.DownloadHistoricalData(context.Background(), june2024Request("AAPL"))
	suite.Require().NoError(err)
	msgs, streamErr := suite.drain(rsp)
	suite.Empty(msgs, "the symbol is not failed with the provider sentence")
	st, ok := status.FromError(streamErr)
	suite.Require().True(ok, "want a gRPC status, got %v", streamErr)
	suite.Equal(codes.DeadlineExceeded, st.Code())
	suite.Equal("the download hit its time limit of 1s at AAPL", st.Message())
	suite.requireNoLimiterText(st.Message())
}

// A provider wait refused against a shorter deadline, such as the HTTP
// client's 30 s, is the provider taking too long, with its own sentence.
func (suite *MarketdataTest) TestDownloadHistoricalDataTellsAWaitRefusedEarlierAsAProviderTimeout() {
	suite.marketdata.On("GetAsset", mock.Anything, "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", mock.Anything, mock.Anything).
		Return(nil, refusedWait(time.Now().Add(30*time.Second)))

	rsp, err := suite.client.DownloadHistoricalData(context.Background(), june2024Request("AAPL"))
	suite.Require().NoError(err)
	msgs, streamErr := suite.drain(rsp)
	suite.Require().NoError(streamErr)
	suite.Require().Len(msgs, 1)
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msgs[0].GetStatus())
	suite.Equal("yahoo took too long to respond for AAPL", msgs[0].GetError())
	suite.requireNoLimiterText(msgs[0].GetError())
}

// A symbol stuck on its symbol lock in the database when the time limit
// ends is told as the limit, not as the provider or a store failure.
func (suite *MarketdataTest) TestDownloadHistoricalDataTellsALockWaitAtTheLimitAsTheLimit() {
	suite.T().Setenv(environment.FinanceTaskTimeLimit, "1s")
	ctx := context.Background()
	holder, err := suite.pgx.Begin(ctx)
	suite.Require().NoError(err)
	defer holder.Rollback(ctx) //nolint:errcheck
	_, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ohlc:' || 'AAPL'))`)
	suite.Require().NoError(err)

	suite.marketdata.On("GetAsset", mock.Anything, "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", mock.Anything, mock.Anything).Return(june2024Bar(), nil)

	rsp, err := suite.client.DownloadHistoricalData(ctx, june2024Request("AAPL"))
	suite.Require().NoError(err)
	msgs, streamErr := suite.drain(rsp)
	suite.Empty(msgs)
	st, ok := status.FromError(streamErr)
	suite.Require().True(ok, "want a gRPC status, got %v", streamErr)
	suite.Equal(codes.DeadlineExceeded, st.Code())
	suite.Equal("the download hit its time limit of 1s at AAPL", st.Message())
}

// A split moves the stored scale: 08-26 and 08-27 are stored at the
// pre-split closes, the download's overlap day no longer matches, so Sync
// refetches from 08-26. The time limit ends during that refetch. Nothing
// changes: both rows keep their old closes, none is deleted, and 08-28 is
// not added on the new scale next to them.
func (suite *MarketdataTest) TestDownloadHistoricalDataLimitDuringASplitRefetchChangesNothing() {
	suite.T().Setenv(environment.FinanceTaskTimeLimit, "2s")
	ctx := context.Background()
	assets := repository.Asset{Conn: suite.pgx}
	ohlc := repository.OHLC{Conn: suite.pgx}
	suite.Require().NoError(assets.Store(ctx, "AAPL", "Apple Inc."))
	bar := func(day string, close float64) *pb.OHLC {
		return &pb.OHLC{Timestamp: timestamppb.New(mustTime(day)), Open: close, High: close, Low: close, Close: close, Volume: 100}
	}
	suite.Require().NoError(ohlc.Store(ctx, "yahoo", "AAPL", mustTime("2020-08-26"), 506.09, 506.09, 506.09, 506.09, 100))
	suite.Require().NoError(ohlc.Store(ctx, "yahoo", "AAPL", mustTime("2020-08-27"), 500.04, 500.04, 500.04, 500.04, 100))

	onDay := func(day string) interface{} {
		return mock.MatchedBy(func(t time.Time) bool { return t.UTC().Format(time.DateOnly) == day })
	}
	suite.marketdata.On("GetAsset", mock.Anything, "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", onDay("2020-08-27"), mock.Anything).
		Return([]*pb.OHLC{bar("2020-08-27", 125.01), bar("2020-08-28", 124.8075)}, nil).Once()
	refetched := make(chan struct{})
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", onDay("2020-08-26"), mock.Anything).Return(
		func(ctx context.Context, _ string, _ time.Time, _ *time.Time) ([]*pb.OHLC, error) {
			close(refetched)
			<-ctx.Done()
			return nil, fmt.Errorf("error getting bars: %w", ctx.Err())
		}).Once()

	rsp, err := suite.client.DownloadHistoricalData(ctx, &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"AAPL"},
		StartDate: &internal_pb.Date{Year: 2020, Month: 8, Day: 27},
		EndDate:   &internal_pb.Date{Year: 2020, Month: 8, Day: 28},
		Provider:  "yahoo",
	})
	suite.Require().NoError(err)
	msgs, streamErr := suite.drain(rsp)
	select {
	case <-refetched:
	default:
		suite.Fail("the split refetch from 08-26 never ran")
	}
	suite.Empty(msgs)
	st, ok := status.FromError(streamErr)
	suite.Require().True(ok, "want a gRPC status, got %v", streamErr)
	suite.Equal(codes.DeadlineExceeded, st.Code())
	suite.Equal("the download hit its time limit of 2s at AAPL", st.Message())

	bars, err := ohlc.List(ctx, "yahoo", []string{"AAPL"}, mustTime("2020-08-31"), 0)
	suite.Require().NoError(err)
	closes := map[string]float64{}
	for _, b := range bars {
		closes[b.GetTimestamp().AsTime().UTC().Format(time.DateOnly)] = b.GetClose()
	}
	suite.Equal(map[string]float64{"2020-08-26": 506.09, "2020-08-27": 500.04}, closes,
		"both rows keep the old scale, none is deleted and 08-28 is not added")
}
