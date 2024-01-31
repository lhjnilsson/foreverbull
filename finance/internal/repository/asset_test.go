package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type AssetTests struct {
	suite.Suite
	conn         *pgxpool.Pool
	assetStorage *Asset
	ohlcStorage  *OHLC
	a1           entity.Asset
	a2           entity.Asset
	a3           entity.Asset
}

func (s *AssetTests) SetupTest() {
	var err error
	config := helper.TestingConfig(s.T(), &helper.Containers{
		Postgres: true,
	})
	s.conn, err = pgxpool.New(context.Background(), config.PostgresURI)
	s.NoError(err)
	err = Recreate(context.Background(), s.conn)
	s.Require().Nil(err)
	s.assetStorage = &Asset{Conn: s.conn}
	s.ohlcStorage = &OHLC{Conn: s.conn}
}

func (s *AssetTests) TearDownTest() {
	s.conn.Close()
}

func (s *AssetTests) LoadSampleData() {
	s.a1 = entity.Asset{Symbol: "ABC123", Name: "Comany ABC"}
	s.a2 = entity.Asset{Symbol: "DEF456", Name: "Comany DEF"}
	s.a3 = entity.Asset{Symbol: "GHI789", Name: "Comany GHI"}
	err := s.assetStorage.Store(context.TODO(), &s.a1)
	s.Require().Nil(err)
	err = s.assetStorage.Store(context.TODO(), &s.a2)
	s.Require().Nil(err)
	err = s.assetStorage.Store(context.TODO(), &s.a3)
	s.Require().Nil(err)
}

func TestAsset(t *testing.T) {
	suite.Run(t, new(AssetTests))
}

func (s *AssetTests) TestListWithoutOHLC() {
	assets, err := s.assetStorage.List(context.TODO())
	s.Require().Nil(err)
	s.Require().Len(*assets, 0)
	s.LoadSampleData()
	assets, err = s.assetStorage.List(context.TODO())
	s.Require().Nil(err)
	s.Require().Len(*assets, 3)
}

func (s *AssetTests) TestListWithOHLC() {
	s.LoadSampleData()
	ohlc := entity.OHLC{Time: time.Now(), Open: 1.0, High: 2.0, Low: 0.5, Close: 1.5, Volume: 1000}
	assets, er := s.assetStorage.List(context.TODO())
	s.Require().Nil(er)
	s.Require().Len(*assets, 3)
	for _, asset := range *assets {
		s.Require().Nil(asset.StartOHLC)
		s.Require().Nil(asset.EndOHLC)
	}

	err := s.ohlcStorage.Store(context.TODO(), s.a1.Symbol, &ohlc)
	s.Require().Nil(err)
	err = s.ohlcStorage.Store(context.TODO(), s.a2.Symbol, &ohlc)
	s.Require().Nil(err)
	err = s.ohlcStorage.Store(context.TODO(), s.a3.Symbol, &ohlc)
	s.Require().Nil(err)
	assets, err = s.assetStorage.List(context.TODO())
	s.Require().Nil(err)
	s.Require().Len(*assets, 3)
	for _, asset := range *assets {
		s.Require().NotNil(asset.StartOHLC)
		s.Require().NotNil(asset.EndOHLC)
	}
}

func (s *AssetTests) TestListBySymbolsNormal() {
	assets, err := s.assetStorage.ListBySymbols(context.TODO(), []string{"ABC123", "DEF456"})
	s.Require().Empty(assets)
	s.Require().NotNil(err)
	s.LoadSampleData()
	assets, err = s.assetStorage.ListBySymbols(context.TODO(), []string{"ABC123", "DEF456"})
	s.Require().Nil(err)
	s.Require().Len(*assets, 2)
}

func (s *AssetTests) TestListBySymbolNotStored() {
	s.LoadSampleData()
	assets, err := s.assetStorage.ListBySymbols(context.TODO(), []string{"ABC123", "DEF456"})
	s.Require().Nil(err)
	s.Require().Len(*assets, 2)
	assets, err = s.assetStorage.ListBySymbols(context.TODO(), []string{"ABC123", "DEF456", "OAB333"})
	s.Require().Nil(assets)
	s.Require().NotNil(err)
	s.Require().Equal("not all symbols found", err.Error())
}

func (s *AssetTests) TestStoreNormal() {
	a1 := entity.Asset{Symbol: "ABC123", Name: "Comany ABC"}
	err := s.assetStorage.Store(context.TODO(), &a1)
	s.Require().Nil(err)
	assets, err := s.assetStorage.List(context.TODO())
	s.Require().Nil(err)
	s.Require().Len(*assets, 1)
	s.Require().Equal(a1.Symbol, (*assets)[0].Symbol)
	s.Require().Equal(a1.Name, (*assets)[0].Name)
}

func (s *AssetTests) TestGetNormal() {
	s.LoadSampleData()
	asset, err := s.assetStorage.Get(context.TODO(), "ABC123")
	s.Require().Nil(err)
	s.Require().NotNil(asset)
	s.Require().Equal("ABC123", asset.Symbol)
	s.Require().Equal("Comany ABC", asset.Name)
}

func (s *AssetTests) TestGetNotFound() {
	s.LoadSampleData()
	asset, err := s.assetStorage.Get(context.TODO(), "ABC123")
	s.Require().Nil(err)
	s.Require().NotNil(asset)
	s.Require().Equal("ABC123", asset.Symbol)
	s.Require().Equal("Comany ABC", asset.Name)
	asset, err = s.assetStorage.Get(context.TODO(), "ABC1234")
	s.Require().NotNil(err)
	s.Require().Nil(asset)
	s.Equal(err, pgx.ErrNoRows)
}

func (s *AssetTests) TestDeleteNormal() {
	s.LoadSampleData()
	err := s.assetStorage.Delete(context.TODO(), "ABC123")
	s.Require().Nil(err)
	assets, err := s.assetStorage.List(context.TODO())
	s.Require().Nil(err)
	s.Require().Len(*assets, 2)
}
