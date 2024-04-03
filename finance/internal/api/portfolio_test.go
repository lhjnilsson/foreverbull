package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/internal/http"
	mocks "github.com/lhjnilsson/foreverbull/tests/mocks/finance/supplier"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"
)

type PortfolioTest struct {
	suite.Suite

	router  *gin.Engine
	trading *mocks.Trading
}

func (test *PortfolioTest) SetupTest() {
	test.trading = new(mocks.Trading)

	test.router = http.NewEngine()
	test.router.Use(
		func(ctx *gin.Context) {
			ctx.Set(TradingDependency, test.trading)
			ctx.Next()
		},
	)
	test.router.GET("/portfolio", GetPortfolio)
}

func TestPortfolio(t *testing.T) {
	suite.Run(t, new(PortfolioTest))
}

func (test *PortfolioTest) TestGetPortfolio() {
	portfolio := entity.Portfolio{
		Cash:      decimal.NewFromFloat(1000.45),
		Value:     decimal.NewFromFloat(0.0),
		Positions: make([]entity.Position, 0),
	}

	test.trading.On("GetPortfolio").Return(&portfolio, nil)

	req := httptest.NewRequest("GET", "/portfolio", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
	test.JSONEq(`{
		"cash": "1000.45",
		"portfolio_value": "0",
		"positions": []
	}`, w.Body.String())
}
