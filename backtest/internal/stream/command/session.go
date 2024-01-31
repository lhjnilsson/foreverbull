package command

import (
	"context"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/backtest/internal/backtest"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/backtest/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/backtest/stream"
	"github.com/lhjnilsson/foreverbull/internal/config"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"go.uber.org/zap"
)

func UpdateSessionStatus(ctx context.Context, message stream.Message) error {
	db := message.MustGet(stream.DBDep).(postgres.Query)

	command := ss.UpdateSessionStatusCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling UpdateSessionStatus payload: %w", err)
	}

	sessions := repository.Session{Conn: db}
	err = sessions.UpdateStatus(ctx, command.SessionID, command.Status, command.Error)
	if err != nil {
		return fmt.Errorf("error updating session status: %w", err)
	}
	return nil
}

func SessionRun(ctx context.Context, message stream.Message) error {
	log := message.MustGet(stream.LoggerDep).(*zap.Logger)
	db := message.MustGet(stream.DBDep).(postgres.Query)
	config := message.MustGet(stream.ConfigDep).(*config.Config)

	command := ss.SessionRunCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		log.Error("Error unmarshalling SessionRun payload", zap.Error(err))
		return err
	}

	sess, err := message.Call(ctx, dependency.GetBacktestSessionKey)
	if err != nil {
		log.Error("Error getting backtest session", zap.Error(err))
		return err
	}
	s := sess.(backtest.Session)

	sessionStorage := repository.Session{Conn: db}
	executionStorage := repository.Execution{Conn: db}

	storedSession, err := sessionStorage.Get(ctx, command.SessionID)
	if err != nil {
		log.Error("Error getting session", zap.Error(err))
		return err
	}

	log.Info("Running backtest", zap.String("session", command.SessionID))
	executions, err := executionStorage.ListBySession(ctx, storedSession.ID)
	if err != nil {
		log.Error("Error listing executions", zap.Error(err))
		return err
	}

	runSession := func(session backtest.Session, executions *[]entity.Execution) error {
		log.Debug("Running session", zap.Any("session", storedSession), zap.Any("executions", executions))
		if err != nil {
			log.Error("Error ingesting session", zap.Error(err))
			return err
		}
		if command.Manual {
			err = sessionStorage.UpdatePort(ctx, storedSession.ID, s.GetSocket().Port)
			if err != nil {
				log.Error("Error updating socket", zap.Error(err))
				return err
			}
			activity := make(chan bool)
			stop := make(chan bool)
			// If there is no activity in the session for X seconds, we tell it to stop
			go func() {
				for {
					select {
					case <-activity:
					case <-time.After(time.Second * 30):
						stop <- true
						return
					case <-stop: // stop is closed, exit
						return
					}
				}
			}()

			err = session.Run(activity, stop)
			if err != nil {
				log.Error("Error running session", zap.Error(err))
				return err
			}
			close(stop)
			close(activity)
		} else {
			for _, execution := range *executions {
				log.Info("Running execution", zap.Any("execution", execution))
				err = session.RunExecution(ctx, config, &execution)
				if err != nil {
					log.Error("Error running execution", zap.Error(err))
					return err
				}
			}
		}
		return nil
	}
	err = runSession(s, executions)
	if err != nil {
		log.Error("Error running session", zap.Error(err))
		return err
	}
	return nil
}
