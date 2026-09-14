package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/structpb"
)

type VersionTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *VersionTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *VersionTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
}

func TestVersion(t *testing.T) {
	suite.Run(t, new(VersionTest))
}

func (test *VersionTest) TestCreateIncrementsRev() {
	ctx := context.Background()
	err := insertService(ctx, test.conn, "algo:local")
	test.Require().NoError(err)

	strategies := repository.Strategy{Conn: test.conn}
	_, err = strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)

	versions := repository.Version{Conn: test.conn}
	v1, err := versions.Create(ctx, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)
	test.Equal(int32(1), v1.Rev)
	test.Equal("mean-rev", v1.StrategyName)
	test.Equal("algo:local", v1.ServiceImage)
	test.NotEmpty(v1.Id)
	test.NotEmpty(v1.StrategyId)

	v2, err := versions.Create(ctx, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)
	test.Equal(int32(2), v2.Rev)
	test.Equal(v1.StrategyId, v2.StrategyId)
	test.NotEqual(v1.Id, v2.Id)
}

func (test *VersionTest) TestCreateRequiresServiceImage() {
	ctx := context.Background()
	strategies := repository.Strategy{Conn: test.conn}
	_, err := strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)

	versions := repository.Version{Conn: test.conn}
	_, err = versions.Create(ctx, "mean-rev", "missing:image", nil)
	test.Require().Error(err)
}

func (test *VersionTest) TestCreateUniqueStrategyRev() {
	ctx := context.Background()
	err := insertService(ctx, test.conn, "algo:local")
	test.Require().NoError(err)

	strategies := repository.Strategy{Conn: test.conn}
	strategy, err := strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)

	versions := repository.Version{Conn: test.conn}
	_, err = versions.Create(ctx, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	_, err = test.conn.Exec(ctx,
		`INSERT INTO strategy_version (strategy_id, rev, service_image) VALUES ($1, $2, $3)`,
		strategy.Id, 1, "algo:local")
	test.Require().Error(err)
}

func (test *VersionTest) TestIndependentRevsPerStrategy() {
	ctx := context.Background()
	err := insertService(ctx, test.conn, "algo:local")
	test.Require().NoError(err)

	strategies := repository.Strategy{Conn: test.conn}
	_, err = strategies.Create(ctx, "a", nil)
	test.Require().NoError(err)
	_, err = strategies.Create(ctx, "b", nil)
	test.Require().NoError(err)

	versions := repository.Version{Conn: test.conn}
	va, err := versions.Create(ctx, "a", "algo:local", nil)
	test.Require().NoError(err)
	vb, err := versions.Create(ctx, "b", "algo:local", nil)
	test.Require().NoError(err)
	test.Equal(int32(1), va.Rev)
	test.Equal(int32(1), vb.Rev)
}

func (test *VersionTest) TestCreateParameters() {
	ctx := context.Background()
	err := insertService(ctx, test.conn, "algo:local")
	test.Require().NoError(err)

	strategies := repository.Strategy{Conn: test.conn}
	_, err = strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)

	params, err := structpb.NewStruct(map[string]interface{}{"lookback": 20.0})
	test.Require().NoError(err)

	versions := repository.Version{Conn: test.conn}
	created, err := versions.Create(ctx, "mean-rev", "algo:local", params)
	test.Require().NoError(err)
	test.InEpsilon(20.0, created.Parameters.Fields["lookback"].GetNumberValue(), 1e-9)

	got, err := versions.Get(ctx, created.Id)
	test.Require().NoError(err)
	test.InEpsilon(20.0, got.Parameters.Fields["lookback"].GetNumberValue(), 1e-9)
}

func (test *VersionTest) TestGetByStrategyAndRev() {
	ctx := context.Background()
	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	versions := repository.Version{Conn: test.conn}
	got, err := versions.GetByStrategyAndRev(ctx, "mean-rev", version.Rev)
	test.Require().NoError(err)
	test.Equal(version.Id, got.Id)
}

func (test *VersionTest) TestListByStrategy() {
	ctx := context.Background()
	err := insertService(ctx, test.conn, "algo:local")
	test.Require().NoError(err)

	strategies := repository.Strategy{Conn: test.conn}
	_, err = strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)
	_, err = strategies.Create(ctx, "other", nil)
	test.Require().NoError(err)

	versions := repository.Version{Conn: test.conn}
	_, err = versions.Create(ctx, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)
	_, err = versions.Create(ctx, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)
	_, err = versions.Create(ctx, "other", "algo:local", nil)
	test.Require().NoError(err)

	listed, err := versions.ListByStrategy(ctx, "mean-rev")
	test.Require().NoError(err)
	test.Len(listed, 2)
	test.Equal(int32(1), listed[0].Rev)
	test.Equal(int32(2), listed[1].Rev)
}
