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
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

type MarketdataTest struct {
	suite.Suite

	pgx        *pgxpool.Pool
	marketdata supplier.Marketdata

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
	suite.marketdata, err = marketdata.NewYahooClient()
	suite.Require().NoError(err)

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
	req := &servicePb.GetAssetRequest{Identifier: &servicePb.GetAssetRequest_Symbol{Symbol: "AAPL"}}
	rsp, err := suite.client.GetAsset(context.Background(), req)
	suite.Require().NoError(err)
	suite.Require().NotNil(rsp)
	suite.Equal("AAPL", rsp.Asset.Symbol)
	suite.Equal("Apple Inc.", rsp.Asset.Name)
}

func (suite *MarketdataTest) TestGetIndex() {
	req := &servicePb.GetIndexRequest{Symbol: "^GDAXI"}
	rsp, err := suite.client.GetIndex(context.Background(), req)
	suite.Require().NoError(err)
	suite.Require().NotNil(rsp)
	suite.NotEmpty(rsp.Assets)
}

func (suite *MarketdataTest) TestDownloadHistoricalData() {
	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"AAPL", "NOCORP"},
		StartDate: &internal_pb.Date{Year: 2020, Month: 1, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2024, Month: 06, Day: 30},
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
	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"AAPL", "NOCORP"},
		StartDate: &internal_pb.Date{Year: 2020, Month: 1, Day: 1},
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
