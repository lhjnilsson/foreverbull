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
		foreverbull = "localhost:50055"
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
		conn:     conn,
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
	conn     *grpc.ClientConn
	grafana.CallResourceHandler
	log log.Logger
}

func (d *Datasource) Dispose() {
	if d == nil || d.conn == nil {
		return
	}
	if err := d.conn.Close(); err != nil {
		d.log.Error("fail to close broker connection", err)
	}
	d.conn = nil
}

func (d *Datasource) QueryData(ctx context.Context, req *grafana.QueryDataRequest) (*grafana.QueryDataResponse, error) {
	return d.queryMux.QueryData(ctx, req)
}

func (d *Datasource) CheckHealth(ctx context.Context, _ *grafana.CheckHealthRequest) (*grafana.CheckHealthResult, error) {
	if d == nil || d.backend == nil {
		return &grafana.CheckHealthResult{
			Status:  grafana.HealthStatusError,
			Message: "broker client is not configured",
		}, nil
	}

	if _, err := d.backend.ListBacktests(ctx, &pb.ListBacktestsRequest{}); err != nil {
		return &grafana.CheckHealthResult{
			Status:  grafana.HealthStatusError,
			Message: fmt.Sprintf("broker unreachable: %v", err),
		}, nil
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
	Returns        QueryMetric = "returns"
	PnL            QueryMetric = "pnl"
	Cash           QueryMetric = "cash"
	MaxDrawdown    QueryMetric = "max_drawdown"
	Benchmark      QueryMetric = "benchmark"
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

var listedMetrics = []QueryMetric{
	PortfolioValue,
	Returns,
	PnL,
	Cash,
	MaxDrawdown,
	Benchmark,
	Alpha,
	Beta,
	Sharpe,
	Sortino,
	CapitalUsed,
	LongCount,
	ShortCount,
	LongValue,
	ShortValue,
}

// Execution is the canonical query-JSON object for an execution id.
// Canonical body (ANALYSIS.md §6):
//
//	{"queryType":"GetExecutionMetric","execution":{"id":"<uuid>"},"metrics":[{"name":"portfolio_value"}]}
//
// Legacy dashboards may send "executionId" as a string; that is still accepted.
type Execution struct {
	ID string `json:"id"`
}

type Metric struct {
	Name QueryMetric `json:"name"`
}

type queryModel struct {
	ExecutionID string
	Metrics     []Metric
}

func parseExecutionRef(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var id string
	if err := json.Unmarshal(raw, &id); err == nil {
		return id
	}
	var obj Execution
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.ID
	}
	return ""
}

func parseMetrics(raw json.RawMessage) ([]Metric, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err == nil {
		metrics := make([]Metric, 0, len(names))
		for _, name := range names {
			metrics = append(metrics, Metric{Name: QueryMetric(name)})
		}
		return metrics, nil
	}
	var metrics []Metric
	if err := json.Unmarshal(raw, &metrics); err != nil {
		return nil, err
	}
	return metrics, nil
}

func (q *queryModel) UnmarshalJSON(b []byte) error {
	var raw struct {
		Execution   json.RawMessage `json:"execution"`
		ExecutionID json.RawMessage `json:"executionId"`
		Metrics     json.RawMessage `json:"metrics"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	q.ExecutionID = parseExecutionRef(raw.Execution)
	if q.ExecutionID == "" {
		q.ExecutionID = parseExecutionRef(raw.ExecutionID)
	}
	metrics, err := parseMetrics(raw.Metrics)
	if err != nil {
		return err
	}
	q.Metrics = metrics
	return nil
}

func periodTime(period *pb.Period) time.Time {
	if period == nil || period.Date == nil {
		return time.Time{}
	}
	return time.Date(int(period.Date.Year), time.Month(int(period.Date.Month)), int(period.Date.Day), 0, 0, 0, 0, time.UTC)
}

func (d *Datasource) handleGetExecutionMetric(ctx context.Context, _ grafana.QueryDataRequest, q grafana.DataQuery) grafana.DataResponse {
	log := d.log.With("GetExecutionMetric")

	var qm queryModel
	err := json.Unmarshal(q.JSON, &qm)
	if err != nil {
		log.Error("fail to parse json", err)
		return grafana.ErrDataResponse(grafana.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	if qm.ExecutionID == "" {
		log.Error("executionId is missing")
		return grafana.ErrDataResponse(grafana.StatusBadRequest, "executionId is missing")
	}

	if len(qm.Metrics) == 0 {
		log.Error("metrics is missing")
		return grafana.ErrDataResponse(grafana.StatusBadRequest, "metrics is missing")
	}

	execution, err := d.backend.GetExecution(ctx, &pb.GetExecutionRequest{ExecutionId: qm.ExecutionID})
	if err != nil {
		return grafana.ErrDataResponse(grafana.StatusBadRequest, fmt.Sprintf("get execution: %v", err.Error()))
	}

	periods := execution.GetPeriods()
	times := make([]time.Time, len(periods))
	portfolioValue := make([]float64, len(periods))
	returns := make([]float64, len(periods))
	pnl := make([]float64, len(periods))
	cash := make([]float64, len(periods))
	maxDrawdown := make([]float64, len(periods))
	benchmark := make([]*float64, len(periods))
	alpha := make([]*float64, len(periods))
	beta := make([]*float64, len(periods))
	sharpe := make([]*float64, len(periods))
	sortino := make([]*float64, len(periods))
	capitalUsed := make([]float64, len(periods))
	longCount := make([]int32, len(periods))
	shortCount := make([]int32, len(periods))
	longValue := make([]float64, len(periods))
	shortValue := make([]float64, len(periods))

	for i, period := range periods {
		times[i] = periodTime(period)
		portfolioValue[i] = period.PortfolioValue
		returns[i] = period.Returns
		pnl[i] = period.PNL
		cash[i] = period.EndingCash
		maxDrawdown[i] = period.MaxDrawdown
		benchmark[i] = period.BenchmarkPeriodReturn
		alpha[i] = period.Alpha
		beta[i] = period.Beta
		sharpe[i] = period.Sharpe
		sortino[i] = period.Sortino
		capitalUsed[i] = period.CapitalUsed
		longCount[i] = period.LongsCount
		shortCount[i] = period.ShortsCount
		longValue[i] = period.LongValue
		shortValue[i] = period.ShortValue
	}

	frame := data.NewFrame("response")
	frame.Fields = append(frame.Fields, data.NewField("time", nil, times))
	for _, metric := range qm.Metrics {
		switch metric.Name {
		case PortfolioValue:
			frame.Fields = append(frame.Fields, data.NewField(string(PortfolioValue), nil, portfolioValue))
		case Returns:
			frame.Fields = append(frame.Fields, data.NewField(string(Returns), nil, returns))
		case PnL:
			frame.Fields = append(frame.Fields, data.NewField(string(PnL), nil, pnl))
		case Cash:
			frame.Fields = append(frame.Fields, data.NewField(string(Cash), nil, cash))
		case MaxDrawdown:
			frame.Fields = append(frame.Fields, data.NewField(string(MaxDrawdown), nil, maxDrawdown))
		case Benchmark:
			frame.Fields = append(frame.Fields, data.NewField(string(Benchmark), nil, benchmark))
		case Alpha:
			frame.Fields = append(frame.Fields, data.NewField(string(Alpha), nil, alpha))
		case Beta:
			frame.Fields = append(frame.Fields, data.NewField(string(Beta), nil, beta))
		case Sharpe:
			frame.Fields = append(frame.Fields, data.NewField(string(Sharpe), nil, sharpe))
		case Sortino:
			frame.Fields = append(frame.Fields, data.NewField(string(Sortino), nil, sortino))
		case CapitalUsed:
			frame.Fields = append(frame.Fields, data.NewField(string(CapitalUsed), nil, capitalUsed))
		case LongCount:
			frame.Fields = append(frame.Fields, data.NewField(string(LongCount), nil, longCount))
		case ShortCount:
			frame.Fields = append(frame.Fields, data.NewField(string(ShortCount), nil, shortCount))
		case LongValue:
			frame.Fields = append(frame.Fields, data.NewField(string(LongValue), nil, longValue))
		case ShortValue:
			frame.Fields = append(frame.Fields, data.NewField(string(ShortValue), nil, shortValue))
		}
	}

	return grafana.DataResponse{
		Frames: []*data.Frame{frame},
		Status: grafana.StatusOK,
	}
}
