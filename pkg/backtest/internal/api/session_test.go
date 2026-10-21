package api_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	service_pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

type SessionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	listener   *bufconn.Listener
	baseServer *grpc.Server
	activity   <-chan bool

	mockEngine        *engine.MockEngine
	mockEngineSession *engine.MockEngineSession
	client            servicePb.SessionServicerClient

	backtest *backtest.Backtest
	session  *backtest.Session
}

func TestSessionManual(t *testing.T) {
	suite.Run(t, new(SessionTest))
}

func (s *SessionTest) SetupSuite() {
	test_helper.SetupEnvironment(s.T(), &test_helper.Containers{
		Postgres: true,
	})

	var err error
	s.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	s.Require().NoError(err)
}

func (s *SessionTest) TearDownSuite() {
}

func (s *SessionTest) SetupTest() {
	err := repository.Recreate(context.Background(), s.conn)
	s.Require().NoError(err)

	backtests := repository.Backtest{Conn: s.conn}
	backtest := backtest.Backtest{
		Name: "backtest",
		Periods: []*backtest.Backtest_Period{
			{StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1}, EndDate: &pb.Date{Year: 2024, Month: 12, Day: 31}},
		},
	}
	s.backtest, err = backtests.Create(context.Background(), &backtest)
	s.Require().NoError(err)
	sessions := repository.Session{Conn: s.conn}
	s.session, err = sessions.Create(context.TODO(), "backtest")
	s.Require().NoError(err)

	s.listener = bufconn.Listen(1024 * 1024)

	s.mockEngine = new(engine.MockEngine)
	s.mockEngineSession = new(engine.MockEngineSession)
	s.mockEngine.On("NewSession", mock.Anything, mock.Anything).Return(s.mockEngineSession, nil)

	s.baseServer, s.activity, err = api.NewGRPCSessionServer(s.session, s.conn, s.mockEngine)
	s.Require().NoError(err)

	go func() {
		if err := s.baseServer.Serve(s.listener); err != nil {
			log.Printf("error serving server: %v", err)
		}
	}()

	resolver.SetDefaultScheme("passthrough")

	conn, err := grpc.NewClient(s.listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return s.listener.Dial()
		}),
	)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	s.client = servicePb.NewSessionServicerClient(conn)
}

func (s *SessionTest) TearDownTest() {
	err := s.listener.Close()
	if err != nil {
		s.Failf("error closing listener: %s", err.Error())
	}

	s.baseServer.Stop()
}

func (s *SessionTest) TestCreateExecution() {
	rsp, err := s.client.CreateExecution(context.Background(), &servicePb.CreateExecutionRequest{
		Id:        "execution_123",
		Session:   s.session.Id,
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2024, Month: 1, Day: 1},
		Algorithm: &service_pb.Algorithm{},
	})
	s.Require().NoError(err)
	s.Require().NotNil(rsp)
	select {
	case <-s.activity:
	case <-time.After(5 * time.Second):
		s.Require().Fail("timeout waiting for activity")
	}
}

func (s *SessionTest) TestCreateExecutionStoresUsedArguments() {
	rsp, err := s.client.CreateExecution(context.Background(), &servicePb.CreateExecutionRequest{
		Session:   s.session.Id,
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 2},
		EndDate:   &pb.Date{Year: 2024, Month: 6, Day: 30},
		Algorithm: &service_pb.Algorithm{
			Functions: []*service_pb.Algorithm_Function{
				{
					Name: "rebalance",
					Parameters: []*service_pb.Algorithm_FunctionParameter{
						{Key: "fastperiod", Value: strp("12"), DefaultValue: strp("10"), ValueType: "int"},
						{Key: "slowperiod", DefaultValue: strp("26"), ValueType: "int"},
						{Key: "signalperiod", ValueType: "int"},
						{Key: "oversold", Value: strp(""), DefaultValue: strp("30"), ValueType: "int"},
					},
				},
				{Name: "screen"},
			},
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(rsp.GetExecution())

	select {
	case <-s.activity:
	case <-time.After(5 * time.Second):
		s.Require().Fail("timeout waiting for activity")
	}

	want := []*backtest.ExecutionFunction{
		{
			Name: "rebalance",
			Parameters: []*backtest.ExecutionFunction_Parameter{
				{Key: "fastperiod", Value: "12", ValueType: "int"},
				{Key: "slowperiod", Value: "26", ValueType: "int"},
				{Key: "signalperiod", Value: "", ValueType: "int"},
				{Key: "oversold", Value: "", ValueType: "int"},
			},
		},
		{Name: "screen", Parameters: []*backtest.ExecutionFunction_Parameter{}},
	}

	executions := repository.Execution{Conn: s.conn}
	stored, err := executions.GetFunctions(context.Background(), rsp.Execution.Id)
	s.Require().NoError(err)
	s.Len(stored, len(want))
	for i := range want {
		s.True(proto.Equal(want[i], stored[i]))
	}

	reader := api.NewBacktestServer(s.conn, nil)
	got, err := reader.GetExecution(context.Background(), &servicePb.GetExecutionRequest{ExecutionId: rsp.Execution.Id})
	s.Require().NoError(err)
	s.Equal(rsp.Execution.Id, got.GetExecution().GetId())
	s.Equal(s.session.Id, got.GetExecution().GetSession())
	s.Equal("backtest", got.GetExecution().GetBacktest())
	s.Equal(int32(2024), got.GetExecution().GetStartDate().GetYear())
	s.Equal(int32(1), got.GetExecution().GetStartDate().GetMonth())
	s.Equal(int32(2), got.GetExecution().GetStartDate().GetDay())
	s.Equal(int32(2024), got.GetExecution().GetEndDate().GetYear())
	s.Equal(int32(6), got.GetExecution().GetEndDate().GetMonth())
	s.Equal(int32(30), got.GetExecution().GetEndDate().GetDay())
	s.Len(got.GetFunctions(), len(want))
	for i := range want {
		s.True(proto.Equal(want[i], got.GetFunctions()[i]))
	}
}

func strp(v string) *string {
	return &v
}

func (s *SessionTest) TestRunExecution() {
	rsp, err := s.client.CreateExecution(context.Background(), &servicePb.CreateExecutionRequest{
		Id:        "execution_123",
		Session:   s.session.Id,
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2024, Month: 1, Day: 1},
		Algorithm: &service_pb.Algorithm{},
	})
	s.Require().NoError(err)
	s.Require().NotNil(rsp)
	select {
	case <-s.activity:
	case <-time.After(5 * time.Second):
		s.Require().Fail("timeout waiting for activity")
	}

	executions := repository.Execution{Conn: s.conn}
	e := backtest.Execution{
		Session:   s.session.Id,
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2024, Month: 1, Day: 1},
	}
	execution, err := executions.Create(context.Background(), &e, nil)
	s.Require().NoError(err)

	ch := make(chan *servicePb.RunExecutionResponse, 5)
	ch <- &servicePb.RunExecutionResponse{}
	ch <- &servicePb.RunExecutionResponse{}
	ch <- &servicePb.RunExecutionResponse{}
	ch <- &servicePb.RunExecutionResponse{}
	ch <- &servicePb.RunExecutionResponse{}
	close(ch)
	s.mockEngineSession.On("RunBacktest", mock.Anything, mock.Anything, mock.Anything).Return(ch, nil)

	stream, err := s.client.RunExecution(context.Background(), &servicePb.RunExecutionRequest{
		ExecutionId: execution.Id,
	})
	s.Require().NoError(err)
	s.Require().NotNil(stream)

	entries := 0

	for {
		_, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		s.Require().NoError(err)

		entries++
	}

	s.Require().Equal(5, entries)
}

