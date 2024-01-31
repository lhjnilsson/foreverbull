package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	"go.uber.org/zap"
)

type instanceURI struct {
	InstanceID string `uri:"instanceID" binding:"required"`
}

func ListInstances(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
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
		log.Error("Issue listing instances", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	c.JSON(http.StatusOK, instances)
}

func GetInstance(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	var uri instanceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug("Issue binding uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_i := repository.Instance{Conn: pgx_tx}

	instance, err := repository_i.Get(c, uri.InstanceID)
	if err != nil {
		log.Error("Issue getting instance", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, instance)
}

// PatchInstance
func PatchInstance(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	var uri instanceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug("Issue binding uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	change := new(entity.Instance)
	err := c.ShouldBindJSON(&change)
	if err != nil && err.Error() != "EOF" {
		log.Debug("Issue binding json", zap.Error(err), zap.Any("change", change))
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_i := repository.Instance{Conn: pgx_tx}

	_, err = repository_i.Get(c, uri.InstanceID)
	if err != nil {
		log.Error("Issue getting instance", zap.Error(err), zap.String("id", uri.InstanceID))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	var instance *entity.Instance

	if change.Host != nil && change.Port != nil {
		err = repository_i.UpdateHostPort(c, uri.InstanceID, *change.Host, *change.Port)
		if err != nil {
			log.Error("Issue updating instance on database", zap.Error(err))
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		err = repository_i.UpdateStatus(c, uri.InstanceID, entity.InstanceStatusRunning, nil)
	} else {
		err = repository_i.UpdateStatus(c, uri.InstanceID, entity.InstanceStatusStopped, nil)
	}
	if err != nil {
		log.Error("Issue updating instance on database", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	instance, err = repository_i.Get(c, uri.InstanceID)
	if err != nil {
		log.Error("Issue getting instance", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	log.Info("Instance updated", zap.Any("instance", instance))
	c.JSON(http.StatusOK, instance)
}
