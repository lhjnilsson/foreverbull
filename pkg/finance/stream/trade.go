package stream

import (
	"time"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	streamPb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	financeServicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
)

type StartTradingSessionCommand struct {
	Id      string   `json:"id"`
	Symbols []string `json:"symbols"`
}

func NewRunTradeCommand(symbols []string) (stream.Message, error) {
	cmd := &StartTradingSessionCommand{Symbols: symbols}
	return stream.NewMessage("finance", "trading", "run", cmd)
}

func NewRunTradeOrchestration(id string, symbols []string) (*streamPb.Orchestration, error) {
	o := &streamPb.Orchestration{}
	start := time.Now().Add(-time.Hour * 120).Format(time.RFC3339)
	msg, err := NewIngestCommand(symbols, start, nil)
	if err != nil {
		return nil, err
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Status: int32(financeServicePb.CreateSessionResponse_UPDATING_MARKETDATA), Messages: []*streamPb.Message{msg.GetMsg()}})

	msg, err = NewRunTradeCommand(symbols)
	if err != nil {
		return nil, err
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Status: int32(financeServicePb.CreateSessionResponse_RUNNING), Messages: []*streamPb.Message{msg.GetMsg()}})
	return o, nil
}
