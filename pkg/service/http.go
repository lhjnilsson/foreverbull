package service

import (
	"context"
	"net/http"

	"github.com/lhjnilsson/foreverbull/internal/web"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
)

// ServiceStore reads stored services. *repository.Service satisfies it.
// There is no service gRPC list. The image is the id and is not a path segment.
type ServiceStore interface {
	List(ctx context.Context) ([]*pb.Service, error)
	Get(ctx context.Context, image string) (*pb.Service, error)
}

var _ ServiceStore = (*repository.Service)(nil)

type serviceRoutes struct {
	store ServiceStore
}

// RegisterHTTP mounts service reads on mux.
// One service is selected with the image query because an image may contain slashes.
func RegisterHTTP(mux *http.ServeMux, store ServiceStore) {
	if mux == nil || store == nil {
		return
	}
	routes := serviceRoutes{store: store}
	mux.HandleFunc("GET /api/services", routes.serve)
}

func (h serviceRoutes) serve(w http.ResponseWriter, r *http.Request) {
	image := r.URL.Query().Get("image")
	if image != "" {
		h.one(w, r, image)
		return
	}
	h.list(w, r)
}

func (h serviceRoutes) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.List(r.Context())
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.WriteMessages(w, items)
}

func (h serviceRoutes) one(w http.ResponseWriter, r *http.Request, image string) {
	item, err := h.store.Get(r.Context(), image)
	if err != nil {
		if missingService(err) {
			web.NotFound(w)
			return
		}
		web.WriteError(w, err)
		return
	}
	if item == nil {
		web.NotFound(w)
		return
	}
	web.WriteMessage(w, item)
}

// repository.Service.Get reports a missing image as this text, not pgx.ErrNoRows.
func missingService(err error) bool {
	return err != nil && err.Error() == "service not found"
}
