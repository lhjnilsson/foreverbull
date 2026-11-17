package stream

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	streamPb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	backtestServicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"google.golang.org/protobuf/proto"
)

type IngestCommand struct {
	BacktestName string
	OwnerID      string `json:"owner_id,omitempty"`
	BacktestID   string `json:"backtest_id,omitempty"`
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
	BacktestName            string `json:"backtest_name"`
	ETFName                 string `json:"etf_name"`
	StartDate               string `json:"start_date"`
	EndDate                 string `json:"end_date"`
	Provider                string `json:"provider,omitempty"`
	Profile                 string `json:"profile,omitempty"`
	OwnerID                 string `json:"owner_id,omitempty"`
	AutoCloseOnUniverseDrop *bool  `json:"auto_close_on_universe_drop,omitempty"`
	// CreateMessageID is this task's id. The insert stores the backtest id
	// against it, so later steps do not look the row up by name.
	CreateMessageID string `json:"create_message_id,omitempty"`
}

func NewCreateBacktestFromETFCommand(backtestName, etfName, startDate, endDate, provider, profile string, closeOnDrop *bool) (stream.Message, error) {
	cmd := &CreateBacktestFromETFCommand{
		BacktestName:            backtestName,
		ETFName:                 etfName,
		StartDate:               startDate,
		EndDate:                 endDate,
		Provider:                provider,
		Profile:                 profile,
		AutoCloseOnUniverseDrop: closeOnDrop,
	}
	msg, err := stream.NewMessage("backtest", "create", "from_etf", cmd)
	if err != nil {
		return nil, err
	}
	if err = stampCreateMessage(msg, func(id string) any {
		cmd.CreateMessageID = id
		return cmd
	}); err != nil {
		return nil, err
	}
	return msg, nil
}

type CreateBacktestFromBacktestCommand struct {
	Backtest []byte `json:"backtest"`
	OwnerID  string `json:"owner_id,omitempty"`
	// CreateMessageID is this task's id. The insert stores the backtest id
	// against it, so later steps do not look the row up by name.
	CreateMessageID string `json:"create_message_id,omitempty"`
}

func NewCreateBacktestFromBacktestCommand(backtest []byte) (stream.Message, error) {
	cmd := &CreateBacktestFromBacktestCommand{
		Backtest: backtest,
	}
	msg, err := stream.NewMessage("backtest", "create", "from_backtest", cmd)
	if err != nil {
		return nil, err
	}
	if err = stampCreateMessage(msg, func(id string) any {
		cmd.CreateMessageID = id
		return cmd
	}); err != nil {
		return nil, err
	}
	return msg, nil
}

// stampCreateMessage writes the task id into the payload the worker reads.
// The id is the message id, which the API already has, so it can load the
// row the insert wrote without asking for the name.
func stampCreateMessage(msg stream.Message, rewrite func(id string) any) error {
	if msg == nil || msg.GetMsg() == nil {
		return errors.New("create message id is empty")
	}
	id := strings.TrimSpace(msg.GetMsg().GetId())
	if id == "" {
		return errors.New("create message id is empty")
	}
	payload, err := json.Marshal(rewrite(id))
	if err != nil {
		return fmt.Errorf("error creating message: %w", err)
	}
	msg.GetMsg().Payload = payload
	return nil
}

type DownloadFinancialDataCommand struct {
	BacktestName string `json:"backtest_name"`
	OwnerID      string `json:"owner_id,omitempty"`
	BacktestID   string `json:"backtest_id,omitempty"`
}

func NewDownloadFinancialDataCommand(backtestName string) (stream.Message, error) {
	cmd := &DownloadFinancialDataCommand{
		BacktestName: backtestName,
	}
	return stream.NewMessage("backtest", "download", "download", cmd)
}

func CreateBacktestFromETF(backtest_name, etf_name, start_date, end_date, provider, profile string, closeOnDrop *bool) (*streamPb.Orchestration, error) {
	o := streamPb.Orchestration{}
	rsp := &backtestServicePb.CreateBacktestResponse{}
	rspFields := rsp.ProtoReflect().Descriptor().Fields()

	// Create Backtest
	cmd, err := NewCreateBacktestFromETFCommand(backtest_name, etf_name, start_date, end_date, provider, profile, closeOnDrop)
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

// CreateBacktestFromETFOwned stamps OwnerID on each step. An empty owner is refused
// by the subscriber once authentication is on.
func CreateBacktestFromETFOwned(backtestName, etfName, startDate, endDate, provider, profile, ownerID string, closeOnDrop *bool) (*streamPb.Orchestration, error) {
	o := streamPb.Orchestration{}
	rsp := &backtestServicePb.CreateBacktestResponse{}
	rspFields := rsp.ProtoReflect().Descriptor().Fields()

	creating := &CreateBacktestFromETFCommand{
		BacktestName:            backtestName,
		ETFName:                 etfName,
		StartDate:               startDate,
		EndDate:                 endDate,
		Provider:                provider,
		Profile:                 profile,
		OwnerID:                 ownerID,
		AutoCloseOnUniverseDrop: closeOnDrop,
	}
	cmd, err := stream.NewMessage("backtest", "create", "from_etf", creating)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	if err = stampCreateMessage(cmd, func(id string) any {
		creating.CreateMessageID = id
		return creating
	}); err != nil {
		return nil, err
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("CREATING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})

	cmd, err = stream.NewMessage("backtest", "download", "download", &DownloadFinancialDataCommand{
		BacktestName: backtestName,
		OwnerID:      ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("DOWNLOADING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})

	cmd, err = stream.NewMessage("backtest", "ingest", "ingest", &IngestCommand{
		BacktestName: backtestName,
		OwnerID:      ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("INGESTING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})
	return &o, nil
}

// CreateBacktestFromBacktestOwned stamps OwnerID on each step.
func CreateBacktestFromBacktestOwned(item *backtest.Backtest, ownerID string) (*streamPb.Orchestration, error) {
	o := streamPb.Orchestration{}
	rsp := &backtestServicePb.CreateBacktestResponse{}
	rspFields := rsp.ProtoReflect().Descriptor().Fields()

	encoded, err := proto.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("error marshalling backtest: %w", err)
	}
	creating := &CreateBacktestFromBacktestCommand{
		Backtest: encoded,
		OwnerID:  ownerID,
	}
	cmd, err := stream.NewMessage("backtest", "create", "from_backtest", creating)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	if err = stampCreateMessage(cmd, func(id string) any {
		creating.CreateMessageID = id
		return creating
	}); err != nil {
		return nil, err
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("CREATING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})

	cmd, err = stream.NewMessage("backtest", "download", "download", &DownloadFinancialDataCommand{
		BacktestName: item.GetName(),
		OwnerID:      ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("DOWNLOADING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})

	cmd, err = stream.NewMessage("backtest", "ingest", "ingest", &IngestCommand{
		BacktestName: item.GetName(),
		OwnerID:      ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	o.Steps = append(o.Steps, &streamPb.Orchestration_Step{Num: int32(rspFields.ByName("INGESTING").Number()), Messages: []*streamPb.Message{cmd.GetMsg()}})
	return &o, nil
}
