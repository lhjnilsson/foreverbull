package command

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/entity"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/dependency"
	fs "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	mockSupplier "github.com/lhjnilsson/foreverbull/tests/mocks/finance/supplier"
	mockStream "github.com/lhjnilsson/foreverbull/tests/mocks/internal_/stream"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type IngestCommandTest struct {
	suite.Suite

	db     *pgxpool.Pool
	assets *repository.Asset
	ohlc   *repository.OHLC

	storedAsset     entity.Asset
	storedOHLCStart time.Time
	storedOHLCEnd   time.Time

	marketdata *mockSupplier.Marketdata
}

func TestIngestCommand(t *testing.T) {
	suite.Run(t, new(IngestCommandTest))
}

func (test *IngestCommandTest) SetupTest() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})

	var err error
	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.TODO(), test.db)
	test.Require().NoError(err)
	test.assets = &repository.Asset{Conn: test.db}
	test.ohlc = &repository.OHLC{Conn: test.db}
	test.storedAsset = entity.Asset{
		Symbol: "Stored123",
		Name:   "Stored Asset",
	}
	test.Require().NoError(test.assets.Store(context.Background(), &test.storedAsset))

	test.storedOHLCStart = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	test.Require().NoError(test.ohlc.Store(context.Background(), "Stored123", &entity.OHLC{
		Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}))
	test.Require().NoError(test.ohlc.Store(context.Background(), "Stored123", &entity.OHLC{
		Time: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
	}))
	test.Require().NoError(test.ohlc.Store(context.Background(), "Stored123", &entity.OHLC{
		Time: time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC),
	}))
	test.storedOHLCEnd = time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC)

	test.marketdata = new(mockSupplier.Marketdata)
}

func (test *IngestCommandTest) TearDownTest() {
}

func (test *IngestCommandTest) TestIngestCommandNoIngestion() {
	m := new(mockStream.Message)
	m.On("MustGet", stream.DBDep).Return(test.db)
	m.On("MustGet", dependency.MarketDataDep).Return(test.marketdata)

	m.On("ParsePayload", &fs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*fs.IngestCommand)
		command.Symbols = []string{test.storedAsset.Symbol}
		command.Start = test.storedOHLCStart
		command.End = test.storedOHLCEnd
	})
	err := Ingest(context.Background(), m)
	test.NoError(err)
}

func (test *IngestCommandTest) TestIngestCommandIngestNewOHLC() {
	m := new(mockStream.Message)
	m.On("MustGet", stream.DBDep).Return(test.db)
	m.On("MustGet", dependency.MarketDataDep).Return(test.marketdata)

	test.marketdata.On("GetOHLC", test.storedAsset.Symbol, test.storedOHLCStart, test.storedOHLCEnd.Add(time.Hour*24)).Return(
		&[]entity.OHLC{
			{
				Time: time.Date(2020, 1, 4, 0, 0, 0, 0, time.UTC),
			},
		},
		nil,
	)

	m.On("ParsePayload", &fs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*fs.IngestCommand)
		command.Symbols = []string{test.storedAsset.Symbol}
		command.Start = test.storedOHLCStart
		command.End = test.storedOHLCEnd.Add(time.Hour * 24)
	})
	err := Ingest(context.Background(), m)
	test.NoError(err)

	exists, err := test.ohlc.Exists(context.Background(), []string{test.storedAsset.Symbol}, test.storedOHLCStart, test.storedOHLCEnd.Add(time.Hour*24))
	test.NoError(err)
	test.True(exists)

	test.marketdata.AssertExpectations(test.T())
}

func (test *IngestCommandTest) TestIngestCommandIngestAll() {
	m := new(mockStream.Message)
	m.On("MustGet", stream.DBDep).Return(test.db)
	m.On("MustGet", dependency.MarketDataDep).Return(test.marketdata)

	newAsset := entity.Asset{
		Symbol: "NEW123",
		Name:   "New Asset",
	}
	test.marketdata.On("GetAsset", newAsset.Symbol).Return(&newAsset, nil)
	test.marketdata.On("GetOHLC", "NEW123", test.storedOHLCStart, test.storedOHLCEnd).Return(
		&[]entity.OHLC{
			{
				Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			{
				Time: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
			},
			{
				Time: time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC),
			},
		},
		nil,
	)

	m.On("ParsePayload", &fs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*fs.IngestCommand)
		command.Symbols = []string{"NEW123"}
		command.Start = test.storedOHLCStart
		command.End = test.storedOHLCEnd
	})
	err := Ingest(context.Background(), m)
	test.NoError(err)

	asset, err := test.assets.Get(context.Background(), newAsset.Symbol)
	test.NoError(err)
	test.Equal(newAsset.Name, asset.Name)

	exists, err := test.ohlc.Exists(context.Background(), []string{"NEW123"}, test.storedOHLCStart, test.storedOHLCEnd)
	test.NoError(err)
	test.True(exists)

	test.marketdata.AssertExpectations(test.T())
}
