package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	grafana "github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ grafana.QueryDataHandler      = (*Datasource)(nil)
	_ grafana.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

func NewDatasource(_ context.Context, _ grafana.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	foreverbull, exists := os.LookupEnv("BROKER_URL")
	if !exists {
		foreverbull = "localhost:50555"
	}

	log := log.DefaultLogger.With("datasource", "foreverbull")

	conn, err := grpc.NewClient(foreverbull, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("Fail to connecto to foreverbull backend", err)
		return nil, fmt.Errorf("could not connect: %w", err)
	}

	client := pb.NewBacktestServicerClient(conn)

	mux := datasource.NewQueryTypeMux()
	ds := &Datasource{
		queryMux: mux,
		backend:  client,
		log:      log,
	}
	ds.registerResources()
	mux.HandleFunc(GetExecutionMetric, ds.HandleGetExecutionMetric)

	return ds, nil
}

type QueryHandlerFunc func(context.Context, grafana.QueryDataRequest, grafana.DataQuery) grafana.DataResponse

func processQueries(ctx context.Context, req *grafana.QueryDataRequest, handler QueryHandlerFunc) *grafana.QueryDataResponse {
	res := grafana.Responses{}
	if req == nil || req.Queries == nil {
		return &grafana.QueryDataResponse{
			Responses: res,
		}
	}
	for _, v := range req.Queries {
		res[v.RefID] = handler(ctx, *req, v)
	}

	return &grafana.QueryDataResponse{
		Responses: res,
	}
}

type Datasource struct {
	queryMux *datasource.QueryTypeMux
	backend  pb.BacktestServicerClient
	grafana.CallResourceHandler
	log log.Logger
}

func (d *Datasource) Dispose() {
	// Clean up datasource instance resources.
}

func (d *Datasource) QueryData(ctx context.Context, req *grafana.QueryDataRequest) (*grafana.QueryDataResponse, error) {
	return d.queryMux.QueryData(ctx, req)
}

func (d *Datasource) CheckHealth(_ context.Context, req *grafana.CheckHealthRequest) (*grafana.CheckHealthResult, error) {
	res := &grafana.CheckHealthResult{}
	_, err := LoadPluginSettings(*req.PluginContext.DataSourceInstanceSettings)

	if err != nil {
		res.Status = grafana.HealthStatusError
		res.Message = "Unable to load settings"
		return res, nil
	}

	return &grafana.CheckHealthResult{
		Status:  grafana.HealthStatusOk,
		Message: "Data source is working",
	}, nil
}

func (d *Datasource) HandleGetExecutionMetric(ctx context.Context, req *grafana.QueryDataRequest) (*grafana.QueryDataResponse, error) {
	return processQueries(ctx, req, d.handleGetExecutionMetric), nil
}

type QueryMetric string

const (
	PortfolioValue QueryMetric = "portfolio_value"
	Alpha          QueryMetric = "alpha"
	Beta           QueryMetric = "beta"
	Sharpe         QueryMetric = "sharpe"
	Sortino        QueryMetric = "sortino"
	CapitalUsed    QueryMetric = "capital_used"
	LongCount      QueryMetric = "long_count"
	ShortCount     QueryMetric = "short_count"
	LongValue      QueryMetric = "long_value"
	ShortValue     QueryMetric = "short_value"
)

type Execution struct {
	ID string `json:"id"`
}

type Metric struct {
	Name QueryMetric `json:"name"`
}

type queryModel struct {
	ExecutionId *Execution `json:"execution"`
	Metrics     []Metric   `json:"metrics"`
}

func (d *Datasource) handleGetExecutionMetric(ctx context.Context, req grafana.QueryDataRequest, q grafana.DataQuery) grafana.DataResponse {
	log := d.log.With("GetExecutionMetric")

	// Unmarshal the JSON into our queryModel.
	var qm queryModel

	err := json.Unmarshal(q.JSON, &qm)
	if err != nil {
		log.Error("fail to parse json", err)
		return grafana.ErrDataResponse(grafana.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	if qm.ExecutionId == nil || qm.ExecutionId.ID == "" {
		log.Error("executionId is missing")
		return grafana.ErrDataResponse(grafana.StatusBadRequest, "executionId is missing")
	}

	if len(qm.Metrics) == 0 {
		log.Error("metrics is missing")
		return grafana.ErrDataResponse(grafana.StatusBadRequest, "metrics is missing")
	}

	execution, err := d.backend.GetExecution(ctx, &pb.GetExecutionRequest{ExecutionId: qm.ExecutionId.ID})
	if err != nil {
		return grafana.ErrDataResponse(grafana.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	times := make([]time.Time, len(execution.Periods))
	portfolio_value := make([]float64, len(execution.Periods))
	alpha := make([]*float64, len(execution.Periods))
	beta := make([]*float64, len(execution.Periods))
	sharpe := make([]*float64, len(execution.Periods))
	sortio := make([]*float64, len(execution.Periods))
	capital_used := make([]float64, len(execution.Periods))
	long_count := make([]int32, len(execution.Periods))
	short_count := make([]int32, len(execution.Periods))
	long_value := make([]float64, len(execution.Periods))
	short_value := make([]float64, len(execution.Periods))

	for i, period := range execution.Periods {
		times[i] = time.Date(int(period.Date.Year), time.Month(int(period.Date.Month)), int(period.Date.Day), 0, 0, 0, 0, time.UTC)
		portfolio_value[i] = period.PortfolioValue
		alpha[i] = period.Alpha
		beta[i] = period.Beta
		sharpe[i] = period.Sharpe
		sortio[i] = period.Sortino
		capital_used[i] = period.CapitalUsed
		long_count[i] = period.LongsCount
		short_count[i] = period.ShortsCount
		long_value[i] = period.LongValue
		short_value[i] = period.ShortValue

	}

	frame := data.NewFrame("response")
	frame.Fields = append(frame.Fields, data.NewField("time", nil, times))
	for _, metric := range qm.Metrics {
		switch metric.Name {
		case PortfolioValue:
			frame.Fields = append(frame.Fields, data.NewField(string(PortfolioValue), nil, portfolio_value))
		case Alpha:
			frame.Fields = append(frame.Fields, data.NewField(string(Alpha), nil, alpha))
		case Beta:
			frame.Fields = append(frame.Fields, data.NewField(string(Beta), nil, beta))
		case Sharpe:
			frame.Fields = append(frame.Fields, data.NewField(string(Sharpe), nil, sharpe))
		case Sortino:
			frame.Fields = append(frame.Fields, data.NewField(string(Sortino), nil, sortio))
		case CapitalUsed:
			frame.Fields = append(frame.Fields, data.NewField(string(CapitalUsed), nil, capital_used))
		case LongCount:
			frame.Fields = append(frame.Fields, data.NewField(string(LongCount), nil, long_count))
		case ShortCount:
			frame.Fields = append(frame.Fields, data.NewField(string(ShortCount), nil, short_count))
		case LongValue:
			frame.Fields = append(frame.Fields, data.NewField(string(LongValue), nil, long_value))
		case ShortValue:
			frame.Fields = append(frame.Fields, data.NewField(string(ShortValue), nil, short_value))
		}
	}

	return grafana.DataResponse{
		Frames: []*data.Frame{frame},
		Status: grafana.StatusOK,
	}
}
