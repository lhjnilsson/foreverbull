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
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
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

type queryModel struct {
	ExecutionId string `json:"execution_id"`
}

func (d *Datasource) handleGetExecutionMetric(ctx context.Context, req grafana.QueryDataRequest, q grafana.DataQuery) grafana.DataResponse {
	var response grafana.DataResponse
	log := d.log.With("GetExecutionMetric")

	// Unmarshal the JSON into our queryModel.
	var qm queryModel

	err := json.Unmarshal(q.JSON, &qm)
	if err != nil {
		log.Error("fail to parse json", err)
		return grafana.ErrDataResponse(grafana.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	execution, err := d.backend.GetExecution(ctx, &pb.GetExecutionRequest{ExecutionId: "0a7378c6-950b-4e8e-af40-4378c9927e9f"})
	if err != nil {
		return grafana.ErrDataResponse(grafana.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}
	times := make([]time.Time, len(execution.Periods))
	values := make([]float64, len(execution.Periods))

	for i, period := range execution.Periods {
		times[i] = time.Date(int(period.Date.Year), time.Month(int(period.Date.Month)), int(period.Date.Day), 0, 0, 0, 0, time.UTC)
		values[i] = period.PortfolioValue
	}

	// create data frame response.
	// For an overview on data frames and how grafana handles them:
	// https://grafana.com/developers/plugin-tools/introduction/data-frames
	frame := data.NewFrame("response")

	// add fields.
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, times),
		data.NewField("values", nil, values),
	)

	// add the frames to the response.
	response.Frames = append(response.Frames, frame)

	return response
}
