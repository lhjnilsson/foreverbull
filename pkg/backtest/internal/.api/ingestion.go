package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	bs "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/rs/zerolog/log"
)

func CreateIngestion(c *gin.Context) {
	stream := c.MustGet(OrchestrationDependency).(*stream.OrchestrationOutput)
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	body := new(api.CreateIngestionBody)
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

	ingestions := repository.Ingestion{Conn: pgx_tx}
	ingestion, err := ingestions.Create(c, environment.GetBacktestIngestionDefaultName(), start, end, body.Calendar, body.Symbols)
	if err != nil {
		log.Err(err).Msg("error creating ingestion")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	orch, err := bs.NewIngestOrchestration(ingestion)
	if err != nil {
		log.Err(err).Msg("error creating backtest ingest orchestration")
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}
	stream.Add(orch)

	c.JSON(http.StatusCreated, ingestion)
}

func GetIngestion(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	ingestions := repository.Ingestion{Conn: pgx_tx}

	ingestion, err := ingestions.Get(c, environment.GetBacktestIngestionDefaultName())
	if err != nil {
		log.Err(err).Msg("error getting ingestion")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	c.JSON(http.StatusOK, ingestion)
}
