package api_test

import (
	"context"
	"io"
	"net"
	"testing"

	internalGrpc "github.com/lhjnilsson/foreverbull/internal/grpc"
	common_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	service_pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/api"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type StrategyServerTest struct {
	suite.Suite

	listener *bufconn.Listener
	server   *grpc.Server
	client   pb.StrategyServicerClient
}

func TestStrategyServer(t *testing.T) {
	suite.Run(t, new(StrategyServerTest))
}

func (test *StrategyServerTest) SetupTest() {
	var err error
	test.listener = bufconn.Listen(1024 * 1024)
	test.server, err = internalGrpc.NewServer()
	test.Require().NoError(err)

	pb.RegisterStrategyServicerServer(test.server, api.NewStrategyServer())

	go func() {
		test.NoError(test.server.Serve(test.listener))
	}()

	resolver.SetDefaultScheme("passthrough")

	conn, err := grpc.NewClient("bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return test.listener.Dial()
		}),
	)
	test.Require().NoError(err)
	test.client = pb.NewStrategyServicerClient(conn)
}

func (test *StrategyServerTest) TearDownTest() {
	if test.server != nil {
		test.server.Stop()
	}
	if test.listener != nil {
		test.NoError(test.listener.Close())
	}
}

func (test *StrategyServerTest) TestRunStrategyUnimplemented() {
	req := &pb.RunStrategyRequest{
		Symbols:   []string{"AAPL"},
		StartDate: &common_pb.Date{Year: 2024, Month: 1, Day: 1},
		Algorithm: &service_pb.Algorithm{},
	}

	stream, err := test.client.RunStrategy(context.Background(), req)
	test.Require().NoError(err)

	msg, err := stream.Recv()
	test.Nil(msg)
	test.Require().Error(err)
	test.Require().NotErrorIs(err, io.EOF)
	st, ok := status.FromError(err)
	test.True(ok)
	test.Equal(codes.Unimplemented, st.Code())
}
