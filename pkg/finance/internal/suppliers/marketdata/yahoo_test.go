package marketdata_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/stretchr/testify/suite"
)

type YahooTest struct {
	suite.Suite

	client *marketdata.YahooClient
}

func (test *YahooTest) SetupTest() {
	if os.Getenv("FOREVERBULL_LIVE_HTTP") == "" {
		test.T().Skip("skipping live Yahoo HTTP; set FOREVERBULL_LIVE_HTTP=1 to run")
	}
	client, err := marketdata.NewYahooClient()
	test.Require().NoError(err)
	test.client = client.(*marketdata.YahooClient)
}

func TestYahooClient(t *testing.T) {
	suite.Run(t, new(YahooTest))
}

func (test *YahooTest) TestGetAsset() {
	type TestCase struct {
		Symbol        string
		ExpectedName  string
		expectedError error
	}

	testCases := []TestCase{
		{"AAPL", "Apple Inc.", nil},
		{"GOOGL", "Alphabet Inc.", nil},
		{"MSFT", "Microsoft Corporation", nil},
		{"^DJI", "Dow Jones Industrial Average", nil},
		{"NVDA", "NVIDIA Corporation", nil},
		{"HD", "The Home Depot, Inc.", nil},
		{"---", "", errors.New("Quote not found for symbol: ---")},
	}

	for _, testCase := range testCases {
		asset, err := test.client.GetAsset(context.Background(), testCase.Symbol)
		if testCase.expectedError != nil {
			test.Require().Error(err)
			test.Equal(testCase.expectedError.Error(), err.Error())
		} else {
			test.Require().NoError(err)
			test.NotNil(asset)
			test.Equal(testCase.ExpectedName, asset.Name)
		}
	}
}

func (test *YahooTest) TestSearchAsset() {
	asset, err := test.client.SearchAsset(context.Background(), "US02079K3059")
	test.Require().NoError(err)
	test.NotNil(asset)
}

func (test *YahooTest) TestGetIndex() {
	type TestCase struct {
		Symbol      string
		ExpectedErr error
	}

	testCases := []TestCase{
		//{"^FCHI", nil},
		{"^DJI", nil},
		{"^OMX", nil},
	}
	for _, testCase := range testCases {
		assets, err := test.client.GetIndex(context.Background(), testCase.Symbol)
		if testCase.ExpectedErr != nil {
			test.Require().Error(err)
			test.Equal(testCase.ExpectedErr.Error(), err.Error())
		} else {
			test.Require().NoError(err)
			test.NotNil(assets)
			test.NotEmpty(assets)
		}
	}
}

func (test *YahooTest) TestGetOHLC() {
	type TestCase struct {
		Symbol         string
		Start          string
		End            string
		ExpectedLength int
		ExpectedErr    error
	}

	testCases := []TestCase{
		{"AAPL", "2021-01-01", "2021-02-01", 19, nil},
		{"GOOGL", "2015-01-01", "2024-02-01", 2285, nil},
		{"^DJI", "2021-01-01", "2021-02-01", 19, nil},
		{"NON_EXISTING", "2021-01-01", "2021-02-01", 0, errors.New("fail to get OHLC data for symbol NON_EXISTING: No data found, symbol may be delisted")},
	}

	for _, testCase := range testCases {
		start, err := time.Parse("2006-01-02", testCase.Start)
		test.Require().NoError(err)
		end, err := time.Parse("2006-01-02", testCase.End)
		test.Require().NoError(err)

		ohlc, err := test.client.GetOHLC(context.Background(), testCase.Symbol, start, &end)
		if testCase.ExpectedErr != nil {
			test.Require().Error(err)
			test.Equal(testCase.ExpectedErr.Error(), err.Error())
		} else {
			test.Require().NoError(err)
			test.NotNil(ohlc)
			test.Len(ohlc, testCase.ExpectedLength)
		}
	}
}

func (test *YahooTest) TestGetOHLCNoEnd() {
	type TestCase struct {
		Symbol      string
		Start       string
		ExpectedErr error
	}

	testCases := []TestCase{
		{"AAPL", "2021-01-01", nil},
		{"GOOGL", "2015-01-01", nil},
	}

	for _, testCase := range testCases {
		start, err := time.Parse("2006-01-02", testCase.Start)
		test.Require().NoError(err)

		ohlc, err := test.client.GetOHLC(context.Background(), testCase.Symbol, start, nil)
		if testCase.ExpectedErr != nil {
			test.Require().Error(err)
			test.Equal(testCase.ExpectedErr.Error(), err.Error())
		} else {
			test.Require().NoError(err)
			test.NotNil(ohlc)
		}
	}
}

// Yahoo has no bars under FB and serves the history from before the
// 2021-10-31 rename under META, so bars of a renamed ticker are read
// through asset_symbol (pkg/backtest/universe.md §7.4).
func (test *YahooTest) TestGetOHLCRenamedTickerIsServedUnderTheNewTicker() {
	start := time.Date(2021, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2021, 11, 30, 0, 0, 0, 0, time.UTC)

	_, err := test.client.GetOHLC(context.Background(), "FB", start, &end)
	test.Require().Error(err)

	bars, err := test.client.GetOHLC(context.Background(), "META", start, &end)
	test.Require().NoError(err)
	test.Require().NotEmpty(bars)
	test.Equal("2021-10-01", bars[0].Timestamp.AsTime().UTC().Format("2006-01-02"))
}
