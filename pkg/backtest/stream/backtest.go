package stream

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	streamPb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	backtestPb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
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

func NewCreateBacktestOrchestration(name string, symbols []string, start, end string) (*streamPb.Orchestration, error) {
	o := streamPb.Orchestration{}

	msg, err := financeStream.NewIngestCommand(symbols, start, &end)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(backtestPb.BacktestStatus_DOWNLOADING), Messages: []*streamPb.Message{msg.GetMsg()}})

	msg, err = NewBacktestIngestCommand(name, symbols, start, end)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(backtestPb.BacktestStatus_INGESTING), Messages: []*streamPb.Message{msg.GetMsg()}})

	return &o, nil
}
