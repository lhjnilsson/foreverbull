package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
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

// execDB answers Exec with err and counts the calls. Other methods are not
// used by logout.
type execDB struct {
	panicDB
	err   error
	calls int
}

func (d *execDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	d.calls++
	return pgconn.CommandTag{}, d.err
}

func logoutRequest(t *testing.T) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req = req.WithContext(WithAccount(req.Context(), Account{ID: "account-1", Role: RoleViewer}))
	value := "session-1." + base64.RawURLEncoding.EncodeToString(make([]byte, sessionSecretLen))
	req.AddCookie(&http.Cookie{Name: sessionCookieName(req), Value: value})
	return req
}

func TestLogoutFailsWhenTheSessionCannotBeRevokedAndStillClearsTheCookie(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	t.Setenv(environment.AuthCookieSecure, "false")
	db := &execDB{err: errors.New("connection reset: postgres://foreverbull:pw@postgres")}
	var logged []string
	h := &handlers{db: db, logf: func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }, now: time.Now}

	rec := httptest.NewRecorder()
	h.logout(rec, logoutRequest(t))

	require.Equal(t, 1, db.calls)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"error":"internal"}`, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "connection reset")
	require.Equal(t, []string{"logout account=account-1 result=revoke_failed err=*errors.errorString"}, logged)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, sessionCookie, cookies[0].Name)
	require.Empty(t, cookies[0].Value)
	require.Negative(t, cookies[0].MaxAge)
}

func TestLogoutLogsTheRevokeErrorWithoutTheCookieOrItsText(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	t.Setenv(environment.AuthCookieSecure, "false")
	pgErr := &pgconn.PgError{Code: "22P02", Message: `invalid input syntax for type uuid: "session-1"`}
	cases := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("revoke: %w", pgErr), "sqlstate=22P02"},
		{context.Canceled, "context_canceled"},
		{fmt.Errorf("exec: %w", context.DeadlineExceeded), "deadline_exceeded"},
	}
	for _, tc := range cases {
		var logged []string
		h := &handlers{
			db:   &execDB{err: tc.err},
			logf: func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) },
			now:  time.Now,
		}
		request := logoutRequest(t)
		cookie, err := request.Cookie(sessionCookieName(request))
		require.NoError(t, err)

		h.logout(httptest.NewRecorder(), request)

		require.Equal(t, []string{"logout account=account-1 result=revoke_failed err=" + tc.want}, logged)
		require.NotContains(t, logged[0], "session-1")
		require.NotContains(t, logged[0], cookie.Value)
	}
}

func TestLogoutRevokesTheSessionAndClearsTheCookie(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	t.Setenv(environment.AuthCookieSecure, "false")
	db := &execDB{}
	h := &handlers{db: db, logf: func(string, ...any) {}, now: time.Now}

	rec := httptest.NewRecorder()
	h.logout(rec, logoutRequest(t))

	require.Equal(t, 1, db.calls)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"ok":true}`, rec.Body.String())
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Empty(t, cookies[0].Value)
	require.Negative(t, cookies[0].MaxAge)
}
