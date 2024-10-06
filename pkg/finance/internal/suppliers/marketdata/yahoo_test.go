package marketdata

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type YahooTest struct {
	suite.Suite

	client *YahooClient
}

func (test *YahooTest) SetupTest() {
	client, err := NewYahooClient()
	test.Require().NoError(err)
	test.client = client.(*YahooClient)
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
		{"---", "", fmt.Errorf("Quote not found for ticker symbol: ---")},
	}

	for _, tc := range testCases {
		asset, err := test.client.GetAsset(tc.Symbol)
		if tc.expectedError != nil {
			test.Error(err)
			test.Equal(tc.expectedError.Error(), err.Error())
		} else {
			test.NoError(err)
			test.NotNil(asset)
			test.Equal(tc.ExpectedName, asset.Name)
		}
	}
}

func (test *YahooTest) TestGetIndex() {
	type TestCase struct {
		Symbol      string
		ExpectedErr error
	}

	testCases := []TestCase{
		{"^FCHI", nil},
		{"^DJI", nil},
		{"^OMX", nil},
	}
	for _, tc := range testCases {
		assets, err := test.client.GetIndex(tc.Symbol)
		fmt.Println(assets)
		if tc.ExpectedErr != nil {
			test.Error(err)
			test.Equal(tc.ExpectedErr.Error(), err.Error())
		} else {
			test.NoError(err)
			test.NotNil(assets)
			test.NotEqual(0, len(assets))
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
		{"NON_EXISTING", "2021-01-01", "2021-02-01", 0, fmt.Errorf("fail to get OHLC data for symbol NON_EXISTING: No data found, symbol may be delisted")},
	}

	for _, tc := range testCases {
		start, err := time.Parse("2006-01-02", tc.Start)
		test.Require().NoError(err)
		end, err := time.Parse("2006-01-02", tc.End)
		test.Require().NoError(err)

		ohlc, err := test.client.GetOHLC(tc.Symbol, start, end)
		if tc.ExpectedErr != nil {
			test.Error(err)
			test.Equal(tc.ExpectedErr.Error(), err.Error())
		} else {
			test.NoError(err)
			test.NotNil(ohlc)
			test.Equal(tc.ExpectedLength, len(ohlc))
		}
	}

}
