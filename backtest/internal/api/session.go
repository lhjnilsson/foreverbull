package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/backtest/api"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	bs "github.com/lhjnilsson/foreverbull/backtest/stream"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/rs/zerolog/log"
)

type sessionUri struct {
	ID string `uri:"id" binding:"required"`
}

func ListSessions(c *gin.Context) {
	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	sessions_b := repository.Session{Conn: pgx_tx}

	backtest, q := c.GetQuery("backtest")
	if q {
		sessions, err := sessions_b.ListByBacktest(c, backtest)
		if err != nil {
			log.Err(err).Msg("error listing sessions")
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		c.JSON(http.StatusOK, *sessions)
	} else {
		sessions, err := sessions_b.List(c)
		if err != nil {
			log.Err(err).Msg("error listing sessions")
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		c.JSON(http.StatusOK, *sessions)
	}
}

func CreateSession(c *gin.Context) {
	s := c.MustGet(OrchestrationDependency).(*stream.OrchestrationOutput)

	body := api.CreateSessionBody{}
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Debug().Err(err).Msg("error binding request")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}
	if body.Manual && len(body.Executions) > 0 {
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "manual session cannot have executions"})
		return
	} else if (!body.Manual) && len(body.Executions) == 0 {
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "automatic session must have executions"})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	repository_b := repository.Backtest{Conn: pgx_tx}
	sessions_b := repository.Session{Conn: pgx_tx}
	executions_b := repository.Execution{Conn: pgx_tx}

	backtest, err := repository_b.Get(c, body.Backtest)
	if err != nil {
		log.Err(err).Msg("error getting backtest")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}

	session, err := sessions_b.Create(c, backtest.Name, body.Manual)
	if err != nil {
		log.Err(err).Msg("error creating session")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	for _, e := range body.Executions {
		var start time.Time
		if e.Start == nil {
			start = backtest.Start
		} else {
			start, err = api.ParseTime(*e.Start)
			if err != nil {
				log.Debug().Err(err).Msg("error parsing start time")
				c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
				return
			}
			if start.Before(backtest.Start) {
				log.Debug().Err(err).Msg("start time before backtest start time")
				c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "start time before backtest start time"})
				return
			}
		}
		var end time.Time
		if e.End == nil {
			end = backtest.End
		} else {
			end, err = api.ParseTime(*e.End)
			if err != nil {
				log.Debug().Err(err).Msg("error parsing end time")
				c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
				return
			}
			if end.Before(start) {
				log.Debug().Err(err).Msg("end time before start time")
				c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "end time before start time"})
				return
			}
			if end.After(backtest.End) {
				log.Debug().Err(err).Msg("end time after backtest end time")
				c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "end time after backtest end time"})
				return
			}
		}
		if start.After(end) {
			log.Debug().Err(err).Msg("start time after end time")
			c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "start time after end time"})
			return
		}
		var symbols []string
		if e.Symbols == nil {
			symbols = backtest.Symbols
		} else {
			symbols = *e.Symbols
		}
		var benchmark *string
		if e.Benchmark == nil {
			benchmark = backtest.Benchmark
		} else {
			benchmark = e.Benchmark
		}

		_, err := executions_b.Create(c, session.ID, backtest.Calendar, start, end, symbols, benchmark)
		if err != nil {
			log.Err(err).Msg("error creating execution")
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
	}

	orchestration, err := bs.NewSessionRunOrchestration(backtest, session)
	if err != nil {
		log.Err(err).Msg("error creating session run orchestration")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	s.Add(orchestration)
	log.Info().Str("session", session.ID).Msg("created session")
	c.JSON(http.StatusCreated, session)
}

func GetSession(c *gin.Context) {

	var uri sessionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("error binding uri")
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	sessions_b := repository.Session{Conn: pgx_tx}

	session, err := sessions_b.Get(c, uri.ID)
	if err != nil {
		log.Err(err).Msg("error getting session")
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, session)
}
