package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func TestMutationAllowedRequiresSameOriginOrPublicOrigin(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	t.Setenv(environment.AuthPublicURL, "https://fb.example.com")

	same := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	same.Header.Set("Sec-Fetch-Site", "same-origin")
	require.True(t, mutationAllowed(same))

	none := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	none.Header.Set("Sec-Fetch-Site", "none")
	require.True(t, mutationAllowed(none))

	sibling := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	sibling.Header.Set("Sec-Fetch-Site", "same-site")
	sibling.Header.Set("Origin", "https://other.example.com")
	require.False(t, mutationAllowed(sibling))

	cross := httptest.NewRequest(http.MethodDelete, "/api/auth/tokens/1", nil)
	cross.Header.Set("Sec-Fetch-Site", "cross-site")
	require.False(t, mutationAllowed(cross))

	matched := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	matched.Header.Set("Origin", "https://fb.example.com")
	require.True(t, mutationAllowed(matched))

	missing := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	require.False(t, mutationAllowed(missing))

	read := httptest.NewRequest(http.MethodGet, "/api/services", nil)
	read.Header.Set("Origin", "https://evil.example.com")
	require.True(t, mutationAllowed(read))

	bearer := httptest.NewRequest(http.MethodDelete, "/api/auth/tokens/1", nil)
	bearer.Header.Set("Authorization", "Bearer fb_token")
	bearer.Header.Set("Sec-Fetch-Site", "cross-site")
	require.True(t, mutationAllowed(bearer))
}
