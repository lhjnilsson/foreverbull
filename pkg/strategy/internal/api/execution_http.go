package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/web"
	backtestpb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicepb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/stream"
)

// ServiceLookup reads one stored service by image.
// The image may contain slashes and is not a path segment.
type ServiceLookup interface {
	Get(ctx context.Context, image string) (*servicepb.Service, error)
}

type executionRoutes struct {
	services ServiceLookup
	launcher stream.ExperimentLauncher
	db       postgres.Query
}

type executionStartBody struct {
	Service      string `json:"service"`
	ServiceOwner string `json:"service_owner"`
}

// RegisterExecutionHTTP mounts POST /api/backtests/{name}/executions.
// The handler loads the stored service and calls launcher.Run.
// It does not dial session gRPC and does not start a container.
func RegisterExecutionHTTP(mux *http.ServeMux, services ServiceLookup, launcher stream.ExperimentLauncher) {
	mountExecutionHTTP(mux, executionRoutes{services: services, launcher: launcher})
}

// RegisterExecutionHTTPWithAccess is the same mount with a database.
// While authentication is on, a missing database refuses the run.
func RegisterExecutionHTTPWithAccess(mux *http.ServeMux, services ServiceLookup, launcher stream.ExperimentLauncher, db postgres.Query) {
	mountExecutionHTTP(mux, executionRoutes{services: services, launcher: launcher, db: db})
}

func mountExecutionHTTP(mux *http.ServeMux, routes executionRoutes) {
	if mux == nil || routes.services == nil || routes.launcher == nil {
		return
	}
	mux.HandleFunc("POST /api/backtests/{name}/executions", routes.start)
}

func (h executionRoutes) start(w http.ResponseWriter, r *http.Request) {
	if h.services == nil || h.launcher == nil {
		web.WriteError(w, errors.New("execution start is not configured"))
		return
	}
	name := r.PathValue("name")
	if name == "" {
		web.NotFound(w)
		return
	}
	image, owner, ok := executionImage(w, r)
	if !ok {
		return
	}
	if environment.GetAuthEnabled() {
		h.startOwned(w, r, name, image, owner)
		return
	}
	stored, err := h.services.Get(r.Context(), image)
	if err != nil {
		if service.IsNotFound(err) {
			web.NotFound(w)
			return
		}
		web.WriteError(w, err)
		return
	}
	if stored == nil {
		web.NotFound(w)
		return
	}
	algorithm := stored.GetAlgorithm()
	if algorithm == nil {
		writeNoAlgorithm(w, image)
		return
	}
	serviceImage := stored.GetImage()
	if serviceImage == "" {
		serviceImage = image
	}
	// StartDate and EndDate stay unset so SessionLauncher uses the backtest period.
	// Algorithm is the stored one. A nil algorithm is refused above and is not passed.
	out, err := h.launcher.Run(r.Context(), stream.RunInput{
		BacktestName: name,
		ServiceImage: serviceImage,
		Algorithm:    algorithm,
	})
	if err != nil {
		if errors.Is(err, stream.ErrBacktestMissing) {
			web.NotFound(w)
			return
		}
		web.WriteError(w, err)
		return
	}
	writeStarted(w, name, out)
}

func executionImage(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	var body executionStartBody
	if err := web.ReadJSON(r, &body); err != nil {
		web.BadRequest(w)
		return "", "", false
	}
	image := strings.TrimSpace(body.Service)
	if image == "" {
		web.BadRequest(w)
		return "", "", false
	}
	return image, strings.TrimSpace(body.ServiceOwner), true
}

func (h executionRoutes) startOwned(w http.ResponseWriter, r *http.Request, name, image, serviceOwner string) {
	account, ok := auth.FromContext(r.Context())
	if !ok || account.ID == "" {
		web.Unauthorized(w)
		return
	}
	if h.db == nil {
		web.Forbidden(w)
		return
	}
	backtestID, err := auth.Resolve(r.Context(), h.db, account, auth.DomainBacktest, name, r.URL.Query().Get("owner"), auth.ActionTrigger)
	if err != nil {
		auth.WriteHTTP(w, err)
		return
	}
	serviceID, err := auth.Resolve(r.Context(), h.db, account, auth.DomainService, image, serviceOwner, auth.ActionRead)
	if err != nil {
		auth.WriteHTTP(w, err)
		return
	}
	byID, ok := h.services.(interface {
		GetByID(ctx context.Context, id string) (*servicepb.Service, error)
	})
	if !ok {
		web.Forbidden(w)
		return
	}
	stored, err := byID.GetByID(r.Context(), serviceID)
	if err != nil {
		if service.IsNotFound(err) {
			web.NotFound(w)
			return
		}
		web.WriteError(w, err)
		return
	}
	if stored == nil || stored.GetAlgorithm() == nil {
		if stored == nil {
			web.NotFound(w)
			return
		}
		writeNoAlgorithm(w, image)
		return
	}
	serviceImage := stored.GetImage()
	if serviceImage == "" {
		serviceImage = image
	}
	out, err := h.launcher.Run(r.Context(), stream.RunInput{
		BacktestName: name,
		BacktestID:   backtestID,
		ServiceImage: serviceImage,
		Algorithm:    stored.GetAlgorithm(),
	})
	if err != nil {
		if errors.Is(err, stream.ErrBacktestMissing) {
			web.NotFound(w)
			return
		}
		web.WriteError(w, err)
		return
	}
	writeStarted(w, name, out)
}

func writeStarted(w http.ResponseWriter, name string, out *stream.RunOutput) {
	if out == nil || out.ExecutionID == "" {
		web.WriteError(w, errors.New("execution id is empty"))
		return
	}
	web.WriteMessage(w, &backtestpb.Execution{
		Id:       out.ExecutionID,
		Backtest: name,
		Session:  out.SessionID,
	})
}

func writeNoAlgorithm(w http.ResponseWriter, image string) {
	encoded, err := json.Marshal(struct {
		Error string `json:"error"`
	}{
		Error: fmt.Sprintf("service %q has no algorithm", image),
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	if _, err := w.Write(encoded); err != nil {
		return
	}
}
