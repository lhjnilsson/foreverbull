package command

import (
	"context"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/rs/zerolog/log"
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

	runSession := func(session backtest.Session) error {
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
			log.Error().Err(err).Msg("error running session")
			return err
		}
		close(stop)
		close(activity)
		return nil
	}
	err = runSession(s)
	if err != nil {
		return fmt.Errorf("error running session: %w", err)
	}
	err = s.Stop(ctx)
	if err != nil {
		return fmt.Errorf("error stopping session: %w", err)
	}
	return nil
}
