package command

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/pb"
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
				"was partly saved and couldn't be removed. Run `fbull backtest delete demo` first. Run " +
				"`fbull backtest create` again once it's fixed." +
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
				"`fbenv status` to check it's running. The backtest was partly saved and couldn't be " +
				"removed. Run `fbull backtest delete demo` first. Run `fbull backtest create` again after that." +
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

// missingSessionsMessage with every day inside the spans, removed and not.
func TestMissingSessionsMessageRemovedOrNot(t *testing.T) {
	t.Parallel()
	days := []time.Time{time.Date(2024, 2, 14, 0, 0, 0, 0, time.UTC)}
	gaps := []symbolGaps{{symbol: "XYZ", sessions: []int{0}}}
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices for XYZ on 2024-02-14, "+
		"a trading day. The backtest wasn't saved. Try another provider with --provider, or change the backtest "+
		"dates, then run `fbull backtest create` again.",
		missingSessionsMessage("oef-2024q1", "yahoo", days, gaps, true))
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices for XYZ on 2024-02-14, "+
		"a trading day. The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` "+
		"first. Try another provider with --provider, or change the backtest dates, then run "+
		"`fbull backtest create` again.",
		missingSessionsMessage("oef-2024q1", "yahoo", days, gaps, false))
}

// Both download-step messages in backtest.go, removed and not.
func TestSymbolDownloadMessage(t *testing.T) {
	t.Parallel()
	start := &pb.Date{Year: 2024, Month: 1, Day: 1}
	end := &pb.Date{Year: 2024, Month: 3, Day: 31}
	for _, tc := range []struct {
		errText, removed, kept string
	}{
		{
			errText: "no price data for XYZ",
			removed: "Couldn't load data for backtest 'oef-2024q1': yahoo has no prices for XYZ between 2024-01-01 and " +
				"2024-03-31. The backtest wasn't saved. Check the symbol and dates, or try another provider with " +
				"--provider, then run `fbull backtest create` again.",
			kept: "Couldn't load data for backtest 'oef-2024q1': yahoo has no prices for XYZ between 2024-01-01 and " +
				"2024-03-31. The backtest was partly saved and couldn't be removed. Run `fbull backtest delete " +
				"oef-2024q1` first. Check the symbol and dates, or try another provider with --provider, then " +
				"run `fbull backtest create` again.",
		},
		{
			errText: "no ISIN for XYZ",
			removed: "Couldn't load data for backtest 'oef-2024q1': yahoo doesn't identify XYZ well enough to follow " +
				"ticker changes, so it can't be used. The backtest wasn't saved. Remove XYZ, then run " +
				"`fbull backtest create` again.",
			kept: "Couldn't load data for backtest 'oef-2024q1': yahoo doesn't identify XYZ well enough to follow " +
				"ticker changes, so it can't be used. The backtest was partly saved and couldn't be removed. Run " +
				"`fbull backtest delete oef-2024q1` first. Remove XYZ, then run `fbull backtest create` again.",
		},
		{
			errText: "yahoo sent an invalid price for XYZ on 2024-02-14 (0, negative or not a number)",
			removed: "Couldn't load data for backtest 'oef-2024q1': yahoo sent an invalid price for XYZ on 2024-02-14 " +
				"(0, negative or not a number). The backtest wasn't saved. Try another provider with --provider, " +
				"or report it at https://github.com/lhjnilsson/foreverbull/issues.",
			kept: "Couldn't load data for backtest 'oef-2024q1': yahoo sent an invalid price for XYZ on 2024-02-14 " +
				"(0, negative or not a number). The backtest was partly saved and couldn't be removed. Run " +
				"`fbull backtest delete oef-2024q1` first. Try another provider with --provider, or report it " +
				"at https://github.com/lhjnilsson/foreverbull/issues.",
		},
		{
			errText: "yahoo has more than one instrument for XYZ",
			removed: "Couldn't load data for backtest 'oef-2024q1': yahoo has more than one instrument for XYZ. The " +
				"backtest wasn't saved. Use the exact ticker with its exchange suffix (for example VOD.L), or use " +
				"another provider with --provider.",
			kept: "Couldn't load data for backtest 'oef-2024q1': yahoo has more than one instrument for XYZ. The " +
				"backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. " +
				"Use the exact ticker with its exchange suffix (for example VOD.L), or use another provider " +
				"with --provider.",
		},
		{
			errText: "connection reset by peer",
			removed: "Couldn't load data for backtest 'oef-2024q1': the price download failed for XYZ. The backtest " +
				"wasn't saved. Try again in a few minutes, or use another provider with --provider.",
			kept: "Couldn't load data for backtest 'oef-2024q1': the price download failed for XYZ. The backtest was " +
				"partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. Try again " +
				"in a few minutes, or use another provider with --provider.",
		},
	} {
		require.Equal(t, tc.removed, symbolDownloadMessage("oef-2024q1", "yahoo", "XYZ", start, end, tc.errText, true))
		require.Equal(t, tc.kept, symbolDownloadMessage("oef-2024q1", "yahoo", "XYZ", start, end, tc.errText, false))
	}

	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': symbol 2 of 3 isn't a valid ticker. Use 1 to 32 "+
		"letters, digits, or . ^ = _ -. The backtest wasn't saved. Fix that symbol and run `fbull backtest create` again.",
		invalidTickerMessage("oef-2024q1", 2, 3, true))
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': symbol 2 of 3 isn't a valid ticker. Use 1 to 32 "+
		"letters, digits, or . ^ = _ -. The backtest was partly saved and couldn't be removed. Run `fbull backtest "+
		"delete oef-2024q1` first. Fix that symbol and run `fbull backtest create` again.",
		invalidTickerMessage("oef-2024q1", 2, 3, false))
}

