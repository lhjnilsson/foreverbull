package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	commonPB "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"google.golang.org/protobuf/proto"
)

const BacktestTable = `
CREATE TABLE IF NOT EXISTS backtest (
	name text PRIMARY KEY CONSTRAINT backtestnamecheck CHECK (char_length(name) > 0),
	benchmark text NOT NULL,
	provider text NOT NULL,
	profile text NOT NULL DEFAULT '',
	auto_close_on_universe_drop boolean NOT NULL DEFAULT true,
	CONSTRAINT backtest_provider_check CHECK (provider IN ('yahoo', 'stooq', 'alpaca'))
);

CREATE TABLE IF NOT EXISTS backtest_period (
	backtest_name text REFERENCES backtest (name) ON DELETE CASCADE,
	start_date date NOT NULL,
	end_date date NOT NULL,
	PRIMARY KEY (backtest_name, start_date, end_date)
);

CREATE TABLE IF NOT EXISTS backtest_period_asset (
    backtest_name text,
    start_date date,
    end_date date,
    symbol text NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    PRIMARY KEY (backtest_name, start_date, end_date, symbol),
    FOREIGN KEY (backtest_name, start_date, end_date) 
        REFERENCES backtest_period(backtest_name, start_date, end_date) ON DELETE CASCADE
);
`

type Backtest struct {
	Conn postgres.Query
}

// ErrCloseOnUniverseDropFixed is returned when a create would change the
// option stored on an existing backtest.
var ErrCloseOnUniverseDropFixed = errors.New(usercopy.CloseOnUniverseDropFixed)

// closeOnUniverseDrop is the stored choice. An omitted field is on.
func closeOnUniverseDrop(item *backtest.Backtest) bool {
	if item == nil || item.AutoCloseOnUniverseDrop == nil {
		return true
	}
	return *item.AutoCloseOnUniverseDrop
}

func assignCloseOnUniverseDrop(item *backtest.Backtest, on bool) {
	if item == nil {
		return
	}
	item.AutoCloseOnUniverseDrop = proto.Bool(on)
}

// MigrateAutoCloseOnUniverseDrop adds the create-time option. Existing rows
// are on, which is the default. Calling it twice is safe.
func MigrateAutoCloseOnUniverseDrop(ctx context.Context, conn postgres.Query) error {
	_, err := conn.Exec(ctx, `
		ALTER TABLE backtest
		ADD COLUMN IF NOT EXISTS auto_close_on_universe_drop boolean NOT NULL DEFAULT true`)
	if err != nil {
		return fmt.Errorf("backtest: auto_close_on_universe_drop: %w", err)
	}
	return nil
}

func normalizeBacktestProvider(provider string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "":
		return supplier.DefaultProvider, nil
	case "yahoo", "stooq", "alpaca":
		return provider, nil
	default:
		return "", fmt.Errorf("unknown price provider %q; valid providers: yahoo, stooq, alpaca", provider)
	}
}

