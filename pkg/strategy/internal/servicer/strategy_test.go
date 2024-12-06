package servicer

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/strategy/pb"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
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
	test.listener = bufconn.Listen(1024 * 1024)
	test.server = grpc.NewServer()
	server := NewStrategyServer()
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
	req := &pb.RunStrategyRequest{}

	rsp, err := test.client.RunStrategy(context.Background(), req)
	test.Require().NoError(err)
	test.Require().NotNil(rsp)
}
