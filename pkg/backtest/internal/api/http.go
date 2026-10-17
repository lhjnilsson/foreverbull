package api

import (
	"context"
	"net/http"

	"github.com/lhjnilsson/foreverbull/internal/web"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
)

// BacktestHTTP is the in-process read API for GET /api/backtests.
// *BacktestServer satisfies it.
type BacktestHTTP interface {
	ListBacktests(context.Context, *servicePb.ListBacktestsRequest) (*servicePb.ListBacktestsResponse, error)
	GetBacktest(context.Context, *servicePb.GetBacktestRequest) (*servicePb.GetBacktestResponse, error)
}

var _ BacktestHTTP = (*BacktestServer)(nil)

type backtestRoutes struct {
	api BacktestHTTP
}

// RegisterHTTP mounts backtest reads on mux.
// The handler calls BacktestServer in-process and does not dial gRPC.
func RegisterHTTP(mux *http.ServeMux, backtests BacktestHTTP) {
	if mux == nil || backtests == nil {
		return
	}
	routes := backtestRoutes{api: backtests}
	mux.HandleFunc("GET /api/backtests", routes.list)
	mux.HandleFunc("GET /api/backtests/{name}", routes.get)
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
