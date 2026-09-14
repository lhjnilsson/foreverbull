package api_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internalGrpc "github.com/lhjnilsson/foreverbull/internal/grpc"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	common "github.com/lhjnilsson/foreverbull/pkg/pb"
	service "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	ss "github.com/lhjnilsson/foreverbull/pkg/strategy/internal/stream"
	"github.com/stretchr/testify/mock"
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

	pgx      *pgxpool.Pool
	launcher *ss.MockExperimentLauncher

	listener *bufconn.Listener
	server   *grpc.Server
	client   servicePb.StrategyServicerClient
}

func TestStrategyServer(t *testing.T) {
	suite.Run(t, new(StrategyServerTest))
}

func (test *StrategyServerTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *StrategyServerTest) SetupSubTest() {
	var err error
	test.pgx, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	test.Require().NoError(repository.Recreate(context.Background(), test.pgx))

	test.launcher = new(ss.MockExperimentLauncher)
	test.listener = bufconn.Listen(1024 * 1024)
	test.server, err = internalGrpc.NewServer()
	test.Require().NoError(err)

	servicePb.RegisterStrategyServicerServer(test.server, api.NewStrategyServer(test.pgx, test.launcher))

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
	test.client = servicePb.NewStrategyServicerClient(conn)
}

func (test *StrategyServerTest) TearDownSubTest() {
	if test.server != nil {
		test.server.Stop()
	}
	if test.listener != nil {
		test.NoError(test.listener.Close())
	}
}

func (test *StrategyServerTest) seedVersion(name, image string, status service.Service_Status_Status) *pb.Version {
	test.T().Helper()
	ctx := context.Background()
	_, err := test.pgx.Exec(ctx, `INSERT INTO service (image, status) VALUES ($1, $2)`, image, int(status))
	test.Require().NoError(err)

	strategies := repository.Strategy{Conn: test.pgx}
	_, err = strategies.Create(ctx, name, nil)
	test.Require().NoError(err)

	versions := repository.Version{Conn: test.pgx}
	version, err := versions.Create(ctx, name, image, nil)
	test.Require().NoError(err)
	return version
}

func (test *StrategyServerTest) insertBacktest(name string) {
	test.T().Helper()
	_, err := test.pgx.Exec(context.Background(), `INSERT INTO backtest (name, benchmark) VALUES ($1, $2)`, name, "SPY")
	test.Require().NoError(err)
}

func (test *StrategyServerTest) insertExecution(backtestName string) string {
	test.T().Helper()
	ctx := context.Background()
	var n int
	err := test.pgx.QueryRow(ctx, `SELECT COUNT(*) FROM backtest WHERE name=$1`, backtestName).Scan(&n)
	test.Require().NoError(err)
	if n == 0 {
		test.insertBacktest(backtestName)
	}

	var sessionID string
	err = test.pgx.QueryRow(ctx, `INSERT INTO session (backtest) VALUES ($1) RETURNING id`, backtestName).Scan(&sessionID)
	test.Require().NoError(err)

	var executionID string
	err = test.pgx.QueryRow(ctx,
		`INSERT INTO backtest_execution (session, start_date, end_date) VALUES ($1, $2, $3) RETURNING id`,
		sessionID, "2020-01-01", "2020-12-31",
	).Scan(&executionID)
	test.Require().NoError(err)
	return executionID
}

func recvAll(stream grpc.ServerStreamingClient[servicePb.CreateExperimentResponse]) ([]*servicePb.CreateExperimentResponse, error) {
	var out []*servicePb.CreateExperimentResponse
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, msg)
	}
}

func (test *StrategyServerTest) TestCreateStrategyUnimplemented() {
	test.Run("unimplemented", func() {
		_, err := test.client.CreateStrategy(context.Background(), &servicePb.CreateStrategyRequest{
			Name: "mean-rev",
		})
		test.Require().Error(err)
		st, ok := status.FromError(err)
		test.True(ok)
		test.Equal(codes.Unimplemented, st.Code())
	})
}

