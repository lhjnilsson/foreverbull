package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/universe"
	financebars "github.com/lhjnilsson/foreverbull/pkg/finance/bars"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/suite"
)

const (
	aaplISIN = "US0378331005"
	metaISIN = "US30303M1027"
	msftISIN = "US5949181045"
)

type UniverseMemberTest struct {
	suite.Suite

	conn      *pgxpool.Pool
	backtests repository.Backtest
	members   repository.UniverseMember
}

func (test *UniverseMemberTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *UniverseMemberTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
	test.backtests = repository.Backtest{Conn: test.conn}
	test.members = repository.UniverseMember{Conn: test.conn}
	instruments := repository.Instrument{Conn: test.conn}
	for _, isin := range []string{aaplISIN, metaISIN, msftISIN} {
		test.Require().NoError(instruments.Upsert(context.Background(), isin, isin))
	}
}

func (test *UniverseMemberTest) TearDownTest() {
	test.conn.Close()
}

func TestUniverseMember(t *testing.T) {
	suite.Run(t, new(UniverseMemberTest))
}

func (test *UniverseMemberTest) date(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	test.Require().NoError(err)
	return d
}

func (test *UniverseMemberTest) toPtr(s string) *time.Time {
	d := test.date(s)
	return &d
}

// Overlapping membership fails after the backtest row is inserted. The
// transaction rolls the row back, so the same name can be created again.
func (test *UniverseMemberTest) TestCreateWithMembersRollsBackAFailedMember() {
	ctx := context.Background()
	from := test.date("2024-01-01")
	later := test.date("2024-06-01")
	item := &backtest.Backtest{
		Name:      "partial-members",
		Benchmark: "OEF",
		Provider:  "yahoo",
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
	_, _, err := test.backtests.CreateWithMembers(ctx, item, overlap)
	test.Require().Error(err)
	test.Require().ErrorContains(err, "universe_member")

	_, err = test.backtests.Get(ctx, item.Name)
	test.Require().ErrorIs(err, pgx.ErrNoRows)
	test.Equal(0, test.countRows(`SELECT COUNT(*) FROM backtest WHERE name=$1`, item.Name))
	test.Equal(0, test.countRows(`SELECT COUNT(*) FROM backtest_period WHERE backtest_name=$1`, item.Name))
	test.Equal(0, test.countRows(`SELECT COUNT(*) FROM universe_member WHERE backtest_name=$1`, item.Name))

	_, stored, err := test.backtests.CreateWithMembers(ctx, item, overlap[:1])
	test.Require().NoError(err)
	test.Equal(item.Name, stored.Name)
	test.Require().Len(stored.Periods, 1)
	test.Require().Len(stored.Periods[0].Assets, 1)
	test.Equal("AAPL", stored.Periods[0].Assets[0].Symbol)
	test.True(stored.Periods[0].Assets[0].Enabled)

	listed, err := test.members.List(ctx, item.Name)
	test.Require().NoError(err)
	test.Equal([]repository.MemberInterval{{ISIN: aaplISIN, FromDate: from}}, listed)
}

func (test *UniverseMemberTest) countRows(query string, args ...any) int {
	test.T().Helper()
	var n int
	test.Require().NoError(test.conn.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

func (test *UniverseMemberTest) createBacktest(name string) {
	_, _, err := test.backtests.Create(context.Background(), &backtest.Backtest{
		Name:      name,
		Benchmark: "OEF",
		Provider:  "yahoo",
	})
	test.Require().NoError(err)
}

func intervalsFrom(result universe.Result) []repository.MemberInterval {
	out := make([]repository.MemberInterval, 0, len(result.Members))
	for _, m := range result.Members {
		out = append(out, repository.MemberInterval{
			ISIN:     m.ISIN,
			FromDate: m.FromDate,
			ToDate:   m.ToDate,
		})
	}
	return out
}

func (test *UniverseMemberTest) TestReplaceListAndAtFromDiffer() {
	ctx := context.Background()
	test.createBacktest("sp100")

	result := universe.Diff([]universe.Snapshot{
		{
			AsOf: test.date("2022-12-31"),
			Holdings: []universe.Holding{
				{ISIN: aaplISIN, Symbol: "AAPL", AssetType: "Equity"},
				{ISIN: metaISIN, Symbol: "FB", AssetType: "Equity"},
			},
		},
		{
			AsOf: test.date("2023-01-31"),
			Holdings: []universe.Holding{
				{ISIN: aaplISIN, Symbol: "AAPL", AssetType: "Equity"},
				{ISIN: metaISIN, Symbol: "META", AssetType: "Equity"},
				{ISIN: msftISIN, Symbol: "MSFT", AssetType: "Equity"},
			},
		},
		{
			AsOf: test.date("2023-02-28"),
			Holdings: []universe.Holding{
				{ISIN: metaISIN, Symbol: "META", AssetType: "Equity"},
				{ISIN: msftISIN, Symbol: "MSFT", AssetType: "Equity"},
			},
		},
	})

	err := test.members.Replace(ctx, "sp100", intervalsFrom(result))
	test.Require().NoError(err)

	listed, err := test.members.List(ctx, "sp100")
	test.Require().NoError(err)
	test.Equal([]repository.MemberInterval{
		{ISIN: aaplISIN, FromDate: test.date("2022-12-31"), ToDate: test.toPtr("2023-02-28")},
		{ISIN: metaISIN, FromDate: test.date("2022-12-31"), ToDate: nil},
		{ISIN: msftISIN, FromDate: test.date("2023-01-31"), ToDate: nil},
	}, listed)

	at, err := test.members.At(ctx, "sp100", test.date("2022-12-31"))
	test.Require().NoError(err)
	test.Equal([]string{aaplISIN, metaISIN}, at)

	at, err = test.members.At(ctx, "sp100", test.date("2023-01-31"))
	test.Require().NoError(err)
	test.Equal([]string{aaplISIN, metaISIN, msftISIN}, at)

	at, err = test.members.At(ctx, "sp100", test.date("2023-02-28"))
	test.Require().NoError(err)
	test.Equal([]string{metaISIN, msftISIN}, at)

	at, err = test.members.At(ctx, "sp100", test.date("2022-12-30"))
	test.Require().NoError(err)
	test.Empty(at)
}

func (test *UniverseMemberTest) TestSymbolsAtMapsTickerAtT() {
	ctx := context.Background()
	test.createBacktest("sp100")

	result := universe.Diff([]universe.Snapshot{
		{
			AsOf: test.date("2022-12-31"),
			Holdings: []universe.Holding{
				{ISIN: aaplISIN, Symbol: "AAPL", AssetType: "Equity"},
				{ISIN: metaISIN, Symbol: "FB", AssetType: "Equity"},
			},
		},
		{
			AsOf: test.date("2023-01-31"),
			Holdings: []universe.Holding{
				{ISIN: aaplISIN, Symbol: "AAPL", AssetType: "Equity"},
				{ISIN: metaISIN, Symbol: "META", AssetType: "Equity"},
				{ISIN: msftISIN, Symbol: "MSFT", AssetType: "Equity"},
			},
		},
		{
			AsOf: test.date("2023-02-28"),
			Holdings: []universe.Holding{
				{ISIN: metaISIN, Symbol: "META", AssetType: "Equity"},
				{ISIN: msftISIN, Symbol: "MSFT", AssetType: "Equity"},
			},
		},
	})

	err := test.members.Replace(ctx, "sp100", intervalsFrom(result))
	test.Require().NoError(err)

	instruments := repository.Instrument{Conn: test.conn}
	for _, alias := range result.Aliases {
		test.Require().NoError(instruments.Upsert(ctx, alias.ISIN, alias.Symbol))
		test.Require().NoError(instruments.StoreAlias(ctx, repository.AliasInterval{
			ISIN:     alias.ISIN,
			Symbol:   alias.Symbol,
			FromDate: alias.FromDate,
			ToDate:   alias.ToDate,
		}))
	}

	symbols, err := test.members.SymbolsAt(ctx, "sp100", test.date("2022-12-31"))
	test.Require().NoError(err)
	test.Equal([]string{"AAPL", "FB"}, symbols)

	symbols, err = test.members.SymbolsAt(ctx, "sp100", test.date("2023-01-31"))
	test.Require().NoError(err)
	test.Equal([]string{"AAPL", "META", "MSFT"}, symbols)

	symbols, err = test.members.SymbolsAt(ctx, "sp100", test.date("2023-02-28"))
	test.Require().NoError(err)
	test.Equal([]string{"META", "MSFT"}, symbols)

	symbols, err = test.members.SymbolsAt(ctx, "sp100", test.date("2022-12-30"))
	test.Require().NoError(err)
	test.Empty(symbols)
}

func (test *UniverseMemberTest) TestOverlapRejectedInsideOneBacktest() {
	ctx := context.Background()
	test.createBacktest("sp100")
	test.createBacktest("sp500")
	test.Require().NoError(repository.MigrateUniverseMember(ctx, test.conn))
	test.Require().NoError(repository.MigrateUniverseMember(ctx, test.conn))

	_, err := test.conn.Exec(ctx, `
		INSERT INTO universe_member (backtest_name, isin, from_date, to_date)
		VALUES ('sp100', $1, '2022-01-01', '2022-06-01')`, aaplISIN)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO universe_member (backtest_name, isin, from_date, to_date)
		VALUES ('sp100', $1, '2022-03-01', NULL)`, aaplISIN)
	test.Require().ErrorContains(err, "universe_member_no_overlap")
	_, err = test.conn.Exec(ctx, `
		INSERT INTO universe_member (backtest_name, isin, from_date, to_date)
		VALUES ('sp100', $1, '2022-06-01', NULL)`, aaplISIN)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO universe_member (backtest_name, isin, from_date, to_date)
		VALUES ('sp500', $1, '2022-01-01', '2022-06-01')`, aaplISIN)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO universe_member (backtest_name, isin, from_date, to_date)
		VALUES ('sp500', $1, '2022-06-01', NULL)`, aaplISIN)
	test.Require().NoError(err)
}

func (test *UniverseMemberTest) TestMigrateUniverseMemberLeavesOverlappingRows() {
	ctx := context.Background()
	test.createBacktest("sp100")
	_, err := test.conn.Exec(ctx, `DROP TABLE universe_member`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE universe_member (
			backtest_name text REFERENCES backtest (name) ON DELETE CASCADE,
			isin text NOT NULL,
			from_date date NOT NULL,
			to_date date,
			PRIMARY KEY (backtest_name, isin, from_date)
		)`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO universe_member (backtest_name, isin, from_date, to_date) VALUES
		('sp100', $1, '2022-01-01', '2022-06-01'),
		('sp100', $1, '2022-03-01', NULL)`, aaplISIN)
	test.Require().NoError(err)

	err = repository.MigrateUniverseMember(ctx, test.conn)
	test.Require().ErrorContains(err, "universe_member")

	var n int64
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM universe_member`).Scan(&n)
	test.Require().NoError(err)
	test.Equal(int64(2), n)
}

func (test *UniverseMemberTest) TestReplaceClearsPreviousIntervals() {
	ctx := context.Background()
	test.createBacktest("sp100")

	err := test.members.Replace(ctx, "sp100", []repository.MemberInterval{
		{ISIN: aaplISIN, FromDate: test.date("2022-12-31"), ToDate: nil},
	})
	test.Require().NoError(err)

	err = test.members.Replace(ctx, "sp100", []repository.MemberInterval{
		{ISIN: msftISIN, FromDate: test.date("2023-01-31"), ToDate: nil},
	})
	test.Require().NoError(err)

	listed, err := test.members.List(ctx, "sp100")
	test.Require().NoError(err)
	test.Equal([]repository.MemberInterval{
		{ISIN: msftISIN, FromDate: test.date("2023-01-31"), ToDate: nil},
	}, listed)
}

func (test *UniverseMemberTest) TestReplaceRejectsMissingISIN() {
	ctx := context.Background()
	test.createBacktest("sp100")

	err := test.members.Replace(ctx, "sp100", []repository.MemberInterval{
		{ISIN: "  ", FromDate: test.date("2022-12-31")},
	})
	test.Require().ErrorContains(err, "missing ISIN")
}

func (test *UniverseMemberTest) TestDeleteBacktestCascadesMembers() {
	ctx := context.Background()
	test.createBacktest("sp100")

	err := test.members.Replace(ctx, "sp100", []repository.MemberInterval{
		{ISIN: aaplISIN, FromDate: test.date("2022-12-31"), ToDate: nil},
	})
	test.Require().NoError(err)

	test.Require().NoError(test.backtests.Delete(ctx, "sp100"))

	listed, err := test.members.List(ctx, "sp100")
	test.Require().NoError(err)
	test.Empty(listed)
}

// financeBars creates the finance asset and ohlc tables the broker shares
// with the backtest domain (same columns as pkg/finance) and stores one bar
// per given UTC time for symbol under provider.
// resetFinanceBars drops the bars an earlier test stored; Recreate only
// rebuilds the backtest tables.
func (test *UniverseMemberTest) resetFinanceBars() {
	_, err := test.conn.Exec(context.Background(), `DROP VIEW IF EXISTS instrument_ohlc; DROP TABLE IF EXISTS ohlc; DROP TABLE IF EXISTS asset`)
	test.Require().NoError(err)
}

func (test *UniverseMemberTest) financeBars(provider, symbol string, times ...time.Time) {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS asset (symbol text primary key, name text);
		CREATE TABLE IF NOT EXISTS ohlc (
			symbol TEXT references asset(symbol),
			open numeric NOT NULL, high numeric NOT NULL, low numeric NOT NULL, close numeric NOT NULL,
			volume bigint NOT NULL,
			time timestamp,
			provider text NOT NULL,
			UNIQUE (provider, symbol, time));`)
	test.Require().NoError(err)
	test.Require().NoError(repository.MigrateProviderSymbol(ctx, test.conn))
	_, err = test.conn.Exec(ctx, `INSERT INTO asset (symbol, name) VALUES ($1, $1) ON CONFLICT DO NOTHING`, symbol)
	test.Require().NoError(err)
	for _, at := range times {
		_, err = test.conn.Exec(ctx, `
			INSERT INTO ohlc (symbol, open, high, low, close, volume, time, provider)
			VALUES ($1, 10, 11, 9, 10, 100, $2, $3) ON CONFLICT DO NOTHING`, symbol, at, provider)
		test.Require().NoError(err)
	}
}

// A member delisted mid-month keeps its membership until the next list drops
// it. LastBars gives each of its ticker intervals the ISIN's last bar, so the
// session can leave it out after that day. A renamed ticker reads the bars
// its ISIN has under the other ticker.
func (test *UniverseMemberTest) TestLastBarsGivesEachAliasItsISINsLastBar() {
	ctx := context.Background()
	test.resetFinanceBars()
	test.createBacktest("sp100")
	test.Require().NoError(test.members.Replace(ctx, "sp100", []repository.MemberInterval{
		{ISIN: aaplISIN, FromDate: test.date("2026-01-01")},
		{ISIN: metaISIN, FromDate: test.date("2026-01-01")},
		{ISIN: msftISIN, FromDate: test.date("2026-01-01"), ToDate: test.toPtr("2026-04-30")},
	}))
	instruments := repository.Instrument{Conn: test.conn}
	for _, alias := range []repository.AliasInterval{
		{ISIN: aaplISIN, Symbol: "AAPL", FromDate: test.date("2026-01-01")},
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2026-01-01"), ToDate: test.toPtr("2026-02-01")},
		{ISIN: metaISIN, Symbol: "META", FromDate: test.date("2026-02-01")},
		{ISIN: msftISIN, Symbol: "MSFT", FromDate: test.date("2026-01-01")},
	} {
		test.Require().NoError(instruments.StoreAlias(ctx, alias))
	}

	open := func(s string) time.Time { return test.date(s).Add(13*time.Hour + 30*time.Minute) }
	_, err := test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, evidence) VALUES
		($1, 'yahoo', 'AAPL', '2026-01-01', 'migration'),
		($2, 'yahoo', 'META', '2026-01-01', 'migration'),
		($3, 'yahoo', 'MSFT', '2026-01-01', 'migration')`, aaplISIN, metaISIN, msftISIN)
	test.Require().NoError(err)
	test.financeBars("yahoo", "AAPL", open("2026-03-30"), open("2026-03-31"), open("2026-04-30"))
	// Yahoo stores the whole history of the renamed ISIN under META.
	test.financeBars("yahoo", "META", open("2026-01-05"), open("2026-04-29"))
	// MSFT was delisted on 2026-03-31; the 2026-04-30 list drops it.
	test.financeBars("yahoo", "MSFT", open("2026-03-30"), open("2026-03-31"))
	// Another provider and bars past the backtest end are not this bundle's.
	test.financeBars("stooq", "MSFT", open("2026-04-15"))
	test.financeBars("yahoo", "AAPL", open("2026-05-04"))

	aliases, err := test.members.LastBars(ctx, "sp100", "yahoo", test.date("2026-05-01"))
	test.Require().NoError(err)

	got := make(map[string]string, len(aliases))
	for _, alias := range aliases {
		test.Require().NotNil(alias.LastBar, alias.Symbol)
		got[alias.Symbol] = alias.LastBar.UTC().Format(time.DateOnly)
	}
	test.Equal(map[string]string{
		"AAPL": "2026-04-30",
		"FB":   "2026-04-29",
		"META": "2026-04-29",
		"MSFT": "2026-03-31",
	}, got)
	for _, alias := range aliases {
		if alias.Symbol == "FB" {
			test.Equal(test.date("2026-01-01"), alias.FromDate)
			test.Require().NotNil(alias.ToDate)
			test.Equal(test.date("2026-02-01"), *alias.ToDate)
		}
	}
}

