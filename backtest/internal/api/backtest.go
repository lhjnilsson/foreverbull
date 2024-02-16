package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/backtest/api"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	bs "github.com/lhjnilsson/foreverbull/backtest/stream"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/rs/zerolog/log"
)

type backtestUri struct {
	Name string `uri:"name" binding:"required"`
}

func ListBacktests(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	repository_b := repository.Backtest{Conn: pgx_tx}

	backtests, err := repository_b.List(c)
	if err != nil {
		log.Err(err).Msg("error listing backtests")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, backtests)
}

func CreateBacktest(c *gin.Context) {
	stream := c.MustGet(OrchestrationDependency).(*stream.OrchestrationOutput)

	var body api.CreateBacktestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	start, err := api.ParseTime(body.Start)
	if err != nil {
		log.Debug().Err(err).Msg("error parsing start time")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	end, err := api.ParseTime(body.End)
	if err != nil {
		log.Debug().Err(err).Msg("error parsing end time")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_b := repository.Backtest{Conn: pgx_tx}
	backtest, err := repository_b.Create(c, body.Name, body.BacktestService, body.WorkerService,
		start, end, body.Calendar, body.Symbols, body.Benchmark)
	if err != nil {
		log.Err(err).Msg("error creating backtest")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	orch, err := bs.NewBacktestIngestOrchestration(backtest)
	if err != nil {
		log.Err(err).Msg("error creating backtest ingest orchestration")
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}
	stream.Add(orch)
	log.Info().Str("backtest", backtest.Name).Msg("created backtest")
	c.JSON(http.StatusCreated, backtest)
}

func GetBacktest(c *gin.Context) {
	var uri backtestUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_b := repository.Backtest{Conn: pgx_tx}

	manual, err := repository_b.Get(c, uri.Name)
	if err != nil {
		log.Err(err).Msg("error getting backtest")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, manual)
}

func UpdateBacktest(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	stream := c.MustGet(OrchestrationDependency).(*stream.OrchestrationOutput)

	var uri backtestUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	body := new(api.CreateBacktestBody)
	if err := c.ShouldBindJSON(body); err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	start, err := api.ParseTime(body.Start)
	if err != nil {
		log.Debug().Err(err).Msg("error parsing start time")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	end, err := api.ParseTime(body.End)
	if err != nil {
		log.Debug().Err(err).Msg("error parsing end time")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	repository_b := repository.Backtest{Conn: pgx_tx}

	backtest, err := repository_b.Update(c, uri.Name, body.BacktestService, body.WorkerService,
		start, end, body.Calendar, body.Symbols, body.Benchmark)
	if err != nil {
		log.Err(err).Msg("error updating backtest")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	orch, err := bs.NewBacktestIngestOrchestration(backtest)
	if err != nil {
		log.Err(err).Msg("error creating backtest ingest orchestration")
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}
	stream.Add(orch)
	log.Info().Str("backtest", backtest.Name).Msg("updated backtest")
	c.JSON(http.StatusOK, backtest)
}

func DeleteBacktest(c *gin.Context) {
	var uri backtestUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_b := repository.Backtest{Conn: pgx_tx}

	err := repository_b.Delete(c, uri.Name)
	if err != nil {
		log.Err(err).Msg("error deleting backtest")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
