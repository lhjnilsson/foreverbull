package repository_test

import (
	"context"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
)

// The stored list is the provider's dividends of the asked symbols inside
// the span. A null pay date stays null. Another provider, another symbol,
// and an ex-date outside the span are left out.
func (test *BacktestTest) TestListDividendsReadsTheWindow() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS asset (symbol text PRIMARY KEY, name text)`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS dividend (
			provider text NOT NULL,
			symbol text NOT NULL REFERENCES asset(symbol),
			ex_date date NOT NULL,
			pay_date date,
			amount numeric NOT NULL
		)`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `DELETE FROM dividend`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `DELETE FROM asset`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `INSERT INTO asset (symbol, name) VALUES ('KO', ''), ('TSM', ''), ('PEP', '')`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO dividend (provider, symbol, ex_date, pay_date, amount) VALUES
			('yahoo', 'KO', '2024-01-01', NULL, 1),
			('yahoo', 'KO', '2024-01-02', NULL, 2),
			('yahoo', 'KO', '2024-03-14', '2024-04-01', 0.25),
			('yahoo', 'KO', '2024-06-14', NULL, 0.5),
			('yahoo', 'KO', '2024-12-31', NULL, 3),
			('yahoo', 'KO', '2025-01-02', '2025-01-15', 0.25),
			('alpaca', 'KO', '2024-03-14', '2024-04-01', 9),
			('yahoo', 'PEP', '2024-03-14', NULL, 0.25)`)
	test.Require().NoError(err)

	pay := "2024-04-01"
	rows, err := test.uut.ListDividends(ctx, "yahoo", []string{"KO", "TSM"},
		time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC))
	test.Require().NoError(err)
	test.Equal([]repository.StoredDividend{
		{Symbol: "KO", ExDate: "2024-01-02", Amount: 2, PayDate: nil},
		{Symbol: "KO", ExDate: "2024-03-14", Amount: 0.25, PayDate: &pay},
		{Symbol: "KO", ExDate: "2024-06-14", Amount: 0.5, PayDate: nil},
		{Symbol: "KO", ExDate: "2024-12-31", Amount: 3, PayDate: nil},
	}, rows)

	empty, err := test.uut.ListDividends(ctx, "yahoo", nil, time.Time{}, time.Time{})
	test.Require().NoError(err)
	test.Empty(empty)
}
