package backtest

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/pb"
	backtest_pb "github.com/lhjnilsson/foreverbull/internal/pb/backtest"
	service_pb "github.com/lhjnilsson/foreverbull/internal/pb/service"
	"github.com/lhjnilsson/foreverbull/internal/socket"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	service "github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"google.golang.org/protobuf/proto"
)

var (
	NoActiveExecution error = fmt.Errorf("no active execution")
)

/*
NewZiplineEngine
Returns a Zipline backtest engine
*/
func NewZiplineEngine(ctx context.Context, service *service.Instance) (engine.Engine, error) {
	requester, err := socket.NewRequester(*service.Host, *service.Port, true)
	if err != nil {
		return nil, fmt.Errorf("error getting requester: %w", err)
	}
	z := Zipline{socket: requester}
	return &z, nil
}

type Zipline struct {
	socket  socket.Requester
	Running bool `json:"running"`
}

func (z *Zipline) Ingest(ctx context.Context, ingestion *entity.Ingestion) error {
	ingest_request := backtest_pb.IngestRequest{
		StartDate: pb.TimeToProtoTimestamp(ingestion.Start),
		EndDate:   pb.TimeToProtoTimestamp(ingestion.End),
		Symbols:   ingestion.Symbols,
	}
	data, err := proto.Marshal(&ingest_request)
	if err != nil {
		return fmt.Errorf("error marshalling ingest request: %w", err)
	}
	request := service_pb.Request{
		Task: "ingest",
		Data: data,
	}
	response := service_pb.Response{}
	err = z.socket.Request(&request, &response)
	if err != nil {
		return fmt.Errorf("error requesting ingest: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("error ingesting: %v", response.Error)
	}
	return nil
}

func (z *Zipline) UploadIngestion(ctx context.Context, ingestion_name string) error {
	request := service_pb.Request{
		Task: "upload_ingestion",
	}
	response := service_pb.Response{}
	err := z.socket.Request(&request, &response)
	if err != nil {
		return fmt.Errorf("error requesting upload: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("error uploading: %v", response.Error)
	}
	return nil
}

func (z *Zipline) DownloadIngestion(ctx context.Context, ingestion_name string) error {
	request := service_pb.Request{
		Task: "download_ingestion",
	}
	response := service_pb.Response{}
	err := z.socket.Request(&request, &response)
	if err != nil {
		return fmt.Errorf("error requesting download: %w", err)
	}
	return nil
}

func (z *Zipline) ConfigureExecution(ctx context.Context, execution *entity.Execution) error {
	configure_req := backtest_pb.ConfigureRequest{
		StartDate: pb.TimeToProtoTimestamp(execution.Start),
		EndDate:   pb.TimeToProtoTimestamp(execution.End),
		Symbols:   execution.Symbols,
		Benchmark: execution.Benchmark,
	}
	data, err := proto.Marshal(&configure_req)
	if err != nil {
		return fmt.Errorf("error marshalling configure request: %w", err)
	}
	request := service_pb.Request{
		Task: "configure_execution",
		Data: data,
	}
	response := service_pb.Response{}
	err = z.socket.Request(&request, &response)
	if err != nil {
		return fmt.Errorf("error requesting configure: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("error configuring: %v", *response.Error)
	}
	configure_rsp := backtest_pb.ConfigureResponse{}
	err = proto.Unmarshal(response.Data, &configure_rsp)
	if err != nil {
		return fmt.Errorf("error unmarshalling configure response: %w", err)
	}
	execution.Start = configure_rsp.StartDate.AsTime()
	execution.End = configure_rsp.EndDate.AsTime()
	execution.Symbols = configure_rsp.Symbols
	execution.Benchmark = configure_rsp.Benchmark
	return nil
}

/*
Run
Runs the execution
*/
func (z *Zipline) RunExecution(ctx context.Context) error {
	request := service_pb.Request{
		Task: "run_execution",
	}
	response := service_pb.Response{}
	err := z.socket.Request(&request, &response)
	if err != nil {
		return fmt.Errorf("error requesting run: %w", err)
	}
	z.Running = true
	return nil
}

/*
GetMessage
Returns feed message from a running execution
*/
func (z *Zipline) GetPortfolio() (*backtest_pb.GetPortfolioResponse, error) {
	if !z.Running {
		return nil, errors.New("backtest engine is not running")
	}
	request := service_pb.Request{
		Task: "get_portfolio",
	}
	response := service_pb.Response{}
	err := z.socket.Request(&request, &response)
	if err != nil {
		return nil, fmt.Errorf("error requesting period: %w", err)
	}
	if response.Error != nil {
		if strings.Contains(*response.Error, NoActiveExecution.Error()) {
			return nil, NoActiveExecution
		}
		return nil, fmt.Errorf("error getting period: %v", response.Error)
	}
	portfolio_pb := backtest_pb.GetPortfolioResponse{}
	err = proto.Unmarshal(response.Data, &portfolio_pb)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling period: %w", err)
	}
	return &portfolio_pb, nil
	/*
		portfolio := engine.Portfolio{
			Timestamp:         portfolio_pb.Timestamp.AsTime(),
			CashFlow:          portfolio_pb.CashFlow,
			StartingCash:      portfolio_pb.StartingCash,
			PortfolioValue:    portfolio_pb.PortfolioValue,
			PNL:               portfolio_pb.Pnl,
			Returns:           portfolio_pb.Returns,
			Cash:              portfolio_pb.Cash,
			PositionsValue:    portfolio_pb.PositionsValue,
			PositionsExposure: portfolio_pb.PositionsExposure,
		}
		for _, position := range portfolio_pb.Positions {
			portfolio.Positions = append(portfolio.Positions, engine.Position{
				Symbol:        position.Symbol,
				Amount:        position.Amount,
				CostBasis:     position.CostBasis,
				LastSalePrice: position.LastSalePrice,
				LastSaleDate:  position.LastSaleDate.AsTime(),
			})
		}
		return &portfolio, nil
	*/
}

/*
Continue
Continue after day completed to trigger a new Day
*/
func (z *Zipline) Continue(orders *[]engine.Order) error {
	continue_req := backtest_pb.ContinueRequest{
		Orders: make([]*backtest_pb.Order, 0),
	}
	for _, order := range *orders {
		continue_req.Orders = append(continue_req.Orders, &backtest_pb.Order{
			Symbol: order.Symbol,
			Amount: order.Amount,
		})
	}
	data, err := proto.Marshal(&continue_req)
	if err != nil {
		return fmt.Errorf("error marshalling continue request: %w", err)
	}
	request := service_pb.Request{
		Task: "continue",
		Data: data,
	}
	response := service_pb.Response{}
	err = z.socket.Request(&request, &response)
	if err != nil {
		return fmt.Errorf("error requesting continue: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("error continuing: %v", response.Error)
	}
	return nil
}

/*
GetResult
Gets the result of the execution

TODO: How to use bigger buffer in req.Process fashion? or how to send result in batches
*/
func (z *Zipline) GetExecutionResult(executionID string) (*engine.Result, error) {
	upload_req := backtest_pb.UploadResultRequest{
		Execution: executionID,
	}
	data, err := proto.Marshal(&upload_req)
	if err != nil {
		return nil, fmt.Errorf("error marshalling upload result request: %w", err)
	}
	request := service_pb.Request{
		Task: "upload_result",
		Data: data,
	}
	response := service_pb.Response{}
	err = z.socket.Request(&request, &response)
	if err != nil {
		return nil, fmt.Errorf("error requesting upload result: %w", err)
	}

	request = service_pb.Request{
		Task: "get_execution_result",
	}
	response = service_pb.Response{}
	err = z.socket.Request(&request, &response)
	if err != nil {
		return nil, fmt.Errorf("error requesting result: %w", err)
	}

	result_rsp := backtest_pb.ResultResponse{}
	err = proto.Unmarshal(response.Data, &result_rsp)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling result: %w", err)
	}
	periods := make([]engine.Period, 0)
	for _, period := range result_rsp.Periods {
		periods = append(periods, engine.Period{
			Timestamp:             period.Timestamp.AsTime(),
			PNL:                   period.PNL,
			Returns:               period.Returns,
			PortfolioValue:        period.PortfolioValue,
			LongsCount:            period.LongsCount,
			ShortsCount:           period.ShortsCount,
			LongValue:             period.LongValue,
			ShortValue:            period.ShortValue,
			StartingExposure:      period.StartingExposure,
			EndingExposure:        period.EndingExposure,
			LongExposure:          period.LongExposure,
			ShortExposure:         period.ShortExposure,
			CapitalUsed:           period.CapitalUsed,
			GrossLeverage:         period.GrossLeverage,
			NetLeverage:           period.NetLeverage,
			StartingValue:         period.StartingValue,
			EndingValue:           period.EndingValue,
			StartingCash:          period.StartingCash,
			EndingCash:            period.EndingCash,
			MaxDrawdown:           period.MaxDrawdown,
			MaxLeverage:           period.MaxLeverage,
			ExcessReturn:          period.ExcessReturn,
			TreasuryPeriodReturn:  period.TreasuryPeriodReturn,
			AlgorithmPeriodReturn: period.AlgorithmPeriodReturn,
			AlgoVolatility:        period.AlgoVolatility,
			Sharpe:                period.Sharpe,
			Sortino:               period.Sortino,
			BenchmarkPeriodReturn: period.BenchmarkPeriodReturn,
			BenchmarkVolatility:   period.BenchmarkVolatility,
			Alpha:                 period.Alpha,
			Beta:                  period.Beta,
		})
	}
	return &engine.Result{Periods: periods}, nil
}

/*
Stop
Stops the running execution
*/
func (z *Zipline) Stop(ctx context.Context) error {
	request := service_pb.Request{
		Task: "stop",
	}
	response := service_pb.Response{}
	err := z.socket.Request(&request, &response)
	if err != nil {
		return fmt.Errorf("error requesting stop: %w", err)
	}
	return nil
}
