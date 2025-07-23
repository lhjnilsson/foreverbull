package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	storedBacktest *backtest.Backtest
	storedSession  *backtest.Session
	execution      *backtest.Execution

	uut repository.Execution
}

func (test *ExecutionTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *ExecutionTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)

	ctx := context.Background()
	backtests := &repository.Backtest{Conn: test.conn}
	b := backtest.Backtest{
		Name: "backtest",
		Periods: []*backtest.Backtest_Period{
			{
				StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
			},
		},
	}
	_, err = backtests.Create(ctx, &b)
	test.Require().NoError(err)
	test.storedBacktest, err = backtests.Get(ctx, "backtest")
	test.Require().NoError(err)

	sessions := repository.Session{Conn: test.conn}

	test.storedSession, err = sessions.Create(ctx, "backtest")
	test.Require().NoError(err)

	test.uut = repository.Execution{Conn: test.conn}
	test.execution = &backtest.Execution{
		Session:   test.storedSession.Id,
		StartDate: b.Periods[0].StartDate,
		EndDate:   b.Periods[0].EndDate,
	}
}

func (test *ExecutionTest) TearDownTest() {
}

func TestExecutions(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (test *ExecutionTest) TestCreate() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()

	execution, err := executions.Create(ctx, test.execution)
	test.Require().NoError(err)
	test.NotNil(execution.Id)
	test.Equal(test.storedSession.Id, execution.Session)
	test.Equal(test.storedBacktest.Periods[0].StartDate, execution.StartDate)
	test.Equal(test.storedBacktest.Periods[0].EndDate, execution.EndDate)
}

func (test *ExecutionTest) TestGet() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	execution, err := executions.Create(ctx, test.execution)
	test.Require().NoError(err)
	execution, err = executions.Get(ctx, execution.Id)
	test.Require().NoError(err)
	test.NotNil(execution.Id)
	test.Equal(test.storedSession.Id, execution.Session)
	test.Equal(test.execution.StartDate, execution.StartDate)
	test.Equal(test.execution.EndDate, execution.EndDate)
}

func (test *ExecutionTest) TestGetPeriods() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	execution, err := executions.Create(ctx, test.execution)
	test.Require().NoError(err)
	periods, err := executions.GetPeriods(ctx, execution.Id)
	test.Require().NoError(err)
	test.NotNil(periods)
}

func (test *ExecutionTest) TestUpdateSimulationDetails() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	execution, err := executions.Create(ctx, test.execution)
	test.Require().NoError(err)
	// e.StartDate = internal_pb.TimeToProtoTimestamp(&common_pb.Date{Year: 2024, Month: 01, Day: 01})
	// e.EndDate = internal_pb.TimeToProtoTimestamp(&common_pb.Date{Year: 2024, Month: 01, Day: 01})
	err = executions.UpdateSimulationDetails(ctx, execution)
	test.Require().NoError(err)
	execution, err = executions.Get(ctx, execution.Id)
	test.Require().NoError(err)
	test.NotNil(execution.Id)
	test.Equal(test.storedSession.Id, execution.Session)
}

func (test *ExecutionTest) TestList() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()

	_, err := executions.Create(ctx, test.execution)
	test.Require().NoError(err)

	_, err = executions.Create(ctx, test.execution)
	test.Require().NoError(err)

	storedExecutions, err := executions.List(ctx)
	test.Require().NoError(err)
	test.Len(storedExecutions, 2)
}

func (test *ExecutionTest) TestListBySession() {
	sessions := repository.Session{Conn: test.conn}

	ctx := context.Background()
	session1, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)

	session2, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)

	executions := repository.Execution{Conn: test.conn}
	test.execution.Session = session1.Id
	_, err = executions.Create(ctx, test.execution)
	test.Require().NoError(err)

	storedExecutions, err := executions.ListBySession(ctx, session1.Id)
	test.Require().NoError(err)
	test.Len(storedExecutions, 1)

	storedExecutions, err = executions.ListBySession(ctx, session2.Id)
	test.Require().NoError(err)
	test.Empty(storedExecutions)
}

func (test *ExecutionTest) TestListByBacktest() {
	sessions := repository.Session{Conn: test.conn}

	ctx := context.Background()
	session, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)
	test.execution.Session = session.Id

	executions := repository.Execution{Conn: test.conn}
	_, err = executions.Create(ctx, test.execution)
	test.Require().NoError(err)

	storedExecutions, err := executions.ListByBacktest(ctx, "backtest")
	test.Require().NoError(err)
	test.Len(storedExecutions, 1)
}
