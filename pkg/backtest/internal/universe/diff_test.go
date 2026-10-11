package universe_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/universe"
	"github.com/stretchr/testify/require"
)

const (
	aaplISIN = "US0378331005"
	metaISIN = "US30303M1027"
	msftISIN = "US5949181045"
	ibmISIN  = "US4592001014"
)

type fixtureHolding struct {
	ISIN      string `json:"isin"`
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	AssetType string `json:"asset_type"`
}

type fixtureFile struct {
	AsOf     string           `json:"as_of"`
	Holdings []fixtureHolding `json:"holdings"`
}

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	require.NoError(t, err)
	return d
}

func toPtr(t time.Time) *time.Time {
	d := t
	return &d
}

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "testdata")
}

func loadFixtureSnapshots(t *testing.T) []universe.Snapshot {
	t.Helper()
	names := []string{"20230228.json", "20221231.json", "20230131.json"}
	snaps := make([]universe.Snapshot, 0, len(names))
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(testdataDir(t), name))
		require.NoError(t, err)
		var fx fixtureFile
		require.NoError(t, json.Unmarshal(body, &fx))
		holdings := make([]universe.Holding, 0, len(fx.Holdings))
		for _, h := range fx.Holdings {
			holdings = append(holdings, universe.Holding{
				ISIN:      h.ISIN,
				Symbol:    h.Symbol,
				Name:      h.Name,
				AssetType: h.AssetType,
			})
		}
		snaps = append(snaps, universe.Snapshot{
			AsOf:     date(t, fx.AsOf),
			Holdings: holdings,
		})
	}
	return snaps
}

func TestDiffFixtureRenameAddDrop(t *testing.T) {
	t.Parallel()

	got := universe.Diff(loadFixtureSnapshots(t))

	s0 := date(t, "2022-12-31")
	s1 := date(t, "2023-01-31")
	s2 := date(t, "2023-02-28")

	require.Equal(t, []universe.Member{
		{ISIN: aaplISIN, FromDate: s0, ToDate: toPtr(s2)},
		{ISIN: metaISIN, FromDate: s0, ToDate: nil},
		{ISIN: msftISIN, FromDate: s1, ToDate: nil},
	}, got.Members)

	require.Equal(t, []universe.Alias{
		{ISIN: aaplISIN, Symbol: "AAPL", FromDate: s0, ToDate: nil},
		{ISIN: metaISIN, Symbol: "FB", FromDate: s0, ToDate: toPtr(s1)},
		{ISIN: metaISIN, Symbol: "META", FromDate: s1, ToDate: nil},
		{ISIN: msftISIN, Symbol: "MSFT", FromDate: s1, ToDate: nil},
	}, got.Aliases)

	require.Equal(t, []string{aaplISIN, metaISIN}, universe.MembersAt(got.Members, s0))
	require.Equal(t, []string{aaplISIN, metaISIN, msftISIN}, universe.MembersAt(got.Members, s1))
	require.Equal(t, []string{metaISIN, msftISIN}, universe.MembersAt(got.Members, s2))
	require.Empty(t, universe.MembersAt(got.Members, date(t, "2022-12-30")))
	require.Equal(t, []string{aaplISIN, metaISIN}, universe.MembersAt(got.Members, date(t, "2023-01-30")))

	require.Equal(t, []string{"AAPL", "FB"}, universe.SymbolsAt(got.Members, got.Aliases, s0))
	require.Equal(t, []string{"AAPL", "META", "MSFT"}, universe.SymbolsAt(got.Members, got.Aliases, s1))
	require.Equal(t, []string{"META", "MSFT"}, universe.SymbolsAt(got.Members, got.Aliases, s2))
	require.Empty(t, universe.SymbolsAt(got.Members, got.Aliases, date(t, "2022-12-30")))
	require.Equal(t, []string{"AAPL", "FB"}, universe.SymbolsAt(got.Members, got.Aliases, date(t, "2023-01-30")))
}

func TestSymbolsAtTickerChangeIsNotMembershipDrop(t *testing.T) {
	t.Parallel()

	s0 := date(t, "2021-06-30")
	s1 := date(t, "2021-10-31")
	members := []universe.Member{{ISIN: metaISIN, FromDate: s0, ToDate: nil}}
	aliases := []universe.Alias{
		{ISIN: metaISIN, Symbol: "FB", FromDate: s0, ToDate: toPtr(s1)},
		{ISIN: metaISIN, Symbol: "META", FromDate: s1, ToDate: nil},
	}

	require.Equal(t, []string{"FB"}, universe.SymbolsAt(members, aliases, s0))
	require.Equal(t, []string{"FB"}, universe.SymbolsAt(members, aliases, date(t, "2021-10-30")))
	require.Equal(t, []string{"META"}, universe.SymbolsAt(members, aliases, s1))
	require.Empty(t, universe.SymbolsAt(members, aliases, date(t, "2021-06-29")))
	require.Empty(t, universe.SymbolsAt(nil, aliases, s0))
	require.Empty(t, universe.SymbolsAt(members, nil, s0))
}

