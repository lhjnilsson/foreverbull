package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/stretchr/testify/suite"
)

type OHLCTests struct {
	suite.Suite
	conn        *pgxpool.Pool
	asset       pb.Asset
	ohlcStorage OHLC
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
	err = Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
	assetStorage := Asset{Conn: test.conn}
	test.asset = pb.Asset{Symbol: "ABC", Name: "Comany ABC"}
	err = assetStorage.Store(context.TODO(), test.asset.Symbol, test.asset.Name)
	test.Require().NoError(err)

	_, err = test.conn.Exec(context.TODO(), "DROP TABLE IF EXISTS ohlc;")
	test.Require().NoError(err)
	_, err = test.conn.Exec(context.TODO(), OHLCTable)
	test.Require().NoError(err)
	test.ohlcStorage = OHLC{Conn: test.conn}
}

func (test *OHLCTests) TearDownTest() {
	test.conn.Close()
}

func TestOHLC(t *testing.T) {
	suite.Run(t, new(OHLCTests))
}

func (test *OHLCTests) SampleOHLC() (string, time.Time, time.Time) {
	count := 5
	ohlcStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	ohlcTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i <= count; i++ {
		err := test.ohlcStorage.Store(context.TODO(), test.asset.Symbol, ohlcTime, 1.2, 1.3, 1.1, 1.2, 1000)
		test.Nil(err)
		if i != count {
			ohlcTime = ohlcTime.Add(time.Hour * 24)
		}
	}
	return test.asset.Symbol, ohlcStart, ohlcTime
}

func (test *OHLCTests) TestStore() {
	err := test.ohlcStorage.Store(context.TODO(), test.asset.Symbol, time.Now(), 1.2, 1.3, 1.1, 1.2, 1000)
	test.Nil(err)
}

func (test *OHLCTests) TestExists() {
	symbol, start, end := test.SampleOHLC()
	exists, err := test.ohlcStorage.Exists(context.TODO(), []string{symbol}, start, end)
	test.Nil(err)
	test.True(exists)
}

func (test *OHLCTests) TestExistsOnlyDate() {
	symbol, start, end := test.SampleOHLC()
	start = start.Add(time.Hour * 3)
	end = end.Add(time.Hour * 3)
	exists, err := test.ohlcStorage.Exists(context.TODO(), []string{symbol}, start, end)
	test.Nil(err)
	test.True(exists)
}

func (test *OHLCTests) TestExistsNot() {
	symbol, start, end := test.SampleOHLC()
	end = end.Add(time.Hour * 24)
	exists, err := test.ohlcStorage.Exists(context.TODO(), []string{symbol}, start, end)
	test.Nil(err)
	test.False(exists)
}

func (test *OHLCTests) TestMinMaxNothingStored() {
	min, max, err := test.ohlcStorage.MinMax(context.Background())
	test.Nil(err)
	test.Nil(min)
	test.Nil(max)
}

func (test *OHLCTests) TestMinMax() {
	_, start, end := test.SampleOHLC()
	min, max, err := test.ohlcStorage.MinMax(context.Background())
	test.Nil(err)
	test.Equal(start, *min)
	test.Equal(end, *max)
}
