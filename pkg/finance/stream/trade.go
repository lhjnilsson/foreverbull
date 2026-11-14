package stream

import (
	"time"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	streamPb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	financeServicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
)

type StartTradingSessionCommand struct {
	Id       string   `json:"id"`
	Symbols  []string `json:"symbols"`
	Provider string   `json:"provider"`
	Profile  string   `json:"profile,omitempty"`
}

func NewRunTradeCommand(id, provider, profile string, symbols []string) (stream.Message, error) {
	cmd := &StartTradingSessionCommand{Id: id, Symbols: symbols, Provider: provider, Profile: profile}
	return stream.NewMessage("finance", "trading", "run", cmd)
}

// NewRunTradeOrchestration ingests and starts a session on provider.
// provider is the backtest's price source. An empty provider is stored as
// given; the session and the ingest refuse it instead of guessing yahoo.
func NewRunTradeOrchestration(id string, symbols []string, provider, profile string) (*streamPb.Orchestration, error) {
	o := &streamPb.Orchestration{}
	start := time.Now().Add(-time.Hour * 120).Format("2006-01-02")
	msg, err := stream.NewMessage("finance", "marketdata", "ingest", &IngestCommand{
		Calendar: "XNYS",
		Symbols:  symbols,
		Start:    start,
		Provider: provider,
		Profile:  profile,
		Trading:  true,
	})
	if err != nil {
		return nil, err
	}

	s := financeServicePb.CreateSessionResponse{}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(s.UPDATING_MARKET_DATA.ProtoReflect().Descriptor().Index()), Messages: []*streamPb.Message{msg.GetMsg()}})

	msg, err = NewRunTradeCommand(id, provider, profile, symbols)
	if err != nil {
		return nil, err
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(s.STARTING.ProtoReflect().Descriptor().Index()), Messages: []*streamPb.Message{msg.GetMsg()}})
	return o, nil
}
