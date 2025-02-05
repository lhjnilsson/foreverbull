package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/dependency"
	fs "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
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

func (test *IngestCommandTest) TestIngestCommandIngest() {
	message := new(stream.MockMessage)
	message.On("MustGet", stream.DBDep).Return(test.db)
	message.On("MustGet", dependency.MarketDataDep).Return(test.marketdata)

	newAsset := pb.Asset{
		Symbol: "NEW123",
		Name:   "New Asset",
	}
	test.marketdata.On("GetAsset", newAsset.Symbol).Return(&newAsset, nil)
	test.marketdata.On("GetOHLC", "NEW123", internal_pb.DateToTime(test.storedOHLCStart),
		mock.Anything).Return(
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

	message.On("ParsePayload", &fs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*fs.IngestCommand)
		command.Symbols = []string{"NEW123"}
		command.Start = internal_pb.DateToDateString(test.storedOHLCStart)
		end := internal_pb.DateToDateString(test.storedOHLCEnd)
		command.End = &end
	})

	err := command.Ingest(context.Background(), message)
	test.Require().NoError(err)

	asset, err := test.assets.Get(context.Background(), newAsset.Symbol)
	test.Require().NoError(err)
	test.Equal(newAsset.Name, asset.Name)

	exists, err := test.ohlc.Exists(context.Background(), []string{"NEW123"}, test.storedOHLCStart, test.storedOHLCEnd)
	test.Require().NoError(err)
	test.True(exists)

	test.marketdata.AssertExpectations(test.T())
}
