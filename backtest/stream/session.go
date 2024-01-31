package event

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	serviceStream "github.com/lhjnilsson/foreverbull/service/stream"
)

type UpdateSessionStatusCommand struct {
	SessionID string                   `json:"session_id"`
	Status    entity.SessionStatusType `json:"status"`
	Error     error                    `json:"error"`
}

func NewUpdateSessionStatusCommand(session string, status entity.SessionStatusType, err error) (stream.Message, error) {
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

func NewSessionRunCommand(backtest, sessionid string, manual bool,
	backtestInstanceID string, workerinstanceIDS []string) (stream.Message, error) {
	entity := &SessionRunCommand{
		Backtest:           backtest,
		SessionID:          sessionid,
		Manual:             manual,
		BacktestInstanceID: backtestInstanceID,
		WorkerInstanceIDs:  workerinstanceIDS,
	}
	return stream.NewMessage("backtest", "session", "run", entity)
}

func NewSessionRunOrchestration(backtest *entity.Backtest, session *entity.Session) (*stream.MessageOrchestration, error) {
	orchestration := stream.NewMessageOrchestration("run backtest session")

	backtestInstanceID := serviceStream.NewInstanceID()
	workerInstanceID := serviceStream.NewInstanceID()

	statusMsg, err := NewUpdateSessionStatusCommand(session.ID, entity.SessionStatusRunning, nil)
	if err != nil {
		return nil, err
	}
	if session.Manual {
		msg1, err := serviceStream.NewServiceStartCommand(backtest.BacktestService, backtestInstanceID)
		if err != nil {
			return nil, err
		}
		orchestration.AddStep("start services", []stream.Message{msg1, statusMsg})

		msg, err := serviceStream.NewInstanceSanityCheckCommand([]string{backtestInstanceID})
		if err != nil {
			return nil, err
		}
		orchestration.AddStep("sanity check", []stream.Message{msg})
	} else {
		msg1, err := serviceStream.NewServiceStartCommand(backtest.BacktestService, backtestInstanceID)
		if err != nil {
			return nil, err
		}
		if backtest.WorkerService == nil {
			return nil, fmt.Errorf("worker service is required but not provided")
		}
		msg2, err := serviceStream.NewServiceStartCommand(*backtest.WorkerService, workerInstanceID)
		if err != nil {
			return nil, err
		}
		orchestration.AddStep("start services", []stream.Message{msg1, msg2, statusMsg})

		msg, err := serviceStream.NewInstanceSanityCheckCommand([]string{backtestInstanceID, workerInstanceID})
		if err != nil {
			return nil, err
		}
		orchestration.AddStep("sanity check", []stream.Message{msg})
	}

	workerInstanceIDs := []string{}
	if !session.Manual {
		workerInstanceIDs = append(workerInstanceIDs, workerInstanceID)
	}

	msg, err := NewSessionRunCommand(backtest.Name, session.ID, session.Manual, backtestInstanceID, workerInstanceIDs)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("run session", []stream.Message{msg})

	statusMsg, err = NewUpdateSessionStatusCommand(session.ID, entity.SessionStatusCompleted, nil)
	if err != nil {
		return nil, err
	}
	if session.Manual {
		msg1, err := serviceStream.NewInstanceStopCommand(backtestInstanceID)
		if err != nil {
			return nil, err
		}
		orchestration.AddStep("stop services", []stream.Message{msg1, statusMsg})
	} else {
		msg1, err := serviceStream.NewInstanceStopCommand(backtestInstanceID)
		if err != nil {
			return nil, err
		}
		msg2, err := serviceStream.NewInstanceStopCommand(workerInstanceID)
		if err != nil {
			return nil, err
		}
		orchestration.AddStep("stop services", []stream.Message{msg1, msg2, statusMsg})
	}

	statusMsg, err = NewUpdateSessionStatusCommand(session.ID, entity.SessionStatusFailed, nil)
	if err != nil {
		return nil, err
	}
	if session.Manual {
		msg1, err := serviceStream.NewInstanceStopCommand(backtestInstanceID)
		if err != nil {
			return nil, err
		}
		orchestration.SettFallback([]stream.Message{msg1, statusMsg})
	} else {
		msg1, err := serviceStream.NewInstanceStopCommand(backtestInstanceID)
		if err != nil {
			return nil, err
		}
		msg2, err := serviceStream.NewInstanceStopCommand(workerInstanceID)
		if err != nil {
			return nil, err
		}
		orchestration.SettFallback([]stream.Message{msg1, msg2, statusMsg})
	}
	return orchestration, nil
}
