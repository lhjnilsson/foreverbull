package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
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
	// TODO: implement
	c.JSON(http.StatusNotImplemented, internalHTTP.APIError{Message: "not implemented"})
}
