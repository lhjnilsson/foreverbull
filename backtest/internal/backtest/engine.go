package backtest

import (
	"context"
	"errors"
	"fmt"

	"github.com/lhjnilsson/foreverbull/backtest/engine"
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	service "github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/socket"
)

/*
NewZiplineEngine
Returns a Zipline backtest engine
*/
func NewZiplineEngine(ctx context.Context, service *service.Instance) (engine.Engine, error) {
	z := Zipline{}
	s, err := service.GetSocket()
	if err != nil {
		return nil, fmt.Errorf("error getting socket for instance: %w", err)
	}
	z.socket, err = socket.GetContextSocket(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("error getting context socket: %w", err)
	}
	return &z, err
}

/*
Configuration
Returned by backtest- configuration to get the hosted sockets for feed, broker etc
*/
type Configuration struct {
	Socket socket.NanomsgSocket `mapstructure:"socket"`
}

type Zipline struct {
	socket              socket.ContextSocket
	SocketConfiguration socket.NanomsgSocket `json:"main" mapstructure:"socket"`
	Running             bool                 `json:"running"`
}

func (z *Zipline) Ingest(ctx context.Context, backtest *entity.Backtest) error {
	sock, err := z.socket.Get()
	if err != nil {
		return fmt.Errorf("error opening context: %w", err)
	}
	defer sock.Close()

	req := message.Request{Task: "ingest", Data: backtest}
	rsp, err := req.Process(sock)
	if err != nil {
		return fmt.Errorf("error ingesting: %w", err)
	}
	if len(rsp.Error) > 0 {
		return errors.New(rsp.Error)
	}
	if err := rsp.DecodeData(backtest); err != nil {
		return fmt.Errorf("error decoding data: %w", err)
	}
	return nil
}

func (z *Zipline) UploadIngestion(ctx context.Context, ingestion_name string) error {
	sock, err := z.socket.Get()
	if err != nil {
		return fmt.Errorf("error opening context: %w", err)
	}
	defer sock.Close()

	req := message.Request{Task: "upload_ingestion", Data: map[string]string{"name": ingestion_name}}
	rsp, err := req.Process(sock)
	if err != nil {
		return fmt.Errorf("error uploading ingestion: %w", err)
	}
	if len(rsp.Error) > 0 {
		return errors.New(rsp.Error)
	}
	return nil
}

func (z *Zipline) DownloadIngestion(ctx context.Context, ingestion_name string) error {
	sock, err := z.socket.Get()
	if err != nil {
		return fmt.Errorf("error opening context: %w", err)
	}
	defer sock.Close()

	req := message.Request{Task: "download_ingestion", Data: map[string]string{"name": ingestion_name}}
	rsp, err := req.Process(sock)
	if err != nil {
		return fmt.Errorf("error downloading ingestion: %w", err)
	}
	if len(rsp.Error) > 0 {
		return errors.New(rsp.Error)
	}
	return nil
}

func (z *Zipline) ConfigureExecution(ctx context.Context, execution *entity.Execution) error {
	socket, err := z.socket.Get()
	if err != nil {
		return fmt.Errorf("error getting socket for instance: %w", err)
	}
	defer socket.Close()

	var rsp *message.Response
	req := message.Request{Task: "configure_execution", Data: execution}
	rsp, err = req.Process(socket)
	if err != nil {
		return fmt.Errorf("error configuring: %w", err)
	}
	return rsp.DecodeData(execution)
}

/*
Run
Runs the execution
*/
func (z *Zipline) RunExecution(ctx context.Context) error {
	socket, err := z.socket.Get()
	if err != nil {
		return fmt.Errorf("error getting socket for instance: %w", err)
	}
	defer socket.Close()

	req := message.Request{Task: "run_execution"}
	if _, err := req.Process(socket); err != nil {
		return fmt.Errorf("error running: %w", err)
	}
	z.Running = true
	return nil
}

/*
GetMessage
Returns feed message from a running execution
*/
func (z *Zipline) GetMessage() (*engine.Period, error) {
	if !z.Running {
		return nil, errors.New("backtest engine is not running")
	}
	var err error
	socket, err := z.socket.Get()
	if err != nil {
		return nil, fmt.Errorf("error getting socket for instance: %w", err)
	}

	req := message.Request{Task: "get_period"}
	rsp, err := req.Process(socket)
	if err != nil {
		return nil, fmt.Errorf("error getting period: %w", err)
	}
	if rsp.Data == nil {
		return nil, nil
	}
	period := engine.Period{}
	if err = rsp.DecodeData(&period); err != nil {
		return nil, fmt.Errorf("error decoding period- data: %w", err)
	}
	return &period, nil
}

/*
Continue
Continue after day completed to trigger a new Day
*/
func (z *Zipline) Continue() error {
	socket, err := z.socket.Get()
	if err != nil {
		return fmt.Errorf("error getting socket for instance: %w", err)
	}
	defer socket.Close()

	req := message.Request{Task: "continue"}
	if _, err := req.Process(socket); err != nil {
		return fmt.Errorf("error continuing: %w", err)
	}
	return nil
}

