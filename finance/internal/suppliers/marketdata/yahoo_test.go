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
	asset, err := test.client.GetAsset("AAPL")
	test.Require().NoError(err)
	test.Require().NotNil(asset)

	fmt.Println("Asset: ", asset)
}

func (test *YahooTest) TestGetOHLC() {
	start, err := time.Parse("2006-01-02", "2021-01-01")
	test.Require().NoError(err)
	end, err := time.Parse("2006-01-02", "2021-02-01")
	test.Require().NoError(err)

	ohlc, err := test.client.GetOHLC("AAPL", start, end)
	test.Require().NoError(err)
	test.Require().NotNil(ohlc)

	fmt.Println("OHLC: ", (*ohlc)[0].Time)
}
