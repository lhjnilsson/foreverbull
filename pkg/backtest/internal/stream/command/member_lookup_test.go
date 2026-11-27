package command_test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// lookupFixture is one month-end list per as-of with the given members.
// Each symbol gets its own made-up ISIN.
func (test *BacktestCommandTest) lookupFixture(symbols ...string) map[time.Time][]dependency.Asset {
	assets := make([]dependency.Asset, 0, len(symbols))
	for i, symbol := range symbols {
		assets = append(assets, dependency.Asset{
			Symbol: symbol, Name: symbol + " Inc.", AssetType: "Equity",
			ISIN: "US00000000" + string(rune('0'+i)) + "0",
		})
	}
	listings := map[time.Time][]dependency.Asset{}
	for _, asOf := range []string{"2023-12-31", "2024-01-31", "2024-02-29"} {
		listings[test.date(asOf)] = assets
	}
	return listings
}

// lookupClient answers GetAsset per symbol: nil is a found asset, any
// other value is the RPC error.
func lookupClient(results map[string]error) *financeServicePB.MockMarketdataClient {
	client := new(financeServicePB.MockMarketdataClient)
	client.On("GetAsset", mock.Anything, mock.MatchedBy(func(req *financeServicePB.GetAssetRequest) bool {
		return req != nil && req.GetIsin() != ""
	})).Return(&financeServicePB.GetAssetResponse{Asset: &finance.Asset{}}, nil).Maybe()
	for symbol, err := range results {
		call := client.On("GetAsset", mock.Anything, &financeServicePB.GetAssetRequest{
			Identifier: &financeServicePB.GetAssetRequest_Symbol{Symbol: symbol},
		})
		if err != nil {
			call.Return(nil, err)
			continue
		}
		call.Return(&financeServicePB.GetAssetResponse{Asset: &finance.Asset{Symbol: symbol}}, nil)
	}
	return client
}

// failedRequest is a GetAsset 503 whose text carries a Yahoo URL with the
// session crumb.
func failedRequest(symbol string) error {
	return status.Error(codes.Unknown, "error getting asset: error: Get \"https://query2.finance.yahoo.com/v10/finance/quoteSummary/"+
		symbol+"?modules=quoteType&crumb=s3cr3tCrumb\": 503 Service Unavailable")
}

// noISIN is the NotFound status the finance service returns for an equity
// it has no ISIN for.
func noISIN(symbol string) error {
	return status.Error(codes.NotFound, "no ISIN for "+symbol)
}

// unknownSymbol is the NotFound status the finance service returns when
// Yahoo answered that it has no quote for the ticker.
func unknownSymbol(symbol string) error {
	return status.Error(codes.NotFound, "unknown symbol "+symbol)
}

// requireNothingStored checks that a failed create left no backtest and no
// universe behind.
func (test *BacktestCommandTest) requireNothingStored(name string) {
	backtests := repository.Backtest{Conn: test.db}
	_, err := backtests.Get(context.TODO(), name)
	test.Require().ErrorIs(err, pgx.ErrNoRows)
	members := repository.UniverseMember{Conn: test.db}
	listed, err := members.List(context.TODO(), name)
	test.Require().NoError(err)
	test.Empty(listed)
}

// A member whose lookup failed fails create. Before, it was logged and
// left out of the universe. The lookup is always Yahoo, so the sentence
// names it and offers no --provider. The URL and status text stay in the
// log, crumb removed.
func (test *BacktestCommandTest) TestCreateBacktestFromETFFailsWhenMemberLookupFails() {
	var buf bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&buf)
	test.T().Cleanup(func() { log.Logger = previous })

	client := lookupClient(map[string]error{"AAA": nil, "BBB": failedRequest("BBB")})
	err := test.createETFBacktestWith(context.TODO(), client, "oef-2024q1", "2024-01-02", "2024-03-28",
		test.lookupFixture("AAA", "BBB"))

	test.Require().EqualError(err,
		"Couldn't load data for backtest 'oef-2024q1': yahoo didn't respond when looking up BBB. The backtest wasn't saved. "+
			"Wait a few minutes, then run `fbull backtest create` again.")
	test.NotContains(err.Error(), "503")
	test.NotContains(err.Error(), "yahoo.com")
	test.NotContains(err.Error(), "--provider")
	test.requireNothingStored("oef-2024q1")
	client.AssertExpectations(test.T())

	test.Contains(buf.String(), "query2.finance.yahoo.com/v10/finance/quoteSummary/BBB")
	test.Contains(buf.String(), "503 Service Unavailable")
	test.NotContains(buf.String(), "modules=")
	test.NotContains(buf.String(), "quoteType")
	test.NotContains(buf.String(), "s3cr3tCrumb")
}

