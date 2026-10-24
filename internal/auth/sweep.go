package auth

import (
	"context"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const sweepInterval = time.Hour

// sweepExpired drops expired oauth state and expired, revoked, or idle browser
// sessions. A failed delete is logged and does not stop the caller.
func sweepExpired(ctx context.Context, db postgres.Query, now time.Time, logf LogFunc) {
	if db == nil {
		return
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if _, err := db.Exec(ctx, `DELETE FROM oauth_state WHERE expires_at <= $1`, now); err != nil {
		logf("sweep oauth_state failed: %v", err)
	}
	idleBefore := now.Add(-sessionLifetime)
	if _, err := db.Exec(ctx, `
		DELETE FROM browser_session
		WHERE expires_at <= $1
		   OR revoked_at IS NOT NULL
		   OR last_seen_at <= $2`, now, idleBefore); err != nil {
		logf("sweep browser_session failed: %v", err)
	}
}

// SweepLoop deletes expired login rows about once an hour until ctx is done.
func SweepLoop(ctx context.Context, db postgres.Query, logf LogFunc) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			sweepExpired(ctx, db, now, logf)
		}
	}
}