func (test *StrategyServerTest) TestCreateExperiment() {
	test.Run("success", func() {
		version := test.seedVersion("mean-rev", "algo:local", service.Service_Status_READY)
		executionID := test.insertExecution("spy")

		test.launcher.On("Run", mock.Anything, mock.MatchedBy(func(in ss.RunInput) bool {
			return in.BacktestName == "spy" && in.ServiceImage == "algo:local"
		})).Return(&ss.RunOutput{ExecutionID: executionID}, nil)

		stream, err := test.client.CreateExperiment(context.Background(), &servicePb.CreateExperimentRequest{
			StrategyName: "mean-rev",
			VersionId:    version.Id,
			BacktestName: "spy",
		})
		test.Require().NoError(err)

		msgs, err := recvAll(stream)
		test.Require().NoError(err)
		test.NotEmpty(msgs)
		last := msgs[len(msgs)-1]
		test.Equal(common.State_STATE_COMPLETED, last.GetQUEUED().GetState())
		test.Equal(common.State_STATE_COMPLETED, last.GetRUNNING().GetState())
		test.Equal(common.State_STATE_COMPLETED, last.GetCOMPLETED().GetState())
		test.Equal(pb.Experiment_COMPLETED, last.GetExperiment().GetStatus())
		test.Equal(executionID, last.GetExperiment().GetExecutionId())
		test.Equal("spy", last.GetExperiment().GetBacktestName())
	})

	test.Run("same version many backtests", func() {
		version := test.seedVersion("mean-rev", "algo:local", service.Service_Status_READY)
		spyExec := test.insertExecution("spy")
		euExec := test.insertExecution("eu")

		test.launcher.On("Run", mock.Anything, mock.MatchedBy(func(in ss.RunInput) bool {
			return in.BacktestName == "spy"
		})).Return(&ss.RunOutput{ExecutionID: spyExec}, nil).Once()
		test.launcher.On("Run", mock.Anything, mock.MatchedBy(func(in ss.RunInput) bool {
			return in.BacktestName == "eu"
		})).Return(&ss.RunOutput{ExecutionID: euExec}, nil).Once()

		for _, name := range []string{"spy", "eu"} {
			stream, err := test.client.CreateExperiment(context.Background(), &servicePb.CreateExperimentRequest{
				StrategyName: "mean-rev",
				VersionId:    version.Id,
				BacktestName: name,
			})
			test.Require().NoError(err)
			msgs, err := recvAll(stream)
			test.Require().NoError(err)
			test.Equal(name, msgs[len(msgs)-1].GetExperiment().GetBacktestName())
			test.Equal(pb.Experiment_COMPLETED, msgs[len(msgs)-1].GetExperiment().GetStatus())
		}

		listed, err := test.client.ListExperiments(context.Background(), &servicePb.ListExperimentsRequest{
			StrategyName: "mean-rev",
			VersionId:    version.Id,
		})
		test.Require().NoError(err)
		test.Len(listed.Experiments, 2)
		names := []string{listed.Experiments[0].BacktestName, listed.Experiments[1].BacktestName}
		test.ElementsMatch([]string{"spy", "eu"}, names)
	})

	test.Run("service not ready", func() {
		version := test.seedVersion("mean-rev", "algo:local", service.Service_Status_CREATED)
		test.insertBacktest("spy")

		stream, err := test.client.CreateExperiment(context.Background(), &servicePb.CreateExperimentRequest{
			StrategyName: "mean-rev",
			VersionId:    version.Id,
			BacktestName: "spy",
		})
		test.Require().NoError(err)
		_, err = stream.Recv()
		test.Require().Error(err)
		st, ok := status.FromError(err)
		test.True(ok)
		test.Equal(codes.FailedPrecondition, st.Code())
		test.Contains(st.Message(), "not READY")
		test.launcher.AssertNotCalled(test.T(), "Run", mock.Anything, mock.Anything)
	})

	test.Run("missing backtest", func() {
		version := test.seedVersion("mean-rev", "algo:local", service.Service_Status_READY)

		stream, err := test.client.CreateExperiment(context.Background(), &servicePb.CreateExperimentRequest{
			StrategyName: "mean-rev",
			VersionId:    version.Id,
			BacktestName: "commodities",
		})
		test.Require().NoError(err)
		_, err = stream.Recv()
		test.Require().Error(err)
		st, ok := status.FromError(err)
		test.True(ok)
		test.Equal(codes.NotFound, st.Code())
		test.Contains(st.Message(), "commodities")
		test.Contains(st.Message(), "fbull backtest create")
		test.Contains(st.Message(), "do not auto-ingest")
		test.launcher.AssertNotCalled(test.T(), "Run", mock.Anything, mock.Anything)
	})

	test.Run("launcher fails", func() {
		version := test.seedVersion("mean-rev", "algo:local", service.Service_Status_READY)
		test.insertBacktest("spy")
		test.launcher.On("Run", mock.Anything, mock.Anything).Return(nil, errors.New("engine down"))

		stream, err := test.client.CreateExperiment(context.Background(), &servicePb.CreateExperimentRequest{
			StrategyName: "mean-rev",
			VersionId:    version.Id,
			BacktestName: "spy",
		})
		test.Require().NoError(err)
		msgs, err := recvAll(stream)
		test.Require().Error(err)
		test.NotEmpty(msgs)
		last := msgs[len(msgs)-1]
		test.Equal(pb.Experiment_FAILED, last.GetExperiment().GetStatus())
		test.Contains(last.GetExperiment().GetError(), "engine down")
	})

	test.Run("version by rev", func() {
		version := test.seedVersion("mean-rev", "algo:local", service.Service_Status_READY)
		executionID := test.insertExecution("spy")
		test.launcher.On("Run", mock.Anything, mock.Anything).Return(&ss.RunOutput{ExecutionID: executionID}, nil)

		stream, err := test.client.CreateExperiment(context.Background(), &servicePb.CreateExperimentRequest{
			StrategyName: "mean-rev",
			VersionRev:   version.Rev,
			BacktestName: "spy",
		})
		test.Require().NoError(err)
		msgs, err := recvAll(stream)
		test.Require().NoError(err)
		test.Equal(version.Id, msgs[len(msgs)-1].GetExperiment().GetVersionId())
	})
}

