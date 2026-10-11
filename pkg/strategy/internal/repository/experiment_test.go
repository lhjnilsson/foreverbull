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

func (test *ExperimentTest) TestListJoinsLastPeriodStats() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	spyExec, err := insertExecution(ctx, test.conn, "spy")
	test.Require().NoError(err)
	euExec, err := insertExecution(ctx, test.conn, "eu")
	test.Require().NoError(err)

	test.Require().NoError(insertPeriod(ctx, test.conn, spyExec, "2020-01-02", 1000, -0.1, 0.01, floatPtr(0.5), nil))
	test.Require().NoError(insertPeriod(ctx, test.conn, spyExec, "2020-06-01", 1100, -0.2, 0.05, floatPtr(1.25), floatPtr(0.03)))
	test.Require().NoError(insertPeriod(ctx, test.conn, euExec, "2020-06-01", 900, -0.15, 0.02, nil, floatPtr(0.01)))

	experiments := repository.Experiment{Conn: test.conn}
	spy, err := experiments.Create(ctx, version.Id, "spy")
	test.Require().NoError(err)
	test.Require().NoError(experiments.SetExecutionID(ctx, spy.Id, spyExec))
	test.Require().NoError(experiments.UpdateStatus(ctx, spy.Id, pb.Experiment_COMPLETED, nil))

	eu, err := experiments.Create(ctx, version.Id, "eu")
	test.Require().NoError(err)
	test.Require().NoError(experiments.SetExecutionID(ctx, eu.Id, euExec))
	test.Require().NoError(experiments.UpdateStatus(ctx, eu.Id, pb.Experiment_COMPLETED, nil))

	queued, err := experiments.Create(ctx, version.Id, "commodities")
	test.Require().NoError(err)

	listed, err := experiments.List(ctx, "mean-rev", version.Id, "")
	test.Require().NoError(err)
	test.Len(listed, 3)

	byID := map[string]*pb.Experiment{}
	for _, e := range listed {
		byID[e.Id] = e
	}

	spyGot := byID[spy.Id]
	test.Require().NotNil(spyGot.Stats)
	test.InDelta(1100, spyGot.Stats.PortfolioValue, 1e-9)
	test.InDelta(-0.2, spyGot.Stats.MaxDrawdown, 1e-9)
	test.Require().NotNil(spyGot.Stats.Sharpe)
	test.InDelta(1.25, *spyGot.Stats.Sharpe, 1e-9)
	test.InDelta(0.05, spyGot.Stats.AlgorithmPeriodReturn, 1e-9)
	test.Require().NotNil(spyGot.Stats.BenchmarkPeriodReturn)
	test.InDelta(0.03, *spyGot.Stats.BenchmarkPeriodReturn, 1e-9)

	euGot := byID[eu.Id]
	test.Require().NotNil(euGot.Stats)
	test.InDelta(900, euGot.Stats.PortfolioValue, 1e-9)
	test.InDelta(-0.15, euGot.Stats.MaxDrawdown, 1e-9)
	test.Nil(euGot.Stats.Sharpe)
	test.InDelta(0.02, euGot.Stats.AlgorithmPeriodReturn, 1e-9)
	test.Require().NotNil(euGot.Stats.BenchmarkPeriodReturn)
	test.InDelta(0.01, *euGot.Stats.BenchmarkPeriodReturn, 1e-9)

	queuedGot := byID[queued.Id]
	test.Nil(queuedGot.Stats)
	test.Empty(queuedGot.GetExecutionId())
}

func (test *ExperimentTest) TestGetLastPeriodMissingIsEmpty() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	executionID, err := insertExecution(ctx, test.conn, "spy")
	test.Require().NoError(err)

	experiments := repository.Experiment{Conn: test.conn}
	experiment, err := experiments.Create(ctx, version.Id, "spy")
	test.Require().NoError(err)
	test.Require().NoError(experiments.SetExecutionID(ctx, experiment.Id, executionID))

	got, err := experiments.Get(ctx, experiment.Id)
	test.Require().NoError(err)
	test.Nil(got.Stats)

	period, err := experiments.GetLastPeriod(ctx, executionID)
	test.Require().NoError(err)
	test.Nil(period)

	period, err = experiments.GetLastPeriod(ctx, "")
	test.Require().NoError(err)
	test.Nil(period)
}

func (test *ExperimentTest) TestGetLastPeriodUsesLatestDate() {
	ctx := context.Background()
	executionID, err := insertExecution(ctx, test.conn, "spy")
	test.Require().NoError(err)
	test.Require().NoError(insertPeriod(ctx, test.conn, executionID, "2020-01-02", 1000, -0.1, 0.01, floatPtr(0.4), nil))
	test.Require().NoError(insertPeriod(ctx, test.conn, executionID, "2020-03-01", 1234.5, -0.33, 0.12, floatPtr(1.8), floatPtr(0.04)))

	experiments := repository.Experiment{Conn: test.conn}
	period, err := experiments.GetLastPeriod(ctx, executionID)
	test.Require().NoError(err)
	test.Require().NotNil(period)
	test.Equal(int32(2020), period.Date.Year)
	test.Equal(int32(3), period.Date.Month)
	test.Equal(int32(1), period.Date.Day)
	test.InDelta(1234.5, period.PortfolioValue, 1e-9)
	test.InDelta(-0.33, period.MaxDrawdown, 1e-9)
	test.Require().NotNil(period.Sharpe)
	test.InDelta(1.8, *period.Sharpe, 1e-9)
	test.InDelta(0.12, period.AlgorithmPeriodReturn, 1e-9)
	test.Require().NotNil(period.BenchmarkPeriodReturn)
	test.InDelta(0.04, *period.BenchmarkPeriodReturn, 1e-9)
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
