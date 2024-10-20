package stream

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
)

type UpdateSessionStatusCommand struct {
	SessionID string
	Status    pb.Session_Status_Status
	Error     error
}

func NewUpdateSessionStatusCommand(session string, status pb.Session_Status_Status, err error) (stream.Message, error) {
	entity := &UpdateSessionStatusCommand{
		SessionID: session,
		Status:    status,
		Error:     err,
	}

	msg, err := stream.NewMessage("backtest", "session", "status", entity)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}

	return msg, nil
}

type SessionRunCommand struct {
	Backtest           string
	SessionID          string
	Manual             bool
	BacktestInstanceID string
	WorkerInstanceIDs  []string
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
