package trading_test

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/stretchr/testify/suite"
)

type AlpacaClientTest struct {
	suite.Suite

	client *trading.AlpacaClient
}

func (test *AlpacaClientTest) SetupTest() {
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
