package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/strategy/api"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"github.com/rs/zerolog/log"
)

type strategyURI struct {
	Name string `uri:"name" binding:"required"`
}

func ListStrategies(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	strategies_b := repository.Strategy{Conn: pgx_tx}

	strategies, err := strategies_b.List(c)
	if err != nil {
		log.Err(err).Msg("error listing strategies")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, *strategies)
}

func CreateStrategy(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	strategies_b := repository.Strategy{Conn: pgx_tx}

	body := api.CreateStrategyBody{}
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	s, err := strategies_b.Create(c, body.Name, body.Symbols, body.MinDays, body.Service)
	if err != nil {
		log.Err(err).Msg("error creating strategy")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusCreated, s)
}

func GetStrategy(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	strategies_b := repository.Strategy{Conn: pgx_tx}

	var uri strategyURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	s, err := strategies_b.Get(c, uri.Name)
	if err != nil {
		log.Err(err).Msg("error getting strategy")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, s)
}

func DeleteStrategy(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	strategies_b := repository.Strategy{Conn: pgx_tx}

	var uri strategyURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	err := strategies_b.Delete(c, uri.Name)
	if err != nil {
		log.Err(err).Msg("error deleting strategy")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.Status(http.StatusNoContent)
}
