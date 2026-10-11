package stream

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	streamPb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	backtestServicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"google.golang.org/protobuf/proto"
)

type IngestCommand struct {
	BacktestName string
}

func NewBacktestIngestCommand(backtest_name string) (stream.Message, error) {
	cmd := &IngestCommand{
		BacktestName: backtest_name,
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

	msg, err = NewBacktestIngestCommand(name)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rsp.INGESTING.ProtoReflect().Descriptor().Index()), Messages: []*streamPb.Message{msg.GetMsg()}})

	return &o, nil
}

type CreateBacktestFromETFCommand struct {
	BacktestName string `json:"backtest_name"`
	ETFName      string `json:"etf_name"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	Provider     string `json:"provider,omitempty"`
	Profile      string `json:"profile,omitempty"`
}

func NewCreateBacktestFromETFCommand(backtestName, etfName, startDate, endDate, provider, profile string) (stream.Message, error) {
	cmd := &CreateBacktestFromETFCommand{
		BacktestName: backtestName,
		ETFName:      etfName,
		StartDate:    startDate,
		EndDate:      endDate,
		Provider:     provider,
		Profile:      profile,
	}
	return stream.NewMessage("backtest", "create", "from_etf", cmd)
}

type CreateBacktestFromBacktestCommand struct {
	Backtest []byte `json:"backtest"`
}

func NewCreateBacktestFromBacktestCommand(backtest []byte) (stream.Message, error) {
	cmd := &CreateBacktestFromBacktestCommand{
		Backtest: backtest,
	}
	return stream.NewMessage("backtest", "create", "from_backtest", cmd)
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

func CreateBacktestFromETF(backtest_name, etf_name, start_date, end_date, provider, profile string) (*streamPb.Orchestration, error) {
	o := streamPb.Orchestration{}
	rsp := &backtestServicePb.CreateBacktestResponse{}
	rspFields := rsp.ProtoReflect().Descriptor().Fields()

	// Create Backtest
	cmd, err := NewCreateBacktestFromETFCommand(backtest_name, etf_name, start_date, end_date, provider, profile)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("CREATING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})

	// Ingest Financial Data
	cmd, err = NewDownloadFinancialDataCommand(backtest_name)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("DOWNLOADING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})

	// Ingest into Backtest
	cmd, err = NewBacktestIngestCommand(backtest_name)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}

	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("INGESTING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})
	return &o, nil
}

func CreateBacktestFromBacktest(backtest *backtest.Backtest) (*streamPb.Orchestration, error) {
	o := streamPb.Orchestration{}
	rsp := &backtestServicePb.CreateBacktestResponse{}
	rspFields := rsp.ProtoReflect().Descriptor().Fields()

	// Create Backtest
	bytes, err := proto.Marshal(backtest)
	if err != nil {
		return nil, fmt.Errorf("error marshalling backtest: %w", err)
	}
	cmd, err := NewCreateBacktestFromBacktestCommand(bytes)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("CREATING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})

	// Ingest Financial Data
	cmd, err = NewDownloadFinancialDataCommand(backtest.Name)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("DOWNLOADING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})

	// Ingest into Backtest
	cmd, err = NewBacktestIngestCommand(backtest.Name)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}

	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("INGESTING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})
	return &o, nil
}
