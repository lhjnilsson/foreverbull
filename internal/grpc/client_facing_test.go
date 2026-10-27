package grpc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClientFacingError(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, clientFacingError(nil))
	})

	t.Run("plain error becomes internal without the cause", func(t *testing.T) {
		t.Parallel()
		err := clientFacingError(fmt.Errorf(
			"error getting backtest: connect postgresql://foreverbull:s3cret@postgres:5432/foreverbull",
		))
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.Internal, st.Code())
		require.Equal(t, clientInternal, st.Message())
		require.NotContains(t, st.Message(), "s3cret")
		require.NotContains(t, st.Message(), "postgres")
		require.NotContains(t, err.Error(), "s3cret")
	})

	t.Run("status message redacts URL passwords", func(t *testing.T) {
		t.Parallel()
		raw := status.Error(codes.FailedPrecondition,
			`could not connect to postgresql://foreverbull:s3cret@postgres:5432/foreverbull`)
		err := clientFacingError(raw)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.FailedPrecondition, st.Code())
		require.NotContains(t, st.Message(), "s3cret")
		require.Contains(t, st.Message(), "***@")
	})

	t.Run("password equals still leaks until replaced", func(t *testing.T) {
		t.Parallel()
		raw := status.Error(codes.Internal, "postgres password=s3cret host=db")
		err := clientFacingError(raw)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.Internal, st.Code())
		require.Equal(t, clientInternalDatabase, st.Message())
		require.NotContains(t, st.Message(), "s3cret")
		require.NotContains(t, err.Error(), "s3cret")
	})

	t.Run("unknown backtest NotFound keeps code and message", func(t *testing.T) {
		t.Parallel()
		raw := status.Errorf(codes.NotFound, "backtest %s not found", "demo")
		err := clientFacingError(raw)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.NotFound, st.Code())
		require.Equal(t, "backtest demo not found", st.Message())
	})

	t.Run("NotFound with a secret-shaped message is scrubbed", func(t *testing.T) {
		t.Parallel()
		raw := status.Error(codes.NotFound, "backtest missing; password=s3cret")
		err := clientFacingError(raw)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.NotFound, st.Code())
		require.Equal(t, "not found", st.Message())
		require.NotContains(t, err.Error(), "s3cret")
	})

	t.Run("safe status passes through", func(t *testing.T) {
		t.Parallel()
		raw := status.Error(codes.FailedPrecondition, "live mode is not enabled; deploy with paper")
		err := clientFacingError(raw)
		require.Equal(t, raw, err)
	})
}

func TestClientFacingUnaryInterceptor(t *testing.T) {
	t.Parallel()

	handler := func(context.Context, interface{}) (interface{}, error) {
		return nil, errors.New("dial tcp: connection refused; password=s3cret")
	}
	resp, err := clientFacingUnary(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	require.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Internal, st.Code())
	require.Equal(t, clientInternal, st.Message())
	require.NotContains(t, err.Error(), "s3cret")
}

func TestClientFacingAfterPgx(t *testing.T) {
	t.Parallel()

	// Innermost pgx maps the PgError; clientFacing must keep the mapped status.
	inner := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return pgxErrorInterceptor(ctx, req, info, handler)
	}
	chain := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return clientFacingUnary(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
			return inner(ctx, req, info, handler)
		})
	}
	handler := func(context.Context, interface{}) (interface{}, error) {
		return nil, fmt.Errorf("wrap: %w", errors.New("not a pg error; password=s3cret"))
	}
	_, err := chain(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Internal, st.Code())
	require.NotContains(t, err.Error(), "s3cret")
}