func (s *SessionTest) TestStopServer() {
	rsp, err := s.client.StopServer(context.Background(), &servicePb.StopServerRequest{})
	s.Require().NoError(err)
	s.Require().NotNil(rsp)
	select {
	case _, done := <-s.activity:
		s.Require().False(done)
	default:
		s.Require().Fail("activity channel should be closed")
	}
}

// A run outside the backtest's dates has no universe and no bars, so the
// broker refuses it instead of returning a flat result.
func (s *SessionTest) TestCreateExecutionRejectsDatesOutsideBacktest() {
	cases := []struct {
		name  string
		start *pb.Date
		end   *pb.Date
		want  []string
	}{
		{
			name:  "start before first period",
			start: &pb.Date{Year: 2023, Month: 12, Day: 29},
			end:   &pb.Date{Year: 2024, Month: 6, Day: 28},
			want:  []string{"2023-12-29", "before"},
		},
		{
			name:  "end after last period",
			start: &pb.Date{Year: 2024, Month: 1, Day: 2},
			end:   &pb.Date{Year: 2025, Month: 1, Day: 3},
			want:  []string{"2025-01-03", "after"},
		},
		{
			name:  "start after end",
			start: &pb.Date{Year: 2024, Month: 6, Day: 28},
			end:   &pb.Date{Year: 2024, Month: 1, Day: 2},
			want:  []string{"2024-06-28", "2024-01-02"},
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := s.client.CreateExecution(context.Background(), &servicePb.CreateExecutionRequest{
				Session:   s.session.Id,
				StartDate: tc.start,
				EndDate:   tc.end,
				Algorithm: &service_pb.Algorithm{},
			})
			s.Require().Error(err)
			st, ok := status.FromError(err)
			s.Require().True(ok)
			s.Equal(codes.InvalidArgument, st.Code(), st.Message())
			s.Contains(st.Message(), "backtest")
			s.Contains(st.Message(), "2024-01-01")
			s.Contains(st.Message(), "2024-12-31")
			for _, w := range tc.want {
				s.Contains(st.Message(), w)
			}
		})
	}

	var stored int
	err := s.conn.QueryRow(context.Background(), `SELECT count(*) FROM backtest_execution`).Scan(&stored)
	s.Require().NoError(err)
	s.Zero(stored, "a rejected request stores no execution")
}

func (s *SessionTest) TestCreateExecutionAcceptsBacktestBounds() {
	rsp, err := s.client.CreateExecution(context.Background(), &servicePb.CreateExecutionRequest{
		Session:   s.session.Id,
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
		Algorithm: &service_pb.Algorithm{},
	})
	s.Require().NoError(err)
	s.Equal("2024-01-01", pb.DateToDateString(rsp.GetExecution().GetStartDate()))
	s.Equal("2024-12-31", pb.DateToDateString(rsp.GetExecution().GetEndDate()))
}

// A request without dates runs over the whole backtest.
func (s *SessionTest) TestCreateExecutionWithoutDatesUsesBacktestRange() {
	rsp, err := s.client.CreateExecution(context.Background(), &servicePb.CreateExecutionRequest{
		Session:   s.session.Id,
		Algorithm: &service_pb.Algorithm{},
	})
	s.Require().NoError(err)
	s.Equal("2024-01-01", pb.DateToDateString(rsp.GetExecution().GetStartDate()))
	s.Equal("2024-12-31", pb.DateToDateString(rsp.GetExecution().GetEndDate()))
}
