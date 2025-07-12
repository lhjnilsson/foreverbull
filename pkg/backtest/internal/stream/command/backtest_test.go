package command_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	bs "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CreateBacktestTest struct {
	suite.Suite

	db         *pgxpool.Pool
	etfscraper *dependency.MockETFScraper
	mdclient   *financeServicePB.MockMarketdataClient
}

func TestCreateBacktest(t *testing.T) {
	suite.Run(t, new(CreateBacktestTest))
}

func (test *CreateBacktestTest) SetupSuite() {
	var err error
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})

	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.db)
	test.Require().NoError(err)

	test.etfscraper = new(dependency.MockETFScraper)
	test.mdclient = new(financeServicePB.MockMarketdataClient)

	backtest := backtest.Backtest{
		Name: "test-backtest",
		Periods: []*backtest.Backtest_Period{
			{
				StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
			},
			{
				StartDate: &pb.Date{Year: 2023, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2023, Month: 12, Day: 31},
			},
		},
	}
	backtests := repository.Backtest{Conn: test.db}
	_, err = backtests.Create(context.TODO(), &backtest)
	test.Require().NoError(err)
}

func (test *CreateBacktestTest) TearDownSuite() {
}

func (test *CreateBacktestTest) SetupTest() {
}

func (test *CreateBacktestTest) TearDownTest() {
}

func (test *CreateBacktestTest) TestCreateBacktest() {
	message := new(stream.MockMessage)
	message.On("ParsePayload", &bs.CreateBacktestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.CreateBacktestCommand)
		payload.BacktestName = "test-backtest"
		payload.ETFName = "iShares S&P 100"
	})
	message.On("MustGet", stream.DBDep).Return(test.db)
	message.On("MustGet", dependency.GetMarketdataClient).Return(test.mdclient)

	assetMap := map[string][]dependency.Asset{
		"2022": []dependency.Asset{
			{
				Symbol:    "AAPL",
				Name:      "Apple Inc.",
				Sector:    "Technology",
				AssetType: "Equity",
				ISIN:      "US0000000000",
			},
		},
		"2023": []dependency.Asset{
			{
				Symbol:    "TSLA",
				Name:      "Tesla, Inc.",
				Sector:    "Technology",
				AssetType: "Equity",
				ISIN:      "US0000000000",
			},
		},
	}
	test.etfscraper.On("GetAssets", "iShares S&P 100", "2022", "2023").Return(assetMap, nil)
	message.On("MustGet", dependency.GetETFScraper).Return(test.etfscraper)

	test.mdclient.On("GetAsset", mock.Anything, &financeServicePB.GetAssetRequest{
		Identifier: &financeServicePB.GetAssetRequest_Symbol{
			Symbol: "AAPL",
		},
	}).Return(&financeServicePB.GetAssetResponse{
		Asset: &finance.Asset{
			Symbol: "AAPL",
		},
	}, nil)
	test.mdclient.On("GetAsset", mock.Anything, &financeServicePB.GetAssetRequest{
		Identifier: &financeServicePB.GetAssetRequest_Symbol{
			Symbol: "TSLA",
		},
	}).Return(&financeServicePB.GetAssetResponse{
		Asset: &finance.Asset{
			Symbol: "TSLA",
		},
	}, nil)

	err := command.CreateBacktest(context.TODO(), message)
	test.Require().NoError(err)

	backtests := repository.Backtest{Conn: test.db}
	backtest, err := backtests.Get(context.TODO(), "test-backtest")
	test.Require().NoError(err)
	test.Require().NotNil(backtest)
}
