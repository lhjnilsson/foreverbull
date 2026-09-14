package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/stretchr/testify/suite"
)

type ExperimentTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *ExperimentTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *ExperimentTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
}

func TestExperiment(t *testing.T) {
	suite.Run(t, new(ExperimentTest))
}

func (test *ExperimentTest) TestCreateQueued() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	experiments := repository.Experiment{Conn: test.conn}
	experiment, err := experiments.Create(ctx, version.Id, "spy")
	test.Require().NoError(err)
	test.NotEmpty(experiment.Id)
	test.Equal(version.Id, experiment.VersionId)
	test.Equal(version.StrategyId, experiment.StrategyId)
	test.Equal("mean-rev", experiment.StrategyName)
	test.Equal(version.Rev, experiment.VersionRev)
	test.Equal("spy", experiment.BacktestName)
	test.Equal(pb.Experiment_QUEUED, experiment.Status)
	test.Nil(experiment.ExecutionId)
	test.Nil(experiment.CompletedAt)
}

func (test *ExperimentTest) TestSameVersionManyBacktests() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	experiments := repository.Experiment{Conn: test.conn}
	spy, err := experiments.Create(ctx, version.Id, "spy")
	test.Require().NoError(err)
	eu, err := experiments.Create(ctx, version.Id, "eu")
	test.Require().NoError(err)
	commodities, err := experiments.Create(ctx, version.Id, "commodities")
	test.Require().NoError(err)

	test.NotEqual(spy.Id, eu.Id)
	test.NotEqual(eu.Id, commodities.Id)
	test.Equal(version.Id, spy.VersionId)
	test.Equal(version.Id, eu.VersionId)
	test.Equal(version.Id, commodities.VersionId)

	listed, err := experiments.List(ctx, "mean-rev", version.Id, "")
	test.Require().NoError(err)
	test.Len(listed, 3)
}

func (test *ExperimentTest) TestListFilters() {
	ctx := context.Background()
	err := insertService(ctx, test.conn, "algo:local")
	test.Require().NoError(err)

	strategies := repository.Strategy{Conn: test.conn}
	_, err = strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)
	_, err = strategies.Create(ctx, "crossover", nil)
	test.Require().NoError(err)

	versions := repository.Version{Conn: test.conn}
	meanRev, err := versions.Create(ctx, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)
	crossover, err := versions.Create(ctx, "crossover", "algo:local", nil)
	test.Require().NoError(err)

	experiments := repository.Experiment{Conn: test.conn}
	_, err = experiments.Create(ctx, meanRev.Id, "spy")
	test.Require().NoError(err)
	_, err = experiments.Create(ctx, meanRev.Id, "eu")
	test.Require().NoError(err)
	_, err = experiments.Create(ctx, crossover.Id, "spy")
	test.Require().NoError(err)

	byName, err := experiments.List(ctx, "mean-rev", "", "")
	test.Require().NoError(err)
	test.Len(byName, 2)

	byBacktest, err := experiments.List(ctx, "", "", "spy")
	test.Require().NoError(err)
	test.Len(byBacktest, 2)

	byVersion, err := experiments.List(ctx, "", meanRev.Id, "eu")
	test.Require().NoError(err)
	test.Len(byVersion, 1)
	test.Equal("eu", byVersion[0].BacktestName)
}

func (test *ExperimentTest) TestUpdateStatusAndExecutionID() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	executionID, err := insertExecution(ctx, test.conn, "spy")
	test.Require().NoError(err)

	experiments := repository.Experiment{Conn: test.conn}
	experiment, err := experiments.Create(ctx, version.Id, "spy")
	test.Require().NoError(err)

	err = experiments.SetExecutionID(ctx, experiment.Id, executionID)
	test.Require().NoError(err)

	err = experiments.UpdateStatus(ctx, experiment.Id, pb.Experiment_RUNNING, nil)
	test.Require().NoError(err)

	got, err := experiments.Get(ctx, experiment.Id)
	test.Require().NoError(err)
	test.Equal(pb.Experiment_RUNNING, got.Status)
	test.Equal(executionID, got.GetExecutionId())
	test.Nil(got.CompletedAt)

	err = experiments.UpdateStatus(ctx, experiment.Id, pb.Experiment_COMPLETED, nil)
	test.Require().NoError(err)

	got, err = experiments.Get(ctx, experiment.Id)
	test.Require().NoError(err)
	test.Equal(pb.Experiment_COMPLETED, got.Status)
	test.NotNil(got.CompletedAt)
}

func (test *ExperimentTest) TestUpdateStatusFailedStoresError() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	experiments := repository.Experiment{Conn: test.conn}
	experiment, err := experiments.Create(ctx, version.Id, "spy")
	test.Require().NoError(err)

	err = experiments.UpdateStatus(ctx, experiment.Id, pb.Experiment_FAILED, errors.New("zipline exploded"))
	test.Require().NoError(err)

	got, err := experiments.Get(ctx, experiment.Id)
	test.Require().NoError(err)
	test.Equal(pb.Experiment_FAILED, got.Status)
	test.Equal("zipline exploded", got.GetError())
	test.NotNil(got.CompletedAt)
}

func (test *ExperimentTest) TestSetExecutionIDRequiresExistingExecution() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	experiments := repository.Experiment{Conn: test.conn}
	experiment, err := experiments.Create(ctx, version.Id, "spy")
	test.Require().NoError(err)

	err = experiments.SetExecutionID(ctx, experiment.Id, "missing-execution")
	test.Require().Error(err)
}
