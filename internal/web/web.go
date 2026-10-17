package web

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"go.uber.org/fx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	// readHeaderTimeout bounds how long the process waits for request headers.
	readHeaderTimeout = 5 * time.Second

	// maxJSONBytes caps one JSON response. A list stops at this size.
	maxJSONBytes = 4 << 20
)

// NewMux is the process HTTP mux. Domain modules add routes to the same mux.
// gRPC stays on its own listener.
func NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health)
	mux.HandleFunc("GET /", serveApp(pageFiles()))
	return mux
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []byte(`{"ok":true}`))
}

func listenAddress() string {
	return ":" + environment.GetHTTPPort()
}

// Marshal encodes one protobuf message the same way the list and detail routes do.
// Default scalars stay (a disabled symbol, an empty provider string, status CREATED).
// Unset optional fields stay absent so the JSON does not invent nulls.
func Marshal(msg proto.Message) ([]byte, error) {
	encoded, err := (protojson.MarshalOptions{EmitDefaultValues: true}).Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encode json: %w", err)
	}
	return encoded, nil
}

// WriteMessage writes one protobuf message as protojson.
func WriteMessage(w http.ResponseWriter, msg proto.Message) {
	encoded, err := Marshal(msg)
	if err != nil {
		WriteError(w, err)
		return
	}
	if len(encoded) > maxJSONBytes {
		WriteError(w, fmt.Errorf("response exceeds %d bytes", maxJSONBytes))
		return
	}
	writeJSON(w, http.StatusOK, encoded)
}

// WriteMessages writes a JSON array of protojson objects, one per list item.
func WriteMessages[T proto.Message](w http.ResponseWriter, items []T) {
	body, err := encodeList(items)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func encodeList[T proto.Message](items []T) ([]byte, error) {
	body := make([]byte, 0, 2)
	body = append(body, '[')
	for i := range items {
		encoded, err := Marshal(items[i])
		if err != nil {
			return nil, err
		}
		next := len(body) + len(encoded) + 1
		if i > 0 {
			next++
		}
		if next > maxJSONBytes {
			return nil, fmt.Errorf("response exceeds %d bytes", maxJSONBytes)
		}
		if i > 0 {
			body = append(body, ',')
		}
		body = append(body, encoded...)
	}
	body = append(body, ']')
	return body, nil
}

// NotFound writes the missing-resource response.
func NotFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, []byte(`{"error":"not found"}`))
}

// WriteError maps a missing row to 404. Other failures stay 500 and do not echo the cause.
func WriteError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	if status.Code(err) == codes.NotFound || errors.Is(err, pgx.ErrNoRows) {
		NotFound(w)
		return
	}
	writeJSON(w, http.StatusInternalServerError, []byte(`{"error":"internal"}`))
}

func writeJSON(w http.ResponseWriter, code int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err := w.Write(body); err != nil {
		return
	}
}

// start listens on HTTP_PORT. Serve runs on its own listener, not the gRPC port.
// Shutdown makes Serve return ErrServerClosed. That is a stop, not a crash.
func start(lc fx.Lifecycle, mux *http.ServeMux) error {
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			listener, err := net.Listen("tcp", listenAddress())
			if err != nil {
				return fmt.Errorf("failed to listen: %w", err)
			}
			go func() {
				if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
					panic(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := server.Shutdown(ctx); err != nil {
				return fmt.Errorf("failed to stop http: %w", err)
			}
			return nil
		},
	})
	return nil
}

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(NewMux),
	fx.Invoke(start),
)
