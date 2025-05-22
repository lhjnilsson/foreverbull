package trading_test

import (
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
	_, found := os.LookupEnv(environment.AlpacaBaseURL)
	if !found {
		test.T().Skip("Skipping test because no Alpaca base URL is set")
	}
	client, err := trading.NewAlpacaClient()
	test.Require().NoError(err)
	test.client = client
}

func TestAlpacaClient(t *testing.T) {
	suite.Run(t, new(AlpacaClientTest))
}

func (test *AlpacaClientTest) TestGetPortfolio() {
	portfolio, err := test.client.GetPortfolio()
	test.Require().NoError(err)
	test.T().Log(portfolio)
}
