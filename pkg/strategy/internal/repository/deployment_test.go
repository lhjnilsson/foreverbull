package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/stretchr/testify/suite"
)

type DeploymentTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *DeploymentTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *DeploymentTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
}

func TestDeployment(t *testing.T) {
	suite.Run(t, new(DeploymentTest))
}

func (test *DeploymentTest) TestCreatePaperDryRun() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	deployments := repository.Deployment{Conn: test.conn}
	deployment, err := deployments.Create(
		ctx, version.Id, "default", "XNYS",
		pb.Deployment_PAPER, []string{"AAPL", "MSFT"},
		pb.Deployment_SESSION_CLOSE, true,
	)
	test.Require().NoError(err)
	test.NotEmpty(deployment.Id)
	test.Equal(version.Id, deployment.VersionId)
	test.Equal("mean-rev", deployment.StrategyName)
	test.Equal(pb.Deployment_PAPER, deployment.Mode)
	test.Equal("default", deployment.Backend)
	test.Equal("XNYS", deployment.CalendarCode)
	test.Equal([]string{"AAPL", "MSFT"}, deployment.Symbols)
	test.Equal(pb.Deployment_SESSION_CLOSE, deployment.ScheduleKind)
	test.True(deployment.DryRun)
	test.Equal(pb.Deployment_RUNNING, deployment.Status)
	test.NotNil(deployment.StartedAt)
	test.Nil(deployment.StoppedAt)
	test.Nil(deployment.LastSessionDate)
}

func (test *DeploymentTest) TestOneRunningDeploymentPerStrategy() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	deployments := repository.Deployment{Conn: test.conn}
	first, err := deployments.Create(
		ctx, version.Id, "default", "XNYS",
		pb.Deployment_PAPER, nil, pb.Deployment_SESSION_CLOSE, true,
	)
	test.Require().NoError(err)

	_, err = deployments.Create(
		ctx, version.Id, "default", "XNYS",
		pb.Deployment_PAPER, nil, pb.Deployment_SESSION_CLOSE, true,
	)
	test.Require().Error(err)

	err = deployments.Stop(ctx, first.Id)
	test.Require().NoError(err)

	second, err := deployments.Create(
		ctx, version.Id, "default", "XNYS",
		pb.Deployment_PAPER, nil, pb.Deployment_SESSION_CLOSE, true,
	)
	test.Require().NoError(err)
	test.NotEqual(first.Id, second.Id)

	stopped, err := deployments.Get(ctx, first.Id)
	test.Require().NoError(err)
	test.Equal(pb.Deployment_STOPPED, stopped.Status)
	test.NotNil(stopped.StoppedAt)
}

func (test *DeploymentTest) TestGetRunningAndList() {
	ctx := context.Background()
	err := insertService(ctx, test.conn, "algo:local")
	test.Require().NoError(err)

	strategies := repository.Strategy{Conn: test.conn}
	_, err = strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)
	_, err = strategies.Create(ctx, "other", nil)
	test.Require().NoError(err)

	versions := repository.Version{Conn: test.conn}
	meanRev, err := versions.Create(ctx, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)
	other, err := versions.Create(ctx, "other", "algo:local", nil)
	test.Require().NoError(err)

	deployments := repository.Deployment{Conn: test.conn}
	created, err := deployments.Create(
		ctx, meanRev.Id, "default", "XNYS",
		pb.Deployment_PAPER, []string{"SPY"}, pb.Deployment_SESSION_CLOSE, true,
	)
	test.Require().NoError(err)
	_, err = deployments.Create(
		ctx, other.Id, "default", "XNYS",
		pb.Deployment_PAPER, []string{"EWU"}, pb.Deployment_SESSION_CLOSE, true,
	)
	test.Require().NoError(err)

	running, err := deployments.GetRunningByStrategyName(ctx, "mean-rev")
	test.Require().NoError(err)
	test.Equal(created.Id, running.Id)

	listed, err := deployments.List(ctx, "mean-rev")
	test.Require().NoError(err)
	test.Len(listed, 1)

	allRunning, err := deployments.ListRunning(ctx)
	test.Require().NoError(err)
	test.Len(allRunning, 2)
}

func (test *DeploymentTest) TestUpdateStatusAndLastSessionDate() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	deployments := repository.Deployment{Conn: test.conn}
	deployment, err := deployments.Create(
		ctx, version.Id, "default", "XNYS",
		pb.Deployment_PAPER, []string{"SPY"}, pb.Deployment_SESSION_CLOSE, true,
	)
	test.Require().NoError(err)

	err = deployments.SetLastSessionDate(ctx, deployment.Id, &commonpb.Date{Year: 2024, Month: 1, Day: 2})
	test.Require().NoError(err)

	err = deployments.UpdateStatus(ctx, deployment.Id, pb.Deployment_ERROR, errors.New("clock failed"))
	test.Require().NoError(err)

	got, err := deployments.Get(ctx, deployment.Id)
	test.Require().NoError(err)
	test.Equal(pb.Deployment_ERROR, got.Status)
	test.Equal("clock failed", got.GetError())
	test.Equal(int32(2024), got.LastSessionDate.Year)
	test.Equal(int32(1), got.LastSessionDate.Month)
	test.Equal(int32(2), got.LastSessionDate.Day)
}