// MigrateBacktestProvider drops the provider column default and adds the
// yahoo/stooq/alpaca check. The column has no default afterwards, so a new
// row must name its provider. Rows that would violate the check stay.
// Calling it twice is safe.
func MigrateBacktestProvider(ctx context.Context, conn postgres.Query) error {
	_, err := conn.Exec(ctx, `ALTER TABLE backtest ALTER COLUMN provider DROP DEFAULT`)
	if err != nil {
		return fmt.Errorf("backtest: provider default: %w", err)
	}

	exists, err := tableConstraintExists(ctx, conn, "backtest", "backtest_provider_check")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	var bad bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM backtest
			WHERE provider IS NULL
			   OR provider NOT IN ('yahoo', 'stooq', 'alpaca')
		)`).Scan(&bad)
	if err != nil {
		return fmt.Errorf("backtest: scan provider: %w", err)
	}
	if bad {
		return fmt.Errorf("backtest: rows violate provider IN ('yahoo', 'stooq', 'alpaca')")
	}

	_, err = conn.Exec(ctx, `
		ALTER TABLE backtest
			ADD CONSTRAINT backtest_provider_check
			CHECK (provider IN ('yahoo', 'stooq', 'alpaca'))`)
	if err != nil {
		return fmt.Errorf("backtest: provider check: %w", err)
	}
	return nil
}

// backtestWrite is one create. writeMembers stores universe membership in the
// same transaction as the backtest. Instrument and alias rows are not part of
// it: they are shared, and a retry finds them.
type backtestWrite struct {
	ownerID      string
	item         *backtest.Backtest
	members      []MemberInterval
	writeMembers bool
}

// Create inserts the backtest, its periods, and its period assets in one
// transaction. A failure rolls those rows back, so the name is not left taken.
func (db *Backtest) Create(ctx context.Context, item *backtest.Backtest) (*backtest.Backtest, error) {
	return db.insertBacktest(ctx, backtestWrite{item: item})
}

// CreateWithMembers is Create plus the universe members, in that same
// transaction. An empty list still replaces membership, which stores none.
func (db *Backtest) CreateWithMembers(ctx context.Context, item *backtest.Backtest, members []MemberInterval) (*backtest.Backtest, error) {
	return db.insertBacktest(ctx, backtestWrite{item: item, members: members, writeMembers: true})
}

func (db *Backtest) insertBacktest(ctx context.Context, in backtestWrite) (*backtest.Backtest, error) {
	provider, err := normalizeBacktestProvider(in.item.GetProvider())
	if err != nil {
		return nil, err
	}
	on := closeOnUniverseDrop(in.item)
	assignCloseOnUniverseDrop(in.item, on)
	if err := refuseCloseOnDropChange(ctx, db.Conn, in, on); err != nil {
		return nil, err
	}
	tx, err := beginBacktestTx(ctx, db.Conn)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	id, err := insertBacktestRow(ctx, tx, in, provider)
	if err != nil {
		return nil, err
	}
	if err := insertBacktestPeriods(ctx, tx, in, id); err != nil {
		return nil, err
	}
	if in.writeMembers {
		if err := insertBacktestMembers(ctx, tx, in, id); err != nil {
			return nil, err
		}
	}
	stored, err := readCreatedBacktest(ctx, tx, in, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to create backtest: %w", err)
	}
	return stored, nil
}

func beginBacktestTx(ctx context.Context, conn postgres.Query) (pgx.Tx, error) {
	begin, ok := conn.(txBeginner)
	if !ok {
		return nil, fmt.Errorf("failed to create backtest: connection cannot begin a transaction")
	}
	tx, err := begin.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create backtest: %w", err)
	}
	return tx, nil
}

func insertBacktestRow(ctx context.Context, conn postgres.Query, in backtestWrite, provider string) (string, error) {
	if in.ownerID != "" {
		var id string
		err := conn.QueryRow(ctx, `
			INSERT INTO backtest (name, benchmark, provider, profile, owner_id, auto_close_on_universe_drop)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id`,
			in.item.GetName(), in.item.GetBenchmark(), provider, in.item.GetProfile(), in.ownerID,
			closeOnUniverseDrop(in.item)).Scan(&id)
		if err != nil {
			return "", fmt.Errorf("create backtest: %w", err)
		}
		return id, nil
	}
	profile := strings.TrimSpace(in.item.GetProfile())
	var id string
	err := conn.QueryRow(ctx,
		`INSERT INTO backtest (name, benchmark, provider, profile, auto_close_on_universe_drop)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		in.item.Name, in.item.Benchmark, provider, profile, closeOnUniverseDrop(in.item)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create backtest: %w", err)
	}
	if strings.TrimSpace(id) == "" {
		return "", errors.New("backtest id is empty")
	}
	return id, nil
}

// IDByName is the surrogate id of the backtest called name. Auth off is the
// only caller: name is the primary key there. An empty id is an error.
func (db *Backtest) IDByName(ctx context.Context, name string) (string, error) {
	if authOn() {
		return "", fmt.Errorf("read backtest id by name is refused while AUTH_ENABLED=true")
	}
	var id string
	err := db.Conn.QueryRow(ctx, `SELECT id FROM backtest WHERE name = $1`, name).Scan(&id)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(id) == "" {
		return "", errors.New("backtest id is empty")
	}
	return id, nil
}

