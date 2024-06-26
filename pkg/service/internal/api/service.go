package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/service/api"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	st "github.com/lhjnilsson/foreverbull/pkg/service/stream"
	"github.com/rs/zerolog/log"
)

func ListServices(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Service{Conn: pgx_tx}

	services, err := repository_s.List(c)
	if err != nil {
		log.Err(err).Msg("error listing services")
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, services)
}

func CreateService(c *gin.Context) {
	stream := c.MustGet(OrchestrationDependency).(*stream.OrchestrationOutput)

	s := new(api.CreateServiceRequest)
	err := c.BindJSON(s)
	if err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, err)
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Service{Conn: pgx_tx}

	service, err := repository_s.Create(c, s.Image)
	if err != nil {
		log.Err(err).Msg("error creating service")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	interviewOrchestration, err := st.NewServiceInterviewOrchestration(s.Image)
	if err != nil {
		log.Err(err).Msg("error creating service interview orchestration")
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}
	stream.Add(interviewOrchestration)
	log.Info().Str("image", s.Image).Msg("service created")
	c.JSON(http.StatusCreated, service)
}

func GetService(c *gin.Context) {
	var uri api.ServiceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	var image string
	if uri.Image[0] == '/' {
		image = uri.Image[1:]
	} else {
		image = uri.Image
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Service{Conn: pgx_tx}

	service, err := repository_s.Get(c, image)
	if err != nil {
		log.Err(err).Msg("error getting service")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, service)
}

func DeleteService(c *gin.Context) {
	var uri api.ServiceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	var image string
	if uri.Image[0] == '/' {
		image = uri.Image[1:]
	} else {
		image = uri.Image
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Service{Conn: pgx_tx}

	err := repository_s.Delete(c, image)
	if err != nil {
		log.Err(err).Msg("error deleting service")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	log.Info().Str("service", image).Msg("deleted service")
	c.JSON(http.StatusNoContent, nil)
}
