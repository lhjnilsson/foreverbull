package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/backtest/api"
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	bs "github.com/lhjnilsson/foreverbull/backtest/stream"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"go.uber.org/zap"
)

type sessionUri struct {
	ID string `uri:"id" binding:"required"`
}

type CreateSessionPayload struct {
	Backtest   string             `json:"backtest" binding:"required"`
	Source     string             `json:"source" binding:"required"`
	SourceKey  string             `json:"source_key" binding:"required"`
	Executions []entity.Execution `json:"executions"`
}

func ListSessions(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	sessions_b := repository.Session{Conn: pgx_tx}

	backtest, q := c.GetQuery("backtest")
	if q {
		sessions, err := sessions_b.ListByBacktest(c, backtest)
		if err != nil {
			log.Info("fail to list sessions", zap.Error(err))
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		c.JSON(http.StatusOK, *sessions)
	} else {
		sessions, err := sessions_b.List(c)
		if err != nil {
			log.Info("fail to list sessions", zap.Error(err))
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
		c.JSON(http.StatusOK, *sessions)
	}
}

func CreateSession(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)
	s := c.MustGet(OrchestrationDependency).(*stream.PendingOrchestration)

	body := api.CreateSessionBody{}
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Error("error binding request", zap.Error(err))
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
		log.Error("error getting backtest", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	if backtest.Statuses[0].Status != entity.BacktestStatusReady {
		log.Error("backtest not ready", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "backtest not ready"})
		return
	}

	session, err := sessions_b.Create(c, backtest.Name, body.Manual)
	if err != nil {
		log.Error("error creating session", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	log.Info("Session: ", zap.Any("session", session))
	for _, e := range body.Executions {
		var start time.Time
		if e.Start == nil {
			start = backtest.Start
		} else {
			start, err = api.ParseTime(*e.Start)
			if err != nil {
				log.Error("error parsing start time", zap.Error(err))
				c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
				return
			}
			if start.Before(backtest.Start) {
				log.Error("start time before backtest start time", zap.Error(err))
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
				log.Error("error parsing end time", zap.Error(err))
				c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
				return
			}
			if end.Before(start) {
				log.Error("end time before start time", zap.Error(err))
				c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "end time before start time"})
				return
			}
			if end.After(backtest.End) {
				log.Error("end time after backtest end time", zap.Error(err))
				c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: "end time after backtest end time"})
				return
			}
		}
		if start.After(end) {
			log.Error("start time after end time", zap.Error(err))
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
			log.Error("error creating execution", zap.Error(err))
			c.JSON(internalHTTP.DatabaseError(err))
			return
		}
	}

	orchestration, err := bs.NewSessionRunOrchestration(backtest, session)
	if err != nil {
		log.Error("error creating orchestration", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	s.Add(orchestration)
	c.JSON(http.StatusCreated, session)
}

func GetSession(c *gin.Context) {
	log := c.MustGet(LoggingDependency).(*zap.Logger)

	var uri sessionUri
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Error("error binding request", zap.Error(err))
		c.JSON(http.StatusBadRequest, internalHTTP.APIError{Message: err.Error()})
		return
	}

	pgx_tx := c.MustGet(TXDependency).(pgx.Tx)
	sessions_b := repository.Session{Conn: pgx_tx}

	session, err := sessions_b.Get(c, uri.ID)
	if err != nil {
		log.Info("fail to get session", zap.Error(err))
		c.JSON(internalHTTP.DatabaseError(err))
		return
	}
	c.JSON(http.StatusOK, session)
}
