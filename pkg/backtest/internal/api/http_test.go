package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	t.Run("no create route", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeBacktests{})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/backtests", nil))

		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}
