package backtest

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

// LazyEngine connects to Zipline on first use so broker boot does not require a healthy container.
type LazyEngine struct {
	ce container.Engine

	mu    sync.Mutex
	inner engine.Engine
}

func NewLazyEngine(ce container.Engine) *LazyEngine {
	return &LazyEngine{ce: ce}
}

func (l *LazyEngine) Init(ctx context.Context) error {
	_, err := l.get(ctx)
	return err
}

func (l *LazyEngine) get(ctx context.Context) (engine.Engine, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.inner != nil {
		return l.inner, nil
	}

	c, err := l.ce.Get(ctx, environment.GetZiplineContainerName())
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

	inner, err := NewZiplineEngine(ctx, c, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zipline engine: %w", err)
	}

	l.inner = inner
	return l.inner, nil
}

func (l *LazyEngine) Ingest(ctx context.Context, bt *pb.Backtest, object *storage.Object) error {
	inner, err := l.get(ctx)
	if err != nil {
		return err
	}
	return inner.Ingest(ctx, bt, object)
}

func (l *LazyEngine) DownloadIngestion(ctx context.Context, object *storage.Object) error {
	inner, err := l.get(ctx)
	if err != nil {
		return err
	}
	return inner.DownloadIngestion(ctx, object)
}

func (l *LazyEngine) NewSession(ctx context.Context, session *pb.Session) (engine.EngineSession, error) {
	inner, err := l.get(ctx)
	if err != nil {
		return nil, err
	}
	return inner.NewSession(ctx, session)
}
