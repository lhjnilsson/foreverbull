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

func NewRunTradeCommand(Id string, symbols []string) (stream.Message, error) {
	cmd := &StartTradingSessionCommand{Id: Id, Symbols: symbols}
	return stream.NewMessage("finance", "trading", "run", cmd)
}

func NewRunTradeOrchestration(id string, symbols []string) (*streamPb.Orchestration, error) {
	o := &streamPb.Orchestration{}
	start := time.Now().Add(-time.Hour * 120).Format("2006-01-02")
	msg, err := NewIngestCommand("XNYS", symbols, start, nil)
	if err != nil {
		return nil, err
	}

	s := financeServicePb.CreateSessionResponse{}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(s.UPDATING_MARKET_DATA.ProtoReflect().Descriptor().Index()), Messages: []*streamPb.Message{msg.GetMsg()}})

	msg, err = NewRunTradeCommand(id, symbols)
	if err != nil {
		return nil, err
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(s.STARTING.ProtoReflect().Descriptor().Index()), Messages: []*streamPb.Message{msg.GetMsg()}})
	return o, nil
}
