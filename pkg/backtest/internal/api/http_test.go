package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/web"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeBacktests struct {
	list    *servicePb.ListBacktestsResponse
	listErr error
	get     *servicePb.GetBacktestResponse
	getErr  error
	name    string

	createReq *servicePb.CreateBacktestRequest
	create    *servicePb.CreateBacktestResponse
	createErr error

	execReq  *servicePb.ListExecutionsRequest
	execList *servicePb.ListExecutionsResponse
	execErr  error

	oneReq *servicePb.GetExecutionRequest
	one    *servicePb.GetExecutionResponse
	oneErr error
}

func (f *fakeBacktests) ListBacktests(context.Context, *servicePb.ListBacktestsRequest) (*servicePb.ListBacktestsResponse, error) {
	return f.list, f.listErr
}

func (f *fakeBacktests) GetBacktest(_ context.Context, req *servicePb.GetBacktestRequest) (*servicePb.GetBacktestResponse, error) {
	f.name = req.GetName()
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.get, nil
}

func (f *fakeBacktests) CreateBacktest(req *servicePb.CreateBacktestRequest, stream servicePb.BacktestServicer_CreateBacktestServer) error {
	f.createReq = req
	if f.createErr != nil {
		return f.createErr
	}
	if f.create == nil {
		return nil
	}
	return stream.Send(f.create)
}

func (f *fakeBacktests) ListExecutions(_ context.Context, req *servicePb.ListExecutionsRequest) (*servicePb.ListExecutionsResponse, error) {
	f.execReq = req
	if f.execErr != nil {
		return nil, f.execErr
	}
	return f.execList, nil
}

func (f *fakeBacktests) GetExecution(_ context.Context, req *servicePb.GetExecutionRequest) (*servicePb.GetExecutionResponse, error) {
	f.oneReq = req
	if f.oneErr != nil {
		return nil, f.oneErr
	}
	return f.one, nil
}

func sampleBacktest() *backtest.Backtest {
	return &backtest.Backtest{
		Name:      "spy",
		Benchmark: "SPY",
		Provider:  "yahoo",
		Periods: []*backtest.Backtest_Period{{
			StartDate: &pb.Date{Year: 2020, Month: 1, Day: 2},
			EndDate:   &pb.Date{Year: 2020, Month: 12, Day: 31},
			Assets: []*backtest.Backtest_Period_Asset{
				{Symbol: "AAPL", Enabled: true},
				{Symbol: "MSFT", Enabled: false},
			},
		}},
	}
}

