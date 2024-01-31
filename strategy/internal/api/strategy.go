package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"go.uber.org/zap"
)

type strategyURI struct {
	Name string `uri:"name" binding:"required"`
}

func ListStrategies(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Strategy{Conn: pgx_tx}

	strategies, err := repository_s.List(c)
	if err != nil {
		log.Info("fail to list strategies", zap.Error(err))
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, strategies)
}

func CreateStrategy(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	strategy := new(entity.Strategy)
	err := c.BindJSON(strategy)
	if err != nil {
		log.Info("fail to bind body", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Strategy{Conn: pgx_tx}
	err = repository_s.Create(c, strategy)
	if err != nil {
		log.Info("fail to create strategy", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	log.Debug("strategy created", zap.Any("strategy", strategy))
	c.JSON(http.StatusCreated, strategy)
}

func GetStrategy(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	uri := new(strategyURI)
	err := c.BindUri(uri)
	if err != nil {
		log.Info("fail to bind uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Strategy{Conn: pgx_tx}
	strategy, err := repository_s.Get(c, uri.Name)
	if err != nil {
		log.Info("fail to get strategy", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	c.JSON(http.StatusOK, strategy)
}

func PatchStrategy(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	uri := new(strategyURI)
	err := c.BindUri(uri)
	if err != nil {
		log.Info("fail to bind uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	change := new(entity.Strategy)
	err = c.Bind(change)
	if err != nil {
		log.Info("fail to bind body", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Strategy{Conn: pgx_tx}

	if change.Backtest != nil {
		err = repository_s.SetBacktest(c, uri.Name, *change.Backtest)
		if err != nil {
			log.Info("fail to update strategy", zap.Error(err))
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
	}
	if change.Schedule != nil {
		err = repository_s.SetSchedule(c, uri.Name, *change.Schedule)
		if err != nil {
			log.Info("fail to update strategy", zap.Error(err))
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
	}

	strategy, err := repository_s.Get(c, uri.Name)
	if err != nil {
		log.Info("fail to get strategy", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	c.JSON(http.StatusOK, strategy)
}

func DeleteStrategy(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	uri := new(strategyURI)
	err := c.BindUri(uri)
	if err != nil {
		log.Info("fail to bind uri", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Strategy{Conn: pgx_tx}
	err = repository_s.Delete(c, uri.Name)
	if err != nil {
		log.Info("fail to delete strategy", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	c.Status(http.StatusNoContent)
}
