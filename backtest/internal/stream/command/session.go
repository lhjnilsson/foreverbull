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
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
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
	db := message.MustGet(stream.DBDep).(postgres.Query)

	command := ss.SessionRunCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling SessionRun payload: %w", err)
	}

	sess, err := message.Call(ctx, dependency.GetBacktestSessionKey)
	if err != nil {
		return fmt.Errorf("error getting backtest session: %w", err)
	}
	s := sess.(backtest.Session)
	sessionStorage := repository.Session{Conn: db}
	executionStorage := repository.Execution{Conn: db}

	storedSession, err := sessionStorage.Get(ctx, command.SessionID)
	if err != nil {
		return fmt.Errorf("error getting session: %w", err)
	}

	executions, err := executionStorage.ListBySession(ctx, storedSession.ID)
	if err != nil {
		return fmt.Errorf("error listing executions: %w", err)
	}

	runSession := func(session backtest.Session, executions *[]entity.Execution) error {
		if err != nil {
			return err
		}
		if command.Manual {
			err = sessionStorage.UpdatePort(ctx, storedSession.ID, s.GetSocket().Port)
			if err != nil {
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
				return err
			}
			close(stop)
			close(activity)
		} else {
			for _, execution := range *executions {
				err = session.RunExecution(ctx, &execution)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	err = runSession(s, executions)
	if err != nil {
		return fmt.Errorf("error running session: %w", err)
	}
	return nil
}
