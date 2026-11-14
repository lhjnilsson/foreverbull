package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internalGrpc "github.com/lhjnilsson/foreverbull/internal/grpc"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	streamPb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	commonPb "github.com/lhjnilsson/foreverbull/pkg/pb"
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
)

type TradingServerTest struct {
	suite.Suite

	pgx     *pgxpool.Pool
	trading *supplier.MockTrading
	stream  *stream.MockStream

	listener *bufconn.Listener
	server   *grpc.Server
	client   servicePb.TradingServicerClient
}

func TestTradingServer(t *testing.T) {
	suite.Run(t, new(TradingServerTest))
}

func (suite *TradingServerTest) SetupSuite() {
	test_helper.SetupEnvironment(suite.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (suite *TradingServerTest) TearDownSuite() {
}

func (suite *TradingServerTest) SetupSubTest() {
	var err error
	suite.pgx, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.Require().NoError(err)
	suite.Require().NoError(repository.Recreate(context.TODO(), suite.pgx))

	suite.trading = supplier.NewMockTrading(suite.T())

	suite.listener = bufconn.Listen(1024 * 1024)
	suite.server, err = internalGrpc.NewServer()
	suite.Require().NoError(err)
	suite.stream = new(stream.MockStream)
	server := api.NewTradingServer(suite.pgx, suite.stream, suite.trading)
	servicePb.RegisterTradingServicerServer(suite.server, server)

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

	suite.client = servicePb.NewTradingServicerClient(conn)
}

func (test *TradingServerTest) TestGetOrders() {
	test.Run("error getting orders", func() {
		test.trading.On("GetOrders", mock.Anything).Return(nil, errors.New("error getting orders"))

		req := &servicePb.GetOrdersRequest{}
		rsp, err := test.client.GetOrders(context.Background(), req)
		test.Require().Error(err)
		test.Require().Nil(rsp)
	})
	test.Run("normal", func() {
		orders := []*pb.Order{
			{
				Symbol: "DEMO",
				Amount: 10,
			},
		}
		test.trading.On("GetOrders", mock.Anything).Return(orders, nil)

		req := &servicePb.GetOrdersRequest{}
		rsp, err := test.client.GetOrders(context.Background(), req)
		test.Require().NoError(err)
		test.Require().NotNil(rsp)
		test.Len(rsp.Orders, 1)
	})
}

func (test *TradingServerTest) TestGetPortfolio() {
	test.Run("error getting portfolio", func() {
		test.trading.On("GetPortfolio", mock.Anything).Return(nil, errors.New("error getting portfolio"))

		req := &servicePb.GetPortfolioRequest{}
		rsp, err := test.client.GetPortfolio(context.Background(), req)
		test.Require().Error(err)
		test.Require().Nil(rsp)
	})
	test.Run("normal", func() {
		portfolio := &pb.Portfolio{}
		test.trading.On("GetPortfolio", mock.Anything).Return(portfolio, nil)

		req := &servicePb.GetPortfolioRequest{}
		rsp, err := test.client.GetPortfolio(context.Background(), req)
		test.Require().NoError(err)
		test.Require().NotNil(rsp)
	})
}

func (test *TradingServerTest) TestCreateSession() {
	test.Run("normal", func() {
		repository := repository.Trading{Conn: test.pgx}
		statusCh := make(chan *streamPb.Orchestration_Step)
		go func() {
			rsp := &servicePb.CreateSessionResponse{}
			statusCh <- &streamPb.Orchestration_Step{
				Num:   int32(rsp.CREATING.ProtoReflect().Descriptor().Index()),
				State: commonPb.State_STATE_COMPLETED,
			}
			sessions, err := repository.List(context.Background())
			test.NoError(err)
			test.Len(sessions, 1)
			test.NoError(repository.UpdatePort(context.Background(), sessions[0].Id, 1337))
			statusCh <- &streamPb.Orchestration_Step{
				Num:   int32(rsp.STARTING.ProtoReflect().Descriptor().Index()),
				State: commonPb.State_STATE_COMPLETED,
			}
			close(statusCh)
		}()
		test.stream.On("RunOrchestration", mock.Anything, mock.Anything).Return((<-chan *streamPb.Orchestration_Step)(statusCh)).
			Run(func(args mock.Arguments) {
				orchestration := args.Get(1).(*streamPb.Orchestration)
				test.Require().Len(orchestration.GetSteps(), 2)
				var ingest financeStream.IngestCommand
				test.Require().NoError(json.Unmarshal(orchestration.GetSteps()[0].GetMessages()[0].GetPayload(), &ingest))
				test.Equal("yahoo", ingest.Provider)
				test.True(ingest.Trading)
				var run financeStream.StartTradingSessionCommand
				test.Require().NoError(json.Unmarshal(orchestration.GetSteps()[1].GetMessages()[0].GetPayload(), &run))
				test.Equal("yahoo", run.Provider)
			})

		req := &servicePb.CreateSessionRequest{Provider: "yahoo"}
		rsp, err := test.client.CreateSession(context.Background(), req)
		test.Require().NoError(err)
		test.Require().NotNil(rsp)
		var session *pb.Session
		for {
			msg, err := rsp.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			test.Require().NoError(err)
			test.Require().NotNil(msg)
			session = msg.GetSession()
		}
		test.Require().NotNil(session)
		test.Require().NotNil(session.Port)
		test.Equal(int64(1337), *session.Port)
	})
}

func (test *TradingServerTest) TestCreateSessionRefusesAMissingOrUnknownProvider() {
	for _, tc := range []struct {
		name     string
		provider string
		want     string
	}{
		{name: "missing", provider: "", want: usercopy.TradingProviderMissing("")},
		{name: "blank", provider: "  ", want: usercopy.TradingProviderMissing("")},
		{name: "unknown", provider: "binance", want: usercopy.TradingProviderUnknown("binance", "")},
	} {
		test.Run(tc.name, func() {
			rsp, err := test.client.CreateSession(context.Background(), &servicePb.CreateSessionRequest{Provider: tc.provider})
			test.Require().NoError(err)
			_, err = rsp.Recv()
			test.Require().Error(err)
			st, ok := status.FromError(err)
			test.Require().True(ok)
			test.Equal(codes.FailedPrecondition, st.Code())
			test.Equal(tc.want, st.Message())
		})
	}
}

func (test *TradingServerTest) TestCreateSessionRefusesStooqWithoutAnExchange() {
	test.T().Setenv(environment.StooqExchange, "")
	rsp, err := test.client.CreateSession(context.Background(), &servicePb.CreateSessionRequest{Provider: "stooq"})
	test.Require().NoError(err)
	_, err = rsp.Recv()
	test.Require().Error(err)
	st, ok := status.FromError(err)
	test.Require().True(ok)
	test.Equal(codes.FailedPrecondition, st.Code())
	test.Equal(usercopy.StooqExchangeMissing, st.Message())

	for _, exchange := range []string{"L", "DE", "UK", "TO"} {
		test.T().Setenv(environment.StooqExchange, exchange)
		rsp, err = test.client.CreateSession(context.Background(), &servicePb.CreateSessionRequest{Provider: "stooq"})
		test.Require().NoError(err)
		_, err = rsp.Recv()
		st, ok = status.FromError(err)
		test.Require().True(ok)
		test.Equal(usercopy.StooqExchangeUnsupported(exchange), st.Message())
	}
}

func (test *TradingServerTest) TestCreateSessionForwardsEachConfiguredProvider() {
	for _, provider := range []string{"yahoo", "stooq"} {
		test.Run(provider, func() {
			if provider == "stooq" {
				test.T().Setenv(environment.StooqExchange, "US")
			}
			done := make(chan *streamPb.Orchestration_Step)
			close(done)
			test.stream.On("RunOrchestration", mock.Anything, mock.Anything).
				Return((<-chan *streamPb.Orchestration_Step)(done)).
				Run(func(args mock.Arguments) {
					orchestration := args.Get(1).(*streamPb.Orchestration)
					var ingest financeStream.IngestCommand
					test.Require().NoError(json.Unmarshal(orchestration.GetSteps()[0].GetMessages()[0].GetPayload(), &ingest))
					test.Equal(provider, ingest.Provider)
					test.True(ingest.Trading)
					var run financeStream.StartTradingSessionCommand
					test.Require().NoError(json.Unmarshal(orchestration.GetSteps()[1].GetMessages()[0].GetPayload(), &run))
					test.Equal(provider, run.Provider)
				}).Once()
			rsp, err := test.client.CreateSession(context.Background(), &servicePb.CreateSessionRequest{
				Provider: provider,
				Symbols:  []string{"AAPL"},
			})
			test.Require().NoError(err)
			for {
				_, err = rsp.Recv()
				if errors.Is(err, io.EOF) {
					break
				}
				test.Require().NoError(err)
			}
		})
	}
}

type TradingServerDisabledTest struct {
	suite.Suite

	listener *bufconn.Listener
	server   *grpc.Server
	client   servicePb.TradingServicerClient
}

func TestTradingServerDisabled(t *testing.T) {
	suite.Run(t, new(TradingServerDisabledTest))
}

func (test *TradingServerDisabledTest) SetupSuite() {
	test_helper.SetupEnv(test.T())
}

func (test *TradingServerDisabledTest) SetupSubTest() {
	var err error
	test.listener = bufconn.Listen(1024 * 1024)
	test.server, err = internalGrpc.NewServer()
	test.Require().NoError(err)
	server := api.NewTradingServer(nil, nil, nil)
	servicePb.RegisterTradingServicerServer(test.server, server)

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

	test.client = servicePb.NewTradingServicerClient(conn)
}

func (test *TradingServerDisabledTest) TestGetOrders() {
	test.Run("error getting orders", func() {
		req := &servicePb.GetOrdersRequest{}
		rsp, err := test.client.GetOrders(context.Background(), req)
		test.Require().Error(err)
		test.Require().ErrorContains(err, "trading is not enabled")
		test.Require().Nil(rsp)
	})
}

func (test *TradingServerDisabledTest) TestGetPortfolio() {
	test.Run("error getting portfolio", func() {
		req := &servicePb.GetPortfolioRequest{}
		rsp, err := test.client.GetPortfolio(context.Background(), req)
		test.Require().Error(err)
		test.Require().ErrorContains(err, "trading is not enabled")
		test.Require().Nil(rsp)
	})
}

func (test *TradingServerDisabledTest) TestCreateSession() {
	test.Run("error creating session", func() {
		req := &servicePb.CreateSessionRequest{}
		rsp, err := test.client.CreateSession(context.Background(), req)
		test.Require().NoError(err)
		test.Require().NotNil(rsp)

		msg, err := rsp.Recv()
		test.Require().Nil(msg)
		test.Require().Error(err)
		test.Require().ErrorContains(err, "trading is not enabled")
	})
}
