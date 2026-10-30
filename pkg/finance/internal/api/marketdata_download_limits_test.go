package api_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/api"
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

// A request with more distinct symbols than the cap is refused before any
// provider call. Repeats don't count toward the cap.
func (suite *MarketdataTest) TestDownloadHistoricalDataCapsSymbols() {
	symbols := make([]string, 0, api.MaxDownloadSymbols+1)
	for i := 0; i <= api.MaxDownloadSymbols; i++ {
		symbols = append(symbols, fmt.Sprintf("S%04d", i))
	}
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), &servicePb.DownloadHistoricalDataRequest{
		Symbols:   symbols,
		StartDate: &internal_pb.Date{Year: 2024, Month: 6, Day: 3},
		Provider:  "yahoo",
	})
	suite.Require().NoError(err)
	_, err = rsp.Recv()
	suite.Require().Error(err)
	st, ok := status.FromError(err)
	suite.Require().True(ok)
	suite.Equal(codes.InvalidArgument, st.Code())
	suite.Equal(fmt.Sprintf("one download can carry at most %d symbols, and this one has %d. "+
		"Split it into smaller downloads.", api.MaxDownloadSymbols, api.MaxDownloadSymbols+1), st.Message())
	suite.marketdata.AssertNotCalled(suite.T(), "GetAsset", mock.Anything, mock.Anything)

	// Exactly the cap, with repeats on top, is allowed.
	allowed := append(append([]string{}, symbols[:api.MaxDownloadSymbols]...), symbols[0], symbols[1])
	suite.Require().Len(allowed, api.MaxDownloadSymbols+2)
	suite.marketdata.On("GetAsset", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
	rsp, err = suite.client.DownloadHistoricalData(context.Background(), &servicePb.DownloadHistoricalDataRequest{
		Symbols:   allowed,
		StartDate: &internal_pb.Date{Year: 2024, Month: 6, Day: 3},
		Provider:  "yahoo",
	})
	suite.Require().NoError(err)
	suite.Len(suite.recvAll(rsp), api.MaxDownloadSymbols)
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
			suite.False(errors.Is(err, io.EOF), "a cancelled download must not end cleanly")
			break
		}
	}
}