const (
	noPricesCopy = "Couldn't load data for backtest 'oef-2024q1': there are no stored prices for META (earlier FB) " +
		"between 2024-01-02 and 2024-03-28. The backtest wasn't saved. " +
		"Try another provider with --provider, then run `fbull backtest create` again."
	noPricesBlockCopy = "Couldn't load data for backtest 'oef-2024q1': 2 symbols have no stored prices. " +
		"The backtest wasn't saved.\n" +
		"  META (earlier FB): no prices between 2024-01-02 and 2024-03-28.\n" +
		"  OEF: no prices between 2024-01-02 and 2024-03-28.\n" +
		"Try another provider with --provider, then run `fbull backtest create` again."
	noPricesAndGapsCopy = "Couldn't load data for backtest 'oef-2024q1': 2 symbols are missing prices. " +
		"The backtest wasn't saved.\n" +
		"  META (earlier FB): no prices between 2024-01-02 and 2024-03-28.\n" +
		"  OEF: no prices on 2024-02-14, 2024-02-20.\n" +
		"Try another provider with --provider, or change the backtest dates, then run `fbull backtest create` again."
	tickerHistoryCopy = "Couldn't load data for backtest 'oef-2024q1': foreverbull couldn't read its record of " +
		"ticker changes, so renamed symbols can't be followed. The backtest wasn't saved. " +
		"Run `fbenv status` to check that the environment is running, then run `fbull backtest create` again."
)

