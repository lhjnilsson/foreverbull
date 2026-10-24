package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/web"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WriteHTTP maps an authorization error onto the JSON body the page reads.
// The cause is not copied onto the wire.
func WriteHTTP(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, ErrUnauthorized):
		web.Unauthorized(w)
	case errors.Is(err, ErrForbidden):
		web.Forbidden(w)
	case errors.Is(err, ErrNotFound):
		web.NotFound(w)
	case errors.Is(err, ErrAccountNotFound):
		web.StatusJSON(w, http.StatusNotFound, `{"error":"account not found"}`)
	case errors.Is(err, ErrGrantNotFound):
		web.StatusJSON(w, http.StatusNotFound, `{"error":"grant not found"}`)
	case errors.Is(err, ErrNameAmbiguous):
		web.Conflict(w, ErrNameAmbiguous.Error())
	case errors.Is(err, ErrLoginAmbiguous):
		web.Conflict(w, ErrLoginAmbiguous.Error())
	case errors.Is(err, ErrNotApproved):
		web.Conflict(w, ErrNotApproved.Error())
	case errors.Is(err, ErrOwnerGrant):
		web.Conflict(w, ErrOwnerGrant.Error())
	case errors.Is(err, ErrAlreadyExists):
		web.Conflict(w, ErrAlreadyExists.Error())
	case errors.Is(err, ErrBadRequest), errors.Is(err, ErrPasswordLength), errors.Is(err, ErrUsername):
		web.BadRequest(w)
	default:
		web.WriteError(w, err)
	}
}

// GRPCStatus maps an authorization error onto a generic status string.
func GRPCStatus(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrUnauthorized):
		return status.Error(codes.Unauthenticated, "unauthorized")
	case errors.Is(err, ErrForbidden):
		return status.Error(codes.PermissionDenied, "forbidden")
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrAccountNotFound), errors.Is(err, ErrGrantNotFound):
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, ErrNameAmbiguous):
		return status.Error(codes.FailedPrecondition, ErrNameAmbiguous.Error())
	case errors.Is(err, ErrLoginAmbiguous):
		return status.Error(codes.FailedPrecondition, ErrLoginAmbiguous.Error())
	case errors.Is(err, ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, ErrAlreadyExists.Error())
	default:
		return err
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	encoded, err := json.Marshal(body)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(encoded)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
