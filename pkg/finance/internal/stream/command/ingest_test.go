package command

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	fs "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type IngestCommandTest struct {
	suite.Suite

	db     *pgxpool.Pool
	assets *repository.Asset
	ohlc   *repository.OHLC

	storedAsset     *pb.Asset
	storedOHLCStart time.Time
	storedOHLCEnd   time.Time

	marketdata *supplier.MockMarketdata
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
	test.storedAsset = &pb.Asset{
		Symbol: "Stored123",
		Name:   "Stored Asset",
	}
	test.Require().NoError(test.assets.Store(context.Background(), test.storedAsset.Symbol, test.storedAsset.Name))

	test.storedOHLCStart = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	test.storedOHLCEnd = time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC)

	test.marketdata = new(supplier.MockMarketdata)
}

func (test *IngestCommandTest) TearDownTest() {
}

func (test *IngestCommandTest) TestIngestCommandNoIngestion() {
	m := new(stream.MockMessage)
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
	m := new(stream.MockMessage)
	m.On("MustGet", stream.DBDep).Return(test.db)
	m.On("MustGet", dependency.MarketDataDep).Return(test.marketdata)

	test.marketdata.On("GetOHLC", test.storedAsset.Symbol, test.storedOHLCStart, test.storedOHLCEnd.Add(time.Hour*24)).Return(
		&[]pb.OHLC{
			{
				Timestamp: timestamppb.New(time.Now()),
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
	m := new(stream.MockMessage)
	m.On("MustGet", stream.DBDep).Return(test.db)
	m.On("MustGet", dependency.MarketDataDep).Return(test.marketdata)

	newAsset := pb.Asset{
		Symbol: "NEW123",
		Name:   "New Asset",
	}
	test.marketdata.On("GetAsset", newAsset.Symbol).Return(&newAsset, nil)
	test.marketdata.On("GetOHLC", "NEW123", test.storedOHLCStart, test.storedOHLCEnd).Return(
		&[]pb.OHLC{
			{
				Timestamp: timestamppb.New(time.Now()),
			},
			{
				Timestamp: timestamppb.New(time.Now()),
			},
			{
				Timestamp: timestamppb.New(time.Now()),
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
