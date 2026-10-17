package api

import (
	"context"
	"net/http"

	"github.com/lhjnilsson/foreverbull/internal/web"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
)

// StrategyHTTP is the in-process read API for GET /api/strategies.
// *StrategyServer satisfies it.
type StrategyHTTP interface {
	ListStrategies(context.Context, *servicePb.ListStrategiesRequest) (*servicePb.ListStrategiesResponse, error)
	GetStrategy(context.Context, *servicePb.GetStrategyRequest) (*servicePb.GetStrategyResponse, error)
}

var _ StrategyHTTP = (*StrategyServer)(nil)

type strategyRoutes struct {
	api StrategyHTTP
}

// RegisterHTTP mounts strategy reads on mux.
// The handler calls StrategyServer in-process and does not dial gRPC.
func RegisterHTTP(mux *http.ServeMux, strategies StrategyHTTP) {
	if mux == nil || strategies == nil {
		return
	}
	routes := strategyRoutes{api: strategies}
	mux.HandleFunc("GET /api/strategies", routes.list)
	mux.HandleFunc("GET /api/strategies/{name}", routes.get)
}

func (h strategyRoutes) list(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ListStrategies(r.Context(), &servicePb.ListStrategiesRequest{})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.WriteMessages(w, resp.GetStrategies())
}

func (h strategyRoutes) get(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		web.NotFound(w)
		return
	}
	resp, err := h.api.GetStrategy(r.Context(), &servicePb.GetStrategyRequest{Name: name})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	if resp.GetStrategy() == nil {
		web.NotFound(w)
		return
	}
	web.WriteMessage(w, resp.GetStrategy())
}
