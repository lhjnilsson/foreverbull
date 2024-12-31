package stream

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
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
	Status pb.IngestionStatus
}

func NewUpdateIngestionStatusCommand(name string, status pb.IngestionStatus) (stream.Message, error) {
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

func NewIngestOrchestration(name string, symbols []string, start, end string) (*stream.MessageOrchestration, error) {
	orchestration := stream.NewMessageOrchestration("ingest backtest")

	msg, err := financeStream.NewIngestCommand(symbols, start, &end)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	msg1, err := NewUpdateIngestionStatusCommand(name, pb.IngestionStatus_DOWNLOADING)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	orchestration.AddStep("ingest financial data", []stream.Message{msg, msg1})

	msg, err = NewBacktestIngestCommand(name, symbols, start, end)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	orchestration.AddStep("ingest into backtest", []stream.Message{msg})

	msg, err = NewUpdateIngestionStatusCommand(name, pb.IngestionStatus_ERROR)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	orchestration.SettFallback([]stream.Message{msg})

	return orchestration, nil
}
