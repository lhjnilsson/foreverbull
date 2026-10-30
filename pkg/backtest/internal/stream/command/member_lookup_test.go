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
// it has no ISIN for (lhjnilsson/foreverbull#240).
func noISIN(symbol string) error {
	return status.Error(codes.NotFound, "no ISIN for "+symbol)
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
