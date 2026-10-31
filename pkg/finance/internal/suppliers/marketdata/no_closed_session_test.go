package marketdata_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
)

// The reason names the dates asked for and the time a session counts as
// closed, the 16:30 New York time every fetch stops at.
func TestNoClosedSessionReason(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	require.Equal(t, "no trading session between 2026-10-31 and 2026-10-31 has closed yet "+
		"(sessions count as closed at 16:30 New York time)", marketdata.NoClosedSessionReason(day, day))
	require.Equal(t, "no trading session between 2026-10-30 and 2026-11-02 has closed yet "+
		"(sessions count as closed at 16:30 New York time)",
		marketdata.NoClosedSessionReason(day.AddDate(0, 0, -1), day.AddDate(0, 0, 2)))
}