// A symbol list has no universe_member rows. PeriodLastBars still returns
// each enabled asset's last bar for this provider, before the window, and
// an asset with an ISIN shares the bars stored under its other tickers.
// A disabled asset and the benchmark are not part of the run.
func (test *UniverseMemberTest) TestPeriodLastBarsReadsASymbolListByISINAndOwnBars() {
	ctx := context.Background()
	test.resetFinanceBars()
	_, _, err := test.backtests.Create(ctx, &backtest.Backtest{
		Name: "list", Benchmark: "SPY", Provider: "yahoo",
		Periods: []*backtest.Backtest_Period{{
			StartDate: &pb.Date{Year: 2026, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2026, Month: 4, Day: 30},
			Assets: []*backtest.Backtest_Period_Asset{
				{Symbol: "AAPL", Enabled: true},
				{Symbol: "ABC", Enabled: true},
				{Symbol: "FB", Enabled: true},
				{Symbol: "NOBARS", Enabled: true},
				{Symbol: "SKIP", Enabled: false},
			},
		}},
	})
	test.Require().NoError(err)
	_, _, err = test.backtests.Create(ctx, &backtest.Backtest{
		Name: "other", Benchmark: "SPY", Provider: "yahoo",
		Periods: []*backtest.Backtest_Period{{
			StartDate: &pb.Date{Year: 2026, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2026, Month: 4, Day: 30},
			Assets:    []*backtest.Backtest_Period_Asset{{Symbol: "OTHER", Enabled: true}},
		}},
	})
	test.Require().NoError(err)

	instruments := repository.Instrument{Conn: test.conn}
	test.Require().NoError(instruments.StoreAlias(ctx, repository.AliasInterval{
		ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2026-01-01"), ToDate: test.toPtr("2026-05-01"),
	}))
	test.Require().NoError(instruments.StoreAlias(ctx, repository.AliasInterval{
		ISIN: metaISIN, Symbol: "META", FromDate: test.date("2026-05-01"),
	}))

	open := func(s string) time.Time { return test.date(s).Add(13*time.Hour + 30*time.Minute) }
	test.financeBars("yahoo", "AAPL", open("2026-04-30"), open("2026-05-04"))
	test.financeBars("yahoo", "ABC", open("2026-03-31"))
	test.financeBars("stooq", "ABC", open("2026-04-20"))
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, evidence)
		VALUES ($1, 'yahoo', 'META', '2026-01-01', 'migration')`, metaISIN)
	test.Require().NoError(err)
	// Yahoo stores the renamed ISIN under META. FB itself has no bars.
	test.financeBars("yahoo", "META", open("2026-01-05"), open("2026-04-29"))
	test.financeBars("yahoo", "SKIP", open("2026-04-30"))
	test.financeBars("yahoo", "SPY", open("2026-06-01"))
	test.financeBars("yahoo", "OTHER", open("2026-04-30"))

	aliases, err := test.members.PeriodLastBars(ctx, "list", "yahoo", test.date("2026-05-01"))
	test.Require().NoError(err)

	got := make(map[string]string, len(aliases))
	counts := make(map[string]int, len(aliases))
	for _, alias := range aliases {
		counts[alias.Symbol]++
		if alias.LastBar == nil {
			got[alias.Symbol] = ""
			continue
		}
		got[alias.Symbol] = alias.LastBar.UTC().Format(time.DateOnly)
	}
	test.Equal(map[string]string{
		"AAPL":   "2026-04-30",
		"ABC":    "2026-03-31",
		"FB":     "2026-04-29",
		"META":   "2026-04-29",
		"NOBARS": "",
	}, got)
	test.Equal(map[string]int{"AAPL": 1, "ABC": 1, "FB": 1, "META": 1, "NOBARS": 1}, counts)
	for _, alias := range aliases {
		switch alias.Symbol {
		case "FB":
			test.Equal(test.date("2026-01-01"), alias.FromDate)
			test.Require().NotNil(alias.ToDate)
			test.Equal(test.date("2026-05-01"), *alias.ToDate)
		case "AAPL", "ABC", "NOBARS":
			test.Equal(test.date("0001-01-01"), alias.FromDate)
			test.Nil(alias.ToDate)
		}
	}
}

func (test *UniverseMemberTest) TestLastBarsLeavesAnISINWithoutBarsNil() {
	ctx := context.Background()
	test.resetFinanceBars()
	test.createBacktest("sp100")
	test.Require().NoError(test.members.Replace(ctx, "sp100", []repository.MemberInterval{
		{ISIN: aaplISIN, FromDate: test.date("2026-01-01")},
	}))
	instruments := repository.Instrument{Conn: test.conn}
	test.Require().NoError(instruments.StoreAlias(ctx, repository.AliasInterval{
		ISIN: aaplISIN, Symbol: "AAPL", FromDate: test.date("2026-01-01"),
	}))
	test.financeBars("yahoo", "OTHER", test.date("2026-01-05"))

	aliases, err := test.members.LastBars(ctx, "sp100", "yahoo", test.date("2026-05-01"))
	test.Require().NoError(err)
	test.Require().Len(aliases, 1)
	test.Equal("AAPL", aliases[0].Symbol)
	test.Nil(aliases[0].LastBar)
}

// A universe member asked under its old ticker reads the same instrument and
// the same series as the later ticker. The bars are the instrument_ohlc join
// of provider_symbol, so a bar before that window is absent and a bar the
// pre-first-window claim already included is present. Two instruments holding
// the ticker is ErrAmbiguousSymbol and no bars.
func (test *UniverseMemberTest) TestMemberAskedUnderTheOldTickerReadsTheInstrumentSeries() {
	ctx := context.Background()
	test.resetFinanceBars()
	test.createBacktest("meta-window")
	test.Require().NoError(test.members.Replace(ctx, "meta-window", []repository.MemberInterval{
		{ISIN: metaISIN, FromDate: test.date("2021-07-01")},
	}))
	instruments := repository.Instrument{Conn: test.conn}
	test.Require().NoError(instruments.StoreAlias(ctx, repository.AliasInterval{
		ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-07-01"), ToDate: test.toPtr("2021-10-31"),
	}))
	test.Require().NoError(instruments.StoreAlias(ctx, repository.AliasInterval{
		ISIN: metaISIN, Symbol: "META", FromDate: test.date("2021-10-31"),
	}))

	open := func(s string) time.Time { return test.date(s).Add(14*time.Hour + 30*time.Minute) }
	test.financeBars("yahoo", "META", open("2020-12-01"), open("2021-06-01"), open("2021-08-02"), open("2021-11-01"))
	// provider_symbol starts before FB's asset_symbol window: the download's
	// pre-first-window claim already stored that. A bar before the mapping is
	// outside every window.
	_, err := test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, evidence)
		VALUES ($1, 'yahoo', 'META', '2021-01-04', 'download')`, metaISIN)
	test.Require().NoError(err)

	symbols, err := test.members.SymbolsAt(ctx, "meta-window", test.date("2021-08-02"))
	test.Require().NoError(err)
	test.Equal([]string{"FB"}, symbols)

	asOf := open("2021-08-15")
	asked, err := financebars.New(test.conn).List(ctx, "yahoo", []string{"FB"}, asOf, 20)
	test.Require().NoError(err)
	test.Equal([]string{"FB", "FB"}, barSymbols(asked))
	test.Equal([]float64{10, 10}, barCloses(asked))
	test.Equal([]string{"2021-06-01", "2021-08-02"}, barDays(asked))

	var instrumentID string
	var series int
	err = test.conn.QueryRow(ctx, `
		SELECT instrument_id, count(*)
		  FROM instrument_ohlc
		 WHERE provider = 'yahoo' AND time <= $1
		 GROUP BY instrument_id`, asOf).Scan(&instrumentID, &series)
	test.Require().NoError(err)
	test.Equal(metaISIN, instrumentID)
	test.Equal(len(asked), series)

	later, err := financebars.New(test.conn).List(ctx, "yahoo", []string{"META"}, open("2021-11-15"), 20)
	test.Require().NoError(err)
	test.Equal([]string{"META", "META", "META"}, barSymbols(later))
	test.Equal([]string{"2021-06-01", "2021-08-02", "2021-11-01"}, barDays(later))
	test.Equal(barCloses(asked), barCloses(later)[:len(asked)])

	// The December bar is stored under META and is not in the series.
	var outside int
	err = test.conn.QueryRow(ctx, `
		SELECT count(*) FROM instrument_ohlc
		 WHERE provider = 'yahoo' AND (time AT TIME ZONE 'UTC')::date = '2020-12-01'`).Scan(&outside)
	test.Require().NoError(err)
	test.Zero(outside)
}

