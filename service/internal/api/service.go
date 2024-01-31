package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/api"
	"github.com/lhjnilsson/foreverbull/service/container"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	st "github.com/lhjnilsson/foreverbull/service/stream"
	"go.uber.org/zap"
)

func ListServices(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Service{Conn: pgx_tx}

	services, err := repository_s.List(c)
	if err != nil {
		log.Error("Error listing services", zap.Error(err))
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, services)
}

func CreateService(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	stream := c.MustGet(OrchestrationDependency).(*stream.PendingOrchestration)

	s := new(api.CreateServiceRequest)
	err := c.BindJSON(s)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Service{Conn: pgx_tx}

	service, err := repository_s.Create(c, s.Name, s.Image)
	if err != nil {
		log.Error("Error creating service", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	interviewOrchestration, err := st.NewServiceInterviewOrchestration(s.Name)
	if err != nil {
		log.Error("Error creating interview orchestration", zap.Error(err))
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}
	stream.Add(interviewOrchestration)
	c.JSON(http.StatusCreated, service)
}

func GetService(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	var uri api.ServiceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Service{Conn: pgx_tx}

	service, err := repository_s.Get(c, uri.Name)
	if err != nil {
		log.Error("error during get of service", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, service)
}

func DeleteService(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	var uri api.ServiceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Service{Conn: pgx_tx}

	err := repository_s.Delete(c, uri.Name)
	if err != nil {
		log.Error("error during delete of service", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func GetServiceImage(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	container := c.MustGet("container").(container.Container)

	var uri api.ServiceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Service{Conn: pgx_tx}

	service, err := repository_s.Get(c, uri.Name)
	if err != nil {
		log.Error("error during get of service", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	info, err := container.Info(c, service.Image)
	if err != nil {
		log.Error("error during get of image info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

func UpdateService(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	//stream := c.MustGet(OrchestrationDependency).(*stream.PendingOrchestration)
	container := c.MustGet("container").(container.Container)

	var uri api.ServiceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Service{Conn: pgx_tx}

	s, err := repository_s.Get(c, uri.Name)
	if err != nil {
		log.Error("error during get of service", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	err = container.Pull(c, s.Image)
	if err != nil {
		log.Error("error during pull of image", zap.Error(err))
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}
	/*
		payload, err := json.Marshal(s)
		if err != nil {
			log.Error("fail to marshal service created payload", zap.Error(err))
			c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
			return
		}
		if err = stream.Publish(c, event.ImageUpdatedTopic, payload); err != nil {
			log.Error("fail to publish service created event", zap.Error(err))
			c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
			return
		}
	*/
	c.JSON(http.StatusCreated, s)
}
