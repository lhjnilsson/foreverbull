package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

type BacktestTest struct {
	suite.Suite

	conn     *pgxpool.Pool
	uut      repository.Backtest
	backtest *backtest.Backtest
}

func (test *BacktestTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *BacktestTest) SetupTest() {
	var err error

	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
	test.uut = repository.Backtest{Conn: test.conn}
	test.backtest = &backtest.Backtest{
		Name:      "test_backtest",
		Benchmark: "benchmark",
	}
}

func (test *BacktestTest) TearDownTest() {
}

func TestBacktest(t *testing.T) {
	suite.Run(t, new(BacktestTest))
}

func (test *BacktestTest) TestCreateAndGet() {
	ctx := context.Background()

	test.Run("not found", func() {
		not_found := proto.Clone(test.backtest).(*backtest.Backtest)
		not_found.Name = "not_found"
		backtests := &repository.Backtest{Conn: test.conn}
		backtest, err := backtests.Get(ctx, not_found.Name)
		test.Require().Error(err)
		test.Nil(backtest)
		test.ErrorContains(err, pgx.ErrNoRows.Error())
	})
	test.Run("no periods", func() {
		backtests := &repository.Backtest{Conn: test.conn}
		no_periods := proto.Clone(test.backtest).(*backtest.Backtest)
		no_periods.Name = "no_periods"
		_, err := backtests.Create(ctx, no_periods)
		test.Require().NoError(err)

		stored, err := backtests.Get(ctx, no_periods.Name)
		test.Require().NoError(err)
		test.Equal(no_periods.Name, stored.Name)
		test.Equal(no_periods.Benchmark, stored.Benchmark)
		test.Equal("yahoo", stored.Provider)
		test.Empty(stored.Periods)
	})
	test.Run("with periods", func() {
		backtests := &repository.Backtest{Conn: test.conn}
		with_periods := proto.Clone(test.backtest).(*backtest.Backtest)
		with_periods.Name = "with_periods"
		with_periods.Periods = []*backtest.Backtest_Period{
			{
				StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
			},
			{
				StartDate: &pb.Date{Year: 2025, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2025, Month: 12, Day: 31},
			},
		}

		_, err := backtests.Create(ctx, with_periods)
		test.Require().NoError(err)

		stored, err := backtests.Get(ctx, with_periods.Name)
		test.Require().NoError(err)
		test.Equal(with_periods.Name, stored.Name)
		test.Equal(with_periods.Benchmark, stored.Benchmark)
		test.Len(with_periods.Periods, 2)
	})
	test.Run("with periods and assets", func() {
		backtests := &repository.Backtest{Conn: test.conn}
		with_periods_and_assets := proto.Clone(test.backtest).(*backtest.Backtest)
		with_periods_and_assets.Name = "with_periods_and_assets"
		with_periods_and_assets.Periods = []*backtest.Backtest_Period{
			{
				StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
				Assets: []*backtest.Backtest_Period_Asset{
					{
						Symbol:  "AAPL",
						Enabled: true,
					},
				},
			},
			{
				StartDate: &pb.Date{Year: 2025, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2025, Month: 12, Day: 31},
				Assets: []*backtest.Backtest_Period_Asset{
					{
						Symbol:  "AAPL",
						Enabled: false,
					},
				},
			},
		}
		_, err := backtests.Create(ctx, with_periods_and_assets)
		test.Require().NoError(err)

		stored, err := backtests.Get(ctx, with_periods_and_assets.Name)
		test.Require().NoError(err)
		test.Equal(with_periods_and_assets.Name, stored.Name)
		test.Equal(with_periods_and_assets.Benchmark, stored.Benchmark)
		test.Len(stored.Periods, 2)
		test.Equal(with_periods_and_assets.Periods[0].StartDate, stored.Periods[0].StartDate)
		test.Equal(with_periods_and_assets.Periods[0].EndDate, stored.Periods[0].EndDate)
		test.Len(stored.Periods[0].Assets, 1)
		test.Equal(with_periods_and_assets.Periods[0].Assets[0].Symbol, stored.Periods[0].Assets[0].Symbol)
		test.Equal(with_periods_and_assets.Periods[0].Assets[0].Enabled, stored.Periods[0].Assets[0].Enabled)
		test.Equal(with_periods_and_assets.Periods[1].StartDate, stored.Periods[1].StartDate)
		test.Equal(with_periods_and_assets.Periods[1].EndDate, stored.Periods[1].EndDate)
		test.Len(stored.Periods[1].Assets, 1)
		test.Equal(with_periods_and_assets.Periods[1].Assets[0].Symbol, stored.Periods[1].Assets[0].Symbol)
		test.Equal(with_periods_and_assets.Periods[1].Assets[0].Enabled, stored.Periods[1].Assets[0].Enabled)
	})
}