// The data service refuses an ingest with ABORTED and the user's sentence,
// with no details marker. That sentence is the step's whole message.
func TestIngestFailureMessageShowsARefusalAsWritten(t *testing.T) {
	tests := []struct {
		name     string
		sentence string
		removed  bool
		expected string
	}{
		{name: "no prices for one symbol", sentence: noPricesCopy, removed: true, expected: noPricesCopy},
		{name: "no prices for several symbols", sentence: noPricesBlockCopy, removed: true, expected: noPricesBlockCopy},
		{name: "ticker history unreadable", sentence: tickerHistoryCopy, removed: true, expected: tickerHistoryCopy},
		{name: "no prices and missing sessions", sentence: noPricesAndGapsCopy, removed: true, expected: noPricesAndGapsCopy},
		{
			name:     "no prices for one symbol, delete failed: partly saved, delete first",
			sentence: noPricesCopy,
			removed:  false,
			expected: "Couldn't load data for backtest 'oef-2024q1': there are no stored prices for META (earlier FB) " +
				"between 2024-01-02 and 2024-03-28. The backtest was partly saved and couldn't be removed. " +
				"Run `fbull backtest delete oef-2024q1` first. Try another provider with --provider, " +
				"then run `fbull backtest create` again.",
		},
		{
			name:     "no prices for several symbols, delete failed: partly saved, delete first",
			sentence: noPricesBlockCopy,
			removed:  false,
			expected: "Couldn't load data for backtest 'oef-2024q1': 2 symbols have no stored prices. " +
				"The backtest was partly saved and couldn't be removed.\n" +
				"  META (earlier FB): no prices between 2024-01-02 and 2024-03-28.\n" +
				"  OEF: no prices between 2024-01-02 and 2024-03-28.\n" +
				"Run `fbull backtest delete oef-2024q1` first. Try another provider with --provider, " +
				"then run `fbull backtest create` again.",
		},
		{
			name:     "no prices and missing sessions, delete failed: partly saved, delete first",
			sentence: noPricesAndGapsCopy,
			removed:  false,
			expected: "Couldn't load data for backtest 'oef-2024q1': 2 symbols are missing prices. " +
				"The backtest was partly saved and couldn't be removed.\n" +
				"  META (earlier FB): no prices between 2024-01-02 and 2024-03-28.\n" +
				"  OEF: no prices on 2024-02-14, 2024-02-20.\n" +
				"Run `fbull backtest delete oef-2024q1` first. Try another provider with --provider, " +
				"or change the backtest dates, then run `fbull backtest create` again.",
		},
		{
			name:     "ticker history unreadable, delete failed: partly saved, delete first",
			sentence: tickerHistoryCopy,
			removed:  false,
			expected: "Couldn't load data for backtest 'oef-2024q1': foreverbull couldn't read its record of " +
				"ticker changes, so renamed symbols can't be followed. " +
				"The backtest was partly saved and couldn't be removed. " +
				"Run `fbull backtest delete oef-2024q1` first. Run `fbenv status` to check that the " +
				"environment is running, then run `fbull backtest create` again.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cause := fmt.Errorf("error ingesting: %w", status.Error(codes.Aborted, tc.sentence))
			message := ingestFailureMessage("oef-2024q1", readIngestFailure(cause), tc.removed)
			require.Equal(t, tc.expected, message)
			require.NotContains(t, message, ingestDetailsMarker)
		})
	}
}

// A refusal that names another backtest, or one that carries details, is not
// shown as the reason: the fixed lead is, with the text behind --verbose.
func TestIngestFailureMessageKeepsTheLeadForAnUnexpectedRefusal(t *testing.T) {
	other := status.Error(codes.Aborted, "Couldn't load data for backtest 'other': boom")
	require.Equal(t, otherFailureCopy+"\nDetails: Couldn't load data for backtest 'other': boom",
		ingestFailureMessage("demo", readIngestFailure(other), true))

	detailed := status.Error(codes.Aborted, "Couldn't load data for backtest 'demo': boom\nDetails: KeyError: 'AAPL'")
	require.Equal(t, otherFailureCopy+"\nDetails: KeyError: 'AAPL'",
		ingestFailureMessage("demo", readIngestFailure(detailed), true))
}

// A refusal for this backtest without "The backtest wasn't saved." can't be
// rewritten for a backtest that wasn't removed, so the fixed lead is used.
func TestIngestFailureMessageKeepsTheLeadWhenARefusalCantSayDeleteFirst(t *testing.T) {
	refusal := status.Error(codes.Aborted, "Couldn't load data for backtest 'demo': boom.")
	require.Equal(t, "Couldn't load data for backtest 'demo': the data service failed while ingesting prices. "+
		"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete demo` first. "+
		"Run `fbull backtest create` again once it's fixed."+
		"\nDetails: Couldn't load data for backtest 'demo': boom.",
		ingestFailureMessage("demo", readIngestFailure(refusal), false))
}
