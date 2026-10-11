package backtest

import (
	"context"
	"errors"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewLazyEngineDoesNotTouchZipline(t *testing.T) {
	t.Parallel()

	ce := container.NewMockEngine(t)
	engine := NewLazyEngine(ce)
	require.NotNil(t, engine)
}

func TestLazyEngineInitMissingContainer(t *testing.T) {
	t.Setenv("ZIPLINE_CONTAINER_NAME", "zipline")

	ce := container.NewMockEngine(t)
	ce.On("Get", mock.Anything, "zipline").Return(nil, errors.New("not found")).Once()

	err := NewLazyEngine(ce).Init(context.Background())
	require.ErrorContains(t, err, "failed to get container for zipline")
}

func TestLazyEngineInitUnhealthy(t *testing.T) {
	t.Setenv("ZIPLINE_CONTAINER_NAME", "zipline")

	ce := container.NewMockEngine(t)
	c := container.NewMockContainer(t)
	ce.On("Get", mock.Anything, "zipline").Return(c, nil).Once()
	c.On("GetHealth").Return("unhealthy", nil).Once()

	err := NewLazyEngine(ce).Init(context.Background())
	require.ErrorContains(t, err, "zipline container is unhealthy")
}

func TestLazyEngineInitHealthError(t *testing.T) {
	t.Setenv("ZIPLINE_CONTAINER_NAME", "zipline")

	ce := container.NewMockEngine(t)
	c := container.NewMockContainer(t)
	ce.On("Get", mock.Anything, "zipline").Return(c, nil).Once()
	c.On("GetHealth").Return("", errors.New("no health")).Once()

	err := NewLazyEngine(ce).Init(context.Background())
	require.ErrorContains(t, err, "failed to get health for zipline container")
}

func TestLazyEngineInitCachesSuccess(t *testing.T) {
	t.Setenv("ZIPLINE_CONTAINER_NAME", "zipline")

	ce := container.NewMockEngine(t)
	c := container.NewMockContainer(t)
	ce.On("Get", mock.Anything, "zipline").Return(c, nil).Once()
	c.On("GetHealth").Return(container.HealthHealthy, nil).Once()
	c.On("GetConnectionString").Return("127.0.0.1:50055", nil).Once()

	engine := NewLazyEngine(ce)
	require.NoError(t, engine.Init(context.Background()))
	require.NoError(t, engine.Init(context.Background()))
}

func TestLazyEngineInitRetriesAfterFailure(t *testing.T) {
	t.Setenv("ZIPLINE_CONTAINER_NAME", "zipline")

	ce := container.NewMockEngine(t)
	c := container.NewMockContainer(t)
	ce.On("Get", mock.Anything, "zipline").Return(nil, errors.New("down")).Once()
	ce.On("Get", mock.Anything, "zipline").Return(c, nil).Once()
	c.On("GetHealth").Return(container.HealthHealthy, nil).Once()
	c.On("GetConnectionString").Return("127.0.0.1:50055", nil).Once()

	engine := NewLazyEngine(ce)
	require.Error(t, engine.Init(context.Background()))
	require.NoError(t, engine.Init(context.Background()))
}

func TestLazyEngineMethodsInitOnUse(t *testing.T) {
	t.Setenv("ZIPLINE_CONTAINER_NAME", "zipline")

	ce := container.NewMockEngine(t)
	ce.On("Get", mock.Anything, "zipline").Return(nil, errors.New("missing"))

	engine := NewLazyEngine(ce)

	err := engine.Ingest(context.Background(), nil, nil)
	require.ErrorContains(t, err, "failed to get container for zipline")

	err = engine.DownloadIngestion(context.Background(), nil)
	require.ErrorContains(t, err, "failed to get container for zipline")

	_, err = engine.NewSession(context.Background(), nil)
	require.ErrorContains(t, err, "failed to get container for zipline")
}
