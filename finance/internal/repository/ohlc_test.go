package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type OHLCTests struct {
	suite.Suite
	conn        *pgxpool.Pool
	asset       entity.Asset
	ohlcStorage *OHLC
}

func (s *OHLCTests) SetupTest() {
	var err error

	config := helper.TestingConfig(s.T(), &helper.Containers{
		Postgres: true,
	})
	s.conn, err = pgxpool.New(context.Background(), config.PostgresURI)
	s.NoError(err)
	err = Recreate(context.Background(), s.conn)
	s.Require().Nil(err)
	assetStorage := Asset{Conn: s.conn}
	s.asset = entity.Asset{Symbol: "ABC", Name: "Comany ABC"}
	err = assetStorage.Store(context.TODO(), &s.asset)
	s.Require().Nil(err)

	s.conn.Exec(context.TODO(), "DROP TABLE ohlc;")
	_, err = s.conn.Exec(context.TODO(), OHLCTable)
	s.Require().Nil(err)
	s.ohlcStorage = &OHLC{Conn: s.conn}

}

func (s *OHLCTests) TearDownTest() {
	s.conn.Close()
}

func TestOHLC(t *testing.T) {
	suite.Run(t, new(OHLCTests))
}

func (s *OHLCTests) SampleOHLC() (string, time.Time, time.Time) {
	count := 5
	ohlcStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	ohlcTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i <= count; i++ {
		ohlc := entity.OHLC{Time: ohlcTime}
		err := s.ohlcStorage.Store(context.TODO(), s.asset.Symbol, &ohlc)
		s.Require().Nil(err)
		if i != count {
			ohlcTime = ohlcTime.Add(time.Hour * 24)
		}
	}
	return s.asset.Symbol, ohlcStart, ohlcTime
}

func (s *OHLCTests) TestStore() {
	ohlc := entity.OHLC{Time: time.Now()}
	err := s.ohlcStorage.Store(context.TODO(), s.asset.Symbol, &ohlc)
	s.Require().Nil(err)
}

func (s *OHLCTests) TestExists() {
	symbol, start, end := s.SampleOHLC()
	exists, err := s.ohlcStorage.Exists(context.TODO(), []string{symbol}, start, end)
	s.Require().Nil(err)
	s.Require().True(exists)
}

func (s *OHLCTests) TestExistsOnlyDate() {
	symbol, start, end := s.SampleOHLC()
	start = start.Add(time.Hour * 3)
	end = end.Add(time.Hour * 3)
	exists, err := s.ohlcStorage.Exists(context.TODO(), []string{symbol}, start, end)
	s.Require().Nil(err)
	s.Require().True(exists)
}

func (s *OHLCTests) TestExistsNot() {
	symbol, start, end := s.SampleOHLC()
	end = end.Add(time.Hour * 24)
	exists, err := s.ohlcStorage.Exists(context.TODO(), []string{symbol}, start, end)
	s.Require().Nil(err)
	s.Require().False(exists)
}

func (s *OHLCTests) TestMinMaxNothingStored() {
	min, max, err := s.ohlcStorage.MinMax(context.Background())
	s.Require().Nil(err)
	s.Require().Nil(min)
	s.Require().Nil(max)
}

func (s *OHLCTests) TestMinMax() {
	_, start, end := s.SampleOHLC()
	min, max, err := s.ohlcStorage.MinMax(context.Background())
	s.Require().Nil(err)
	s.Require().Equal(start, *min)
	s.Require().Equal(end, *max)
}
