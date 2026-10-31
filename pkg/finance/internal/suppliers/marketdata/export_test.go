package marketdata

import (
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
)

// NewYahooRangeClient is a Yahoo client against yahooRangeServer, with one
// daily bar at the 09:30 New York open on each day given, on a fixed clock.
// It is for tests outside the package that store what GetOHLC returns.
func NewYahooRangeClient(t *testing.T, now time.Time, days ...string) supplier.Marketdata {
	t.Helper()
	return &YahooClient{
		chartURL: yahooRangeServer(t, stamps(t, "09:30", days...)...),
		now:      func() time.Time { return now },
	}
}