// An equity the provider has no ISIN for can't be followed across ticker
// changes. A retry won't help, so the sentence doesn't say "try again".
func (test *BacktestCommandTest) TestCreateBacktestFromETFFailsWhenMemberHasNoISIN() {
	client := lookupClient(map[string]error{"AAA": nil, "XYZ": noISIN("XYZ")})
	err := test.createETFBacktestWith(context.TODO(), client, "oef-2024q1", "2024-01-02", "2024-03-28",
		test.lookupFixture("AAA", "XYZ"))

	test.Require().EqualError(err,
		"Couldn't load data for backtest 'oef-2024q1': yahoo doesn't identify XYZ well enough to follow ticker changes, "+
			"so it can't be used. The backtest wasn't saved. Remove XYZ, then run `fbull backtest create` again.")
	test.requireNothingStored("oef-2024q1")
}

// Only NotFound with the "no ISIN for " message is the no-ISIN answer. A
// NotFound with another message, or the ErrNoISIN text on another code, is
// a failed lookup.
func (test *BacktestCommandTest) TestCreateBacktestFromETFOtherErrorsAreFailedLookups() {
	want := "Couldn't load data for backtest 'oef-2024q1': yahoo didn't respond when looking up XYZ. The backtest wasn't saved. " +
		"Wait a few minutes, then run `fbull backtest create` again."
	causes := map[string]error{
		"not-found":   status.Error(codes.NotFound, "asset not found: XYZ"),
		"internal":    status.Error(codes.Internal, "no ISIN for XYZ"),
		"err-no-isin": status.Error(codes.Internal, "error getting asset: provider has no ISIN for the equity"),
	}
	for name, cause := range causes {
		test.Run(name, func() {
			client := lookupClient(map[string]error{"XYZ": cause})
			err := test.createETFBacktestWith(context.TODO(), client, "oef-2024q1", "2024-01-02", "2024-03-28",
				test.lookupFixture("XYZ"))

			test.Require().EqualError(err, want)
		})
	}
}

// Every failing member is named in one error, not just the first. Each
// cause gets one line, symbols A-Z, lookup failures first, and the line
// says "it" for one symbol and "them" for several.
func (test *BacktestCommandTest) TestCreateBacktestFromETFNamesEveryFailedMember() {
	const header = "Couldn't load data for backtest 'oef-2024q1': %d symbols couldn't be used. The backtest wasn't saved.\n"
	const footer = "Then run `fbull backtest create` again."
	const lookupOne = "  AAA: yahoo didn't respond when looking it up. Try again in a few minutes.\n"
	const lookupTwo = "  AAA, BBB: yahoo didn't respond when looking them up. Try again in a few minutes.\n"
	const noISINOne = "  XYZ: yahoo doesn't identify it well enough to follow ticker changes. Remove it.\n"
	const noISINTwo = "  QQQ, XYZ: yahoo doesn't identify them well enough to follow ticker changes. Remove them.\n"
	cases := []struct {
		name    string
		results map[string]error
		want    string
	}{
		{
			name:    "two-lookup-failures",
			results: map[string]error{"AAA": failedRequest("AAA"), "BBB": failedRequest("BBB")},
			want:    fmt.Sprintf(header, 2) + lookupTwo + footer,
		},
		{
			name:    "two-no-isin",
			results: map[string]error{"QQQ": noISIN("QQQ"), "XYZ": noISIN("XYZ")},
			want:    fmt.Sprintf(header, 2) + noISINTwo + footer,
		},
		{
			name:    "one-of-each",
			results: map[string]error{"AAA": failedRequest("AAA"), "XYZ": noISIN("XYZ")},
			want:    fmt.Sprintf(header, 2) + lookupOne + noISINOne + footer,
		},
		{
			name: "two-lookup-failures-one-no-isin",
			results: map[string]error{
				"AAA": failedRequest("AAA"), "BBB": failedRequest("BBB"), "XYZ": noISIN("XYZ"),
			},
			want: fmt.Sprintf(header, 3) + lookupTwo + noISINOne + footer,
		},
		{
			name: "two-of-each",
			results: map[string]error{
				"AAA": failedRequest("AAA"), "BBB": failedRequest("BBB"),
				"QQQ": noISIN("QQQ"), "XYZ": noISIN("XYZ"),
			},
			want: fmt.Sprintf(header, 4) + lookupTwo + noISINTwo + footer,
		},
	}
	for _, tc := range cases {
		test.Run(tc.name, func() {
			results := map[string]error{"CCC": nil}
			symbols := []string{"CCC"}
			for symbol, err := range tc.results {
				results[symbol] = err
				symbols = append(symbols, symbol)
			}
			client := lookupClient(results)
			err := test.createETFBacktestWith(context.TODO(), client, "oef-2024q1", "2024-01-02", "2024-03-28",
				test.lookupFixture(symbols...))

			test.Require().EqualError(err, tc.want)
			test.requireNothingStored("oef-2024q1")
			client.AssertExpectations(test.T())
		})
	}
}