func TestHTTP(t *testing.T) {
	sample := sampleBacktest()
	want, err := web.Marshal(sample)
	require.NoError(t, err)

	t.Run("list", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeBacktests{
			list: &servicePb.ListBacktestsResponse{Backtests: []*backtest.Backtest{sample}},
		})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/backtests", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		require.Equal(t, "["+string(want)+"]", rec.Body.String())
	})

	t.Run("get", func(t *testing.T) {
		fake := &fakeBacktests{
			get: &servicePb.GetBacktestResponse{Name: sample.GetName(), Backtest: sample},
		}
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, fake)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/backtests/spy", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, string(want), rec.Body.String())
		require.Equal(t, "spy", fake.name)
	})

	t.Run("missing", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeBacktests{
			getErr: fmt.Errorf("error getting backtest: %w", pgx.ErrNoRows),
		})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/backtests/missing", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
		require.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
	})

	t.Run("grpc not found", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeBacktests{
			getErr: status.Error(codes.NotFound, "missing"),
		})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/backtests/missing", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list failure hides the cause", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeBacktests{listErr: fmt.Errorf("postgres password=secret")})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/backtests", nil))

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.NotContains(t, rec.Body.String(), "secret")
	})

	t.Run("create", func(t *testing.T) {
		final := &servicePb.CreateBacktestResponse{Backtest: sample}
		wantFinal, err := web.Marshal(final)
		require.NoError(t, err)
		fake := &fakeBacktests{create: final}
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, fake)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/backtests", `{
			"name":" spy ",
			"start":"2024-01-02",
			"end":"2024-06-28",
			"provider":"Stooq"
		}`))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, string(wantFinal), rec.Body.String())
		require.Nil(t, fake.createReq.GetEtf())
		got := fake.createReq.GetBacktest().GetBacktest()
		require.Equal(t, "spy", got.GetName())
		require.Empty(t, got.GetBenchmark())
		require.Empty(t, got.GetProfile())
		require.Equal(t, "stooq", got.GetProvider())
		require.Len(t, got.GetPeriods(), 1)
		startDate := got.GetPeriods()[0].GetStartDate()
		endDate := got.GetPeriods()[0].GetEndDate()
		require.Equal(t, int32(2024), startDate.GetYear())
		require.Equal(t, int32(1), startDate.GetMonth())
		require.Equal(t, int32(2), startDate.GetDay())
		require.Equal(t, int32(2024), endDate.GetYear())
		require.Equal(t, int32(6), endDate.GetMonth())
		require.Equal(t, int32(28), endDate.GetDay())
		require.Empty(t, got.GetPeriods()[0].GetAssets())
	})

	t.Run("create omits provider", func(t *testing.T) {
		fake := &fakeBacktests{create: &servicePb.CreateBacktestResponse{Backtest: sample}}
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, fake)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/backtests", `{"name":"spy","start":"2024-01-02","end":"2024-06-28"}`))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, fake.createReq.GetBacktest().GetBacktest().GetProvider())
	})

	t.Run("create rejects a bad date", func(t *testing.T) {
		fake := &fakeBacktests{}
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, fake)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/backtests", `{"name":"spy","start":"2024-13-02","end":"2024-06-28"}`))

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Nil(t, fake.createReq)
	})

	t.Run("create rejects an end before the start", func(t *testing.T) {
		fake := &fakeBacktests{}
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, fake)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/backtests", `{"name":"spy","start":"2024-06-28","end":"2024-01-02"}`))

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Nil(t, fake.createReq)
	})

	t.Run("create rejects an unknown provider", func(t *testing.T) {
		fake := &fakeBacktests{}
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, fake)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/backtests", `{"name":"spy","start":"2024-01-02","end":"2024-06-28","provider":"nope"}`))

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Nil(t, fake.createReq)
	})

	t.Run("create failure hides the cause", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeBacktests{createErr: fmt.Errorf("postgres password=secret")})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/backtests", `{"name":"spy","start":"2024-01-02","end":"2024-06-28"}`))

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.NotContains(t, rec.Body.String(), "secret")
	})

	t.Run("list executions", func(t *testing.T) {
		execution := sampleExecution()
		wantExecution, err := web.Marshal(execution)
		require.NoError(t, err)
		fake := &fakeBacktests{
			execList: &servicePb.ListExecutionsResponse{Executions: []*backtest.Execution{execution}},
		}
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, fake)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/backtests/spy/executions", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "["+string(wantExecution)+"]", rec.Body.String())
		require.Equal(t, "spy", fake.execReq.GetBacktest())
		require.Empty(t, fake.execReq.GetSessionId())
		require.NotContains(t, rec.Body.String(), `"status"`)
	})

	t.Run("get execution", func(t *testing.T) {
		resp := &servicePb.GetExecutionResponse{
			Execution: sampleExecution(),
			Periods: []*backtest.Period{{
				Date:           &pb.Date{Year: 2024, Month: 1, Day: 2},
				PortfolioValue: 10,
			}},
			Functions: []*backtest.ExecutionFunction{{Name: "handle_data"}},
		}
		wantResp, err := web.Marshal(resp)
		require.NoError(t, err)
		fake := &fakeBacktests{one: resp}
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, fake)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/executions/ex-1", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, string(wantResp), rec.Body.String())
		require.Equal(t, "ex-1", fake.oneReq.GetExecutionId())
		require.NotContains(t, rec.Body.String(), `"status"`)
		require.Contains(t, rec.Body.String(), `"handle_data"`)
		require.Contains(t, rec.Body.String(), `"portfolioValue":100`)
	})

	t.Run("missing execution", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeBacktests{oneErr: pgx.ErrNoRows})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/executions/missing", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("execution grpc not found", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeBacktests{oneErr: status.Error(codes.NotFound, "missing")})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/executions/missing", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	// POST is mounted by the strategy module. This mux only has the backtest routes.
	t.Run("start stays unmounted", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeBacktests{})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/backtests/spy/executions", `{"service":"algo:local"}`))

		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}

func sampleExecution() *backtest.Execution {
	return &backtest.Execution{
		Id:        "ex-1",
		Backtest:  "spy",
		Session:   "sess-1",
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 2},
		EndDate:   &pb.Date{Year: 2024, Month: 6, Day: 28},
		Result: &backtest.Period{
			PortfolioValue: 100,
		},
	}
}

func jsonRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
