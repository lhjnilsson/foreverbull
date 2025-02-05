package servicer

import (
	"context"
	"log"
	"net"
	"testing"

	common_pb "github.com/lhjnilsson/foreverbull/pkg/pb"

	internalGrpc "github.com/lhjnilsson/foreverbull/internal/grpc"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	service_pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

type StrategyServerTest struct {
	suite.Suite

	marketdata *finance_pb.MockMarketdataClient

	listener *bufconn.Listener
	server   *grpc.Server
	client   pb.StrategyServicerClient
}

func TestStrategyServer(t *testing.T) {
	suite.Run(t, new(StrategyServerTest))
}

func (test *StrategyServerTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{})
}

func (test *StrategyServerTest) SetupTest() {
	var err error
	test.listener = bufconn.Listen(1024 * 1024)
	test.server, err = internalGrpc.NewServer()
	test.Require().NoError(err)

	test.marketdata = finance_pb.NewMockMarketdataClient(test.T())

	server := NewStrategyServer(test.marketdata)
	pb.RegisterStrategyServicerServer(test.server, server)

	go func() {
		test.NoError(test.server.Serve(test.listener))
	}()

	resolver.SetDefaultScheme("passthrough")

	conn, err := grpc.NewClient(test.listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return test.listener.Dial()
		}),
	)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}
	test.client = pb.NewStrategyServicerClient(conn)
}

func (test *StrategyServerTest) TestRunStrategy() {
	test.marketdata.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(nil, nil)

	req := &pb.RunStrategyRequest{
		Symbols:   []string{"AAPL"},
		StartDate: &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
		Algorithm: &service_pb.Algorithm{},
	}

	stream, err := test.client.RunStrategy(context.Background(), req)
	test.Require().NoError(err)
	for {
		msg, err := stream.Recv()
		if err != nil {
			break
		}
		test.T().Log(msg)
	}
}
