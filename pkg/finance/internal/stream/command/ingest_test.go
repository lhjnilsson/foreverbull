package command

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internal_pb "github.com/lhjnilsson/foreverbull/internal/pb"
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
	storedOHLCStart *internal_pb.Date
	storedOHLCEnd   *internal_pb.Date

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

	test.storedOHLCStart = &internal_pb.Date{Year: 2020, Month: 1, Day: 1}
	test.Require().NoError(test.ohlc.Store(context.Background(),
		"Stored123", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), 1.0, 2.0, 3.0, 4.0, 5))
	test.Require().NoError(test.ohlc.Store(context.Background(),
		"Stored123", time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), 1.0, 2.0, 3.0, 4.0, 5))
	test.Require().NoError(test.ohlc.Store(context.Background(),
		"Stored123", time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC), 1.0, 2.0, 3.0, 4.0, 5))
	test.storedOHLCEnd = &internal_pb.Date{Year: 2020, Month: 1, Day: 3}

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
		command.Start = internal_pb.DateToDateString(test.storedOHLCStart)
		command.End = internal_pb.DateToDateString(test.storedOHLCEnd)
	})
	err := Ingest(context.Background(), m)
	test.NoError(err)
}

func (test *IngestCommandTest) TestIngestCommandIngestNewOHLC() {
	m := new(stream.MockMessage)
	m.On("MustGet", stream.DBDep).Return(test.db)
	m.On("MustGet", dependency.MarketDataDep).Return(test.marketdata)

	test.marketdata.On("GetOHLC", test.storedAsset.Symbol,
		internal_pb.DateToTime(test.storedOHLCStart),
		internal_pb.DateToTime(test.storedOHLCStart).Add(time.Hour*24)).Return(
		[]*pb.OHLC{
			{
				Timestamp: timestamppb.New(time.Date(2020, 1, 4, 0, 0, 0, 0, time.UTC)),
			},
		},
		nil,
	)

	m.On("ParsePayload", &fs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*fs.IngestCommand)
		command.Symbols = []string{test.storedAsset.Symbol}
		command.Start = internal_pb.DateToDateString(test.storedOHLCStart)
		test.storedOHLCEnd.Day += 1
		command.End = internal_pb.DateToDateString(test.storedOHLCEnd)
	})
	err := Ingest(context.Background(), m)
	test.NoError(err)

	test.storedOHLCEnd.Day += 1

	exists, err := test.ohlc.Exists(context.Background(), []string{test.storedAsset.Symbol},
		test.storedOHLCStart,
		test.storedOHLCEnd)
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
		[]*pb.OHLC{
			{
				Timestamp: timestamppb.New(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
			{
				Timestamp: timestamppb.New(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)),
			},
			{
				Timestamp: timestamppb.New(time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC)),
			},
		},
		nil,
	)

	m.On("ParsePayload", &fs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*fs.IngestCommand)
		command.Symbols = []string{"NEW123"}
		command.Start = internal_pb.DateToDateString(test.storedOHLCStart)
		command.End = internal_pb.DateToDateString(test.storedOHLCEnd)
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
