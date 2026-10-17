package service_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/web"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeServices struct {
	items  []*pb.Service
	err    error
	got    string
	listed bool
}

func (f *fakeServices) List(context.Context) ([]*pb.Service, error) {
	f.listed = true
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

func (f *fakeServices) Get(_ context.Context, image string) (*pb.Service, error) {
	f.got = image
	if f.err != nil {
		return nil, f.err
	}
	for _, item := range f.items {
		if item.GetImage() == image {
			return item, nil
		}
	}
	return nil, errors.New("service not found")
}

func sampleService() *pb.Service {
	lookback := "20"
	return &pb.Service{
		Image: "lhjnilsson/algo:local",
		Algorithm: &pb.Algorithm{
			FilePath:   "algo.py",
			Namespaces: []string{"close"},
			Functions: []*pb.Algorithm_Function{{
				Name:     "handle_data",
				RunFirst: true,
				Parameters: []*pb.Algorithm_FunctionParameter{{
					Key:          "lookback",
					DefaultValue: &lookback,
					ValueType:    "int",
				}},
			}},
		},
		Statuses: []*pb.Service_Status{{
			Status:     pb.Service_Status_READY,
			OccurredAt: timestamppb.New(time.Date(2024, 3, 9, 0, 0, 0, 0, time.UTC)),
		}},
	}
}

func TestHTTP(t *testing.T) {
	sample := sampleService()
	want, err := web.Marshal(sample)
	require.NoError(t, err)

	t.Run("list", func(t *testing.T) {
		mux := http.NewServeMux()
		service.RegisterHTTP(mux, &fakeServices{items: []*pb.Service{sample}})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/services", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "["+string(want)+"]", rec.Body.String())
	})

	t.Run("image query", func(t *testing.T) {
		fake := &fakeServices{items: []*pb.Service{sample}}
		mux := http.NewServeMux()
		service.RegisterHTTP(mux, fake)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/services?image=lhjnilsson%2Falgo%3Alocal", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, string(want), rec.Body.String())
		require.Equal(t, "lhjnilsson/algo:local", fake.got)
		require.False(t, fake.listed)
	})

	t.Run("missing image", func(t *testing.T) {
		mux := http.NewServeMux()
		service.RegisterHTTP(mux, &fakeServices{})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/services?image=missing/image:local", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
		require.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
	})

	t.Run("list failure hides the cause", func(t *testing.T) {
		mux := http.NewServeMux()
		service.RegisterHTTP(mux, &fakeServices{err: errors.New("postgres password=secret")})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/services", nil))

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.NotContains(t, rec.Body.String(), "secret")
	})

	t.Run("no register route", func(t *testing.T) {
		mux := http.NewServeMux()
		service.RegisterHTTP(mux, &fakeServices{})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/services", nil))

		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}
