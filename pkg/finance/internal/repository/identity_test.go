package repository_test

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
)

func (test *InstrumentTests) TestIdentityISINAndMICAreUnique() {
	ctx := context.Background()
	identities := repository.Identities{Conn: test.conn}
	from := test.date(2020, time.January, 1)
	_, err := identities.Store(ctx, repository.Identity{
		ISIN: metaISIN, MIC: "XNAS", Currency: "USD", PriceUnit: "USD", ValidFrom: from,
	})
	test.Require().NoError(err)
	_, err = identities.Store(ctx, repository.Identity{
		ISIN: metaISIN, MIC: "XNAS", Currency: "USD", PriceUnit: "USD", ValidFrom: from,
	})
	test.Require().Error(err)
	test.ErrorContains(err, "instrument_identity_no_overlap")
}

func (test *InstrumentTests) TestIdentityStoresOneISINOnTwoVenues() {
	ctx := context.Background()
	identities := repository.Identities{Conn: test.conn}
	from := test.date(2020, time.January, 1)
	on := test.date(2020, time.June, 1)
	_, err := identities.Store(ctx, repository.Identity{
		ISIN: "US0378331005", MIC: "XNAS", Currency: "USD", PriceUnit: "USD", ValidFrom: from,
	})
	test.Require().NoError(err)
	_, err = identities.Store(ctx, repository.Identity{
		ISIN: "US0378331005", MIC: "XNYS", Currency: "USD", PriceUnit: "USD", ValidFrom: from,
	})
	test.Require().NoError(err)

	xnas, err := identities.At(ctx, "US0378331005", "XNAS", on)
	test.Require().NoError(err)
	test.Equal("XNAS", xnas.MIC)
	test.Equal("USD", xnas.Currency)
	test.Equal("USD", xnas.PriceUnit)

	xnys, err := identities.At(ctx, "US0378331005", "XNYS", on)
	test.Require().NoError(err)
	test.Equal("XNYS", xnys.MIC)
	test.Equal("USD", xnys.Currency)
	test.Equal("USD", xnys.PriceUnit)

	_, err = identities.BySymbol(ctx, "AAPL", on)
	test.Require().Error(err)

	_, err = test.instruments.Store(ctx, "US0378331005", "Apple Inc")
	test.Require().NoError(err)
	test.storeSymbol(ctx, "US0378331005", "AAPL", from, time.Time{})
	_, err = identities.BySymbol(ctx, "AAPL", on)
	var ambiguous *repository.AmbiguousVenueError
	test.Require().ErrorAs(err, &ambiguous)
	test.Equal("US0378331005 has more than one listing on 2020-06-01", ambiguous.Error())
}

func (test *InstrumentTests) TestIdentityLookupFollowsTheWindow() {
	ctx := context.Background()
	identities := repository.Identities{Conn: test.conn}
	end := test.date(2021, time.January, 1)
	_, err := identities.Store(ctx, repository.Identity{
		ISIN: metaISIN, MIC: "XNAS", Currency: "USD", PriceUnit: "USD",
		ValidFrom: test.date(2020, time.January, 1), ValidTo: &end,
	})
	test.Require().NoError(err)
	_, err = identities.Store(ctx, repository.Identity{
		ISIN: metaISIN, MIC: "XNAS", Currency: "USD", PriceUnit: "USD",
		ValidFrom: end,
	})
	test.Require().NoError(err)

	first, err := identities.At(ctx, metaISIN, "XNAS", test.date(2020, time.January, 1))
	test.Require().NoError(err)
	test.NotNil(first.ValidTo)

	_, err = identities.At(ctx, metaISIN, "XNAS", test.date(2019, time.December, 31))
	test.Require().ErrorContains(err, "no listing for "+metaISIN+" on XNAS at 2019-12-31")

	second, err := identities.At(ctx, metaISIN, "XNAS", end)
	test.Require().NoError(err)
	test.Nil(second.ValidTo)

	_, err = test.instruments.Store(ctx, metaISIN, "Meta Platforms Inc")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "FB", test.date(2020, time.January, 1), end)
	test.storeSymbol(ctx, metaISIN, "META", end, time.Time{})

	got, err := identities.BySymbol(ctx, "FB", test.date(2020, time.June, 1))
	test.Require().NoError(err)
	test.Equal("XNAS", got.MIC)
	test.NotNil(got.ValidTo)

	got, err = identities.BySymbol(ctx, "META", test.date(2021, time.June, 1))
	test.Require().NoError(err)
	test.Nil(got.ValidTo)

	_, err = identities.BySymbol(ctx, "FB", test.date(2021, time.June, 1))
	var missing *repository.NoIdentityError
	test.Require().ErrorAs(err, &missing)
	test.Equal("FB has no instrument identity", missing.Error())
}

