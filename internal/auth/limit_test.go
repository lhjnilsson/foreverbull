package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func TestClientAddrUsesRightmostUntrustedHop(t *testing.T) {
	t.Setenv(environment.AuthTrustProxy, "true")
	t.Setenv(environment.AuthTrustedProxies, "10.0.0.0/8")
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	request.RemoteAddr = "10.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "1.1.1.1, 10.0.0.5")
	require.Equal(t, "1.1.1.1", clientAddr(request))

	request.Header.Set("X-Forwarded-For", "10.1.1.1, 203.0.113.9")
	require.Equal(t, "203.0.113.9", clientAddr(request))
}

func TestClientAddrIgnoresForwardedWithoutTrust(t *testing.T) {
	t.Setenv(environment.AuthTrustProxy, "false")
	t.Setenv(environment.AuthTrustedProxies, "10.0.0.0/8")
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	request.RemoteAddr = "10.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "1.1.1.1")
	require.Equal(t, "10.0.0.1", clientAddr(request))
}

func TestBackoffStaysOnTheAddressThatFailed(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	limiter := newFailureLimiter()
	now := time.Unix(1_700_000_000, 0)
	home := "203.0.113.8"
	away := "203.0.113.9"
	require.True(t, limiter.Begin("admin", home, now))
	limiter.End("admin", home, true, now)
	require.False(t, limiter.Begin("admin", home, now))
	require.True(t, limiter.Begin("admin", away, now))
	require.True(t, limiter.Begin("admin", home, now.Add(time.Second)))
	limiter.End("admin", home, true, now.Add(time.Second))
	require.False(t, limiter.Begin("admin", home, now.Add(2*time.Second)))
	require.True(t, limiter.Begin("admin", home, now.Add(3*time.Second)))
}

func TestBeginCountsInFlightBeforeThePasswordIsChecked(t *testing.T) {
	limiter := newFailureLimiter()
	now := time.Now()
	for range loginIPLimit {
		require.True(t, limiter.Begin("ada", "203.0.113.5", now))
	}
	require.False(t, limiter.Begin("ada", "203.0.113.5", now))
	limiter.End("ada", "203.0.113.5", false, now)
	require.False(t, limiter.Begin("other", "203.0.113.5", now))
}

func TestFloodOfNewNamesDoesNotClearABackoff(t *testing.T) {
	limiter := newFailureLimiter()
	now := time.Now()
	home := "203.0.113.8"
	require.True(t, limiter.Begin("admin", home, now))
	limiter.End("admin", home, true, now)
	for i := range loginKeyCap + 10 {
		name := fmt.Sprintf("other-%d", i)
		other := fmt.Sprintf("198.51.100.%d", i%200)
		if limiter.Begin(name, other, now) {
			limiter.End(name, other, true, now)
		}
	}
	require.False(t, limiter.Begin("admin", home, now))
	require.True(t, limiter.Begin("admin", "203.0.113.9", now))
}