func TestDiffTickerChangeDoesNotSplitMembership(t *testing.T) {
	t.Parallel()

	snaps := []universe.Snapshot{
		{
			AsOf: date(t, "2021-06-30"),
			Holdings: []universe.Holding{
				{ISIN: metaISIN, Symbol: "FB", AssetType: "Equity"},
			},
		},
		{
			AsOf: date(t, "2021-10-31"),
			Holdings: []universe.Holding{
				{ISIN: metaISIN, Symbol: "META", AssetType: "Equity"},
			},
		},
	}

	got := universe.Diff(snaps)
	require.Equal(t, []universe.Member{
		{ISIN: metaISIN, FromDate: date(t, "2021-06-30"), ToDate: nil},
	}, got.Members)
	require.Len(t, got.Aliases, 2)
	require.Equal(t, "FB", got.Aliases[0].Symbol)
	require.Equal(t, "META", got.Aliases[1].Symbol)
}

func TestDiffSkipEmptyISINAndNonEquity(t *testing.T) {
	t.Parallel()

	got := universe.Diff([]universe.Snapshot{{
		AsOf: date(t, "2022-12-31"),
		Holdings: []universe.Holding{
			{ISIN: "", Symbol: "CASH_USD", AssetType: "Cash"},
			{ISIN: "   ", Symbol: "FAKE", AssetType: "Equity"},
			{ISIN: "US0000000001", Symbol: "BOND", AssetType: "Fixed Income"},
			{ISIN: aaplISIN, Symbol: "AAPL", AssetType: "Equity"},
		},
	}})

	require.Equal(t, []universe.Member{
		{ISIN: aaplISIN, FromDate: date(t, "2022-12-31"), ToDate: nil},
	}, got.Members)
}

func TestDiffReaddAfterDropOpensNewInterval(t *testing.T) {
	t.Parallel()

	snaps := []universe.Snapshot{
		{
			AsOf: date(t, "2022-12-31"),
			Holdings: []universe.Holding{
				{ISIN: aaplISIN, Symbol: "AAPL", AssetType: "Equity"},
				{ISIN: ibmISIN, Symbol: "IBM", AssetType: "Equity"},
			},
		},
		{
			AsOf: date(t, "2023-01-31"),
			Holdings: []universe.Holding{
				{ISIN: aaplISIN, Symbol: "AAPL", AssetType: "Equity"},
			},
		},
		{
			AsOf: date(t, "2023-02-28"),
			Holdings: []universe.Holding{
				{ISIN: aaplISIN, Symbol: "AAPL", AssetType: "Equity"},
				{ISIN: ibmISIN, Symbol: "IBM", AssetType: "Equity"},
			},
		},
	}

	got := universe.Diff(snaps)
	require.Equal(t, []universe.Member{
		{ISIN: aaplISIN, FromDate: date(t, "2022-12-31"), ToDate: nil},
		{ISIN: ibmISIN, FromDate: date(t, "2022-12-31"), ToDate: toPtr(date(t, "2023-01-31"))},
		{ISIN: ibmISIN, FromDate: date(t, "2023-02-28"), ToDate: nil},
	}, got.Members)
	require.Equal(t, []string{aaplISIN}, universe.MembersAt(got.Members, date(t, "2023-01-31")))
	require.Equal(t, []string{aaplISIN, ibmISIN}, universe.MembersAt(got.Members, date(t, "2023-02-28")))
}

func TestDiffEmptyAndSingleSnapshot(t *testing.T) {
	t.Parallel()

	require.Empty(t, universe.Diff(nil).Members)
	require.Empty(t, universe.Diff([]universe.Snapshot{}).Members)

	got := universe.Diff([]universe.Snapshot{{
		AsOf: date(t, "2022-12-31"),
		Holdings: []universe.Holding{
			{ISIN: aaplISIN, Symbol: "AAPL", AssetType: "Equity"},
		},
	}})
	require.Equal(t, []universe.Member{
		{ISIN: aaplISIN, FromDate: date(t, "2022-12-31"), ToDate: nil},
	}, got.Members)
}