/*
GetResult
Gets the result of the execution

TODO: How to use bigger buffer in req.Process fashion? or how to send result in batches
*/
func (z *Zipline) GetExecutionResult(executionID string) (*engine.Result, error) {
	var err error
	var data []byte
	var rspData []byte
	execution := map[string]string{
		"execution": executionID,
	}

	rsp := message.Response{}
	req := message.Request{Task: "upload_result", Data: execution}
	data, err = req.Encode()
	if err != nil {
		return nil, fmt.Errorf("UploadResult encoding config: %v", err)
	}
	socket, err := z.socket.Get()
	if err != nil {
		return nil, fmt.Errorf("error getting socket for instance: %w", err)
	}
	defer socket.Close()

	if err = socket.Write(data); err != nil {
		return nil, fmt.Errorf("UploadResult writing to socket: %v", err)
	}
	if rspData, err = socket.Read(); err != nil {
		return nil, fmt.Errorf("UploadResult reading from socket: %v", err)
	}
	if err = rsp.Decode(rspData); err != nil {
		return nil, fmt.Errorf("GetResult decoding response: %v", err)
	}
	if rsp.HasError() {
		return nil, fmt.Errorf("UploadResult from zipline backtest: %v", rsp.Error)
	}

	rsp = message.Response{}
	req = message.Request{Task: "get_execution_result"}
	data, err = req.Encode()
	if err != nil {
		return nil, fmt.Errorf("GetResult encoding config: %v", err)
	}

	socket, err = z.socket.Get()
	if err != nil {
		return nil, fmt.Errorf("error getting socket for instance: %w", err)
	}
	defer socket.Close()

	if err = socket.Write(data); err != nil {
		return nil, fmt.Errorf("GetResult writing to socket: %v", err)
	}
	if rspData, err = socket.Read(); err != nil {
		return nil, fmt.Errorf("GetResult reading from socket: %v", err)
	}
	if err = rsp.Decode(rspData); err != nil {
		return nil, fmt.Errorf("GetResult decoding response: %v", err)
	}
	if rsp.HasError() {
		return nil, fmt.Errorf("GetResult from zipline backtest: %v", rsp.Error)
	}
	var result engine.Result
	if err = rsp.DecodeData(&result); err != nil {
		return nil, fmt.Errorf("GetResult decoding data: %v", err)
	}
	return &result, nil
}

/*
Order
places a new order, can be positive or negative value for long or short
*/
func (b *Zipline) Order(order *engine.Order) (*engine.Order, error) {
	ctxSock, err := b.socket.Get()
	if err != nil {
		return nil, fmt.Errorf("error opening context socket: %w", err)
	}
	defer ctxSock.Close()
	req := message.Request{Task: "order", Data: order}

	rsp, err := req.Process(ctxSock)
	if err != nil {
		return nil, fmt.Errorf("error processing order: %w", err)
	}
	if err = rsp.DecodeData(order); err != nil {
		return nil, fmt.Errorf("error decoding order- data: %w", err)
	}
	return order, nil
}

/*
GetOrder
Get information about an order, if its filled or not
*/
func (b *Zipline) GetOrder(order *engine.Order) (*engine.Order, error) {
	ctxSock, err := b.socket.Get()
	if err != nil {
		return nil, fmt.Errorf("error opening context socket: %w", err)
	}
	defer ctxSock.Close()
	req := message.Request{Task: "get_order", Data: order}

	rsp, err := req.Process(ctxSock)
	if err != nil {
		return nil, fmt.Errorf("error processing order: %w", err)
	}
	newOrder := engine.Order{}
	if err = rsp.DecodeData(&newOrder); err != nil {
		return nil, fmt.Errorf("error decoding order- data: %w", err)
	}
	return &newOrder, nil
}

/*
CancelOrder
Cancels an order being placed
*/
func (b *Zipline) CancelOrder(order *engine.Order) error {
	ctxSock, err := b.socket.Get()
	if err != nil {
		return fmt.Errorf("error opening context socket: %w", err)
	}
	defer ctxSock.Close()

	req := message.Request{Task: "cancel_order", Data: order}
	rsp, err := req.Process(ctxSock)
	if err != nil {
		return fmt.Errorf("error processing order: %w", err)
	}
	if err = rsp.DecodeData(order); err != nil {
		return fmt.Errorf("error decoding order- data: %w", err)
	}
	return nil
}

/*
Stop
Stops the running execution
*/
func (z *Zipline) Stop(ctx context.Context) error {
	socket, err := z.socket.Get()
	if err != nil {
		return fmt.Errorf("error getting socket for instance: %w", err)
	}
	defer socket.Close()

	req := message.Request{Task: "stop"}
	if _, err := req.Process(socket); err != nil {
		return fmt.Errorf("error stopping: %w", err)
	}
	z.Running = false
	return nil
}
