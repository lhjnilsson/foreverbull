package api_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internalGrpc "github.com/lhjnilsson/foreverbull/internal/grpc"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MarketdataTest struct {
	suite.Suite

	pgx        *pgxpool.Pool
	marketdata *supplier.MockMarketdata

	listener *bufconn.Listener
	server   *grpc.Server
	client   servicePb.MarketdataClient
}

func TestMarketData(t *testing.T) {
	suite.Run(t, new(MarketdataTest))
}

func (suite *MarketdataTest) SetupSuite() {
	test_helper.SetupEnvironment(suite.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (suite *MarketdataTest) TearDownSuite() {
}

func (suite *MarketdataTest) SetupTest() {
	var err error
	suite.pgx, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.Require().NoError(err)
	suite.Require().NoError(repository.Recreate(context.TODO(), suite.pgx))
	suite.marketdata = supplier.NewMockMarketdata(suite.T())

	suite.listener = bufconn.Listen(1024 * 1024)
	suite.server, err = internalGrpc.NewServer()
	suite.Require().NoError(err)
	server := api.NewMarketdataServer(suite.pgx, suite.marketdata)
	servicePb.RegisterMarketdataServer(suite.server, server)

	go func() {
		suite.NoError(suite.server.Serve(suite.listener))
	}()

	resolver.SetDefaultScheme("passthrough")

	conn, err := grpc.NewClient(suite.listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return suite.listener.Dial()
		}),
	)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	suite.client = servicePb.NewMarketdataClient(conn)
}

func (suite *MarketdataTest) TestGetAsset() {
	suite.marketdata.On("GetAsset", "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)

	req := &servicePb.GetAssetRequest{Identifier: &servicePb.GetAssetRequest_Symbol{Symbol: "AAPL"}}
	rsp, err := suite.client.GetAsset(context.Background(), req)
	suite.Require().NoError(err)
	suite.Require().NotNil(rsp)
	suite.Equal("AAPL", rsp.Asset.Symbol)
	suite.Equal("Apple Inc.", rsp.Asset.Name)
}

func (suite *MarketdataTest) TestGetIndex() {
	suite.marketdata.On("GetIndex", "^GDAXI").Return([]*pb.Asset{
		{Symbol: "ADS.DE", Name: "Adidas AG"},
	}, nil)

	req := &servicePb.GetIndexRequest{Symbol: "^GDAXI"}
	rsp, err := suite.client.GetIndex(context.Background(), req)
	suite.Require().NoError(err)
	suite.Require().NotNil(rsp)
	suite.NotEmpty(rsp.Assets)
}

func (suite *MarketdataTest) TestDownloadHistoricalData() {
	suite.marketdata.On("GetAsset", "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetAsset", "NOCORP").Return(nil, errors.New("not found"))
	suite.marketdata.On("GetOHLC", "AAPL", mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{
			Timestamp: timestamppb.Now(),
			Open:      1,
			High:      2,
			Low:       0.5,
			Close:     1.5,
			Volume:    100,
		},
	}, nil)

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"AAPL", "NOCORP"},
		StartDate: &internal_pb.Date{Year: 2020, Month: 1, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2024, Month: 06, Day: 30},
		// yahoo routes GetOHLC to the injected mock.
		Provider: "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), req)
	suite.Require().NoError(err)

	result := make(map[string]servicePb.DownloadHistoricalDataResponse_Status)
	for {
		msg, err := rsp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			suite.Require().NoError(err)
			break
		}
		suite.Require().NoError(err)
		result[msg.GetSymbol()] = msg.GetStatus()
	}
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_OK, result["AAPL"])
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, result["NOCORP"])
}

func (suite *MarketdataTest) TestDownloadAssetsNoEndDate() {
	suite.marketdata.On("GetAsset", "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetAsset", "NOCORP").Return(nil, errors.New("not found"))
	suite.marketdata.On("GetOHLC", "AAPL", mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{
			Timestamp: timestamppb.Now(),
			Open:      1,
			High:      2,
			Low:       0.5,
			Close:     1.5,
			Volume:    100,
		},
	}, nil)

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"AAPL", "NOCORP"},
		StartDate: &internal_pb.Date{Year: 2020, Month: 1, Day: 1},
		// yahoo routes GetOHLC to the injected mock.
		Provider: "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), req)
	suite.Require().NoError(err)

	result := make(map[string]servicePb.DownloadHistoricalDataResponse_Status)
	for {
		msg, err := rsp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			suite.Require().NoError(err)
			break
		}
		suite.Require().NoError(err)
		result[msg.GetSymbol()] = msg.GetStatus()
	}
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_OK, result["AAPL"])
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, result["NOCORP"])
}

func (suite *MarketdataTest) TestDownloadHistoricalDataStopsOnBlockedProvider() {
	symbols := []string{"AAPL", "MSFT", "NVDA", "AMZN", "GOOG", "META", "TSLA", "AVGO", "COST", "NFLX", "AMD", "PEP"}
	for _, symbol := range symbols {
		suite.marketdata.On("GetAsset", symbol).Return(&pb.Asset{Symbol: symbol, Name: symbol}, nil).Maybe()
		suite.marketdata.On("GetOHLC", symbol, mock.Anything, mock.Anything).
			Return(nil, &marketdata.BlockedError{Provider: "yahoo", Symbol: symbol}).Maybe()
	}

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   symbols,
		StartDate: &internal_pb.Date{Year: 2020, Month: 1, Day: 1},
		// yahoo routes GetOHLC to the injected mock.
		Provider: "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), req)
	suite.Require().NoError(err)

	var streamErr error
	responses := 0
	for {
		msg, err := rsp.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				streamErr = err
			}
			break
		}
		responses++
		suite.NotEqual(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msg.GetStatus(),
			"%s was reported as failed, which disables it", msg.GetSymbol())
	}
	suite.Zero(responses, "a blocked symbol must not be reported per symbol")

	st, ok := status.FromError(streamErr)
	suite.Require().True(ok, "want a gRPC status, got %v", streamErr)
	suite.Equal(codes.FailedPrecondition, st.Code())
	suite.Regexp(`^Yahoo blocked the download for [A-Z]+ \(it returned a bot-check page instead of data\)\. Retry with --provider stooq\.$`, st.Message())
}
