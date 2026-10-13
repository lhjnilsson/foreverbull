package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/universe"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
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

func (test *UniverseMemberTest) createBacktest(name string) {
	_, err := test.backtests.Create(context.Background(), &backtest.Backtest{
		Name:      name,
		Benchmark: "OEF",
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
