package stream

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

type IngestCommand struct {
	Name    string
	Symbols []string
	Start   string
	End     string
}

func NewBacktestIngestCommand(name string, symbols []string, start, end string) (stream.Message, error) {
	cmd := &IngestCommand{
		Name:    name,
		Symbols: symbols,
		Start:   start,
		End:     end,
	}

	msg, err := stream.NewMessage("backtest", "ingest", "ingest", cmd)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}

	return msg, nil
}

type UpdateStatusCommand struct {
	Name   string
	Status pb.BacktestStatus
}

func NewUpdateBacktestStatusCommand(name string, status pb.BacktestStatus) (stream.Message, error) {
	cmd := &UpdateStatusCommand{
		Name:   name,
		Status: status,
	}

	msg, err := stream.NewMessage("backtest", "status", "update", cmd)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	return msg, nil
}

func NewCreateBacktestOrchestration(name string, symbols []string, start, end string) (*stream.MessageOrchestration, error) {
	orchestration := stream.NewMessageOrchestration("create backtest")

	// Ingest financial data
	msg, err := financeStream.NewIngestCommand(symbols, start, &end)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	statusMsg, err := NewUpdateBacktestStatusCommand(name, pb.BacktestStatus_DOWNLOADING)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	orchestration.AddStep("ingest financial data", []stream.Message{msg, statusMsg})

	// Ingest into backtest engine
	msg, err = NewBacktestIngestCommand(name, symbols, start, end)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	statusMsg, err = NewUpdateBacktestStatusCommand(name, pb.BacktestStatus_INGESTING)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	orchestration.AddStep("ingest into backtest", []stream.Message{msg, statusMsg})

	// Update backtest status
	statusMsg, err = NewUpdateBacktestStatusCommand(name, pb.BacktestStatus_READY)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	orchestration.AddStep("mark as ready", []stream.Message{statusMsg})

	// Fallback
	statusMsg, err = NewUpdateBacktestStatusCommand(name, pb.BacktestStatus_ERROR)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	orchestration.SettFallback([]stream.Message{statusMsg})

	return orchestration, nil
}
