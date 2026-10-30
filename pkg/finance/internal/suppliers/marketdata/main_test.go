package marketdata

import (
	"os"
	"testing"
	"time"
)

// TestMain shortens the shared provider limiters before any test starts, so
// the fixtures don't wait out the production rates. Tests of the rate itself
// build their own limiter.
func TestMain(m *testing.M) {
	for _, limiter := range providerLimiters {
		limiter.interval = time.Millisecond
	}
	os.Exit(m.Run())
}
