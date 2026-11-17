package api_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// memoryResults is the stored Zipline result. It has no address and no credential.
type memoryResults struct {
	bodies map[string][]byte
	err    error
	opened []string
}

func (m *memoryResults) ReadResult(_ context.Context, id string) (io.ReadCloser, int64, error) {
	m.opened = append(m.opened, id)
	if m.err != nil {
		return nil, 0, m.err
	}
	body, ok := m.bodies[id]
	if !ok {
		return nil, 0, storage.ErrResultMissing
	}
	return io.NopCloser(bytes.NewReader(body)), int64(len(body)), nil
}

func (test *BacktestServerTest) TestGetExecutionResult() {
	test.Run("streams the stored bytes", func() {
		backtestRow := test.createBacktest("result-backtest")
		session := test.createSession(backtestRow.Name)
		execution := test.createExecution(session.Id)
		want := []byte("portfolio_value,pnl\n100,1\n")
		test.results.bodies = map[string][]byte{execution.Id: want}

		got, err := readResult(test.client, execution.Id)
		test.Require().NoError(err)
		test.Equal(want, got)
		test.NotContains(string(got), "http://")
		test.NotContains(string(got), "STORAGE_")
		test.NotContains(string(got), "minio")
		test.Equal([]string{execution.Id}, test.results.opened)
	})

	test.Run("missing execution does not open storage", func() {
		_, err := readResult(test.client, uuid.NewString())
		requireStatus(test.T(), err, codes.NotFound)
		test.Empty(test.results.opened)
	})

	test.Run("missing object", func() {
		backtestRow := test.createBacktest("result-backtest")
		session := test.createSession(backtestRow.Name)
		execution := test.createExecution(session.Id)

		_, err := readResult(test.client, execution.Id)
		requireStatus(test.T(), err, codes.FailedPrecondition)
		test.Equal(usercopy.ExecutionResultMissing(), status.Convert(err).Message())
		test.NotContains(status.Convert(err).Message(), "minio")
		test.NotContains(status.Convert(err).Message(), "STORAGE_")
	})

	test.Run("too large", func() {
		backtestRow := test.createBacktest("result-backtest")
		session := test.createSession(backtestRow.Name)
		execution := test.createExecution(session.Id)
		test.results.err = storage.ErrResultTooLarge

		_, err := readResult(test.client, execution.Id)
		requireStatus(test.T(), err, codes.ResourceExhausted)
		test.Equal(usercopy.ExecutionResultTooLarge(), status.Convert(err).Message())
	})

	test.Run("rejects a path", func() {
		_, err := readResult(test.client, "../secrets")
		requireStatus(test.T(), err, codes.InvalidArgument)
		test.Equal(usercopy.ExecutionIDInvalid(), status.Convert(err).Message())
		test.Empty(test.results.opened)
	})

	test.Run("storage fault is not a credential", func() {
		backtestRow := test.createBacktest("result-backtest")
		session := test.createSession(backtestRow.Name)
		execution := test.createExecution(session.Id)
		test.results.err = fmt.Errorf("Get \"http://minio:9000/results/%s\": access_key=minioadmin secret_key=minioadmin", execution.Id)

		_, err := readResult(test.client, execution.Id)
		requireStatus(test.T(), err, codes.Internal)
		message := status.Convert(err).Message()
		test.NotContains(message, "minio")
		test.NotContains(message, "minioadmin")
		test.NotContains(message, "access_key")
		test.NotContains(message, "secret_key")
		test.NotContains(message, "http://")
		test.NotContains(message, "STORAGE_")
	})
}

func readResult(client servicePb.BacktestServicerClient, id string) ([]byte, error) {
	stream, err := client.GetExecutionResult(context.Background(), &servicePb.GetExecutionResultRequest{ExecutionId: id})
	if err != nil {
		return nil, err
	}
	var got []byte
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return got, nil
		}
		if err != nil {
			return nil, err
		}
		got = append(got, msg.GetData()...)
	}
}

func TestGetExecutionResultRequiresTheOwner(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(ctx, pool))
	require.NoError(t, auth.Migrate(ctx, pool))

	owner := insertAccount(t, pool, auth.RoleEditor, "owner")
	stranger := insertAccount(t, pool, auth.RoleEditor, "stranger")
	admin := insertAccount(t, pool, auth.RoleAdmin, "admin")
	require.NoError(t, repository.MigrateBacktestOwner(ctx, pool, admin.ID, quietCopier{}))

	backtests := repository.Backtest{Conn: pool}
	backtestID := createOwned(t, backtests, owner.ID, "spy")
	sessions := repository.Session{Conn: pool}
	session, err := sessions.CreateForBacktest(ctx, backtestID)
	require.NoError(t, err)
	executions := repository.Execution{Conn: pool}
	created, err := executions.Create(ctx, &backtest.Execution{
		Session:   session.GetId(),
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 2},
		EndDate:   &pb.Date{Year: 2024, Month: 1, Day: 3},
	}, nil)
	require.NoError(t, err)

	want := []byte("result-bytes")
	stored := &memoryResults{bodies: map[string][]byte{created.GetId(): want}}
	server := api.NewBacktestServer(pool, new(stream.MockStream), stored)

	got, err := collectResult(server, auth.WithAccount(ctx, owner), created.GetId())
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, []string{created.GetId()}, stored.opened)

	stored.opened = nil
	_, err = collectResult(server, auth.WithAccount(ctx, stranger), created.GetId())
	requireStatus(t, err, codes.NotFound)
	require.Equal(t, "not found", status.Convert(err).Message())
	require.Empty(t, stored.opened)

	_, err = collectResult(server, context.Background(), created.GetId())
	requireStatus(t, err, codes.Unauthenticated)
	require.Empty(t, stored.opened)
}

func collectResult(server *api.BacktestServer, ctx context.Context, id string) ([]byte, error) {
	capture := &resultCapture{ctx: ctx}
	err := server.GetExecutionResult(&servicePb.GetExecutionResultRequest{ExecutionId: id}, capture)
	if err != nil {
		return nil, err
	}
	return bytes.Join(capture.chunks, nil), nil
}

type resultCapture struct {
	ctx    context.Context
	chunks [][]byte
}

func (s *resultCapture) Context() context.Context { return s.ctx }

func (s *resultCapture) Send(msg *servicePb.GetExecutionResultResponse) error {
	s.chunks = append(s.chunks, append([]byte(nil), msg.GetData()...))
	return nil
}

func (s *resultCapture) SetHeader(metadata.MD) error  { return nil }
func (s *resultCapture) SendHeader(metadata.MD) error { return nil }
func (s *resultCapture) SetTrailer(metadata.MD)       {}
func (s *resultCapture) SendMsg(any) error            { return nil }
func (s *resultCapture) RecvMsg(any) error            { return io.EOF }
