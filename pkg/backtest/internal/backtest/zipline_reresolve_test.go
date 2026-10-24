package backtest

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"syscall"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeZipline answers the engine API. ingestErrors are returned in order,
// then Ingest succeeds.
type fakeZipline struct {
	servicePb.UnimplementedEngineServer

	mu           sync.Mutex
	ingestErrors []error
	ingestCalls  int
	sessionIDs   []string
	sessionPort  int64
}

func (f *fakeZipline) Ingest(context.Context, *servicePb.IngestRequest) (*servicePb.IngestResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ingestCalls++
	if len(f.ingestErrors) > 0 {
		err := f.ingestErrors[0]
		f.ingestErrors = f.ingestErrors[1:]
		return nil, err
	}
	return &servicePb.IngestResponse{}, nil
}

func (f *fakeZipline) NewSession(_ context.Context, req *servicePb.NewSessionRequest) (*servicePb.NewSessionResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sessionIDs = append(f.sessionIDs, req.GetId())
	return &servicePb.NewSessionResponse{Port: f.sessionPort}, nil
}

func (f *fakeZipline) sessions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sessionIDs...)
}

// serveZipline starts fake on address and returns the port it listens on.
func serveZipline(t *testing.T, address string, fake *fakeZipline) int {
	t.Helper()

	listener, err := net.Listen("tcp", address)
	if err != nil && errors.Is(err, syscall.EADDRNOTAVAIL) {
		t.Skipf("host has no address for %s", address)
	}
	require.NoError(t, err)

	server := grpc.NewServer()
	servicePb.RegisterEngineServer(server, fake)
	go server.Serve(listener) //nolint: errcheck
	t.Cleanup(server.Stop)

	return listener.Addr().(*net.TCPAddr).Port
}

func healthyPod(t *testing.T, ip string) *container.MockContainer {
	t.Helper()

	c := container.NewMockContainer(t)
	c.On("GetHealth").Return(container.HealthHealthy, nil)
	c.On("GetIpAddress").Return(ip, nil).Maybe()
	return c
}

func ingestObject() *storage.Object {
	return &storage.Object{Bucket: storage.IngestionsBucket, Name: "demo"}
}

func TestLazyEngineResolvesAgainAfterUnavailable(t *testing.T) {
	fake := &fakeZipline{ingestErrors: []error{status.Error(codes.Unavailable, "no route to host")}}
	port := serveZipline(t, "127.0.0.1:0", fake)
	t.Setenv("ZIPLINE_CONTAINER_NAME", "127.0.0.1")

	ce := container.NewMockEngine(t)
	ce.On("Get", mock.Anything, "127.0.0.1").Return(healthyPod(t, "127.0.0.1"), nil).Twice()

	lazy := NewLazyEngine(ce)
	lazy.port = port

	err := lazy.Ingest(context.Background(), &pb.Backtest{Name: "demo"}, ingestObject())
	require.Equal(t, codes.Unavailable, status.Code(err))

	// The failed engine is gone, so this call looks Zipline up again.
	require.NoError(t, lazy.Ingest(context.Background(), &pb.Backtest{Name: "demo"}, ingestObject()))
	// And the new one is cached.
	require.NoError(t, lazy.Ingest(context.Background(), &pb.Backtest{Name: "demo"}, ingestObject()))
	require.Equal(t, 3, fake.ingestCalls)
}

