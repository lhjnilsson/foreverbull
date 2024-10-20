package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internal_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/stretchr/testify/suite"
)

type OHLCTests struct {
	suite.Suite
	conn        *pgxpool.Pool
	asset       pb.Asset
	ohlcStorage repository.OHLC
}

func (test *OHLCTests) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *OHLCTests) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
	assetStorage := repository.Asset{Conn: test.conn}
	test.asset = pb.Asset{Symbol: "ABC", Name: "Comany ABC"}
	err = assetStorage.Store(context.TODO(), test.asset.Symbol, test.asset.Name)
	test.Require().NoError(err)

	_, err = test.conn.Exec(context.TODO(), "DROP TABLE IF EXISTS ohlc;")
	test.Require().NoError(err)
	_, err = test.conn.Exec(context.TODO(), repository.OHLCTable)
	test.Require().NoError(err)
	test.ohlcStorage = repository.OHLC{Conn: test.conn}
}

func (test *OHLCTests) TearDownTest() {
	test.conn.Close()
}

func TestOHLC(t *testing.T) {
	suite.Run(t, new(OHLCTests))
}

func (test *OHLCTests) SampleOHLC() (string, *internal_pb.Date, *internal_pb.Date) {
	count := 5
	ohlcStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	ohlcTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i <= count; i++ {
		err := test.ohlcStorage.Store(context.TODO(), test.asset.Symbol, ohlcTime, 1.2, 1.3, 1.1, 1.2, 1000)
		test.NoError(err)

		if i != count {
			ohlcTime = ohlcTime.Add(time.Hour * 24)
		}
	}

	return test.asset.Symbol, internal_pb.GoTimeToDate(ohlcStart), internal_pb.GoTimeToDate(ohlcTime)
}

func (test *OHLCTests) TestStore() {
	err := test.ohlcStorage.Store(context.TODO(), test.asset.Symbol, time.Now(), 1.2, 1.3, 1.1, 1.2, 1000)
	test.NoError(err)
}

func (test *OHLCTests) TestExists() {
	symbol, start, end := test.SampleOHLC()
	exists, err := test.ohlcStorage.Exists(context.TODO(), []string{symbol}, start, end)
	test.NoError(err)
	test.True(exists)
}

func (test *OHLCTests) TestExistsNot() {
	symbol, start, end := test.SampleOHLC()
	end.Day++
	exists, err := test.ohlcStorage.Exists(context.TODO(), []string{symbol}, start, end)
	test.NoError(err)
	test.False(exists)
}

func (test *OHLCTests) TestMinMaxNothingStored() {
	storedMin, storedMax, err := test.ohlcStorage.MinMax(context.Background())
	test.NoError(err)
	test.Nil(storedMin)
	test.Nil(storedMax)
}

func (test *OHLCTests) TestMinMax() {
	_, start, end := test.SampleOHLC()
	storedMin, storedMax, err := test.ohlcStorage.MinMax(context.Background())
	test.NoError(err)
	test.Equal(start, storedMin)
	test.Equal(end, storedMax)
}
