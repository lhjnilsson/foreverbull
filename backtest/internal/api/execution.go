package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"go.uber.org/zap"
)

type executionUri struct {
	ID     string `uri:"id"`
	Metric string `uri:"metric"`
}

func ListExecutions(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	executions_b := repository.Execution{Conn: pgx_tx}

	session, ok := c.GetQuery("session")
	if ok {
		executions, err := executions_b.ListBySession(c, session)
		if err != nil {
			log.Info("fail to list backtests", zap.Error(err))
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		c.JSON(http.StatusOK, *executions)
		return
	} else {
		executions, err := executions_b.List(c)
		if err != nil {
			log.Info("fail to list backtests", zap.Error(err))
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		c.JSON(http.StatusOK, *executions)
		return
	}
}

func GetExecution(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Error("error binding uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	executions_b := repository.Execution{Conn: pgx_tx}

	backtest, err := executions_b.Get(c, uri.ID)
	if err != nil {
		log.Error("error getting backtest", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, backtest)
}

func GetExecutionPeriods(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Error("error binding uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	periods_b := repository.Period{Conn: pgx_tx}

	periods, err := periods_b.List(c, uri.ID)
	if err != nil {
		log.Error("error getting periods", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, periods)
}

func GetExecutionPeriodMetrics(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Error("error binding uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	periods_b := repository.Period{Conn: pgx_tx}

	periodMetrics, err := periods_b.Metrics(c, uri.ID)
	if err != nil {
		log.Error("error getting period metrics", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, periodMetrics)
}

func GetExecutionPeriodMetric(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Error("error binding uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	periods_b := repository.Period{Conn: pgx_tx}

	periodMetrics, err := periods_b.Metric(c, uri.ID, uri.Metric)
	if err != nil {
		log.Error("error getting period metrics", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, periodMetrics)
}

func GetExecutionOrders(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Error("error binding uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	repository_o := repository.Order{Conn: pgx_tx}

	orders, err := repository_o.List(c, uri.ID)
	if err != nil {
		log.Error("error when receiving orders", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	log.Info("successfully received orders")
	c.JSON(http.StatusOK, orders)
}

func GetExecutionPortfolio(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Error("error binding uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	repository_p := repository.Portfolio{Conn: pgx_tx}

	positions, err := repository_p.GetLatest(c, uri.ID)
	if err != nil {
		log.Error("error when receiving positions", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	log.Info("successfully received positions")
	c.JSON(http.StatusOK, positions)
}

func GetExecutionDataframe(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	storage := c.MustGet(StorageDependency).(storage.BlobStorage)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Error("error binding uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	result, err := storage.GetResultInfo(c, uri.ID)
	if err != nil {
		log.Info("fail to get backtest result", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, result)
}
