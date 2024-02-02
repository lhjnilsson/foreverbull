package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	LoggingDependency       = "logging"
	TXDependency            = "sql_tx"
	ConfigDependency        = "config"
	OrchestrationDependency = "stream_orchestration"
)

func NewEngine() *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(gin.Logger())
	return engine
}

func CorsHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func TransactionMiddleware(dependencyKey string, sql *pgxpool.Pool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tx, err := sql.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			ctx.AbortWithStatusJSON(DatabaseError(err))
			return
		}

		defer func() {
			if r := recover(); r != nil {
				tx.Rollback(ctx)
				panic(r)
			}
		}()

		ctx.Set(dependencyKey, tx)
		ctx.Next()

		if ctx.Writer.Status() >= 400 {
			tx.Rollback(ctx)
		} else {
			err = tx.Commit(ctx)
			if err != nil {
				ctx.AbortWithStatusJSON(DatabaseError(err))
				return
			}
		}
	}
}

func OrchestrationMiddleware(dependencyKey string, s stream.Stream) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		pendingOrch := stream.PendingOrchestration{}
		ctx.Set(dependencyKey, &pendingOrch)
		ctx.Next()
		for _, orch := range pendingOrch.Get() {
			err := s.CreateOrchestration(ctx, orch)
			if err != nil {
				panic(err)
			}
			err = s.RunOrchestration(ctx, orch.OrchestrationID)
			if err != nil {
				panic(err)
			}
		}
	}
}

func NewLifeCycleRouter(lc fx.Lifecycle, engine *gin.Engine, log *zap.Logger) error {

	server := http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%s", environment.GetHTTPPort()),
		Handler: engine,
	}

	engine.Static("/assets/", fmt.Sprintf("%s/assets/", environment.GetUIStaticPath()))
	engine.StaticFile("/", fmt.Sprintf("%s/index.html", environment.GetUIStaticPath()))
	engine.Use(CorsHeaders())
	engine.NoRoute(func(c *gin.Context) {
		if strings.Contains(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, APIError{Message: "Not found"})
			return
		}

		c.File(fmt.Sprintf("%s/index.html", environment.GetUIStaticPath()))
	})

	lc.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				go func() {
					if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						log.Error("error starting http- server", zap.Error(err))
					}
				}()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				return server.Shutdown(ctx)
			},
		},
	)
	return nil
}

type Route interface {
	Setup(group *gin.RouterGroup, pool *pgxpool.Pool) error
	Path() string
}

/*
APIError
message returned by handlers in case of error
*/
type APIError struct {
	Message string `json:"message"`
}

func DatabaseError(err error) (int, APIError) {
	if err == pgx.ErrNoRows {
		return 404, APIError{Message: "Not Found"}
	}

	// Postgres error codes https://www.postgresql.org/docs/current/errcodes-appendix.html
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "02000":
			return 404, APIError{Message: "No data"}
		case "23503":
			return 400, APIError{Message: "Foreign key constraint failed"}
		case "23505":
			return 409, APIError{Message: "Conflict"}
		case "23502":
			return 400, APIError{Message: "Value cant be null"}
		case "23514":
			return 400, APIError{Message: "Value does not meet requirements"}
		default:
			return 500, APIError{Message: pgError.Error()}
		}
	} else {
		return 500, APIError{Message: err.Error()}
	}
}
