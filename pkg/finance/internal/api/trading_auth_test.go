package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func TestTradingAccountRequiresAdminWhenAuthIsOn(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")

	err := requireTradingAccount(auth.WithAccount(context.Background(), auth.Account{ID: "editor", Role: auth.RoleEditor}))
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	err = requireTradingAccount(auth.WithAccount(context.Background(), auth.Account{ID: "viewer", Role: auth.RoleViewer}))
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	require.NoError(t, requireTradingAccount(auth.WithAccount(context.Background(), auth.Account{ID: "admin", Role: auth.RoleAdmin})))

	t.Setenv(environment.AuthEnabled, "false")
	require.NoError(t, requireTradingAccount(context.Background()))
}
