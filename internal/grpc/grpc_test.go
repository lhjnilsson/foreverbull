package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPgxErrorToStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code     string
		wantCode codes.Code
		wantMsg  string
	}{
		{"23505", codes.AlreadyExists, "Resource already exists"},
		{"23503", codes.FailedPrecondition, "Referenced resource not found"},
		{"23502", codes.InvalidArgument, "Required field missing"},
		{"22P02", codes.InvalidArgument, "Invalid input format"},
		{"42P01", codes.NotFound, "Resource not found"},
		{"42703", codes.Internal, "Invalid field reference"},
		{"53300", codes.ResourceExhausted, "Database connection limit reached"},
		{"57014", codes.DeadlineExceeded, "Query timeout"},
		{"XXXXX", codes.Internal, "Internal database error"},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()

			err := pgxErrorToStatus(&pgconn.PgError{Code: tc.code})
			st, ok := status.FromError(err)
			require.True(t, ok)
			require.Equal(t, tc.wantCode, st.Code())
			require.Equal(t, tc.wantMsg, st.Message())
		})
	}

	t.Run("unary interceptor maps pg error", func(t *testing.T) {
		t.Parallel()

		handler := func(context.Context, interface{}) (interface{}, error) {
			return "ignored", &pgconn.PgError{Code: "23505"}
		}
		resp, err := pgxErrorInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
		require.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.AlreadyExists, st.Code())
	})

	t.Run("unary interceptor passes through other errors", func(t *testing.T) {
		t.Parallel()

		want := errors.New("not postgres")
		handler := func(context.Context, interface{}) (interface{}, error) {
			return "ok", want
		}
		resp, err := pgxErrorInterceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
		require.Equal(t, "ok", resp)
		require.ErrorIs(t, err, want)
	})
}
