package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/rs/zerolog/log"
)

type executionUri struct {
	ID     string `uri:"id"`
	Metric string `uri:"metric"`
}

func ListExecutions(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	executions_b := repository.Execution{Conn: pgx_tx}

	session, ok := c.GetQuery("session")
	if ok {
		executions, err := executions_b.ListBySession(c, session)
		if err != nil {
			log.Err(err).Msg("error listing executions")
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		c.JSON(http.StatusOK, *executions)
		return
	} else {
		executions, err := executions_b.List(c)
		if err != nil {
			log.Err(err).Msg("error listing executions")
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		c.JSON(http.StatusOK, *executions)
		return
	}
}

func GetExecution(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	executions_b := repository.Execution{Conn: pgx_tx}

	backtest, err := executions_b.Get(c, uri.ID)
	if err != nil {
		log.Err(err).Msg("error getting execution")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, backtest)
}

func GetExecutionPeriods(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	periods_b := repository.Period{Conn: pgx_tx}

	periods, err := periods_b.List(c, uri.ID)
	if err != nil {
		log.Err(err).Msg("error getting periods")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, periods)
}

func GetExecutionPeriodMetrics(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	periods_b := repository.Period{Conn: pgx_tx}
	periodMetrics, err := periods_b.Metrics(c, uri.ID)
	if err != nil {
		log.Err(err).Msg("error getting period metrics")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, periodMetrics)
}

func GetExecutionPeriodMetric(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	periods_b := repository.Period{Conn: pgx_tx}

	periodMetrics, err := periods_b.Metric(c, uri.ID, uri.Metric)
	if err != nil {
		log.Err(err).Msg("error getting period metric")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, periodMetrics)
}

func GetExecutionOrders(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	repository_o := repository.Order{Conn: pgx_tx}

	orders, err := repository_o.List(c, uri.ID)
	if err != nil {
		log.Err(err).Msg("error getting orders")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, orders)
}

func GetExecutionPortfolio(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	repository_p := repository.Portfolio{Conn: pgx_tx}
	positions, err := repository_p.GetLatest(c, uri.ID)
	if err != nil {
		log.Err(err).Msg("error getting portfolio")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, positions)
}

func GetExecutionDataframe(c *gin.Context) {
	storage := c.MustGet(StorageDependency).(storage.BlobStorage)

	var uri executionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	result, err := storage.GetResultInfo(c, uri.ID)
	if err != nil {
		log.Err(err).Msg("error getting dataframe")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, result)
}