func (test *StrategyServerTest) TestGetAndListExperiments() {
	test.Run("get and list", func() {
		version := test.seedVersion("mean-rev", "algo:local", service.Service_Status_READY)
		executionID := test.insertExecution("spy")
		test.insertBacktest("eu")

		experiments := repository.Experiment{Conn: test.pgx}
		spy, err := experiments.Create(context.Background(), version.Id, "spy")
		test.Require().NoError(err)
		test.Require().NoError(experiments.SetExecutionID(context.Background(), spy.Id, executionID))
		test.Require().NoError(experiments.UpdateStatus(context.Background(), spy.Id, pb.Experiment_COMPLETED, nil))
		eu, err := experiments.Create(context.Background(), version.Id, "eu")
		test.Require().NoError(err)

		got, err := test.client.GetExperiment(context.Background(), &servicePb.GetExperimentRequest{
			ExperimentId: spy.Id,
		})
		test.Require().NoError(err)
		test.Equal(spy.Id, got.GetExperiment().GetId())
		test.Equal(executionID, got.GetExperiment().GetExecutionId())
		test.Require().NotNil(got.GrafanaPath)
		test.Contains(*got.GrafanaPath, executionID)

		listed, err := test.client.ListExperiments(context.Background(), &servicePb.ListExperimentsRequest{
			StrategyName: "mean-rev",
		})
		test.Require().NoError(err)
		test.Len(listed.Experiments, 2)

		byBacktest, err := test.client.ListExperiments(context.Background(), &servicePb.ListExperimentsRequest{
			StrategyName: "mean-rev",
			BacktestName: "eu",
		})
		test.Require().NoError(err)
		test.Len(byBacktest.Experiments, 1)
		test.Equal(eu.Id, byBacktest.Experiments[0].Id)
	})

	test.Run("not found", func() {
		_, err := test.client.GetExperiment(context.Background(), &servicePb.GetExperimentRequest{
			ExperimentId: "missing",
		})
		test.Require().Error(err)
		st, ok := status.FromError(err)
		test.True(ok)
		test.Equal(codes.NotFound, st.Code())
	})
}