// Yahoo answered that it has no quote for the ticker. One symbol is one
// sentence. Several share a line, after any no-ISIN line. A failed request
// stays the lookup sentence.
func (test *BacktestCommandTest) TestCreateBacktestFromETFFailsWhenYahooDoesNotKnowTheSymbol() {
	const header = "Couldn't load data for backtest 'oef-2024q1': %d symbols couldn't be used. The backtest wasn't saved.\n"
	const footer = "Then run `fbull backtest create` again."
	cases := []struct {
		name    string
		results map[string]error
		symbols []string
		want    string
	}{
		{
			name:    "one",
			results: map[string]error{"XYZ": unknownSymbol("XYZ")},
			symbols: []string{"XYZ"},
			want: "Couldn't load data for backtest 'oef-2024q1': yahoo doesn't know the symbol XYZ. " +
				"The backtest wasn't saved. Remove XYZ, then run `fbull backtest create` again.",
		},
		{
			name:    "one grouped",
			results: map[string]error{"AAA": failedRequest("AAA"), "XYZ": unknownSymbol("XYZ")},
			symbols: []string{"AAA", "XYZ"},
			want: fmt.Sprintf(header, 2) +
				"  AAA: yahoo didn't respond when looking it up. Try again in a few minutes.\n" +
				"  XYZ: yahoo doesn't know this symbol. Remove it.\n" + footer,
		},
		{
			name:    "two grouped",
			results: map[string]error{"QQQ": unknownSymbol("QQQ"), "XYZ": unknownSymbol("XYZ")},
			symbols: []string{"QQQ", "XYZ"},
			want: fmt.Sprintf(header, 2) +
				"  QQQ, XYZ: yahoo doesn't know these symbols. Remove them.\n" + footer,
		},
		{
			name: "after no-isin",
			results: map[string]error{
				"QQQ": unknownSymbol("QQQ"), "XYZ": noISIN("XYZ"),
			},
			symbols: []string{"QQQ", "XYZ"},
			want: fmt.Sprintf(header, 2) +
				"  XYZ: yahoo doesn't identify it well enough to follow ticker changes. Remove it.\n" +
				"  QQQ: yahoo doesn't know this symbol. Remove it.\n" + footer,
		},
	}
	for _, tc := range cases {
		test.Run(tc.name, func() {
			client := lookupClient(tc.results)
			err := test.createETFBacktestWith(context.TODO(), client, "oef-2024q1", "2024-01-02", "2024-03-28",
				test.lookupFixture(tc.symbols...))
			test.Require().EqualError(err, tc.want)
			test.requireNothingStored("oef-2024q1")
		})
	}
}

// A transport error, a non-404, and a timeout are failed lookups. They are
// not Yahoo's answer that it doesn't know the ticker.
func (test *BacktestCommandTest) TestCreateBacktestFromETFFailedRequestIsNotAnUnknownSymbol() {
	want := "Couldn't load data for backtest 'oef-2024q1': yahoo didn't respond when looking up XYZ. The backtest wasn't saved. " +
		"Wait a few minutes, then run `fbull backtest create` again."
	causes := map[string]error{
		"transport":       failedRequest("XYZ"),
		"non-404":         status.Error(codes.Internal, "fail to get asset for symbol XYZ: HTTP 500 Internal Server Error"),
		"timeout":         status.Error(codes.DeadlineExceeded, "yahoo took too long to respond for XYZ"),
		"other not found": status.Error(codes.NotFound, "Quote not found for symbol: XYZ"),
	}
	for name, cause := range causes {
		test.Run(name, func() {
			client := lookupClient(map[string]error{"XYZ": cause})
			err := test.createETFBacktestWith(context.TODO(), client, "oef-2024q1", "2024-01-02", "2024-03-28",
				test.lookupFixture("XYZ"))
			test.Require().EqualError(err, want)
		})
	}
}

