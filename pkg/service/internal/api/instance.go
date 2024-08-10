package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/pkg/service/api"
	"github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	"github.com/rs/zerolog/log"
)

type instanceURI struct {
	InstanceID string `uri:"instanceID" binding:"required"`
}

func ListInstances(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_i := repository.Instance{Conn: pgx_tx}

	var instances *[]entity.Instance
	var err error

	if c.Request.URL.Query().Has("image") {
		instances, err = repository_i.ListByImage(c, c.Request.URL.Query().Get("image"))
	} else {
		instances, err = repository_i.List(c)
	}

	if err != nil {
		log.Err(err).Msg("error listing instances")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	c.JSON(http.StatusOK, instances)
}

func GetInstance(c *gin.Context) {

	var uri instanceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_i := repository.Instance{Conn: pgx_tx}

	instance, err := repository_i.Get(c, uri.InstanceID)
	if err != nil {
		log.Err(err).Msg("error getting instance")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, instance)
}

// PatchInstance
func PatchInstance(c *gin.Context) {
	var uri instanceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	change := new(entity.Instance)
	err := c.ShouldBindJSON(&change)
	if err != nil && err.Error() != "EOF" {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_i := repository.Instance{Conn: pgx_tx}

	_, err = repository_i.Get(c, uri.InstanceID)
	if err != nil {
		log.Err(err).Msg("error getting instance")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	var instance *entity.Instance

	if change.Host != nil && change.Port != nil {
		err = repository_i.UpdateHostPort(c, uri.InstanceID, *change.Host, *change.Port)
		if err != nil {
			log.Err(err).Msg("error updating instance")
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		err = repository_i.UpdateStatus(c, uri.InstanceID, entity.InstanceStatusRunning, nil)
	} else {
		err = repository_i.UpdateStatus(c, uri.InstanceID, entity.InstanceStatusStopped, nil)
	}
	if err != nil {
		log.Err(err).Msg("error updating instance")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	instance, err = repository_i.Get(c, uri.InstanceID)
	if err != nil {
		log.Err(err).Msg("error getting instance")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	log.Info().Str("id", uri.InstanceID).Msg("updated instance")
	c.JSON(http.StatusOK, instance)
}

func ConfigureInstance(c *gin.Context) {
	var uri instanceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	var request api.ConfigureInstanceRequest
	err := c.ShouldBindJSON(&request)
	if err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_i := repository.Instance{Conn: pgx_tx}

	err = repository_i.UpdateBrokerPort(c, uri.InstanceID, request.BrokerPort)
	if err != nil {
		log.Err(err).Msg("error updating instance")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	err = repository_i.UpdateNamespacePort(c, uri.InstanceID, request.NamespacePort)
	if err != nil {
		log.Err(err).Msg("error updating instance")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	err = repository_i.UpdateDatabaseURL(c, uri.InstanceID, request.DatabaseURL)
	if err != nil {
		log.Err(err).Msg("error updating instance")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	instance, err := repository_i.Get(c, uri.InstanceID)
	if err != nil {
		log.Err(err).Msg("error getting instance")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	err = instance.Configure(&request.Functions)
	if err != nil {
		log.Err(err).Msg("error configuring instance")
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	log.Info().Str("id", uri.InstanceID).Msg("instance configured")
	c.JSON(http.StatusOK, instance)
}

func StopInstance(c *gin.Context) {
	var uri instanceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_i := repository.Instance{Conn: pgx_tx}

	instance, err := repository_i.Get(c, uri.InstanceID)
	if err != nil {
		log.Err(err).Msg("error getting instance")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	err = instance.Stop()
	if err != nil {
		log.Err(err).Msg("error stopping instance")
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	log.Info().Str("id", uri.InstanceID).Msg("instance stopped")
	c.JSON(http.StatusOK, instance)
}