func (test *BacktestTest) TestUpdateBenchmark() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, test.backtest)
	test.Require().NoError(err)

	_, err = backtests.UpdateBenchmark(ctx, test.backtest.Name, "new_benchmark")
	test.Require().NoError(err)

	backtest, err := backtests.Get(ctx, test.backtest.Name)
	test.Require().NoError(err)
	test.Equal(test.backtest.Name, backtest.Name)
	test.Equal("new_benchmark", backtest.Benchmark)
}

func (test *BacktestTest) TestUpdatePeriod() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	test.backtest.Periods = []*backtest.Backtest_Period{
		{
			StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
		},
	}

	_, err := backtests.Create(ctx, test.backtest)
	test.Require().NoError(err)
	period := &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
		Assets: []*backtest.Backtest_Period_Asset{
			{
				Symbol:  "AAPL",
				Enabled: true,
			},
		},
	}
	err = backtests.UpdatePeriod(ctx, test.backtest.Name, period)
	test.Require().NoError(err)

	backtest, err := backtests.Get(ctx, test.backtest.Name)
	test.Require().NoError(err)
	test.Equal(test.backtest.Name, backtest.Name)
	test.Equal(period.StartDate, backtest.Periods[0].StartDate)
	test.Equal(period.EndDate, backtest.Periods[0].EndDate)
	test.Equal(period.Assets[0].Symbol, backtest.Periods[0].Assets[0].Symbol)
}

func (test *BacktestTest) TestUpdatePeriodAsset() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	test.backtest.Periods = []*backtest.Backtest_Period{
		{
			StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
			Assets: []*backtest.Backtest_Period_Asset{
				{
					Symbol:  "AAPL",
					Enabled: true,
				},
			},
		},
	}

	_, err := backtests.Create(ctx, test.backtest)
	test.Require().NoError(err)

	period := &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
	}
	err = backtests.UpdatePeriodAsset(ctx, test.backtest.Name, "AAPL", false)
	test.Require().NoError(err)

	backtest, err := backtests.Get(ctx, test.backtest.Name)
	test.Require().NoError(err)
	test.Equal(test.backtest.Name, backtest.Name)
	test.Equal(period.StartDate, backtest.Periods[0].StartDate)
	test.Equal(period.EndDate, backtest.Periods[0].EndDate)
	test.Equal("AAPL", backtest.Periods[0].Assets[0].Symbol)
	test.False(backtest.Periods[0].Assets[0].Enabled)
}

func (test *BacktestTest) TestList() {
	ctx := context.Background()
	backtests := &repository.Backtest{Conn: test.conn}

	b1 := proto.Clone(test.backtest).(*backtest.Backtest)
	b1.Name = "test_backtest_1"
	b1.Benchmark = "1"
	b1.Periods = []*backtest.Backtest_Period{
		{
			StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
			Assets: []*backtest.Backtest_Period_Asset{
				{
					Symbol:  "AAPL",
					Enabled: true,
				},
			},
		},
	}
	b2 := proto.Clone(test.backtest).(*backtest.Backtest)
	b2.Name = "test_backtest_2"
	b3 := proto.Clone(test.backtest).(*backtest.Backtest)
	b3.Name = "test_backtest_3"
	b3.Benchmark = "3"
	b3.Periods = []*backtest.Backtest_Period{
		{
			StartDate: &pb.Date{Year: 2025, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2025, Month: 12, Day: 31},
			Assets: []*backtest.Backtest_Period_Asset{
				{
					Symbol:  "TSLA",
					Enabled: true,
				},
				{
					Symbol:  "MSFT",
					Enabled: false,
				},
			},
		},
		{
			StartDate: &pb.Date{Year: 2026, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2026, Month: 12, Day: 31},
			Assets: []*backtest.Backtest_Period_Asset{
				{
					Symbol:  "MSFT",
					Enabled: true,
				},
			},
		},
	}

	for _, b := range []*backtest.Backtest{b1, b2, b3} {
		_, err := backtests.Create(ctx, b)
		test.Require().NoError(err)
	}

	result, err := backtests.List(ctx)
	test.Require().NoError(err)
	test.Len(result, 3)

	for _, b := range result {
		if b.Name == "test_backtest_1" {
			test.Len(b.Periods, 1)
			test.Len(b.Periods[0].Assets, 1)
			test.Equal("AAPL", b.Periods[0].Assets[0].Symbol)
			test.True(b.Periods[0].Assets[0].Enabled)
		}
		if b.Name == "test_backtest_2" {
			test.Empty(b.Periods)
		}
		if b.Name == "test_backtest_3" {
			test.Len(b.Periods, 2)
			test.Len(b.Periods[0].Assets, 2)
			test.Equal("TSLA", b.Periods[0].Assets[0].Symbol)
			test.True(b.Periods[0].Assets[0].Enabled)
			test.Equal("MSFT", b.Periods[0].Assets[1].Symbol)
			test.False(b.Periods[0].Assets[1].Enabled)
			test.Len(b.Periods[1].Assets, 1)
			test.Equal("MSFT", b.Periods[1].Assets[0].Symbol)
			test.True(b.Periods[1].Assets[0].Enabled)
		}
	}
}

