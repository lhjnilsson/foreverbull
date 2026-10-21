package main

import (
	"context"
	"net"
	"os"
	"testing"

	backtestsvc "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

type listCapture struct {
	backtestsvc.UnimplementedBacktestServicerServer
	md metadata.MD
}

func (s *listCapture) ListBacktests(ctx context.Context, _ *backtestsvc.ListBacktestsRequest) (*backtestsvc.ListBacktestsResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.md = md
	return &backtestsvc.ListBacktestsResponse{}, nil
}

func dialList(t *testing.T) metadata.MD {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	capture := &listCapture{}
	backtestsvc.RegisterBacktestServicerServer(srv, capture)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = listener.Close()
	})

	conn, err := dialBroker("passthrough:///"+listener.Addr().String(),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := backtestsvc.NewBacktestServicerClient(conn)
	if _, err := client.ListBacktests(context.Background(), &backtestsvc.ListBacktestsRequest{}); err != nil {
		t.Fatalf("list: %v", err)
	}
	return capture.md
}

func TestDialBrokerUsesTrimmedToken(t *testing.T) {
	t.Setenv("GRAFANA_BROKER_TOKEN", " fb_editor ")
	got := dialList(t).Get("authorization")
	if len(got) != 1 || got[0] != "Bearer fb_editor" {
		t.Fatalf("authorization = %#v", got)
	}
}

func TestDialBrokerOmitsBearerWhenTokenBlank(t *testing.T) {
	t.Setenv("GRAFANA_BROKER_TOKEN", "   ")
	if got := dialList(t).Get("authorization"); len(got) != 0 {
		t.Fatalf("authorization = %#v", got)
	}
}

func TestDialBrokerOmitsBearerWhenTokenUnset(t *testing.T) {
	t.Setenv("GRAFANA_BROKER_TOKEN", "fb_editor")
	os.Unsetenv("GRAFANA_BROKER_TOKEN")
	if got := dialList(t).Get("authorization"); len(got) != 0 {
		t.Fatalf("authorization = %#v", got)
	}
}

func TestBearerTokenDoesNotRequireTLS(t *testing.T) {
	var creds credentials.PerRPCCredentials = bearerToken("fb_editor")
	if creds.RequireTransportSecurity() {
		t.Fatal("bearer must dial the plaintext broker")
	}
	md, err := creds.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if md["authorization"] != "Bearer fb_editor" {
		t.Fatalf("authorization = %q", md["authorization"])
	}
}
