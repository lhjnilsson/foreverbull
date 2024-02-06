package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
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

	if c.Request.URL.Query().Has("service") {
		instances, err = repository_i.ListByService(c, c.Request.URL.Query().Get("service"))
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
