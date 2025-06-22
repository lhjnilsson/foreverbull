package api_test

import (
	"context"
	"net"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

type SessionTest struct {
	suite.Suite

	listener *bufconn.Listener
	server   *grpc.Server
	client   servicePb.SessionServicerClient
	activity <-chan bool

	mockWorkers *worker.MockPool
	mockTrading *supplier.MockTrading
}

func (test *SessionTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{})
}

func (test *SessionTest) SetupTest() {
	var err error
	test.mockWorkers = new(worker.MockPool)
	test.mockTrading = new(supplier.MockTrading)

	test.listener = bufconn.Listen(1024 * 1024)
	test.server, test.activity, err = api.NewGRPCSessionServer(test.mockTrading)
	test.Require().NoError(err)
	go func() {
		if err := test.server.Serve(test.listener); err != nil {
			test.T().Logf("error serving server: %v", err)
		}
	}()

	resolver.SetDefaultScheme("passthrough")

	conn, err := grpc.NewClient(test.listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return test.listener.Dial()
		}),
	)
	test.Require().NoError(err)
	test.client = servicePb.NewSessionServicerClient(conn)
}

func (test *SessionTest) TearDownTest() {
	err := test.listener.Close()
	if err != nil {
		test.T().Errorf("Error closing listener: %v", err)
	}
	test.server.Stop()
}

func TestSession(t *testing.T) {
	suite.Run(t, new(SessionTest))
}
