package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"go.uber.org/zap"
)

func ListExecutions(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	uri := new(strategyURI)
	err := c.BindUri(uri)
	if err != nil {
		log.Info("fail to bind uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_e := repository.Execution{Conn: pgx_tx}
	executions, err := repository_e.List(c, uri.Name)
	if err != nil {
		log.Info("fail to list executions", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	c.JSON(http.StatusOK, executions)
}

type CreateExecutionPayload struct {
	Strategy string `json:"strategy" binding:"required"`
}

func CreateExecution(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	//stream := c.MustGet(internalHTTP.StreamOrchDep).(*stream.PendingOrchestration)

	payload := new(CreateExecutionPayload)
	err := c.BindJSON(payload)
	if err != nil {
		log.Info("fail to bind json", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_e := repository.Execution{Conn: pgx_tx}

	execution, err := repository_e.Create(c, payload.Strategy)
	if err != nil {
		log.Info("fail to create execution", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	/*bytes, err := json.Marshal(execution)
	if err != nil {
		log.Info("fail to marshal execution", zap.Error(err))
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}

	stream.Publish(c, event.ExecutionCreatedTopic, bytes)
	*/
	c.JSON(http.StatusCreated, execution)
}