func (test *InstrumentTests) TestAssetWithoutISINKeepsItsTicker() {
	ctx := context.Background()
	test.Require().NoError(test.assets.Store(ctx, "AAPL", "Apple Inc"))
	day := test.date(2024, time.January, 2)

	_, err := test.instruments.Resolve(ctx, "AAPL", day)
	test.Require().ErrorIs(err, pgx.ErrNoRows)

	listed, err := test.assets.ListBySymbols(ctx, []string{"AAPL"})
	test.Require().NoError(err)
	test.Require().Len(listed, 1)
	test.Equal("AAPL", listed[0].GetSymbol())
	test.Empty(listed[0].GetISIN())

	var n int
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM instrument_identity`).Scan(&n)
	test.Require().NoError(err)
	test.Equal(0, n)
}

func (test *InstrumentTests) TestLookupWithoutIdentityNamesTheSymbol() {
	ctx := context.Background()
	test.Require().NoError(test.assets.Store(ctx, "AAPL", "Apple Inc"))
	identities := repository.Identities{Conn: test.conn}

	_, err := identities.Store(ctx, repository.Identity{
		ISIN: "US0378331005", MIC: "XNAS", Currency: "USD", PriceUnit: "",
		ValidFrom: test.date(2020, time.January, 1),
	})
	test.Require().NoError(err)

	var priceUnitNull bool
	var isin, currency string
	err = test.conn.QueryRow(ctx, `
		SELECT isin, currency, price_unit IS NULL
		  FROM instrument_identity`).Scan(&isin, &currency, &priceUnitNull)
	test.Require().NoError(err)
	test.Equal("US0378331005", isin)
	test.Equal("USD", currency)
	test.True(priceUnitNull, "an empty price unit stays null")

	_, err = identities.BySymbol(ctx, "AAPL", test.date(2024, time.January, 2))
	var missing *repository.NoIdentityError
	test.Require().ErrorAs(err, &missing)
	test.Equal("AAPL has no instrument identity", missing.Error())
	test.Require().NotErrorIs(err, repository.ErrMissingISIN)

	_, err = test.instruments.Resolve(ctx, "AAPL", test.date(2024, time.January, 2))
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

func (test *InstrumentTests) TestPriceUnitMapsPenceAndRefusesUnknown() {
	ctx := context.Background()
	identities := repository.Identities{Conn: test.conn}
	from := test.date(2020, time.January, 1)
	stored, err := identities.Store(ctx, repository.Identity{
		ISIN: "GB0000000001", MIC: "XXXX", Currency: "GBP", PriceUnit: "GBp", ValidFrom: from,
	})
	test.Require().NoError(err)
	test.Equal("GBP", stored.Currency)
	test.Equal("GBX", stored.PriceUnit)

	_, err = identities.Store(ctx, repository.Identity{
		ISIN: "US0378331005", MIC: "XNAS", Currency: "USD", PriceUnit: "banana", ValidFrom: from,
	})
	test.Require().EqualError(err, "price unit 'banana' is not a unit of USD")

	_, err = identities.Store(ctx, repository.Identity{
		ISIN: "US0378331005", MIC: "XNAS", Currency: "USD", PriceUnit: "USX", ValidFrom: from,
	})
	test.Require().EqualError(err, "price unit 'USX' is not a unit of USD")

	pounds, err := identities.Store(ctx, repository.Identity{
		ISIN: "GB0000000002", MIC: "XXXX", Currency: "gbp", PriceUnit: "gbp", ValidFrom: from,
	})
	test.Require().NoError(err)
	test.Equal("GBP", pounds.Currency)
	test.Equal("GBP", pounds.PriceUnit)
}

func (test *InstrumentTests) TestIdentityCheckRefusesABadUnitAndANullISIN() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `
		INSERT INTO instrument_identity (isin, mic, currency, price_unit, valid_from)
		VALUES ('US0378331005', 'XNAS', 'USD', 'USX', '2020-01-01')`)
	test.Require().Error(err)
	test.Require().ErrorContains(err, "instrument_identity_price_unit")

	_, err = test.conn.Exec(ctx, `
		INSERT INTO instrument_identity (isin, mic, currency, price_unit, valid_from)
		VALUES ('US0378331005', 'XNAS', NULL, 'USD', '2020-01-01')`)
	test.Require().Error(err)
	test.Require().ErrorContains(err, "instrument_identity_price_unit")

	_, err = test.conn.Exec(ctx, `
		INSERT INTO instrument_identity (isin, mic, currency, valid_from)
		VALUES (NULL, 'XNAS', 'USD', '2020-01-01')`)
	test.Require().Error(err)
	test.Require().ErrorContains(err, `null value in column "isin"`)
}
