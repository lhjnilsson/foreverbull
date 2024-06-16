package api

import (
	"github.com/gin-gonic/gin"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
)

func GetPortfolio(c *gin.Context) {
	trading := c.MustGet(TradingDependency).(supplier.Trading)

	portfolio, err := trading.GetPortfolio()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, portfolio)
}