func (test *BacktestTest) TestProviderCheckRejectsNasdaq() {
	ctx := context.Background()
	test.Require().NoError(repository.MigrateBacktestProvider(ctx, test.conn))
	test.Require().NoError(repository.MigrateBacktestProvider(ctx, test.conn))

	_, err := test.conn.Exec(ctx, `
		INSERT INTO backtest (name, benchmark, provider) VALUES ('yahoo-bt', 'QQQ', 'yahoo')`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO backtest (name, benchmark, provider) VALUES ('stooq-bt', 'QQQ', 'stooq')`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO backtest (name, benchmark, provider) VALUES ('alpaca-bt', 'QQQ', 'alpaca')`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO backtest (name, benchmark, provider) VALUES ('nasdaq-bt', 'QQQ', 'nasdaq')`)
	test.Require().ErrorContains(err, "backtest_provider_check")
}

func (test *BacktestTest) TestMigrateBacktestProviderLeavesInvalidRows() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `ALTER TABLE backtest DROP CONSTRAINT backtest_provider_check`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO backtest (name, benchmark, provider) VALUES ('nasdaq-bt', 'QQQ', 'nasdaq')`)
	test.Require().NoError(err)

	err = repository.MigrateBacktestProvider(ctx, test.conn)
	test.Require().ErrorContains(err, "backtest")

	var provider string
	err = test.conn.QueryRow(ctx, `SELECT provider FROM backtest WHERE name='nasdaq-bt'`).Scan(&provider)
	test.Require().NoError(err)
	test.Equal("nasdaq", provider)
}

