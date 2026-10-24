package api_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

type quietCopier struct{}

func (quietCopier) Readable(context.Context, string) (bool, error) { return false, nil }
func (quietCopier) Copy(context.Context, string, string) error {
	return fmt.Errorf("copy should not run")
}

type sessionCapture struct {
	ctx    context.Context
	frames []*servicePb.CreateSessionResponse
}

func (s *sessionCapture) Context() context.Context { return s.ctx }
func (s *sessionCapture) Send(msg *servicePb.CreateSessionResponse) error {
	cloned, ok := proto.Clone(msg).(*servicePb.CreateSessionResponse)
	if !ok {
		return fmt.Errorf("clone session response")
	}
	s.frames = append(s.frames, cloned)
	return nil
}
func (s *sessionCapture) SetHeader(metadata.MD) error  { return nil }
func (s *sessionCapture) SendHeader(metadata.MD) error { return nil }
func (s *sessionCapture) SetTrailer(metadata.MD)       {}
func (s *sessionCapture) SendMsg(any) error            { return nil }
func (s *sessionCapture) RecvMsg(any) error            { return io.EOF }

func TestGrantsAuthorizeBacktest(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(ctx, pool))
	require.NoError(t, auth.Migrate(ctx, pool))

	owner := insertAccount(t, pool, auth.RoleEditor, "owner")
	viewer := insertAccount(t, pool, auth.RoleViewer, "viewer")
	editor := insertAccount(t, pool, auth.RoleEditor, "editor")
	admin := insertAccount(t, pool, auth.RoleAdmin, "admin")
	capped := insertAccount(t, pool, auth.RoleEditor, "capped")
	pat := insertAccount(t, pool, auth.RoleViewer, "pat")
	require.NotEmpty(t, pat.ID)
	require.NoError(t, repository.MigrateBacktestOwner(ctx, pool, admin.ID, quietCopier{}))

	backtests := repository.Backtest{Conn: pool}
	shared := createOwned(t, backtests, owner.ID, "shared")
	createOwned(t, backtests, owner.ID, "doomed")
	createOwned(t, backtests, owner.ID, "foreign")
	createOwned(t, backtests, capped.ID, "mine")
	grant(t, pool, shared, viewer.ID, auth.LevelViewer)
	grant(t, pool, shared, editor.ID, auth.LevelEditor)
	grant(t, pool, shared, capped.ID, auth.LevelViewer)

	published := new(stream.MockStream)
	published.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	server := api.NewBacktestServer(pool, published)

	mux := http.NewServeMux()
	auth.RegisterHTTP(mux, pool)
	httpHandler := auth.Protect(pool)(mux)

	t.Run("viewer", func(t *testing.T) {
		ctx := auth.WithAccount(context.Background(), viewer)
		got, err := server.GetBacktest(ctx, &servicePb.GetBacktestRequest{Name: "shared"})
		require.NoError(t, err)
		require.Equal(t, "shared", got.GetBacktest().GetName())

		frames, err := runSession(server, ctx, "shared")
		requireStatus(t, err, codes.PermissionDenied)
		requireNoSecret(t, frames)
		_, err = server.DeleteBacktest(ctx, &servicePb.DeleteBacktestRequest{Name: "shared"})
		requireStatus(t, err, codes.PermissionDenied)
	})

	t.Run("editor grant", func(t *testing.T) {
		ctx := auth.WithAccount(context.Background(), editor)
		frames, err := runSession(server, ctx, "shared")
		require.NoError(t, err)
		require.NotEmpty(t, frames)
		requireNoSecret(t, frames)
		_, err = server.DeleteBacktest(ctx, &servicePb.DeleteBacktestRequest{Name: "shared"})
		requireStatus(t, err, codes.PermissionDenied)

		rec := putGrant(httpHandler, editorCookie(t, pool, editor.ID), "shared", "pat", auth.LevelEditor)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("owner deletes", func(t *testing.T) {
		ctx := auth.WithAccount(context.Background(), owner)
		_, err := server.DeleteBacktest(ctx, &servicePb.DeleteBacktestRequest{Name: "doomed"})
		require.NoError(t, err)
	})

	t.Run("admin deletes and grants", func(t *testing.T) {
		ctx := auth.WithAccount(context.Background(), admin)
		_, err := server.DeleteBacktest(ctx, &servicePb.DeleteBacktestRequest{Name: "foreign"})
		require.NoError(t, err)
		rec := putGrant(httpHandler, editorCookie(t, pool, admin.ID), "shared", "pat", auth.LevelViewer)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("global editor with a viewer grant", func(t *testing.T) {
		ctx := auth.WithAccount(context.Background(), capped)
		_, err := server.GetBacktest(ctx, &servicePb.GetBacktestRequest{Name: "shared"})
		require.NoError(t, err)
		_, err = runSession(server, ctx, "shared")
		requireStatus(t, err, codes.PermissionDenied)
		_, err = server.DeleteBacktest(ctx, &servicePb.DeleteBacktestRequest{Name: "shared"})
		requireStatus(t, err, codes.PermissionDenied)

		got, err := server.GetBacktest(ctx, &servicePb.GetBacktestRequest{Name: "mine"})
		require.NoError(t, err)
		require.Equal(t, "mine", got.GetBacktest().GetName())
		_, err = runSession(server, ctx, "mine")
		require.NoError(t, err)
		_, err = server.DeleteBacktest(ctx, &servicePb.DeleteBacktestRequest{Name: "mine"})
		require.NoError(t, err)
	})
}

func TestSessionListenerRejectsMissingSecret(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	eng := new(engine.MockEngine)
	eng.On("NewSession", mock.Anything, mock.Anything).Return(new(engine.MockEngineSession), nil)
	session := &backtest.Session{Id: "session-1"}

	_, _, err := api.NewSecuredGRPCSessionServer(session, nil, eng, "")
	require.ErrorContains(t, err, auth.SessionMetadata())

	srv, activity, err := api.NewSecuredGRPCSessionServer(session, nil, eng, "listener-secret")
	require.NoError(t, err)
	listener := bufconn.Listen(1024 * 1024)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient("passthrough:///"+listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	client := servicePb.NewSessionServicerClient(conn)
	_, err = client.StopServer(context.Background(), &servicePb.StopServerRequest{})
	requireStatus(t, err, codes.Unauthenticated)
	select {
	case <-activity:
		t.Fatal("session handler ran without a secret")
	default:
	}
}

func insertAccount(t *testing.T, pool *pgxpool.Pool, role, username string) auth.Account {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO account (role) VALUES ($1) RETURNING id`, role).Scan(&id)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO account_password (account_id, username, password_hash)
		VALUES ($1, $2, 'x')`, id, username)
	require.NoError(t, err)
	return auth.Account{ID: id, Role: role}
}

func createOwned(t *testing.T, backtests repository.Backtest, ownerID, name string) string {
	t.Helper()
	item, err := backtests.CreateOwned(context.Background(), ownerID, &backtest.Backtest{Name: name})
	require.NoError(t, err)
	require.Equal(t, name, item.GetName())
	var id string
	err = backtests.Conn.QueryRow(context.Background(), `
		SELECT id FROM backtest WHERE owner_id = $1 AND name = $2`, ownerID, name).Scan(&id)
	require.NoError(t, err)
	return id
}

func grant(t *testing.T, pool *pgxpool.Pool, domainID, granteeID, level string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO domain_grant (domain_kind, domain_id, grantee_id, level)
		VALUES ('backtest', $1, $2, $3)`, domainID, granteeID, level)
	require.NoError(t, err)
}

func runSession(server *api.BacktestServer, ctx context.Context, name string) ([]*servicePb.CreateSessionResponse, error) {
	capture := &sessionCapture{ctx: ctx}
	err := server.CreateSession(&servicePb.CreateSessionRequest{BacktestName: name}, capture)
	return capture.frames, err
}

func requireNoSecret(t *testing.T, frames []*servicePb.CreateSessionResponse) {
	t.Helper()
	for _, frame := range frames {
		require.Empty(t, frame.GetSessionSecret())
	}
}

func requireStatus(t *testing.T, err error, code codes.Code) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, code, st.Code())
}

func editorCookie(t *testing.T, pool *pgxpool.Pool, accountID string) string {
	t.Helper()
	value, err := auth.OpenSession(context.Background(), pool, accountID, time.Now())
	require.NoError(t, err)
	return value
}

func putGrant(handler http.Handler, cookie, name, login, level string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"login":%q,"level":%q}`, login, level)
	req := httptest.NewRequest(http.MethodPut, "/api/backtests/"+name+"/grants", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "fb_session="+cookie)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
