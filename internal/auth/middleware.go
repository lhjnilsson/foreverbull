package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/web"
)

// Protect refuses /api routes that are not anonymous unless the cookie is valid.
// It is installed only when AUTH_ENABLED is true. While the flag is false it
// serves the next handler unchanged.
func Protect(db postgres.Query) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !environment.GetAuthEnabled() {
				next.ServeHTTP(w, r)
				return
			}
			if anonymousRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			account, err := AccountFromRequest(r.Context(), db, r, time.Now())
			if err != nil {
				ClearSessionCookie(w, r)
				web.Unauthorized(w)
				return
			}
			r = r.WithContext(WithAccount(r.Context(), account))
			if anyRoleRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			if account.Role == RoleNone || account.Role == RolePending {
				web.Forbidden(w)
				return
			}
			if account.Role == RoleViewer && r.Method != http.MethodGet && r.Method != http.MethodHead {
				web.Forbidden(w)
				return
			}
			if account.Role != RoleViewer && account.Role != RoleEditor && account.Role != RoleAdmin {
				web.Forbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func anonymousRequest(r *http.Request) bool {
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && path == "/api/health":
		return true
	case r.Method == http.MethodGet && path == "/api/auth/providers":
		return true
	case r.Method == http.MethodPost && path == "/api/auth/login":
		return true
	case r.Method == http.MethodGet && path == "/api/auth/github/start":
		return true
	case r.Method == http.MethodGet && path == "/api/auth/github/callback":
		return true
	case (r.Method == http.MethodGet || r.Method == http.MethodHead) && !underAPI(path):
		return true
	default:
		return false
	}
}

func anyRoleRequest(r *http.Request) bool {
	path := r.URL.Path
	if r.Method == http.MethodGet && path == "/api/auth/me" {
		return true
	}
	return r.Method == http.MethodPost && path == "/api/auth/logout"
}

func underAPI(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}