// A canceled create returns the context error and stores nothing; it is
// not reported as a failed lookup.
func (test *BacktestCommandTest) TestCreateBacktestFromETFMemberLookupCanceled() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := lookupClient(map[string]error{})
	err := test.createETFBacktestWith(ctx, client, "oef-2024q1", "2024-01-02", "2024-03-28",
		test.lookupFixture("AAA"))

	test.Require().ErrorIs(err, context.Canceled)
	client.AssertNotCalled(test.T(), "GetAsset", mock.Anything, mock.Anything)
	test.requireNothingStored("oef-2024q1")
}

// Yahoo doesn't know FB, but asset_symbol already gives FB to Meta on the
// backtest dates. Create does not tell the user to remove FB. The download
// looks up META.
func (test *BacktestCommandTest) TestCreateBacktestFromETFKnownAliasIsNotToldToRemove() {
	_, err := test.db.Exec(context.TODO(), `
		INSERT INTO instrument (id, isin, name) VALUES ('US30303M1027', 'US30303M1027', 'Meta Platforms Inc.');
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES
			('US30303M1027', 'FB', '2012-05-18', '2022-06-09'),
			('US30303M1027', 'META', '2022-06-09', NULL);`)
	test.Require().NoError(err)
	fb := dependency.Asset{Symbol: "FB", Name: "Facebook Inc.", AssetType: "Equity", ISIN: metaISIN}
	listings := map[time.Time][]dependency.Asset{}
	for _, asOf := range []string{"2021-06-30", "2021-07-31", "2021-08-31", "2021-09-30"} {
		listings[test.date(asOf)] = []dependency.Asset{fb}
	}
	client := lookupClient(map[string]error{"FB": unknownSymbol("FB")})
	err = test.createETFBacktestWith(context.TODO(), client, "fb-window", "2021-07-01", "2021-09-30", listings)
	test.Require().NoError(err)
	client.AssertExpectations(test.T())
}

func (test *BacktestCommandTest) seedTwoCompanies(symbol, isinA, isinB string) {
	test.T().Helper()
	_, err := test.db.Exec(context.TODO(), `
		INSERT INTO instrument (id, isin, name) VALUES ($1, $1, $2), ($3, $3, $4)`,
		isinA, symbol+" A", isinB, symbol+" B")
	test.Require().NoError(err)
	_, err = test.db.Exec(context.TODO(), `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES
			($1, $3, '2021-01-01', '2021-08-15'),
			($2, $3, '2021-08-15', NULL)`,
		isinA, isinB, symbol)
	test.Require().NoError(err)
}

func (test *BacktestCommandTest) fbWindowListings(assets ...dependency.Asset) map[time.Time][]dependency.Asset {
	listings := map[time.Time][]dependency.Asset{}
	for _, asOf := range []string{"2021-06-30", "2021-07-31", "2021-08-31", "2021-09-30"} {
		listings[test.date(asOf)] = assets
	}
	return listings
}

func equity(symbol, isin string) dependency.Asset {
	return dependency.Asset{Symbol: symbol, Name: symbol + " Inc.", AssetType: "Equity", ISIN: isin}
}

