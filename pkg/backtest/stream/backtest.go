package stream

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	streamPb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	backtestServicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
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
	rsp := &backtestServicePb.CreateBacktestResponse{}

	msg, err := financeStream.NewIngestCommand("XNYS", symbols, start, &end)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rsp.CREATING.ProtoReflect().Descriptor().Index()), Messages: []*streamPb.Message{msg.GetMsg()}})

	msg, err = NewBacktestIngestCommand(name, symbols, start, end)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rsp.INGESTING.ProtoReflect().Descriptor().Index()), Messages: []*streamPb.Message{msg.GetMsg()}})

	return &o, nil
}

type CreateBacktestCommand struct {
	BacktestName string `json:"backtest_name"`
	ETFName      string `json:"etf_name"`
}

func NewCreateBacktestCommand(backtestName string, etfName string) (stream.Message, error) {
	cmd := &CreateBacktestCommand{
		BacktestName: backtestName,
		ETFName:      etfName,
	}
	return stream.NewMessage("backtest", "create", "create", cmd)
}

type DownloadFinancialDataCommand struct {
	BacktestName string `json:"backtest_name"`
}

func NewDownloadFinancialDataCommand(backtestName string) (stream.Message, error) {
	cmd := &DownloadFinancialDataCommand{
		BacktestName: backtestName,
	}
	return stream.NewMessage("backtest", "download", "download", cmd)
}

type IngestBacktestDataCommand struct{}

func NewIngestBacktestDataCommand() (stream.Message, error) {
	cmd := &IngestBacktestDataCommand{}
	return stream.NewMessage("backtest", "ingest", "ingest", cmd)
}

func CreateBacktestFromETF(backtest_name string, etf_name string, start string, end string) (*streamPb.Orchestration, error) {
	o := streamPb.Orchestration{}
	rsp := &backtestServicePb.CreateBacktestResponse{}

	// Create Backtest
	cmd, err := NewCreateBacktestCommand(backtest_name, etf_name)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rsp.CREATING.ProtoReflect().Descriptor().Index()), Messages: []*streamPb.Message{cmd.GetMsg()}})

	// Ingest Financial Data
	cmd, err = NewDownloadFinancialDataCommand(backtest_name)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rsp.DOWNLOADING.ProtoReflect().Descriptor().Index()), Messages: []*streamPb.Message{cmd.GetMsg()}})

	// Ingest into Backtest
	cmd, err = NewIngestBacktestDataCommand()
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}

	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rsp.INGESTING.ProtoReflect().Descriptor().Index()), Messages: []*streamPb.Message{cmd.GetMsg()}})
	return &o, nil
}
