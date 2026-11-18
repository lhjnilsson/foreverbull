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
	for provider := range providerLimiters {
		limiter := newRequestLimiterEvery(time.Millisecond)
		// The name stays, so a long pause in a test can still be reported.
		limiter.provider = provider
		providerLimiters[provider] = limiter
	}
	os.Exit(m.Run())
}
