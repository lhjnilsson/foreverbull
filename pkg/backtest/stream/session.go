package stream

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
)

type SessionRunCommand struct {
	Backtest  string
	SessionID string
}

func NewSessionRunCommand(backtest, sessionID string) (stream.Message, error) {
	entity := &SessionRunCommand{
		Backtest:  backtest,
		SessionID: sessionID,
	}

	msg, err := stream.NewMessage("backtest", "session", "run", entity)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}

	return msg, nil
}
