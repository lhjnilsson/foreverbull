package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"github.com/rs/zerolog/log"
)

type strategyURI struct {
	Name string `uri:"name" binding:"required"`
}

func ListStrategies(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Strategy{Conn: pgx_tx}

	strategies, err := repository_s.List(c)
	if err != nil {
		log.Debug().Err(err).Msg("error listing strategies")
		c.JSON(http.StatusInternalServerError, internalHTTP.APIError{Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, strategies)
}

func CreateStrategy(c *gin.Context) {
	strategy := new(entity.Strategy)
	err := c.BindJSON(strategy)
	if err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Strategy{Conn: pgx_tx}
	err = repository_s.Create(c, strategy)
	if err != nil {
		log.Err(err).Msg("error creating strategy")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	log.Info().Str("name", *strategy.Name).Msg("strategy created")
	c.JSON(http.StatusCreated, strategy)
}

func GetStrategy(c *gin.Context) {
	uri := new(strategyURI)
	err := c.BindUri(uri)
	if err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Strategy{Conn: pgx_tx}
	strategy, err := repository_s.Get(c, uri.Name)
	if err != nil {
		log.Err(err).Msg("error getting strategy")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	c.JSON(http.StatusOK, strategy)
}

func PatchStrategy(c *gin.Context) {
	uri := new(strategyURI)
	err := c.BindUri(uri)
	if err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	change := new(entity.Strategy)
	err = c.Bind(change)
	if err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Strategy{Conn: pgx_tx}

	if change.Backtest != nil {
		err = repository_s.SetBacktest(c, uri.Name, *change.Backtest)
		if err != nil {
			log.Err(err).Msg("error updating strategy")
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
	}
	if change.Schedule != nil {
		err = repository_s.SetSchedule(c, uri.Name, *change.Schedule)
		if err != nil {
			log.Err(err).Msg("error updating strategy")
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
	}

	strategy, err := repository_s.Get(c, uri.Name)
	if err != nil {
		log.Err(err).Msg("error getting strategy")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	log.Info().Str("name", *strategy.Name).Msg("strategy updated")
	c.JSON(http.StatusOK, strategy)
}

func DeleteStrategy(c *gin.Context) {
	uri := new(strategyURI)
	err := c.BindUri(uri)
	if err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_s := repository.Strategy{Conn: pgx_tx}
	err = repository_s.Delete(c, uri.Name)
	if err != nil {
		log.Err(err).Msg("error deleting strategy")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	log.Info().Str("name", uri.Name).Msg("strategy deleted")
	c.Status(http.StatusNoContent)
}
