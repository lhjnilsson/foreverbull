package command_test

import (
	"context"

	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

func (test *BacktestCommandTest) TestSymbolListOfOneCompanyIsNotSaved() {
	ctx := context.Background()
	_, err := test.db.Exec(ctx, `
		INSERT INTO instrument (id, isin, name) VALUES ('US30303M1027', 'US30303M1027', 'Meta');
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES
			('US30303M1027', 'FB', '2012-05-18', '2022-06-09'),
			('US30303M1027', 'META', '2022-06-09', NULL)`)
	test.Require().NoError(err)

	err = test.createFromFile("demo", []*backtest.Backtest_Period_Asset{
		{Symbol: "FB", Enabled: true},
		{Symbol: "META", Enabled: true},
	})
	test.Require().EqualError(err, "Couldn't create backtest 'demo': FB and META are the same company "+
		"(META, earlier FB). The backtest wasn't saved. Remove one of them, then run `fbull backtest create` again.")
	test.requireNotStored("demo")
}