func TestLazyEngineResolvesAgainAfterConnectionRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	t.Setenv("ZIPLINE_CONTAINER_NAME", "127.0.0.1")

	ce := container.NewMockEngine(t)
	ce.On("Get", mock.Anything, "127.0.0.1").Return(healthyPod(t, "127.0.0.1"), nil).Twice()

	lazy := NewLazyEngine(ce)
	lazy.port = port

	err = lazy.Ingest(context.Background(), &pb.Backtest{Name: "demo"}, ingestObject())
	require.Equal(t, codes.Unavailable, status.Code(err))

	err = lazy.Ingest(context.Background(), &pb.Backtest{Name: "demo"}, ingestObject())
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestLazyEngineKeepsEngineWhenZiplineAnswers(t *testing.T) {
	fake := &fakeZipline{ingestErrors: []error{status.Error(codes.Internal, "bad bundle")}}
	port := serveZipline(t, "127.0.0.1:0", fake)
	t.Setenv("ZIPLINE_CONTAINER_NAME", "127.0.0.1")

	ce := container.NewMockEngine(t)
	ce.On("Get", mock.Anything, "127.0.0.1").Return(healthyPod(t, "127.0.0.1"), nil).Once()

	lazy := NewLazyEngine(ce)
	lazy.port = port

	err := lazy.Ingest(context.Background(), &pb.Backtest{Name: "demo"}, ingestObject())
	require.Equal(t, codes.Internal, status.Code(err))
	require.NoError(t, lazy.Ingest(context.Background(), &pb.Backtest{Name: "demo"}, ingestObject()))
}

func TestZiplineNewSessionFollowsTheCurrentPod(t *testing.T) {
	oldPod := &fakeZipline{sessionPort: 27001}
	port := serveZipline(t, "127.0.0.1:0", oldPod)
	newPod := &fakeZipline{sessionPort: 27002}
	serveZipline(t, net.JoinHostPort("127.0.0.2", strconv.Itoa(port)), newPod)

	ce := container.NewMockEngine(t)
	ce.On("Get", mock.Anything, "zipline").Return(healthyPod(t, "127.0.0.1"), nil).Once()
	ce.On("Get", mock.Anything, "zipline").Return(healthyPod(t, "127.0.0.2"), nil).Once()

	z, err := NewZiplineEngine(context.Background(), ZiplineTarget{
		ControlAddress:     "zipline:50055",
		Pods:               ce,
		Workload:           "zipline",
		SessionControlPort: port,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { z.Close() }) //nolint: errcheck

	first, err := z.NewSession(context.Background(), &pb.Session{Id: "first"})
	require.NoError(t, err)
	require.NotNil(t, first)

	// Zipline restarted on a new address between the two sessions.
	second, err := z.NewSession(context.Background(), &pb.Session{Id: "second"})
	require.NoError(t, err)
	require.NotNil(t, second)

	require.Equal(t, []string{"first"}, oldPod.sessions())
	require.Equal(t, []string{"second"}, newPod.sessions())
}

func TestZiplineNewSessionRefusesUnreadyPod(t *testing.T) {
	c := container.NewMockContainer(t)
	c.On("GetHealth").Return(container.HealthStarting, nil).Once()
	ce := container.NewMockEngine(t)
	ce.On("Get", mock.Anything, "zipline").Return(c, nil).Once()

	z, err := NewZiplineEngine(context.Background(), ZiplineTarget{
		ControlAddress:     "zipline:50055",
		Pods:               ce,
		Workload:           "zipline",
		SessionControlPort: ziplineControlPort,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { z.Close() }) //nolint: errcheck

	_, err = z.NewSession(context.Background(), &pb.Session{Id: "s"})
	require.ErrorContains(t, err, "zipline container is unhealthy")
}

func TestZiplineNewSessionRefusesPortOutOfRange(t *testing.T) {
	cases := []struct {
		name string
		port int64
	}{
		{name: "zero", port: 0},
		{name: "negative", port: -1},
		{name: "above 65535", port: 65536},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeZipline{sessionPort: tc.port}
			port := serveZipline(t, "127.0.0.1:0", fake)

			ce := container.NewMockEngine(t)
			ce.On("Get", mock.Anything, "zipline").Return(healthyPod(t, "127.0.0.1"), nil).Once()

			z, err := NewZiplineEngine(context.Background(), ZiplineTarget{
				ControlAddress:     "zipline:50055",
				Pods:               ce,
				Workload:           "zipline",
				SessionControlPort: port,
			}, nil)
			require.NoError(t, err)
			t.Cleanup(func() { z.Close() }) //nolint: errcheck

			_, err = z.NewSession(context.Background(), &pb.Session{Id: "s"})
			require.ErrorContains(t, err, "outside 1-65535")
		})
	}
}
