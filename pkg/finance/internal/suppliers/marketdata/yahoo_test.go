package marketdata_test

import (
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
		asset, err := test.client.GetAsset(testCase.Symbol)
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
	asset, err := test.client.SearchAsset("US02079K3059")
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
		assets, err := test.client.GetIndex(testCase.Symbol)
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

		ohlc, err := test.client.GetOHLC(testCase.Symbol, start, &end)
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

		ohlc, err := test.client.GetOHLC(testCase.Symbol, start, nil)
		if testCase.ExpectedErr != nil {
			test.Require().Error(err)
			test.Equal(testCase.ExpectedErr.Error(), err.Error())
		} else {
			test.Require().NoError(err)
			test.NotNil(ohlc)
		}
	}
}
