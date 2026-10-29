package backtest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LazyEngine connects to Zipline on first use so broker boot does not require a healthy container.
// A call that fails with Unavailable drops the cached engine, so the next
// call resolves Zipline again instead of holding a dead connection until
// the broker restarts.
type LazyEngine struct {
	ce container.Engine
	// port is the Zipline engine gRPC port on the Service and on the pod.
	port int

	mu    sync.Mutex
	inner *Zipline
}

func NewLazyEngine(ce container.Engine) *LazyEngine {
	return &LazyEngine{ce: ce, port: ziplineControlPort}
}

func (l *LazyEngine) Init(ctx context.Context) error {
	_, err := l.get(ctx)
	return err
}

func (l *LazyEngine) get(ctx context.Context) (*Zipline, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.inner != nil {
		return l.inner, nil
	}

	workload := environment.GetZiplineContainerName()

	c, err := l.ce.Get(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("failed to get container for zipline: %w", err)
	}

	health, err := c.GetHealth()
	if err != nil {
		return nil, fmt.Errorf("failed to get health for zipline container: %w", err)
	}
	if health != container.HealthHealthy {
		return nil, errors.New("zipline container is unhealthy")
	}

	// The workload name is also the Service name (Helm sets
	// ZIPLINE_CONTAINER_NAME=zipline), and the Service follows the pod
	// across restarts.
	inner, err := NewZiplineEngine(ctx, ZiplineTarget{
		ControlAddress:     net.JoinHostPort(workload, strconv.Itoa(l.port)),
		Pods:               l.ce,
		Workload:           workload,
		SessionControlPort: l.port,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zipline engine: %w", err)
	}

	l.inner = inner
	return l.inner, nil
}

// release drops inner when err says Zipline could not be reached. A later
// call resolves the pod again. Other errors come from a Zipline that
// answered, so the connection stays.
func (l *LazyEngine) release(inner *Zipline, err error) {
	if status.Code(err) != codes.Unavailable {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.inner != inner {
		return
	}

	l.inner = nil
	inner.Close() //nolint: errcheck
}

func (l *LazyEngine) Ingest(ctx context.Context, bt *pb.Backtest, object *storage.Object) ([]string, error) {
	inner, err := l.get(ctx)
	if err != nil {
		return nil, err
	}
	notices, err := inner.Ingest(ctx, bt, object)
	l.release(inner, err)
	return notices, err
}

func (l *LazyEngine) DownloadIngestion(ctx context.Context, object *storage.Object) error {
	inner, err := l.get(ctx)
	if err != nil {
		return err
	}
	err = inner.DownloadIngestion(ctx, object)
	l.release(inner, err)
	return err
}

func (l *LazyEngine) NewSession(ctx context.Context, session *pb.Session) (engine.EngineSession, error) {
	inner, err := l.get(ctx)
	if err != nil {
		return nil, err
	}
	backtestSession, err := inner.NewSession(ctx, session)
	l.release(inner, err)
	return backtestSession, err
}
