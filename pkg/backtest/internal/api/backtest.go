package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
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
	ingestion_b := repository.Ingestion{Conn: c.MustGet(TXDependency).(pgx.Tx)}
	i, err := ingestion_b.Get(c, environment.GetBacktestIngestionDefaultName())
	if err != nil {
		log.Err(err).Msg("error getting ingestion")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "No ingestion found, create one before creating a backtest"})
		return
	}

	if i.Statuses[0].Status != entity.IngestionStatusCompleted {
		log.Debug().Msg("ingestion not ready")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "Ingestion not ready"})
		return
	}

	var body api.CreateBacktestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	var start time.Time
	if body.Start != nil {
		start, err = api.ParseTime(*body.Start)
		if err != nil {
			log.Debug().Err(err).Msg("error parsing start time")
			c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
			return
		}
		if start.Before(i.Start) {
			log.Debug().Msg("start time before ingestion start time")
			c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "Start time before ingestion start time"})
			return
		}
	} else {
		start = i.Start
	}

	var end time.Time
	if body.End != nil {
		end, err = api.ParseTime(*body.End)
		if err != nil {
			log.Debug().Err(err).Msg("error parsing end time")
			c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
			return
		}
		if end.After(i.End) {
			log.Debug().Msg("end time after ingestion end time")
			c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "End time after ingestion end time"})
			return
		}
	} else {
		end = i.End
	}

	var symbols []string
	if body.Symbols != nil && len(*body.Symbols) > 0 {
		for _, symbol := range *body.Symbols {
			if !slices.Contains(i.Symbols, symbol) {
				log.Debug().Str("symbol", symbol).Msg("symbol not in ingestion")
				c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "Symbol not in ingestion"})
				return
			}
		}
		symbols = *body.Symbols
	} else {
		symbols = i.Symbols
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_b := repository.Backtest{Conn: pgx_tx}
	backtest, err := repository_b.Create(c, body.Name, body.Service,
		start, end, i.Calendar, symbols, body.Benchmark)
	if err != nil {
		log.Err(err).Msg("error creating backtest")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	log.Info().Any("backtest", backtest).Msg("created backtest")
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

	var uri backtestUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	ingestion_b := repository.Ingestion{Conn: pgx_tx}
	i, err := ingestion_b.Get(c, environment.GetBacktestIngestionDefaultName())
	if err != nil {
		log.Err(err).Msg("error getting ingestion")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "No ingestion found, create one before creating a backtest"})
		return
	}

	body := new(api.CreateBacktestBody)
	if err := c.ShouldBindJSON(body); err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	start, err := api.ParseTime(*body.Start)
	if err != nil {
		log.Debug().Err(err).Msg("error parsing start time")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	if start.Before(i.Start) {
		log.Debug().Msg("start time before ingestion start time")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "Start time before ingestion start time"})
		return
	}

	end, err := api.ParseTime(*body.End)
	if err != nil {
		log.Debug().Err(err).Msg("error parsing end time")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	if end.After(i.End) {
		log.Debug().Msg("end time after ingestion end time")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "End time after ingestion end time"})
		return
	}

	for _, symbol := range *body.Symbols {
		if !slices.Contains(i.Symbols, symbol) {
			log.Debug().Str("symbol", symbol).Msg("symbol not in ingestion")
			c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "Symbol not in ingestion"})
			return
		}
	}

	repository_b := repository.Backtest{Conn: pgx_tx}

	backtest, err := repository_b.Update(c, uri.Name, body.Service,
		start, end, i.Calendar, *body.Symbols, body.Benchmark)
	if err != nil {
		log.Err(err).Msg("error updating backtest")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
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