// A row that omits provider must fail loudly rather than be stored as yahoo.
func (test *BacktestTest) TestInsertWithoutProviderFails() {
	ctx := context.Background()
	test.Require().NoError(repository.MigrateBacktestProvider(ctx, test.conn))
	test.Require().NoError(repository.MigrateBacktestProvider(ctx, test.conn))

	_, err := test.conn.Exec(ctx, `
		INSERT INTO backtest (name, benchmark) VALUES ('no-provider-bt', 'QQQ')`)
	test.Require().ErrorContains(err, "provider")

	var count int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM backtest WHERE name='no-provider-bt'`).Scan(&count)
	test.Require().NoError(err)
	test.Equal(0, count)
}

// A period asset that fails after the backtest row is inserted rolls the
// create back. The same name can be created again.
func (test *BacktestTest) TestCreateRollsBackAFailedPeriodAsset() {
	ctx := context.Background()
	backtests := &repository.Backtest{Conn: test.conn}
	partial := &backtest.Backtest{
		Name:      "partial-asset",
		Benchmark: "SPY",
		Periods: []*backtest.Backtest_Period{{
			StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
			Assets: []*backtest.Backtest_Period_Asset{
				{Symbol: "AAPL", Enabled: true},
				{Symbol: "AAPL", Enabled: false},
			},
		}},
	}
	_, err := backtests.Create(ctx, partial)
	test.Require().Error(err)
	test.ErrorContains(err, "period asset")

	_, err = backtests.Get(ctx, partial.Name)
	test.Require().ErrorIs(err, pgx.ErrNoRows)
	test.Equal(0, test.countRows(`SELECT COUNT(*) FROM backtest WHERE name=$1`, partial.Name))
	test.Equal(0, test.countRows(`SELECT COUNT(*) FROM backtest_period WHERE backtest_name=$1`, partial.Name))
	test.Equal(0, test.countRows(`SELECT COUNT(*) FROM backtest_period_asset WHERE backtest_name=$1`, partial.Name))

	partial.Periods[0].Assets = []*backtest.Backtest_Period_Asset{{Symbol: "AAPL", Enabled: true}}
	stored, err := backtests.Create(ctx, partial)
	test.Require().NoError(err)
	test.Equal(partial.Name, stored.Name)
	test.Equal("yahoo", stored.Provider)
	test.Require().Len(stored.Periods, 1)
	test.Require().Len(stored.Periods[0].Assets, 1)
	test.Equal("AAPL", stored.Periods[0].Assets[0].Symbol)
	test.True(stored.Periods[0].Assets[0].Enabled)
}

func (test *BacktestTest) countRows(query string, args ...any) int {
	test.T().Helper()
	var n int
	test.Require().NoError(test.conn.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

func (test *BacktestTest) TestDelete() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, test.backtest)
	test.Require().NoError(err)

	err = backtests.Delete(ctx, test.backtest.Name)
	test.Require().NoError(err)

	storedBacktests, err := backtests.List(ctx)
	test.Require().NoError(err)
	test.Empty(storedBacktests)
}

// Auth-on create uses backtest id. A member write that fails after the row
// is inserted rolls the backtest back, and the same name can be created again.
func TestCreateOwnedWithMembersRollsBackAFailedMember(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	pool := newBacktestPool(t)
	resetBacktests(t, pool)
	ctx := context.Background()
	admin := insertBacktestAccount(t, pool, auth.RoleAdmin)
	require.NoError(t, repository.MigrateBacktestOwner(ctx, pool, admin, &memIngest{keys: map[string]bool{}}))
	require.NoError(t, repository.CreateTables(ctx, pool))

	instruments := repository.Instrument{Conn: pool}
	require.NoError(t, instruments.Upsert(ctx, aaplISIN, "Apple"))

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	item := &backtest.Backtest{
		Name:      "partial-owned",
		Benchmark: "OEF",
		Periods: []*backtest.Backtest_Period{{
			StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
			Assets:    []*backtest.Backtest_Period_Asset{{Symbol: "AAPL", Enabled: true}},
		}},
	}
	overlap := []repository.MemberInterval{
		{ISIN: aaplISIN, FromDate: from},
		{ISIN: aaplISIN, FromDate: later},
	}
	backtests := repository.Backtest{Conn: pool}
	_, err := backtests.CreateOwnedWithMembers(ctx, admin, item, overlap)
	require.Error(t, err)
	require.ErrorContains(t, err, "universe_member")
	require.Equal(t, 0, countBacktestRows(t, pool, `SELECT COUNT(*) FROM backtest WHERE name=$1`, item.Name))
	require.Equal(t, 0, countBacktestRows(t, pool, `
		SELECT COUNT(*) FROM backtest_period AS p
		JOIN backtest AS b ON b.id = p.backtest_id
		WHERE b.name=$1`, item.Name))
	require.Equal(t, 0, countBacktestRows(t, pool, `
		SELECT COUNT(*) FROM universe_member AS m
		JOIN backtest AS b ON b.id = m.backtest_id
		WHERE b.name=$1`, item.Name))

	stored, err := backtests.CreateOwnedWithMembers(ctx, admin, item, overlap[:1])
	require.NoError(t, err)
	require.Equal(t, item.Name, stored.Name)
	require.Len(t, stored.Periods, 1)
	require.Len(t, stored.Periods[0].Assets, 1)
	require.Equal(t, "AAPL", stored.Periods[0].Assets[0].Symbol)
	require.True(t, stored.Periods[0].Assets[0].Enabled)

	id, err := backtests.IDByOwnerAndName(ctx, admin, item.Name)
	require.NoError(t, err)
	listed, err := (&repository.UniverseMember{Conn: pool}).ListByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, []repository.MemberInterval{{ISIN: aaplISIN, FromDate: from}}, listed)
}
