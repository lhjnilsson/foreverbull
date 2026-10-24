package api

import (
	"context"
	"io"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
)

func TestCreateExperimentRequiresTriggerOnTheBacktest(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(ctx, pool))
	require.NoError(t, auth.Migrate(ctx, pool))

	owner := insertExperimentAccount(t, ctx, pool, auth.RoleAdmin, "owner")
	caller := insertExperimentAccount(t, ctx, pool, auth.RoleEditor, "caller")
	require.NoError(t, repository.MigrateStrategyOwner(ctx, pool, owner))

	var strategyID string
	err = pool.QueryRow(ctx, `
		INSERT INTO strategy (name, owner_id) VALUES ('mean-rev', $1) RETURNING id`, owner).Scan(&strategyID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `ALTER TABLE backtest ADD COLUMN IF NOT EXISTS id text`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `ALTER TABLE backtest ADD COLUMN IF NOT EXISTS owner_id text`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO backtest (name, benchmark, id, owner_id) VALUES ('spy', 'SPY', 'bt-spy', $1)`, owner)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO domain_grant (domain_kind, domain_id, grantee_id, level)
		VALUES ('strategy', $1, $2, 'editor'), ('backtest', 'bt-spy', $2, 'viewer')`, strategyID, caller)
	require.NoError(t, err)

	server := &StrategyServer{pgx: pool}
	call := experimentStream{ctx: auth.WithAccount(ctx, auth.Account{ID: caller, Role: auth.RoleEditor})}
	req := &servicePb.CreateExperimentRequest{StrategyName: "mean-rev", BacktestName: "spy"}
	err = server.createExperimentAuthed(req, call)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = pool.Exec(ctx, `
		UPDATE domain_grant SET level = 'editor'
		WHERE domain_kind = 'backtest' AND domain_id = 'bt-spy' AND grantee_id = $1`, caller)
	require.NoError(t, err)
	err = server.createExperimentAuthed(req, call)
	require.NotEqual(t, codes.PermissionDenied, status.Code(err))
}

func insertExperimentAccount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, role, username string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx, `INSERT INTO account (role) VALUES ($1) RETURNING id`, role).Scan(&id)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO account_password (account_id, username, password_hash)
		VALUES ($1, $2, 'hash')`, id, username)
	require.NoError(t, err)
	return id
}

type experimentStream struct {
	ctx context.Context
}

func (s experimentStream) Context() context.Context { return s.ctx }
func (s experimentStream) Send(*servicePb.CreateExperimentResponse) error {
	return nil
}
func (s experimentStream) SetHeader(metadata.MD) error  { return nil }
func (s experimentStream) SendHeader(metadata.MD) error { return nil }
func (s experimentStream) SetTrailer(metadata.MD)       {}
func (s experimentStream) SendMsg(any) error            { return nil }
func (s experimentStream) RecvMsg(any) error            { return io.EOF }

var _ grpc.ServerStreamingServer[servicePb.CreateExperimentResponse] = experimentStream{}
