package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/stretchr/testify/suite"
)

type DeploymentRunTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *DeploymentRunTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *DeploymentRunTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
}

func TestDeploymentRun(t *testing.T) {
	suite.Run(t, new(DeploymentRunTest))
}

func (test *DeploymentRunTest) createDeployment(ctx context.Context) *pb.Deployment {
	test.T().Helper()

	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	deployments := repository.Deployment{Conn: test.conn}
	deployment, err := deployments.Create(
		ctx, version.Id, "default", "XNYS",
		pb.Deployment_PAPER, []string{"AAPL"}, pb.Deployment_SESSION_CLOSE, true,
	)
	test.Require().NoError(err)

	return deployment
}

func (test *DeploymentRunTest) TestCreateAndList() {
	ctx := context.Background()
	deployment := test.createDeployment(ctx)

	orders := []*financepb.Order{
		{
			Symbol:      "AAPL",
			Amount:      10,
			Type:        financepb.OrderType_ORDER_TYPE_MARKET,
			TimeInForce: financepb.TimeInForce_TIME_IN_FORCE_DAY,
			Intent:      financepb.PositionIntent_POSITION_INTENT_BUY_TO_OPEN,
		},
	}

	runs := repository.DeploymentRun{Conn: test.conn}
	created, err := runs.Create(ctx, deployment.Id, &commonpb.Date{Year: 2024, Month: 3, Day: 4}, orders, false, nil)
	test.Require().NoError(err)
	test.NotEmpty(created.Id)
	test.Equal(deployment.Id, created.DeploymentId)
	test.Equal(int32(2024), created.SessionDate.Year)
	test.Equal(int32(3), created.SessionDate.Month)
	test.Equal(int32(4), created.SessionDate.Day)
	test.False(created.Skipped)
	test.Nil(created.Error)
	test.Require().Len(created.Orders, 1)
	test.Equal("AAPL", created.Orders[0].Symbol)
	test.Equal(int32(10), created.Orders[0].Amount)
	test.Equal(financepb.OrderType_ORDER_TYPE_MARKET, created.Orders[0].Type)

	listed, err := runs.List(ctx, "mean-rev", deployment.Id)
	test.Require().NoError(err)
	test.Len(listed, 1)
	test.Equal(created.Id, listed[0].Id)
}

func (test *DeploymentRunTest) TestUniqueSessionDatePerDeployment() {
	ctx := context.Background()
	deployment := test.createDeployment(ctx)

	runs := repository.DeploymentRun{Conn: test.conn}
	date := &commonpb.Date{Year: 2024, Month: 3, Day: 4}
	_, err := runs.Create(ctx, deployment.Id, date, nil, true, errors.New("skipped_missed_close"))
	test.Require().NoError(err)

	_, err = runs.Create(ctx, deployment.Id, date, nil, false, nil)
	test.Require().Error(err)
}

func (test *DeploymentRunTest) TestListFilters() {
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
	d1, err := deployments.Create(
		ctx, meanRev.Id, "default", "XNYS",
		pb.Deployment_PAPER, nil, pb.Deployment_SESSION_CLOSE, true,
	)
	test.Require().NoError(err)
	d2, err := deployments.Create(
		ctx, other.Id, "default", "XNYS",
		pb.Deployment_PAPER, nil, pb.Deployment_SESSION_CLOSE, true,
	)
	test.Require().NoError(err)

	runs := repository.DeploymentRun{Conn: test.conn}
	_, err = runs.Create(ctx, d1.Id, &commonpb.Date{Year: 2024, Month: 1, Day: 1}, nil, false, nil)
	test.Require().NoError(err)
	_, err = runs.Create(ctx, d2.Id, &commonpb.Date{Year: 2024, Month: 1, Day: 1}, nil, false, nil)
	test.Require().NoError(err)

	byStrategy, err := runs.List(ctx, "mean-rev", "")
	test.Require().NoError(err)
	test.Len(byStrategy, 1)
	test.Equal(d1.Id, byStrategy[0].DeploymentId)

	byDeployment, err := runs.List(ctx, "", d2.Id)
	test.Require().NoError(err)
	test.Len(byDeployment, 1)
	test.Equal(d2.Id, byDeployment[0].DeploymentId)
}
