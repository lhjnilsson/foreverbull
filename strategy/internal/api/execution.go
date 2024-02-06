package api

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
)

func ListExecutions(c *gin.Context) {
	uri := new(strategyURI)
	err := c.BindUri(uri)
	if err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_e := repository.Execution{Conn: pgx_tx}
	executions, err := repository_e.List(c, uri.Name)
	if err != nil {
		log.Err(err).Msg("error listing executions")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	c.JSON(http.StatusOK, executions)
}

type CreateExecutionPayload struct {
	Strategy string `json:"strategy" binding:"required"`
}

func CreateExecution(c *gin.Context) {
	payload := new(CreateExecutionPayload)
	err := c.BindJSON(payload)
	if err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_e := repository.Execution{Conn: pgx_tx}

	execution, err := repository_e.Create(c, payload.Strategy)
	if err != nil {
		log.Err(err).Msg("error creating execution")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	/*bytes, err := json.Marshal(execution)
	if err != nil {
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}

	stream.Publish(c, event.ExecutionCreatedTopic, bytes)
	*/
	c.JSON(http.StatusCreated, execution)
}
