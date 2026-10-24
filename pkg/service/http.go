package service

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
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
	db    postgres.Query
}

// RegisterHTTP mounts service reads and create on mux.
// One service is selected with the image query because an image may contain slashes.
func RegisterHTTP(mux *http.ServeMux, store ServiceStore) {
	mountServiceHTTP(mux, serviceRoutes{store: store})
}

// RegisterHTTPWithAccess is the same mount with a database for authorization.
// While authentication is off the database is unused.
func RegisterHTTPWithAccess(mux *http.ServeMux, store ServiceStore, db postgres.Query) {
	mountServiceHTTP(mux, serviceRoutes{store: store, db: db})
}

func mountServiceHTTP(mux *http.ServeMux, routes serviceRoutes) {
	if mux == nil || routes.store == nil {
		return
	}
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
	if environment.GetAuthEnabled() {
		h.createOwned(w, r, image)
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
	if environment.GetAuthEnabled() {
		h.listOwned(w, r)
		return
	}
	items, err := h.store.List(r.Context())
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.WriteMessages(w, items)
}

func (h serviceRoutes) one(w http.ResponseWriter, r *http.Request, image string) {
	if environment.GetAuthEnabled() {
		h.oneOwned(w, r, image)
		return
	}
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

func (h serviceRoutes) createOwned(w http.ResponseWriter, r *http.Request, image string) {
	account, ok := h.account(w, r)
	if !ok {
		return
	}
	if err := auth.MayCreate(account); err != nil {
		auth.WriteHTTP(w, err)
		return
	}
	owned, ok := h.store.(interface {
		CreateOwned(ctx context.Context, ownerID, image string) (*pb.Service, error)
	})
	if !ok {
		web.Forbidden(w)
		return
	}
	item, err := owned.CreateOwned(r.Context(), account.ID, image)
	if err != nil {
		auth.WriteHTTP(w, err)
		return
	}
	if item == nil {
		web.WriteError(w, errors.New("create service returned nothing"))
		return
	}
	web.WriteMessage(w, item)
}

func (h serviceRoutes) listOwned(w http.ResponseWriter, r *http.Request) {
	account, ok := h.account(w, r)
	if !ok {
		return
	}
	visible, ok := h.store.(interface {
		ListVisible(ctx context.Context, caller auth.Account) ([]*pb.Service, error)
	})
	if !ok {
		web.Forbidden(w)
		return
	}
	items, err := visible.ListVisible(r.Context(), account)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.WriteMessages(w, items)
}

func (h serviceRoutes) oneOwned(w http.ResponseWriter, r *http.Request, image string) {
	account, ok := h.account(w, r)
	if !ok {
		return
	}
	if h.db == nil {
		web.Forbidden(w)
		return
	}
	id, err := auth.Resolve(r.Context(), h.db, account, auth.DomainService, image, r.URL.Query().Get("owner"), auth.ActionRead)
	if err != nil {
		auth.WriteHTTP(w, err)
		return
	}
	byID, ok := h.store.(interface {
		GetByID(ctx context.Context, id string) (*pb.Service, error)
	})
	if !ok {
		web.Forbidden(w)
		return
	}
	item, err := byID.GetByID(r.Context(), id)
	if err != nil {
		if IsNotFound(err) {
			web.NotFound(w)
			return
		}
		web.WriteError(w, err)
		return
	}
	web.WriteMessage(w, item)
}

func (h serviceRoutes) account(w http.ResponseWriter, r *http.Request) (auth.Account, bool) {
	account, ok := auth.FromContext(r.Context())
	if !ok || account.ID == "" {
		web.Unauthorized(w)
		return auth.Account{}, false
	}
	return account, true
}
