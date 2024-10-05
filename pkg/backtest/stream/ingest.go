package stream

import (
	"github.com/lhjnilsson/foreverbull/internal/stream"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
)

type IngestCommand struct {
	Name    string   `json:"name"`
	Symbols []string `json:"symbols"`
	Start   string   `json:"start"`
	End     string   `json:"end"`
}

func NewBacktestIngestCommand(name string, symbols []string, start, end string) (stream.Message, error) {
	cmd := &IngestCommand{
		Name:    name,
		Symbols: symbols,
		Start:   start,
		End:     end,
	}
	return stream.NewMessage("backtest", "ingest", "ingest", cmd)
}

func NewIngestOrchestration(name string, symbols []string, start, end string) (*stream.MessageOrchestration, error) {
	orchestration := stream.NewMessageOrchestration("ingest backtest")
	msg, err := financeStream.NewIngestCommand(symbols, start, end)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("ingest financial data", []stream.Message{msg})
	msg, err = NewBacktestIngestCommand(name, symbols, start, end)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("ingest into backtest", []stream.Message{msg})
	orchestration.SettFallback([]stream.Message{})
	return orchestration, nil
}
