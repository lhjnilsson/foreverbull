package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/web"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
)

// StrategyHTTP is the in-process read API for GET /api/strategies.
// *StrategyServer satisfies it. CreateStrategy is unimplemented and is not part of this API.
type StrategyHTTP interface {
	ListStrategies(context.Context, *servicePb.ListStrategiesRequest) (*servicePb.ListStrategiesResponse, error)
	GetStrategy(context.Context, *servicePb.GetStrategyRequest) (*servicePb.GetStrategyResponse, error)
}

var _ StrategyHTTP = (*StrategyServer)(nil)

// StrategyWriter is the stored-strategy insert. *repository.Strategy satisfies it.
// Description is omitted (nil). The unimplemented CreateStrategy RPC is not this method.
type StrategyWriter interface {
	Create(ctx context.Context, name string, description *string) (*pb.Strategy, error)
}

var _ StrategyWriter = (*repository.Strategy)(nil)

type strategyRoutes struct {
	api    StrategyHTTP
	writer StrategyWriter
}

// RegisterHTTP mounts strategy reads and create on mux.
// The handler calls StrategyServer and the strategy repository in-process and does not dial gRPC.
// A nil writer leaves POST unmounted. CreateStrategy stays unimplemented.
func RegisterHTTP(mux *http.ServeMux, strategies StrategyHTTP, writer StrategyWriter) {
	if mux == nil || strategies == nil {
		return
	}
	routes := strategyRoutes{api: strategies, writer: writer}
	mux.HandleFunc("GET /api/strategies", routes.list)
	mux.HandleFunc("GET /api/strategies/{name}", routes.get)
	if writer != nil {
		mux.HandleFunc("POST /api/strategies", routes.create)
	}
}

type strategyCreateBody struct {
	Name string `json:"name"`
}

func (h strategyRoutes) create(w http.ResponseWriter, r *http.Request) {
	if h.writer == nil {
		web.WriteError(w, errors.New("strategy writer is not configured"))
		return
	}
	var body strategyCreateBody
	if err := web.ReadJSON(r, &body); err != nil {
		web.BadRequest(w)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		web.BadRequest(w)
		return
	}
	// The form has no description. Persist nil rather than an empty string.
	item, err := h.writer.Create(r.Context(), name, nil)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	if item == nil {
		web.WriteError(w, errors.New("create strategy returned nothing"))
		return
	}
	web.WriteMessage(w, item)
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
