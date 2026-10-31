package api_test

import (
	"context"
	"errors"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// waitForCtx blocks a provider call until its ctx ends and reports the ctx
// error on stopped, or an error after 10s if the ctx never ends, so a call
// that doesn't get the request ctx fails the test quickly.
func waitForCtx(ctx context.Context, entered chan<- struct{}, stopped chan<- error) error {
	close(entered)
	select {
	case <-ctx.Done():
		stopped <- ctx.Err()
	case <-time.After(10 * time.Second):
		stopped <- errors.New("the provider call's ctx never ended")
	}
	return ctx.Err()
}

// cancelDuring starts a download of symbols from start through end, cancels
// it once a provider call has entered, and checks that call saw its ctx end.
func (suite *MarketdataTest) cancelDuring(symbols []string, start, end *internal_pb.Date,
	entered <-chan struct{}, stopped <-chan error,
) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	rsp, err := suite.client.DownloadHistoricalData(ctx, &servicePb.DownloadHistoricalDataRequest{
		Symbols: symbols, StartDate: start, EndDate: end, Provider: "yahoo",
	})
	suite.Require().NoError(err)
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		suite.FailNow("the provider call never started")
	}
	cancel()
	select {
	case err := <-stopped:
		suite.Require().ErrorIs(err, context.Canceled)
	case <-time.After(15 * time.Second):
		suite.FailNow("the provider call never returned")
	}
	for {
		if _, err := rsp.Recv(); err != nil {
			break
		}
	}
}

// The asset lookup gets the request ctx: a cancelled download stops it.
func (suite *MarketdataTest) TestDownloadHistoricalDataCancelStopsTheAssetLookup() {
	entered := make(chan struct{})
	stopped := make(chan error, 1)
	suite.marketdata.On("GetAsset", mock.Anything, "AAPL").
		Return(func(ctx context.Context, _ string) (*pb.Asset, error) {
			return nil, waitForCtx(ctx, entered, stopped)
		}).Once()

	suite.cancelDuring([]string{"AAPL"}, &internal_pb.Date{Year: 2024, Month: 6, Day: 3}, nil, entered, stopped)
}

// A fetch outside the span already fetched (here the incremental fetch from
// the last stored day, which is before the request's start) goes to the
// provider with the request ctx too.
func (suite *MarketdataTest) TestDownloadHistoricalDataCancelStopsTheRefetch() {
	ctx := context.Background()
	stored := time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)
	suite.Require().NoError((&repository.Asset{Conn: suite.pgx}).Store(ctx, "AAPL", "Apple Inc."))
	suite.Require().NoError((&repository.OHLC{Conn: suite.pgx}).Store(ctx, "yahoo", "AAPL", stored, 100, 100, 100, 100, 10))

	entered := make(chan struct{})
	stopped := make(chan error, 1)
	suite.marketdata.On("GetAsset", mock.Anything, "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetOHLC", mock.Anything, "AAPL", mock.Anything, mock.Anything).
		Return(func(ctx context.Context, _ string, from time.Time, _ *time.Time) ([]*pb.OHLC, error) {
			if from.UTC().Format(time.DateOnly) == "2024-06-04" {
				return []*pb.OHLC{{
					Timestamp: timestamppb.New(time.Date(2024, 6, 4, 0, 0, 0, 0, time.UTC)),
					Open:      100, High: 100, Low: 100, Close: 100, Volume: 10,
				}}, nil
			}
			return nil, waitForCtx(ctx, entered, stopped)
		})

	suite.cancelDuring([]string{"AAPL"}, &internal_pb.Date{Year: 2024, Month: 6, Day: 4},
		&internal_pb.Date{Year: 2024, Month: 6, Day: 5}, entered, stopped)
}