func refuseCloseOnDropChange(ctx context.Context, conn postgres.Query, in backtestWrite, next bool) error {
	var query string
	args := []any{in.item.GetName()}
	if in.ownerID != "" {
		query = `SELECT auto_close_on_universe_drop FROM backtest WHERE owner_id = $1 AND name = $2`
		args = []any{in.ownerID, in.item.GetName()}
	} else {
		query = `SELECT auto_close_on_universe_drop FROM backtest WHERE name = $1`
	}
	var stored bool
	err := conn.QueryRow(ctx, query, args...).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to create backtest: %w", err)
	}
	if stored != next {
		return ErrCloseOnUniverseDropFixed
	}
	return nil
}

func insertBacktestPeriods(ctx context.Context, conn postgres.Query, in backtestWrite, id string) error {
	if in.ownerID != "" {
		return insertOwnedPeriods(ctx, conn, id, in.item.GetPeriods())
	}
	return insertNamedPeriods(ctx, conn, in.item.Name, in.item.Periods)
}

func insertNamedPeriods(ctx context.Context, conn postgres.Query, name string, periods []*backtest.Backtest_Period) error {
	for _, period := range periods {
		start := commonPB.DateToDateString(period.StartDate)
		end := commonPB.DateToDateString(period.EndDate)
		_, err := conn.Exec(ctx,
			`INSERT INTO backtest_period (backtest_name, start_date, end_date)
			VALUES ($1, $2, $3)`,
			name, start, end)
		if err != nil {
			return fmt.Errorf("failed to create backtest period: %w", err)
		}
		for _, asset := range period.Assets {
			_, err := conn.Exec(ctx,
				`INSERT INTO backtest_period_asset (backtest_name, start_date, end_date, symbol, enabled)
				VALUES ($1, $2, $3, $4, $5)`,
				name, start, end, asset.Symbol, asset.Enabled)
			if err != nil {
				return fmt.Errorf("failed to create backtest period asset: %w", err)
			}
		}
	}
	return nil
}

func insertOwnedPeriods(ctx context.Context, conn postgres.Query, id string, periods []*backtest.Backtest_Period) error {
	for _, period := range periods {
		start := commonPB.DateToDateString(period.GetStartDate())
		end := commonPB.DateToDateString(period.GetEndDate())
		_, err := conn.Exec(ctx, `
			INSERT INTO backtest_period (backtest_id, start_date, end_date)
			VALUES ($1, $2, $3)`, id, start, end)
		if err != nil {
			return fmt.Errorf("create backtest period: %w", err)
		}
		for _, asset := range period.GetAssets() {
			_, err = conn.Exec(ctx, `
				INSERT INTO backtest_period_asset (backtest_id, start_date, end_date, symbol, enabled)
				VALUES ($1, $2, $3, $4, $5)`,
				id, start, end, asset.GetSymbol(), asset.GetEnabled())
			if err != nil {
				return fmt.Errorf("create backtest period asset: %w", err)
			}
		}
	}
	return nil
}

func insertBacktestMembers(ctx context.Context, conn postgres.Query, in backtestWrite, id string) error {
	members := &UniverseMember{Conn: conn}
	if in.ownerID != "" {
		return members.ReplaceByID(ctx, id, in.members)
	}
	return members.Replace(ctx, in.item.Name, in.members)
}

func readCreatedBacktest(ctx context.Context, conn postgres.Query, in backtestWrite, id string) (*backtest.Backtest, error) {
	stored := &Backtest{Conn: conn}
	if in.ownerID != "" {
		return stored.GetByID(ctx, id)
	}
	return stored.Get(ctx, in.item.Name)
}