// Yahoo doesn't know the ticker, and asset_symbol gives it to two companies
// on the backtest dates. Every such member is named. The sentence does not
// blame an exchange suffix.
func (test *BacktestCommandTest) TestCreateBacktestFromETFNamesEveryAmbiguousTicker() {
	const from = "2021-07-01"
	const to = "2021-09-30"
	test.seedTwoCompanies("XYZ", "US1111111111", "US2222222222")
	test.seedTwoCompanies("QQQ", "US3333333333", "US4444444444")

	one := "Couldn't load data for backtest 'fb-window': yahoo doesn't know the symbol XYZ, " +
		"and more than one company used XYZ between 2021-07-01 and 2021-09-30, so foreverbull can't tell which one to read. " +
		"The backtest wasn't saved. Remove XYZ, then run `fbull backtest create` again."
	header := "Couldn't load data for backtest 'fb-window': %d symbols couldn't be used. The backtest wasn't saved.\n"
	footer := "Then run `fbull backtest create` again."
	oneLine := "  XYZ: yahoo doesn't know this symbol, and more than one company used it between 2021-07-01 and 2021-09-30. Remove it.\n"
	twoLine := "  QQQ, XYZ: yahoo doesn't know these symbols, and more than one company used each of them between 2021-07-01 and 2021-09-30. Remove them.\n"

	cases := []struct {
		name    string
		results map[string]error
		assets  []dependency.Asset
		want    string
	}{
		{
			name:    "one",
			results: map[string]error{"XYZ": unknownSymbol("XYZ")},
			assets:  []dependency.Asset{equity("XYZ", "US1111111111")},
			want:    one,
		},
		{
			name:    "one grouped",
			results: map[string]error{"AAA": failedRequest("AAA"), "XYZ": unknownSymbol("XYZ")},
			assets:  []dependency.Asset{equity("AAA", "US5555555555"), equity("XYZ", "US1111111111")},
			want: fmt.Sprintf(header, 2) +
				"  AAA: yahoo didn't respond when looking it up. Try again in a few minutes.\n" +
				oneLine + footer,
		},
		{
			name:    "two grouped",
			results: map[string]error{"QQQ": unknownSymbol("QQQ"), "XYZ": unknownSymbol("XYZ")},
			assets:  []dependency.Asset{equity("QQQ", "US3333333333"), equity("XYZ", "US1111111111")},
			want:    fmt.Sprintf(header, 2) + twoLine + footer,
		},
		{
			name:    "unknown then ambiguous",
			results: map[string]error{"AAA": unknownSymbol("AAA"), "XYZ": unknownSymbol("XYZ")},
			assets:  []dependency.Asset{equity("AAA", "US5555555555"), equity("XYZ", "US1111111111")},
			want: fmt.Sprintf(header, 2) +
				"  AAA: yahoo doesn't know this symbol. Remove it.\n" +
				oneLine + footer,
		},
	}
	for _, tc := range cases {
		test.Run(tc.name, func() {
			client := lookupClient(tc.results)
			err := test.createETFBacktestWith(context.TODO(), client, "fb-window", from, to, test.fbWindowListings(tc.assets...))
			test.Require().EqualError(err, tc.want)
			test.requireNothingStored("fb-window")
		})
	}
}

// A failed read of asset_symbol is logged and create fails with the report
// sentence. It is not an unknown symbol and it does not say to remove it.
func (test *BacktestCommandTest) TestCreateBacktestFromETFTickerHistoryReadFailure() {
	_, err := test.db.Exec(context.TODO(), `DROP TABLE asset_symbol`)
	test.Require().NoError(err)
	var buf bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&buf)
	test.T().Cleanup(func() { log.Logger = previous })

	client := lookupClient(map[string]error{"XYZ": unknownSymbol("XYZ")})
	err = test.createETFBacktestWith(context.TODO(), client, "fb-window", "2021-07-01", "2021-09-30",
		test.fbWindowListings(equity("XYZ", "US1111111111")))
	test.Require().EqualError(err, "Couldn't load data for backtest 'fb-window': foreverbull couldn't read its ticker history for XYZ. "+
		"The backtest wasn't saved. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.")
	test.NotContains(err.Error(), "unknown symbol")
	test.NotContains(err.Error(), "Remove")
	test.Contains(buf.String(), "asset_symbol")
	test.requireNothingStored("fb-window")
}

// No asset_symbol row keeps the unknown-symbol sentence.
func (test *BacktestCommandTest) TestCreateBacktestFromETFUnknownSymbolWithNoHolder() {
	client := lookupClient(map[string]error{"XYZ": unknownSymbol("XYZ")})
	err := test.createETFBacktestWith(context.TODO(), client, "fb-window", "2021-07-01", "2021-09-30",
		test.fbWindowListings(equity("XYZ", "US1111111111")))
	test.Require().EqualError(err, "Couldn't load data for backtest 'fb-window': yahoo doesn't know the symbol XYZ. "+
		"The backtest wasn't saved. Remove XYZ, then run `fbull backtest create` again.")
	test.requireNothingStored("fb-window")
}

// The later-ticker skip runs only when asset_symbol's ISIN is the member's.
// A different company that once used the ticker is not skipped.
func (test *BacktestCommandTest) TestCreateBacktestFromETFKnownAliasSkipRequiresTheMemberISIN() {
	_, err := test.db.Exec(context.TODO(), `
		INSERT INTO instrument (id, isin, name) VALUES ('US30303M1027', 'US30303M1027', 'Meta Platforms Inc.');
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES
			('US30303M1027', 'FB', '2012-05-18', '2022-06-09')`)
	test.Require().NoError(err)
	client := lookupClient(map[string]error{"FB": unknownSymbol("FB")})
	err = test.createETFBacktestWith(context.TODO(), client, "fb-window", "2021-07-01", "2021-09-30",
		test.fbWindowListings(equity("FB", "US1111111111")))
	test.Require().EqualError(err, "Couldn't load data for backtest 'fb-window': yahoo doesn't know the symbol FB. "+
		"The backtest wasn't saved. Remove FB, then run `fbull backtest create` again.")
	test.requireNothingStored("fb-window")
}
