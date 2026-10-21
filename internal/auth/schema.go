package auth

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const accountTable = `
CREATE TABLE IF NOT EXISTS account (
	id text PRIMARY KEY DEFAULT uuid_generate_v4()::text,
	role text NOT NULL CHECK (role IN ('none', 'pending', 'viewer', 'editor', 'admin')),
	created_at timestamptz NOT NULL DEFAULT now()
)`

const accountPasswordTable = `
CREATE TABLE IF NOT EXISTS account_password (
	account_id text PRIMARY KEY REFERENCES account (id) ON DELETE CASCADE,
	username text NOT NULL UNIQUE CHECK (username ~ '^[A-Za-z0-9._-]{1,64}$'),
	password_hash text NOT NULL
)`

const accountGitHubTable = `
CREATE TABLE IF NOT EXISTS account_github (
	account_id text PRIMARY KEY REFERENCES account (id) ON DELETE CASCADE,
	github_subject text NOT NULL UNIQUE,
	github_login text NOT NULL
)`

const browserSessionTable = `
CREATE TABLE IF NOT EXISTS browser_session (
	id text PRIMARY KEY,
	account_id text NOT NULL REFERENCES account (id) ON DELETE CASCADE,
	secret_hash bytea NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	expires_at timestamptz NOT NULL,
	revoked_at timestamptz
)`

const apiTokenTable = `
CREATE TABLE IF NOT EXISTS api_token (
	id text PRIMARY KEY DEFAULT uuid_generate_v4()::text,
	account_id text NOT NULL REFERENCES account (id) ON DELETE CASCADE,
	token_hash bytea NOT NULL UNIQUE,
	hint text NOT NULL,
	name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 64),
	created_at timestamptz NOT NULL DEFAULT now(),
	last_used_at timestamptz,
	expires_at timestamptz,
	revoked_at timestamptz
)`

const oauthStateTable = `
CREATE TABLE IF NOT EXISTS oauth_state (
	state_hash bytea PRIMARY KEY,
	created_at timestamptz NOT NULL DEFAULT now(),
	expires_at timestamptz NOT NULL
)`

const domainGrantTable = `
CREATE TABLE IF NOT EXISTS domain_grant (
	domain_kind text NOT NULL CHECK (domain_kind IN ('service', 'backtest', 'strategy')),
	domain_id text NOT NULL,
	grantee_id text NOT NULL REFERENCES account (id) ON DELETE CASCADE,
	level text NOT NULL CHECK (level IN ('viewer', 'editor')),
	created_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (domain_kind, domain_id, grantee_id)
)`

const (
	apiTokenAccountIndex    = `CREATE INDEX IF NOT EXISTS api_token_account_id_idx ON api_token (account_id)`
	browserSessionIndex     = `CREATE INDEX IF NOT EXISTS browser_session_account_id_idx ON browser_session (account_id)`
	domainGrantGranteeIndex = `CREATE INDEX IF NOT EXISTS domain_grant_grantee_id_domain_kind_idx ON domain_grant (grantee_id, domain_kind)`
	uuidExtension           = `CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`
)

// Migrate creates the auth tables. It does not alter service, backtest, or
// strategy. Calling it twice is safe.
func Migrate(ctx context.Context, conn postgres.Query) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if conn == nil {
		return fmt.Errorf("auth migrate: nil database")
	}

	statements := []struct {
		name string
		sql  string
	}{
		{"uuid-ossp", uuidExtension},
		{"account", accountTable},
		{"account_password", accountPasswordTable},
		{"account_github", accountGitHubTable},
		{"browser_session", browserSessionTable},
		{"api_token", apiTokenTable},
		{"oauth_state", oauthStateTable},
		{"domain_grant", domainGrantTable},
		{"api_token_account_id_idx", apiTokenAccountIndex},
		{"browser_session_account_id_idx", browserSessionIndex},
		{"domain_grant_grantee_id_domain_kind_idx", domainGrantGranteeIndex},
	}
	for _, statement := range statements {
		if _, err := conn.Exec(ctx, statement.sql); err != nil {
			return fmt.Errorf("auth migrate %s: %w", statement.name, err)
		}
	}
	return nil
}