func (db *Backtest) Get(ctx context.Context, name string) (*backtest.Backtest, error) {
	if authOn() {
		return nil, fmt.Errorf("get backtest by name is refused while AUTH_ENABLED=true")
	}
	type rowData struct {
		Name        string
		Benchmark   string
		Provider    string
		Profile     string
		CloseOnDrop bool
		HasUniverse bool
		Resolved    []byte
		StartDate   *time.Time
		EndDate     *time.Time
		Symbol      *string
		Enabled     *bool
	}
	rows, err := db.Conn.Query(ctx, `
	    SELECT 
			backtest.name,
			backtest.benchmark,
			backtest.provider,
			backtest.profile,
			backtest.auto_close_on_universe_drop,
			EXISTS (
				SELECT 1 FROM universe_member AS member
				WHERE member.backtest_name = backtest.name
			),
			backtest.ticker_resolutions,
			backtest_period.start_date,
			backtest_period.end_date,
			backtest_period_asset.symbol,
			backtest_period_asset.enabled
		FROM backtest
		LEFT JOIN backtest_period ON backtest.name = backtest_period.backtest_name
		LEFT JOIN backtest_period_asset ON backtest_period.backtest_name = backtest_period_asset.backtest_name
		AND backtest_period.start_date = backtest_period_asset.start_date
		WHERE backtest.name = $1
		ORDER BY backtest_period.start_date, backtest_period_asset.symbol DESC
	`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get backtest: %w", err)
	}
	defer rows.Close()

	b := backtest.Backtest{}

	for rows.Next() {
		var row rowData
		err = rows.Scan(&row.Name, &row.Benchmark, &row.Provider, &row.Profile, &row.CloseOnDrop, &row.HasUniverse, &row.Resolved, &row.StartDate, &row.EndDate, &row.Symbol, &row.Enabled)
		if err != nil {
			return nil, fmt.Errorf("failed to scan period: %w", err)
		}

		if b.Name == "" {
			b.TickerResolutions, err = decodeTickerResolutions(row.Resolved)
			if err != nil {
				return nil, fmt.Errorf("backtest %s: %w", row.Name, err)
			}
		}
		b.Name = row.Name
		b.Benchmark = row.Benchmark
		b.Provider = row.Provider
		b.Profile = row.Profile
		assignCloseOnUniverseDrop(&b, row.CloseOnDrop)
		b.HasUniverseMembers = row.HasUniverse

		if row.StartDate == nil || row.EndDate == nil {
			continue
		}
		var p *backtest.Backtest_Period
		startDate := commonPB.GoTimeToDate(*row.StartDate)
		endDate := commonPB.GoTimeToDate(*row.EndDate)
		for _, existing := range b.Periods {
			if existing.StartDate.Year == startDate.Year && existing.StartDate.Month == startDate.Month && existing.StartDate.Day == startDate.Day &&
				existing.EndDate.Year == endDate.Year && existing.EndDate.Month == endDate.Month && existing.EndDate.Day == endDate.Day {
				p = existing
				break
			}
		}
		if p == nil {
			p = &backtest.Backtest_Period{
				StartDate: startDate,
				EndDate:   endDate,
				Assets:    make([]*backtest.Backtest_Period_Asset, 0),
			}
			b.Periods = append(b.Periods, p)
		}
		if row.Symbol == nil {
			continue
		}
		p.Assets = append(p.Assets, &backtest.Backtest_Period_Asset{
			Symbol:  *row.Symbol,
			Enabled: *row.Enabled,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read backtest: %w", err)
	}
	if b.Name == "" {
		return nil, pgx.ErrNoRows
	}
	return &b, nil
}

func (db *Backtest) UpdateBenchmark(ctx context.Context, name string, benchmark string,
) (*backtest.Backtest, error) {
	_, err := db.Conn.Exec(ctx, `UPDATE backtest SET benchmark=$2 WHERE name=$1`, name, benchmark)
	if err != nil {
		return nil, fmt.Errorf("failed to update backtest: %w", err)
	}
	return db.Get(ctx, name)
}

func (db *Backtest) UpdatePeriod(ctx context.Context, name string, period *backtest.Backtest_Period) error {
	for _, asset := range period.Assets {
		_, err := db.Conn.Exec(ctx, `INSERT INTO backtest_period_asset (backtest_name, start_date, end_date, symbol, enabled)
		VALUES ($1, $2, $3, $4, $5)`,
			name, commonPB.DateToDateString(period.StartDate), commonPB.DateToDateString(period.EndDate), asset.Symbol, asset.Enabled)
		if err != nil {
			return fmt.Errorf("failed to update backtest period asset: %w", err)
		}
	}
	return nil
}

func (db *Backtest) UpdatePeriodAsset(ctx context.Context, name string, asset_symbol string, enabled bool) error {
	if authOn() {
		return fmt.Errorf("update period asset by name is refused while AUTH_ENABLED=true")
	}
	_, err := db.Conn.Exec(ctx, `UPDATE backtest_period_asset SET enabled=$3 WHERE backtest_name=$1 AND symbol=$2`,
		name, asset_symbol, enabled)
	if err != nil {
		return fmt.Errorf("failed to update backtest period asset: %w", err)
	}
	return nil
}

func (db *Backtest) List(ctx context.Context) ([]*backtest.Backtest, error) {
	type rowData struct {
		Name        string
		Benchmark   string
		Provider    string
		Profile     string
		CloseOnDrop bool
		HasUniverse bool
		Resolved    []byte
		StartDate   *time.Time
		EndDate     *time.Time
		Symbol      *string
		Enabled     *bool
	}
	rows, err := db.Conn.Query(ctx, `
	    SELECT 
			backtest.name,
			backtest.benchmark,
			backtest.provider,
			backtest.profile,
			backtest.auto_close_on_universe_drop,
			EXISTS (
				SELECT 1 FROM universe_member AS member
				WHERE member.backtest_name = backtest.name
			),
			backtest.ticker_resolutions,
			backtest_period.start_date,
			backtest_period.end_date,
			backtest_period_asset.symbol,
			backtest_period_asset.enabled
		FROM backtest
		LEFT JOIN backtest_period ON backtest.name = backtest_period.backtest_name
		LEFT JOIN backtest_period_asset ON backtest_period.backtest_name = backtest_period_asset.backtest_name
		AND backtest_period.start_date = backtest_period_asset.start_date
		ORDER BY backtest_period.start_date, backtest_period_asset.symbol DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list backtests: %w", err)
	}
	defer rows.Close()

	backtests := make([]*backtest.Backtest, 0)
	for rows.Next() {
		var row rowData
		err = rows.Scan(&row.Name, &row.Benchmark, &row.Provider, &row.Profile, &row.CloseOnDrop, &row.HasUniverse, &row.Resolved, &row.StartDate, &row.EndDate, &row.Symbol, &row.Enabled)
		if err != nil {
			return nil, fmt.Errorf("failed to scan period: %w", err)
		}
		var b *backtest.Backtest
		for _, existing := range backtests {
			if existing.Name == row.Name {
				b = existing
				break
			}
		}
		if b == nil {
			resolved, err := decodeTickerResolutions(row.Resolved)
			if err != nil {
				return nil, fmt.Errorf("backtest %s: %w", row.Name, err)
			}
			b = &backtest.Backtest{
				Name:               row.Name,
				Benchmark:          row.Benchmark,
				Provider:           row.Provider,
				Profile:            row.Profile,
				HasUniverseMembers: row.HasUniverse,
				TickerResolutions:  resolved,
			}
			assignCloseOnUniverseDrop(b, row.CloseOnDrop)
			backtests = append(backtests, b)
		}
		if row.StartDate == nil || row.EndDate == nil {
			continue
		}
		var p *backtest.Backtest_Period
		startDate := commonPB.GoTimeToDate(*row.StartDate)
		endDate := commonPB.GoTimeToDate(*row.EndDate)
		for _, existing := range b.Periods {
			if existing.StartDate.Year == startDate.Year && existing.StartDate.Month == startDate.Month && existing.StartDate.Day == startDate.Day &&
				existing.EndDate.Year == endDate.Year && existing.EndDate.Month == endDate.Month && existing.EndDate.Day == endDate.Day {
				p = existing
				break
			}
		}
		if p == nil {
			p = &backtest.Backtest_Period{
				StartDate: startDate,
				EndDate:   endDate,
				Assets:    make([]*backtest.Backtest_Period_Asset, 0),
			}
			b.Periods = append(b.Periods, p)
		}
		if row.Symbol == nil {
			continue
		}
		p.Assets = append(p.Assets, &backtest.Backtest_Period_Asset{
			Symbol:  *row.Symbol,
			Enabled: *row.Enabled,
		})

	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read backtests: %w", err)
	}
	return backtests, nil
}

func (db *Backtest) Delete(ctx context.Context, name string) error {
	if authOn() {
		return fmt.Errorf("delete backtest by name is refused while AUTH_ENABLED=true")
	}
	result, err := db.Conn.Exec(ctx, `DELETE FROM backtest WHERE name=$1`, name)
	if err != nil {
		return fmt.Errorf("failed to delete backtest: %w", err)
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
