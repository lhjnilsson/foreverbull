package servicer

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internal_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type FinanceServerTest struct {
	suite.Suite

	pgx        *pgxpool.Pool
	marketdata supplier.Marketdata

	listener *bufconn.Listener
	server   *grpc.Server
	client   pb.FinanceClient
}

func TestFinanceServerTest(t *testing.T) {
	suite.Run(t, new(FinanceServerTest))
}

func (suite *FinanceServerTest) SetupSuite() {
	test_helper.SetupEnvironment(suite.T(), &test_helper.Containers{
		Postgres: true,
	})

}

func (suite *FinanceServerTest) TearDownSuite() {
}

func (suite *FinanceServerTest) SetupTest() {
	var err error
	suite.pgx, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.Require().NoError(err)
	suite.Require().NoError(repository.Recreate(context.TODO(), suite.pgx))
	suite.marketdata, err = marketdata.NewYahooClient()
	suite.Require().NoError(err)

	suite.listener = bufconn.Listen(1024 * 1024)
	suite.server = grpc.NewServer()
	server := NewFinanceServer(suite.pgx, suite.marketdata)
	pb.RegisterFinanceServer(suite.server, server)
	go func() {
		suite.server.Serve(suite.listener)
	}()

	conn, err := grpc.DialContext(context.Background(), "",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return suite.listener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}
	suite.client = pb.NewFinanceClient(conn)
}

func (suite *FinanceServerTest) TestGetAsset() {
	req := &pb.GetAssetRequest{Symbol: "AAPL"}
	rsp, err := suite.client.GetAsset(context.Background(), req)
	suite.Require().NoError(err)
	suite.Require().NotNil(rsp)
	suite.Equal("AAPL", rsp.Asset.Symbol)
	suite.Equal("Apple Inc.", rsp.Asset.Name)
}

func (suite *FinanceServerTest) TestGetIndex() {
	req := &pb.GetIndexRequest{Symbol: "^GDAXI"}
	rsp, err := suite.client.GetIndex(context.Background(), req)
	suite.Require().NoError(err)
	suite.Require().NotNil(rsp)
	suite.Greater(len(rsp.Assets), 0)
}

func (suite *FinanceServerTest) TestDownloadHistoricalData() {
	req := &pb.DownloadHistoricalDataRequest{
		Symbol:    "^GDAXI",
		StartDate: &internal_pb.Date{Year: 2020, Month: 1, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2024, Month: 06, Day: 30},
	}
	_, err := suite.client.DownloadHistoricalData(context.Background(), req)
	suite.Require().NoError(err)
}