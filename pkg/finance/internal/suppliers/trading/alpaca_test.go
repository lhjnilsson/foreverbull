package trading_test

import (
	"context"
	"os"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/stretchr/testify/suite"
)

type AlpacaClientTest struct {
	suite.Suite

	client *trading.AlpacaClient
}

func (test *AlpacaClientTest) SetupTest() {
	if os.Getenv(environment.AlpacaApiKey) == "" || os.Getenv(environment.AlpacaApiSecret) == "" {
		test.T().Skip("skipping live Alpaca HTTP; set ALPACA_MARKETS_API_KEY and ALPACA_MARKETS_API_SECRET to run")
	}
	client, err := trading.NewAlpacaClient()
	test.Require().NoError(err)
	test.client = client
}

func TestAlpacaClient(t *testing.T) {
	suite.Run(t, new(AlpacaClientTest))
}

func (test *AlpacaClientTest) TestGetPortfolio() {
	portfolio, err := test.client.GetPortfolio(context.Background())
	test.Require().NoError(err)
	test.T().Log(portfolio)
}
