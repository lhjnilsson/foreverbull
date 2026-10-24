package account

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

// ErrRoleChangesAreSQL is returned when a caller passes --role.
// account create inserts editor. account rotate does not change role.
var ErrRoleChangesAreSQL = errors.New("role changes are SQL")

const postgresPingTimeout = 5 * time.Second

// Command is the foreverbull account command. It does not start the broker.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "account",
		Usage: "create or rotate an editor password",
		Subcommands: []*cli.Command{
			{
				Name:  "create",
				Usage: "insert an editor account",
				Flags: []cli.Flag{usernameFlag()},
				Action: func(c *cli.Context) error {
					return runAccount(c, auth.CreateEditor)
				},
			},
			{
				Name:  "rotate",
				Usage: "replace a password hash",
				Flags: []cli.Flag{usernameFlag()},
				Action: func(c *cli.Context) error {
					return runAccount(c, auth.RotatePassword)
				},
			},
		},
	}
}

func usernameFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:     "username",
		Required: true,
		Usage:    "login name",
	}
}

// RejectRoleFlag reports --role before flag parsing. There is no role flag.
func RejectRoleFlag(args []string) error {
	for _, arg := range args {
		name, _, _ := strings.Cut(arg, "=")
		if name == "--role" || name == "-role" {
			return ErrRoleChangesAreSQL
		}
	}
	return nil
}

// Run executes the account command. Stdout is the account id on success.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := RejectRoleFlag(args); err != nil {
		return err
	}

	app := &cli.App{
		Name:           "foreverbull",
		Commands:       []*cli.Command{Command()},
		Reader:         stdin,
		Writer:         stdout,
		ErrWriter:      stderr,
		ExitErrHandler: func(*cli.Context, error) {},
	}
	return app.Run(args)
}

func runAccount(c *cli.Context, op func(context.Context, auth.DB, string, string) (string, error)) error {
	password, err := readPassword(c.App.Reader, c.App.ErrWriter)
	if err != nil {
		return err
	}
	if err := auth.CheckPasswordLength(password); err != nil {
		return err
	}
	if _, err := auth.NormalizeUsername(c.String("username")); err != nil {
		return err
	}

	ctx := c.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := environment.Setup(); err != nil {
		return fmt.Errorf("setup environment: %w", err)
	}

	pool, err := openPool(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := auth.Migrate(ctx, pool); err != nil {
		return err
	}

	id, err := op(ctx, pool, c.String("username"), password)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(c.App.Writer, id); err != nil {
		return fmt.Errorf("write account id: %w", err)
	}
	return nil
}

func readPassword(in io.Reader, errOut io.Writer) (string, error) {
	if in == nil {
		return "", errors.New("stdin is nil")
	}

	file, ok := in.(*os.File)
	if ok && term.IsTerminal(int(file.Fd())) {
		if errOut == nil {
			errOut = io.Discard
		}
		if _, err := fmt.Fprint(errOut, "Password: "); err != nil {
			return "", fmt.Errorf("password prompt: %w", err)
		}
		password, err := term.ReadPassword(int(file.Fd()))
		_, _ = fmt.Fprintln(errOut)
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		return string(password), nil
	}

	return readPasswordLine(in)
}

// readPasswordLine reads at most one password plus a newline. The bound is
// PasswordMaxLen plus a CRLF so a longer line fails the length check.
func readPasswordLine(in io.Reader) (string, error) {
	limited := io.LimitReader(in, int64(auth.PasswordMaxLen)+2)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	if len(data) >= 2 && data[len(data)-2] == '\r' && data[len(data)-1] == '\n' {
		data = data[:len(data)-2]
	} else if len(data) >= 1 && (data[len(data)-1] == '\n' || data[len(data)-1] == '\r') {
		data = data[:len(data)-1]
	}
	return string(data), nil
}

func openPool(ctx context.Context) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool from %s: %w", environment.PostgresURL, err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, postgresPingTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		cfg := pool.Config().ConnConfig
		address := net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port)))
		pool.Close()
		return nil, fmt.Errorf(
			"could not reach postgres at %s; check %s and that Postgres is running: %w",
			address, environment.PostgresURL, err,
		)
	}
	return pool, nil
}
