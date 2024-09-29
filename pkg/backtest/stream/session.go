package stream

import (
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
)

type UpdateSessionStatusCommand struct {
	SessionID string                   `json:"session_id"`
	Status    pb.Session_Status_Status `json:"status"`
	Error     error                    `json:"error"`
}

func NewUpdateSessionStatusCommand(session string, status pb.Session_Status_Status, err error) (stream.Message, error) {
	entity := &UpdateSessionStatusCommand{
		SessionID: session,
		Status:    status,
		Error:     err,
	}
	return stream.NewMessage("backtest", "session", "status", entity)
}

type SessionRunCommand struct {
	Backtest           string   `json:"backtest"`
	SessionID          string   `json:"session_id"`
	Manual             bool     `json:"manual"`
	BacktestInstanceID string   `json:"backtest_instance_id"`
	WorkerInstanceIDs  []string `json:"worker_instance_ids"`
}

func NewSessionRunCommand(backtest, sessionid string) (stream.Message, error) {
	entity := &SessionRunCommand{
		Backtest:  backtest,
		SessionID: sessionid,
	}
	return stream.NewMessage("backtest", "session", "run", entity)
}
