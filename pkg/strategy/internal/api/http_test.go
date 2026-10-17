package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/web"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/api"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeStrategies struct {
	list    *servicePb.ListStrategiesResponse
	listErr error
	get     *servicePb.GetStrategyResponse
	getErr  error
	name    string
}

func (f *fakeStrategies) ListStrategies(context.Context, *servicePb.ListStrategiesRequest) (*servicePb.ListStrategiesResponse, error) {
	return f.list, f.listErr
}

func (f *fakeStrategies) GetStrategy(_ context.Context, req *servicePb.GetStrategyRequest) (*servicePb.GetStrategyResponse, error) {
	f.name = req.GetName()
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.get, nil
}

func sampleStrategy() *pb.Strategy {
	description := "mean reversion"
	return &pb.Strategy{
		Id:          "st-1",
		Name:        "mean-rev",
		Description: &description,
		CreatedAt:   timestamppb.New(time.Date(2024, 3, 9, 0, 0, 0, 0, time.UTC)),
	}
}

func TestHTTP(t *testing.T) {
	sample := sampleStrategy()
	want, err := web.Marshal(sample)
	require.NoError(t, err)

	t.Run("list", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeStrategies{
			list: &servicePb.ListStrategiesResponse{Strategies: []*pb.Strategy{sample}},
		})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/strategies", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "["+string(want)+"]", rec.Body.String())
	})

	t.Run("get", func(t *testing.T) {
		fake := &fakeStrategies{
			get: &servicePb.GetStrategyResponse{Strategy: sample},
		}
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, fake)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/strategies/mean-rev", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, string(want), rec.Body.String())
		require.Equal(t, "mean-rev", fake.name)
	})

	t.Run("missing", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeStrategies{getErr: pgx.ErrNoRows})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/strategies/missing", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("grpc not found", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeStrategies{
			getErr: status.Error(codes.NotFound, "strategy \"missing\" not found"),
		})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/strategies/missing", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("no create route", func(t *testing.T) {
		mux := http.NewServeMux()
		api.RegisterHTTP(mux, &fakeStrategies{})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/strategies", nil))

		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}
