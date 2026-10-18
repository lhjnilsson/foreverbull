package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/web"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"google.golang.org/grpc/metadata"
)

// BacktestHTTP is the in-process API for backtest and execution routes.
// *BacktestServer satisfies it. Handlers call it directly and do not dial gRPC.
type BacktestHTTP interface {
	ListBacktests(context.Context, *servicePb.ListBacktestsRequest) (*servicePb.ListBacktestsResponse, error)
	GetBacktest(context.Context, *servicePb.GetBacktestRequest) (*servicePb.GetBacktestResponse, error)
	CreateBacktest(*servicePb.CreateBacktestRequest, servicePb.BacktestServicer_CreateBacktestServer) error
	ListExecutions(context.Context, *servicePb.ListExecutionsRequest) (*servicePb.ListExecutionsResponse, error)
	GetExecution(context.Context, *servicePb.GetExecutionRequest) (*servicePb.GetExecutionResponse, error)
}

var _ BacktestHTTP = (*BacktestServer)(nil)

type backtestRoutes struct {
	api BacktestHTTP
}

// RegisterHTTP mounts backtest and execution reads, and backtest create, on mux.
// POST /api/backtests/{name}/executions is mounted by the strategy module, which
// has the experiment launcher. Nothing on BacktestServer accepts a service image,
// so this handler does not start an execution and does not dial session gRPC.
func RegisterHTTP(mux *http.ServeMux, backtests BacktestHTTP) {
	if mux == nil || backtests == nil {
		return
	}
	routes := backtestRoutes{api: backtests}
	mux.HandleFunc("GET /api/backtests", routes.list)
	mux.HandleFunc("POST /api/backtests", routes.create)
	mux.HandleFunc("GET /api/backtests/{name}/executions", routes.executions)
	mux.HandleFunc("GET /api/backtests/{name}", routes.get)
	mux.HandleFunc("GET /api/executions/{id}", routes.execution)
}

func (h backtestRoutes) list(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ListBacktests(r.Context(), &servicePb.ListBacktestsRequest{})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.WriteMessages(w, resp.GetBacktests())
}

func (h backtestRoutes) get(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		web.NotFound(w)
		return
	}
	resp, err := h.api.GetBacktest(r.Context(), &servicePb.GetBacktestRequest{Name: name})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	if resp.GetBacktest() == nil {
		web.NotFound(w)
		return
	}
	web.WriteMessage(w, resp.GetBacktest())
}

type backtestCreateBody struct {
	Name     string `json:"name"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Provider string `json:"provider"`
}

func (h backtestRoutes) create(w http.ResponseWriter, r *http.Request) {
	entity, err := backtestFromRequest(r)
	if err != nil {
		web.BadRequest(w)
		return
	}
	// The ETF oneof needs base_etf and downloads a catalog. This route sends the backtest oneof only.
	req := &servicePb.CreateBacktestRequest{
		Create: &servicePb.CreateBacktestRequest_Backtest{
			Backtest: &servicePb.CreateBacktestRequest_CreateFromBacktest{Backtest: entity},
		},
	}
	stream := &createBacktestStream{ctx: r.Context()}
	if err := h.api.CreateBacktest(req, stream); err != nil {
		web.WriteError(w, err)
		return
	}
	if stream.last == nil {
		web.WriteError(w, errors.New("create backtest returned nothing"))
		return
	}
	web.WriteMessage(w, stream.last)
}

func (h backtestRoutes) executions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		web.NotFound(w)
		return
	}
	resp, err := h.api.ListExecutions(r.Context(), &servicePb.ListExecutionsRequest{Backtest: name})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.WriteMessages(w, resp.GetExecutions())
}

func (h backtestRoutes) execution(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		web.NotFound(w)
		return
	}
	resp, err := h.api.GetExecution(r.Context(), &servicePb.GetExecutionRequest{ExecutionId: id})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	if resp.GetExecution() == nil {
		web.NotFound(w)
		return
	}
	web.WriteMessage(w, resp)
}

// backtestFromRequest maps name, start, end, and optional provider onto one period.
// Start and end are YYYY-MM-DD. An empty provider stays empty so repository create assigns yahoo.
// Benchmark and profile stay empty. The form has no symbols.
func backtestFromRequest(r *http.Request) (*backtest.Backtest, error) {
	var body backtestCreateBody
	if err := web.ReadJSON(r, &body); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return nil, errors.New("missing name")
	}
	start, err := parseDate(body.Start)
	if err != nil {
		return nil, err
	}
	end, err := parseDate(body.End)
	if err != nil {
		return nil, err
	}
	if dateBefore(end, start) {
		return nil, errors.New("end before start")
	}
	provider, err := normalizeProvider(body.Provider)
	if err != nil {
		return nil, err
	}
	return &backtest.Backtest{
		Name:     name,
		Provider: provider,
		Periods: []*backtest.Backtest_Period{{
			StartDate: start,
			EndDate:   end,
		}},
	}, nil
}

func parseDate(value string) (*pb.Date, error) {
	parsed, err := time.Parse(time.DateOnly, strings.TrimSpace(value))
	if err != nil {
		return nil, errors.New("invalid date")
	}
	return &pb.Date{
		Year:  int32(parsed.Year()),
		Month: int32(parsed.Month()),
		Day:   int32(parsed.Day()),
	}, nil
}

func dateBefore(left, right *pb.Date) bool {
	if left.GetYear() != right.GetYear() {
		return left.GetYear() < right.GetYear()
	}
	if left.GetMonth() != right.GetMonth() {
		return left.GetMonth() < right.GetMonth()
	}
	return left.GetDay() < right.GetDay()
}

// normalizeProvider accepts yahoo, stooq, alpaca, or empty.
// Empty is not rewritten here. (*repository.Backtest).Create maps that to yahoo.
func normalizeProvider(raw string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(raw))
	switch provider {
	case "":
		return "", nil
	case "yahoo", "stooq", "alpaca":
		return provider, nil
	default:
		return "", errors.New("unknown provider")
	}
}

// createBacktestStream keeps the last CreateBacktestResponse.
// CreateBacktest sends progress, then the stored backtest on the final message.
type createBacktestStream struct {
	ctx  context.Context
	last *servicePb.CreateBacktestResponse
}

var _ servicePb.BacktestServicer_CreateBacktestServer = (*createBacktestStream)(nil)

func (s *createBacktestStream) Send(msg *servicePb.CreateBacktestResponse) error {
	s.last = msg
	return nil
}

func (s *createBacktestStream) Context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *createBacktestStream) SetHeader(metadata.MD) error { return nil }

func (s *createBacktestStream) SendHeader(metadata.MD) error { return nil }

func (s *createBacktestStream) SetTrailer(metadata.MD) {}

func (s *createBacktestStream) SendMsg(any) error { return nil }

func (s *createBacktestStream) RecvMsg(any) error { return io.EOF }
