package command

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	missingDriverCopy = "Couldn't load data for backtest 'demo': the data service is missing its database driver " +
		"(psycopg). This is a bug in the Foreverbull install, not in your strategy or settings. Nothing was saved, " +
		"so run `fbull backtest create` again once it's fixed."
	secretPassword = "s3cr3t-Pa55"
)

func TestIngestFailureMessage(t *testing.T) {
	missingDriver := status.Error(codes.FailedPrecondition,
		"the data service is missing its database driver (psycopg)\nDetails: ModuleNotFoundError: No module named 'psycopg'")
	connection := status.Error(codes.Unavailable,
		"the data service could not connect to database at postgresql://foreverbull:***@db.internal:5432/foreverbull"+
			"\nDetails: OperationalError: connection refused")

	tests := []struct {
		name     string
		cause    error
		removed  bool
		expected string
	}{
		{
			name:    "missing driver, backtest removed: UX copy and the cause after the details marker",
			cause:   fmt.Errorf("error ingesting: %w", missingDriver),
			removed: true,
			expected: missingDriverCopy +
				"\nDetails: ModuleNotFoundError: No module named 'psycopg'",
		},
		{
			name:    "missing driver, delete failed: tells the user to delete first, not that nothing was saved",
			cause:   fmt.Errorf("error ingesting: %w", missingDriver),
			removed: false,
			expected: "Couldn't load data for backtest 'demo': the data service is missing its database driver " +
				"(psycopg). This is a bug in the Foreverbull install, not in your strategy or settings. The backtest " +
				"could not be removed, so run `fbull backtest delete demo` before you run `fbull backtest create` again." +
				"\nDetails: ModuleNotFoundError: No module named 'psycopg'",
		},
		{
			name:    "connection error is still reported as a connection error, without the install-bug sentence",
			cause:   fmt.Errorf("error ingesting: %w", connection),
			removed: true,
			expected: "Couldn't load data for backtest 'demo': the data service could not connect to database at " +
				"postgresql://foreverbull:***@db.internal:5432/foreverbull. Nothing was saved, so run " +
				"`fbull backtest create` again once it's fixed.\nDetails: OperationalError: connection refused",
		},
		{
			name:    "plain error without a gRPC status",
			cause:   errors.New("error getting zipline engine: pod not ready"),
			removed: true,
			expected: "Couldn't load data for backtest 'demo': error getting zipline engine: pod not ready. " +
				"Nothing was saved, so run `fbull backtest create` again once it's fixed.",
		},
		{
			name:    "empty status message falls back to a generic reason",
			cause:   status.Error(codes.Internal, ""),
			removed: true,
			expected: "Couldn't load data for backtest 'demo': the data service failed while ingesting prices. " +
				"Nothing was saved, so run `fbull backtest create` again once it's fixed.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, ingestFailureMessage("demo", readIngestFailure(tc.cause), tc.removed))
		})
	}
}

// An older data service image sends the whole URL. The broker must not copy
// the password into its log or the CLI message.
func TestIngestFailureMessageRedactsPasswords(t *testing.T) {
	cause := status.Error(codes.Unknown, "Exception calling application: Could not connect to database at "+
		"postgresql://foreverbull:"+secretPassword+"@postgres:5432/foreverbull\nDetails: OperationalError: "+
		"postgres://admin:"+secretPassword+"@10.0.0.1/x refused")

	message := ingestFailureMessage("demo", readIngestFailure(fmt.Errorf("error ingesting: %w", cause)), true)

	require.NotContains(t, message, secretPassword)
	require.Contains(t, message, "postgresql://foreverbull:***@postgres:5432/foreverbull")
	require.Contains(t, message, "postgres://admin:***@10.0.0.1/x")
}

func TestRedactURLPasswords(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"postgresql://u:p@h:5432/db", "postgresql://u:***@h:5432/db"},
		{"at postgresql+psycopg://u:p%40ss@h/db.", "at postgresql+psycopg://u:***@h/db."},
		{"no password postgresql://u@h/db", "no password postgresql://u@h/db"},
		{"no url here: a:b@c", "no url here: a:b@c"},
	}
	for _, tc := range tests {
		require.Equal(t, tc.expected, redactURLPasswords(tc.text))
	}
}
