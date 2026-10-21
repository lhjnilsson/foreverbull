package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

// LogFunc receives startup lines. It must not be handed a password or a hash.
type LogFunc func(format string, args ...any)

// Startup prepares authentication before the process accepts requests.
// A false flag keeps today's schema. After owner_id is NOT NULL, a false
// start exits. A true flag returns the admin account id and does not migrate
// domain rows; the caller does that before it listens.
func Startup(ctx context.Context, db DB, logf LogFunc) (string, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	enabled := environment.GetAuthEnabled()
	logf("AUTH_ENABLED %t", enabled)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if db == nil {
		return "", errors.New("auth startup: nil database")
	}
	if !enabled {
		locked, err := OwnersLocked(ctx, db)
		if err != nil {
			return "", err
		}
		if locked {
			return "", errors.New("owner_id is NOT NULL; set AUTH_ENABLED=true")
		}
		return "", nil
	}
	if err := Migrate(ctx, db); err != nil {
		return "", err
	}
	if err := checkGitHubConfig(); err != nil {
		return "", err
	}
	return EnsureAdmin(ctx, db, environment.GetAuthAdminUsername(), environment.GetAuthAdminPasswordFile())
}

// OwnersLocked reports whether service, backtest, or strategy already has
// owner_id NOT NULL. Turning authentication off is not a rollback.
func OwnersLocked(ctx context.Context, db postgres.Query) (bool, error) {
	for _, table := range []string{DomainService, DomainBacktest, DomainStrategy} {
		exists, err := TableExists(ctx, db, table)
		if err != nil {
			return false, err
		}
		if !exists {
			continue
		}
		has, err := ColumnExists(ctx, db, table, "owner_id")
		if err != nil {
			return false, err
		}
		if !has {
			continue
		}
		notNull, err := ColumnNotNull(ctx, db, table, "owner_id")
		if err != nil {
			return false, err
		}
		if notNull {
			return true, nil
		}
	}
	return false, nil
}

func checkGitHubConfig() error {
	id := environment.GetAuthGitHubClientID()
	secret := environment.GetAuthGitHubClientSecret()
	redirect := environment.GetAuthGitHubRedirectURL()
	if id == "" && secret == "" && redirect == "" {
		return nil
	}
	var missing []string
	if id == "" {
		missing = append(missing, environment.AuthGitHubClientID)
	}
	if secret == "" {
		missing = append(missing, environment.AuthGitHubClientSecret)
	}
	if redirect == "" {
		missing = append(missing, environment.AuthGitHubRedirectURL)
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%s is empty", strings.Join(missing, ", "))
}

// EnsureAdmin inserts the platform admin from the password file when that
// username has no password row. An existing admin is left unchanged. Any
// other role is an error and is not updated.
func EnsureAdmin(ctx context.Context, db DB, username, passwordFile string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", errors.New("AUTH_ADMIN_USERNAME is empty")
	}
	normalized, err := NormalizeUsername(username)
	if err != nil {
		return "", fmt.Errorf("AUTH_ADMIN_USERNAME: %w", err)
	}

	var (
		id   string
		role string
	)
	err = db.QueryRow(ctx, `
		SELECT a.id, a.role
		FROM account_password AS p
		JOIN account AS a ON a.id = p.account_id
		WHERE p.username = $1`, normalized).Scan(&id, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return insertAdmin(ctx, db, normalized, passwordFile)
	}
	if err != nil {
		return "", fmt.Errorf("lookup admin: %w", err)
	}
	if role != RoleAdmin {
		return "", fmt.Errorf("AUTH_ADMIN_USERNAME %q has role %s; role changes are SQL", normalized, role)
	}
	return id, nil
}

func insertAdmin(ctx context.Context, db DB, username, passwordFile string) (string, error) {
	password, err := readAdminPassword(passwordFile)
	if err != nil {
		return "", err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("AUTH_ADMIN_PASSWORD_FILE: %w", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin admin insert: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `INSERT INTO account (role) VALUES ('admin') RETURNING id`).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert admin: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO account_password (account_id, username, password_hash)
		VALUES ($1, $2, $3)`, id, username, hash)
	if err != nil {
		if isUniqueViolation(err) {
			return "", fmt.Errorf("AUTH_ADMIN_USERNAME: %w", ErrUsernameExists)
		}
		return "", fmt.Errorf("insert admin password: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit admin insert: %w", err)
	}
	return id, nil
}

func readAdminPassword(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("AUTH_ADMIN_PASSWORD_FILE is empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("AUTH_ADMIN_PASSWORD_FILE: %w", err)
	}
	password := strings.TrimSpace(string(raw))
	if password == "" {
		return "", errors.New("AUTH_ADMIN_PASSWORD_FILE is empty")
	}
	if err := CheckPasswordLength(password); err != nil {
		return "", fmt.Errorf("AUTH_ADMIN_PASSWORD_FILE: %w", err)
	}
	return password, nil
}
