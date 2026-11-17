package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
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
	create_message_id text,
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

// userSentence is a sentence a person reads. errors.New rejects a final
// period; the text is that sentence, unchanged. The pointer is the
// errors.Is identity, as errors.New's pointer is.
type userSentence struct{ text string }

func (e *userSentence) Error() string { return e.text }

// ErrCloseOnUniverseDropFixed is returned when a create would change the
// option stored on an existing backtest.
var ErrCloseOnUniverseDropFixed error = &userSentence{text: usercopy.CloseOnUniverseDropFixed}

// ErrProviderMissing is returned when create is given no price provider.
// Its text is the user sentence. Yahoo is not stored in its place.
var ErrProviderMissing error = &userSentence{text: usercopy.BacktestProviderMissing}

// ErrBacktestNameTaken is the create that lost the name. Auth off, the name
// is the primary key. Auth on, the pair is (owner_id, name). The text is the
// duplicate wording a conflict already uses. The other create's row is not
// returned.
var ErrBacktestNameTaken = errors.New("already exists")

// backtestNamePrimaryKey is the name primary key while auth is off.
// backtestOwnerNameKey is (owner_id, name) once auth is on.
const (
	backtestNamePrimaryKey = "backtest_pkey"
	backtestOwnerNameKey   = "backtest_owner_id_name_key"
)

type createMessageKey struct{}

// WithCreateMessage marks the task that is inserting this backtest. The id
// the insert returns is stored against that task, so a caller loads and
// removes that row instead of whatever row currently has the name.
func WithCreateMessage(ctx context.Context, messageID string) context.Context {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return ctx
	}
	return context.WithValue(ctx, createMessageKey{}, messageID)
}

func createMessageID(ctx context.Context) string {
	id, _ := ctx.Value(createMessageKey{}).(string)
	return id
}

func createMessageArg(ctx context.Context) any {
	id := createMessageID(ctx)
	if id == "" {
		return nil
	}
	return id
}

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

// MigrateCreateMessageID adds the task id a create stores its row under.
// The index is unique for a non-empty task, so two creates cannot share it.
// Calling it twice is safe.
func MigrateCreateMessageID(ctx context.Context, conn postgres.Query) error {
	_, err := conn.Exec(ctx, `ALTER TABLE backtest ADD COLUMN IF NOT EXISTS create_message_id text`)
	if err != nil {
		return fmt.Errorf("backtest: create_message_id: %w", err)
	}
	_, err = conn.Exec(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS backtest_create_message_id_key
		ON backtest (create_message_id)
		WHERE create_message_id IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("backtest: create_message_id: %w", err)
	}
	return nil
}

