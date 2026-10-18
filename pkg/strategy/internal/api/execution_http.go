package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
}

type executionStartBody struct {
	Service string `json:"service"`
}

// RegisterExecutionHTTP mounts POST /api/backtests/{name}/executions.
// The handler loads the stored service and calls launcher.Run.
// It does not dial session gRPC and does not start a container.
func RegisterExecutionHTTP(mux *http.ServeMux, services ServiceLookup, launcher stream.ExperimentLauncher) {
	if mux == nil || services == nil || launcher == nil {
		return
	}
	routes := executionRoutes{services: services, launcher: launcher}
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
	image, ok := executionImage(w, r)
	if !ok {
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

func executionImage(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body executionStartBody
	if err := web.ReadJSON(r, &body); err != nil {
		web.BadRequest(w)
		return "", false
	}
	image := strings.TrimSpace(body.Service)
	if image == "" {
		web.BadRequest(w)
		return "", false
	}
	return image, true
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
