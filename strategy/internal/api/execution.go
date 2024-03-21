package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/strategy/api"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	ss "github.com/lhjnilsson/foreverbull/strategy/stream"
	"github.com/rs/zerolog/log"
)

type executionUri struct {
	ID string `uri:"id"`
}

func ListExecutions(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)

	executions_b := repository.Execution{Conn: pgx_tx}

	strategy, ok := c.GetQuery("strategy")
	if ok {
		executions, err := executions_b.List(c, strategy)
		if err != nil {
			log.Err(err).Msg("error listing executions")
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		c.JSON(http.StatusOK, *executions)
	} else {
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "missing strategy"})
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

	execution, err := executions_b.Get(c, uri.ID)
	if err != nil {
		log.Err(err).Msg("error getting execution")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, *execution)
}

func CreateExecution(c *gin.Context) {
	s := c.MustGet(OrchestrationDependency).(*stream.OrchestrationOutput)

	body := api.CreateExecutionBody{}
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	strategies_b := repository.Strategy{Conn: pgx_tx}
	executions_b := repository.Execution{Conn: pgx_tx}

	strategy, err := strategies_b.Get(c, body.Strategy)
	if err != nil {
		log.Err(err).Msg("error getting strategy")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	end := time.Now()
	start := end.Add(-time.Minute * time.Duration(strategy.MinDays*24*60))

	execution, err := executions_b.Create(c, strategy.Name, start, end, strategy.Service)
	if err != nil {
		log.Err(err).Msg("error creating execution")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	orchestration, err := ss.RunStrategyExecutionOrchestration(strategy, execution)
	if err != nil {
		log.Err(err).Msg("error creating execution run orchestration")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	s.Add(orchestration)
	log.Info().Str("execution", execution.ID).Msg("created execution")
	c.JSON(http.StatusCreated, execution)
}