func normalizeBacktestProvider(provider string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "":
		return "", ErrProviderMissing
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
// transaction. It returns the id the insert wrote. A failure rolls those
// rows back, so the name is not left taken. A name that is already stored
// is ErrBacktestNameTaken, and the stored row is not returned.
func (db *Backtest) Create(ctx context.Context, item *backtest.Backtest) (string, *backtest.Backtest, error) {
	return db.insertBacktest(ctx, backtestWrite{item: item})
}

// CreateWithMembers is Create plus the universe members, in that same
// transaction. An empty list still replaces membership, which stores none.
func (db *Backtest) CreateWithMembers(ctx context.Context, item *backtest.Backtest, members []MemberInterval) (string, *backtest.Backtest, error) {
	return db.insertBacktest(ctx, backtestWrite{item: item, members: members, writeMembers: true})
}

func (db *Backtest) insertBacktest(ctx context.Context, in backtestWrite) (string, *backtest.Backtest, error) {
	provider, err := normalizeBacktestProvider(in.item.GetProvider())
	if err != nil {
		return "", nil, err
	}
	on := closeOnUniverseDrop(in.item)
	assignCloseOnUniverseDrop(in.item, on)
	if err := refuseCloseOnDropChange(ctx, db.Conn, in, on); err != nil {
		return "", nil, err
	}
	tx, err := beginBacktestTx(ctx, db.Conn)
	if err != nil {
		return "", nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	id, err := insertBacktestRow(ctx, tx, in, provider)
	if err != nil {
		return "", nil, err
	}
	if err := insertBacktestPeriods(ctx, tx, in, id); err != nil {
		return "", nil, err
	}
	if in.writeMembers {
		if err := insertBacktestMembers(ctx, tx, in, id); err != nil {
			return "", nil, err
		}
	}
	stored, err := readCreatedBacktest(ctx, tx, in, id)
	if err != nil {
		return "", nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", nil, fmt.Errorf("failed to create backtest: %w", err)
	}
	return id, stored, nil
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
	messageID := createMessageArg(ctx)
	if in.ownerID != "" {
		var id string
		err := conn.QueryRow(ctx, `
			INSERT INTO backtest (name, benchmark, provider, profile, owner_id, auto_close_on_universe_drop, create_message_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id`,
			in.item.GetName(), in.item.GetBenchmark(), provider, in.item.GetProfile(), in.ownerID,
			closeOnUniverseDrop(in.item), messageID).Scan(&id)
		if err != nil {
			return "", insertBacktestError(err, true)
		}
		if strings.TrimSpace(id) == "" {
			return "", errors.New("backtest id is empty")
		}
		return id, nil
	}
	profile := strings.TrimSpace(in.item.GetProfile())
	var id string
	err := conn.QueryRow(ctx,
		`INSERT INTO backtest (name, benchmark, provider, profile, auto_close_on_universe_drop, create_message_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		in.item.Name, in.item.Benchmark, provider, profile, closeOnUniverseDrop(in.item), messageID).Scan(&id)
	if err != nil {
		return "", insertBacktestError(err, false)
	}
	if strings.TrimSpace(id) == "" {
		return "", errors.New("backtest id is empty")
	}
	return id, nil
}

// backtestNameTaken reports a unique violation on the name. Auth off, that
// is the primary key. Auth on, it is (owner_id, name). Another unique
// violation, such as a repeated period, stays a failed create.
func backtestNameTaken(err error, owned bool) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	if owned {
		return pgErr.ConstraintName == backtestOwnerNameKey
	}
	return pgErr.ConstraintName == backtestNamePrimaryKey
}

func insertBacktestError(err error, owned bool) error {
	if backtestNameTaken(err, owned) {
		return ErrBacktestNameTaken
	}
	if owned {
		return fmt.Errorf("create backtest: %w", err)
	}
	return fmt.Errorf("failed to create backtest: %w", err)
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

// IDByCreateMessage is the id the create task stored. It is not a lookup by
// name: a second create of that name has a different task, or none.
func (db *Backtest) IDByCreateMessage(ctx context.Context, messageID string) (string, error) {
	if strings.TrimSpace(messageID) == "" {
		return "", errors.New("create message id is empty")
	}
	var id string
	err := db.Conn.QueryRow(ctx, `SELECT id FROM backtest WHERE create_message_id = $1`, messageID).Scan(&id)
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
	return stored.GetBySurrogateID(ctx, id)
}

func (db *Backtest) Get(ctx context.Context, name string) (*backtest.Backtest, error) {
	if authOn() {
		return nil, fmt.Errorf("get backtest by name is refused while AUTH_ENABLED=true")
	}
	return db.getOne(ctx, "backtest.name = $1", name)
}

// GetBySurrogateID loads the backtest whose id the insert returned.
// It does not select by name.
func (db *Backtest) GetBySurrogateID(ctx context.Context, id string) (*backtest.Backtest, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("backtest id is empty")
	}
	return db.getOne(ctx, "backtest.id = $1", id)
}

func (db *Backtest) getOne(ctx context.Context, where string, arg any) (*backtest.Backtest, error) {
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
		WHERE `+where+`
		ORDER BY backtest_period.start_date, backtest_period_asset.symbol DESC
	`, arg)
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
