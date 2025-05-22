package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/stretchr/testify/suite"
)

type TradingTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *TradingTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *TradingTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
}

func (test *TradingTest) TearDownTest() {
}

func TestTrading(t *testing.T) {
	suite.Run(t, new(TradingTest))
}

func (test *TradingTest) TestCreate() {
	uut := repository.Trading{Conn: test.conn}
	ctx := context.Background()

	session, err := uut.Create(ctx)
	test.Require().NoError(err)
	test.NotNil(session.Id)
	test.Nil(session.Port)
}

func (test *TradingTest) TestGet() {
	uut := repository.Trading{Conn: test.conn}
	ctx := context.Background()

	session, err := uut.Create(ctx)
	test.Require().NoError(err)
	test.NotNil(session.Id)

	session2, err := uut.Get(ctx, session.Id)
	test.Require().NoError(err)
	test.Equal(session.Id, session2.Id)
	test.Nil(session2.Port)
}

func (test *TradingTest) TestUpdatePort() {
	uut := repository.Trading{Conn: test.conn}
	ctx := context.Background()

	session, err := uut.Create(ctx)
	test.Require().NoError(err)
	test.NotNil(session.Id)

	err = uut.UpdatePort(ctx, session.Id, 1337)
	test.Require().NoError(err)

	session2, err := uut.Get(ctx, session.Id)
	test.Require().NoError(err)
	test.Equal(session.Id, session2.Id)
	test.NotNil(session2.Port)
	test.Equal(int64(1337), *session2.Port)

}
