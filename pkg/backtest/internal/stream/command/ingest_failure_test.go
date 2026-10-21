package command

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	missingDriverCopy = "Couldn't load data for backtest 'demo': the data service is missing its database driver " +
		"(psycopg). This is a bug in the Foreverbull install, not in your strategy or settings. Nothing was saved, " +
		"so run `fbull backtest create` again once it's fixed."
	missingDriverNoNameCopy = "Couldn't load data for backtest 'demo': the data service is missing its database " +
		"driver. This is a bug in the Foreverbull install, not in your strategy or settings. Nothing was saved, " +
		"so run `fbull backtest create` again once it's fixed."
	unreachableCopy = "Couldn't load data for backtest 'demo': Foreverbull's data service isn't reachable. Run " +
		"`fbenv status` to check it's running. Nothing was saved, so run `fbull backtest create` again after that."
	otherFailureCopy = "Couldn't load data for backtest 'demo': the data service failed while ingesting prices. " +
		"Nothing was saved, so run `fbull backtest create` again once it's fixed."
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
			name:     "missing driver, backtest removed: UX copy and the cause after the details marker",
			cause:    fmt.Errorf("error ingesting: %w", missingDriver),
			removed:  true,
			expected: missingDriverCopy + "\nDetails: ModuleNotFoundError: No module named 'psycopg'",
		},
		{
			name:    "missing driver, delete failed: tells the user to delete first, not that nothing was saved",
			cause:   fmt.Errorf("error ingesting: %w", missingDriver),
			removed: false,
			expected: "Couldn't load data for backtest 'demo': the data service is missing its database driver " +
				"(psycopg). This is a bug in the Foreverbull install, not in your strategy or settings. The backtest " +
				"couldn't be removed, so run `fbull backtest delete demo` before you run `fbull backtest create` again." +
				"\nDetails: ModuleNotFoundError: No module named 'psycopg'",
		},
		{
			name:     "missing driver with an unexpected reason: the reason text is not echoed",
			cause:    status.Error(codes.FailedPrecondition, "Traceback: boom\nDetails: ImportError: boom"),
			removed:  true,
			expected: missingDriverNoNameCopy + "\nDetails: ImportError: boom",
		},
		{
			name:    "database unreachable: fixed UX copy, the Zipline reason is not echoed",
			cause:   fmt.Errorf("error ingesting: %w", connection),
			removed: true,
			expected: unreachableCopy + "\nDetails: the Zipline pod couldn't reach Postgres: the data service could not connect to database at " +
				"postgresql://foreverbull:***@db.internal:5432/foreverbull: OperationalError: connection refused",
		},
		{
			name:    "database unreachable, delete failed",
			cause:   fmt.Errorf("error ingesting: %w", connection),
			removed: false,
			expected: "Couldn't load data for backtest 'demo': Foreverbull's data service isn't reachable. Run " +
				"`fbenv status` to check it's running. The backtest couldn't be removed, so run " +
				"`fbull backtest delete demo` before you run `fbull backtest create` again." +
				"\nDetails: the Zipline pod couldn't reach Postgres: the data service could not connect to database at " +
				"postgresql://foreverbull:***@db.internal:5432/foreverbull: OperationalError: connection refused",
		},
		{
			name: "broker can't reach the Zipline pod: same line, details name that hop",
			cause: fmt.Errorf("error ingesting: %w", status.Error(codes.Unavailable,
				"connection error: desc = \"transport: Error while dialing: dial tcp 10.96.0.12:50055: connect: connection refused\"")),
			removed: true,
			expected: unreachableCopy + "\nDetails: the broker couldn't reach the Zipline pod: connection error: desc = " +
				"\"transport: Error while dialing: dial tcp 10.96.0.12:50055: connect: connection refused\"",
		},
		{
			name:     "other data service failure: fixed reason, the exception only in details",
			cause:    status.Error(codes.Internal, "the data service failed\nDetails: KeyError: 'AAPL'"),
			removed:  true,
			expected: otherFailureCopy + "\nDetails: KeyError: 'AAPL'",
		},
		{
			name:     "plain error without a gRPC status: its whole text goes to details",
			cause:    errors.New("error getting zipline engine: pod not ready"),
			removed:  true,
			expected: otherFailureCopy + "\nDetails: error getting zipline engine: pod not ready",
		},
		{
			name:     "older data service: raw exception text is never the reason",
			cause:    status.Error(codes.Unknown, "Exception calling application: KeyError: 'AAPL'"),
			removed:  true,
			expected: otherFailureCopy + "\nDetails: Exception calling application: KeyError: 'AAPL'",
		},
		{
			name:     "empty status message: no details line",
			cause:    status.Error(codes.Internal, ""),
			removed:  true,
			expected: otherFailureCopy,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			message := ingestFailureMessage("demo", readIngestFailure(tc.cause), tc.removed)
			require.Equal(t, tc.expected, message)
			// The CLI shows only the part before the details marker by default.
			shown, _, _ := strings.Cut(message, ingestDetailsMarker)
			require.NotContains(t, shown, "Error")
			require.NotContains(t, shown, "Exception")
		})
	}
}

// An older data service image sends the whole URL. The broker must not copy
// the password into its log or the CLI message.
func TestIngestFailureMessageRedactsPasswords(t *testing.T) {
	cause := status.Error(codes.Unknown, "Exception calling application: Could not connect to database at "+
		"postgresql://foreverbull:"+secretPassword+"@postgres:5432/foreverbull (OperationalError: "+
		"postgres://admin:"+secretPassword+"@10.0.0.1/x refused)")

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
