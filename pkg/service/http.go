package service

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/web"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
)

// ServiceStore reads and creates stored services. *repository.Service satisfies it.
// There is no service gRPC list or CreateService RPC. The image is the id and is not a path segment.
// Create takes only the image. ServiceStart also creates, then starts a container; this route does not call it.
type ServiceStore interface {
	List(ctx context.Context) ([]*pb.Service, error)
	Get(ctx context.Context, image string) (*pb.Service, error)
	Create(ctx context.Context, image string) (*pb.Service, error)
}

var _ ServiceStore = (*repository.Service)(nil)

type serviceRoutes struct {
	store ServiceStore
}

// RegisterHTTP mounts service reads and create on mux.
// One service is selected with the image query because an image may contain slashes.
func RegisterHTTP(mux *http.ServeMux, store ServiceStore) {
	if mux == nil || store == nil {
		return
	}
	routes := serviceRoutes{store: store}
	mux.HandleFunc("GET /api/services", routes.serve)
	mux.HandleFunc("POST /api/services", routes.create)
}

type serviceCreateBody struct {
	Image string `json:"image"`
}

func (h serviceRoutes) create(w http.ResponseWriter, r *http.Request) {
	var body serviceCreateBody
	if err := web.ReadJSON(r, &body); err != nil {
		web.BadRequest(w)
		return
	}
	image := strings.TrimSpace(body.Image)
	if image == "" {
		web.BadRequest(w)
		return
	}
	item, err := h.store.Create(r.Context(), image)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	if item == nil {
		web.WriteError(w, errors.New("create service returned nothing"))
		return
	}
	web.WriteMessage(w, item)
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
		if IsNotFound(err) {
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
