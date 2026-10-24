package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func TestLoginHashCapacityIsAtLeastTwo(t *testing.T) {
	got := loginHashCapacity()
	require.GreaterOrEqual(t, got, 2)
	if runtime.NumCPU() > 2 {
		require.Equal(t, runtime.NumCPU(), got)
	}
}

func TestLoginRejectsALongPasswordBeforeTheDatabase(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	h := &handlers{
		db:      panicDB{},
		logf:    func(string, ...any) {},
		limiter: newFailureLimiter(),
		now:     time.Now,
		hashes:  make(chan struct{}, 1),
	}
	for _, length := range []int{PasswordMaxLen + 1, 1500} {
		body := `{"username":"ada","password":"` + strings.Repeat("a", length) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		req.RemoteAddr = "203.0.113.5:1234"
		rec := httptest.NewRecorder()
		h.login(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.NotContains(t, rec.Body.String(), strings.Repeat("a", 12))
	}
}

func TestLoginBusyDoesNotRecordAPasswordFailure(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	now := time.Unix(1_700_000_000, 0)
	h := &handlers{
		db:      panicDB{},
		logf:    func(string, ...any) {},
		limiter: newFailureLimiter(),
		now:     func() time.Time { return now },
		hashes:  make(chan struct{}, 1),
	}
	h.hashes <- struct{}{}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"ada","password":"twelve-chars"}`))
	req.RemoteAddr = "203.0.113.5:1234"
	rec := httptest.NewRecorder()
	h.login(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.NotContains(t, rec.Body.String(), "twelve-chars")
	require.Zero(t, h.limiter.userFailures("ada", now))
}

func TestEveryFinishedAttemptCountsTowardTheIP(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	limiter := newFailureLimiter()
	now := time.Now()
	for range loginIPLimit {
		require.True(t, limiter.Begin("ada", "203.0.113.5", now))
		limiter.End("ada", "203.0.113.5", false, now)
	}
	require.False(t, limiter.Begin("ada", "203.0.113.5", now))
	require.False(t, limiter.Begin("other", "203.0.113.5", now))
	require.True(t, limiter.Begin("ada", "203.0.113.9", now))
	require.Zero(t, limiter.userFailures("ada", now))
}

type panicDB struct{}

func (panicDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("query")
}

func (panicDB) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("query")
}

func (panicDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("exec")
}