func (test *UniverseMemberTest) TestMemberOldTickerSharedByTwoInstrumentsIsAmbiguous() {
	ctx := context.Background()
	test.resetFinanceBars()
	test.createBacktest("meta-window")
	test.Require().NoError(test.members.Replace(ctx, "meta-window", []repository.MemberInterval{
		{ISIN: metaISIN, FromDate: test.date("2021-07-01")},
	}))
	instruments := repository.Instrument{Conn: test.conn}
	test.Require().NoError(instruments.StoreAlias(ctx, repository.AliasInterval{
		ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-07-01"), ToDate: test.toPtr("2021-10-31"),
	}))
	test.Require().NoError(instruments.StoreAlias(ctx, repository.AliasInterval{
		ISIN: aaplISIN, Symbol: "FB", FromDate: test.date("2021-07-01"),
	}))
	test.financeBars("yahoo", "FB", test.date("2021-08-02").Add(14*time.Hour+30*time.Minute))

	bars, err := financebars.New(test.conn).List(ctx, "yahoo", []string{"FB"}, test.date("2021-08-15"), 20)
	test.Require().ErrorContains(err, "symbol FB maps to 2 instruments at 2021-08-15")
	test.Require().ErrorContains(err, "symbol maps to more than one instrument")
	test.Nil(bars)
}

func barSymbols(bars []*financepb.OHLC) []string {
	out := make([]string, len(bars))
	for i, bar := range bars {
		out[i] = bar.GetSymbol()
	}
	return out
}

func barCloses(bars []*financepb.OHLC) []float64 {
	out := make([]float64, len(bars))
	for i, bar := range bars {
		out[i] = bar.GetClose()
	}
	return out
}

func barDays(bars []*financepb.OHLC) []string {
	out := make([]string, len(bars))
	for i, bar := range bars {
		out[i] = bar.GetTimestamp().AsTime().UTC().Format(time.DateOnly)
	}
	return out
}
