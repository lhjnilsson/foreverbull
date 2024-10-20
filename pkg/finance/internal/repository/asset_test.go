package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/stretchr/testify/suite"
)

type AssetTests struct {
	suite.Suite
	conn         *pgxpool.Pool
	assetStorage repository.Asset
	ohlcStorage  repository.OHLC
	a1           pb.Asset
	a2           pb.Asset
	a3           pb.Asset
}

func (test *AssetTests) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *AssetTests) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
	test.assetStorage = repository.Asset{Conn: test.conn}
	test.ohlcStorage = repository.OHLC{Conn: test.conn}
}

func (test *AssetTests) TearDownTest() {
	test.conn.Close()
}

func (test *AssetTests) LoadSampleData() {
	test.a1 = pb.Asset{Symbol: "ABC123", Name: "Company ABC"}
	test.a2 = pb.Asset{Symbol: "DEF456", Name: "Company DEF"}
	test.a3 = pb.Asset{Symbol: "GHI789", Name: "Company GHI"}
	err := test.assetStorage.Store(context.TODO(), test.a1.Symbol, test.a1.Name)
	test.Require().NoError(err)
	err = test.assetStorage.Store(context.TODO(), test.a2.Symbol, test.a2.Name)
	test.Require().NoError(err)
	err = test.assetStorage.Store(context.TODO(), test.a3.Symbol, test.a3.Name)
	test.Require().NoError(err)
}

func TestAsset(t *testing.T) {
	suite.Run(t, new(AssetTests))
}

func (test *AssetTests) TestListWithoutOHLC() {
	assets, err := test.assetStorage.List(context.TODO())
	test.Require().NoError(err)
	test.Empty(assets)
	test.LoadSampleData()
	assets, err = test.assetStorage.List(context.TODO())
	test.Require().NoError(err)
	test.Len(assets, 3)
}

func (test *AssetTests) TestListBySymbolsNormal() {
	assets, err := test.assetStorage.ListBySymbols(context.TODO(), []string{"ABC123", "DEF456"})
	test.Empty(assets)
	test.Error(err)
	test.LoadSampleData()
	assets, err = test.assetStorage.ListBySymbols(context.TODO(), []string{"ABC123", "DEF456"})
	test.Require().NoError(err)
	test.Len(assets, 2)
}

func (test *AssetTests) TestListBySymbolNotStored() {
	test.LoadSampleData()
	assets, err := test.assetStorage.ListBySymbols(context.TODO(), []string{"ABC123", "DEF456"})
	test.Require().NoError(err)
	test.Len(assets, 2)
	assets, err = test.assetStorage.ListBySymbols(context.TODO(), []string{"ABC123", "DEF456", "OAB333"})
	test.Nil(assets)
	test.Error(err)
	test.Equal("not all symbols found", err.Error())
}

func (test *AssetTests) TestStoreNormal() {
	asset := pb.Asset{Symbol: "ABC123", Name: "Company ABC"}
	err := test.assetStorage.Store(context.TODO(), asset.Symbol, asset.Name)
	test.Require().NoError(err)
	assets, err := test.assetStorage.List(context.TODO())
	test.Require().NoError(err)
	test.Require().Len(assets, 1)
	test.Equal(asset.Symbol, assets[0].Symbol)
	test.Equal(asset.Name, assets[0].Name)
}

func (test *AssetTests) TestGetNormal() {
	test.LoadSampleData()
	asset, err := test.assetStorage.Get(context.TODO(), "ABC123")
	test.Require().NoError(err)
	test.NotNil(asset)
	test.Equal("ABC123", asset.Symbol)
	test.Equal("Company ABC", asset.Name)
}

func (test *AssetTests) TestGetNotFound() {
	test.LoadSampleData()
	asset, err := test.assetStorage.Get(context.TODO(), "ABC123")
	test.Require().NoError(err)
	test.NotNil(asset)
	test.Equal("ABC123", asset.Symbol)
	test.Equal("Company ABC", asset.Name)
	asset, err = test.assetStorage.Get(context.TODO(), "ABC1234")
	test.Error(err)
	test.Nil(asset)
	test.Equal(err, pgx.ErrNoRows)
}

func (test *AssetTests) TestDeleteNormal() {
	test.LoadSampleData()
	err := test.assetStorage.Delete(context.TODO(), "ABC123")
	test.Require().NoError(err)
	assets, err := test.assetStorage.List(context.TODO())
	test.Require().NoError(err)
	test.Len(assets, 2)
}
