package mapping

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The strings are the tickers each provider was asked for before this layer
// owned the rules. A share class changes shape. An exchange suffix does not.
func TestProviderTickerMatchesTheOldRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		asked, yahoo, alpaca string
	}{
		{"BRK.B", "BRK-B", "BRK.B"},
		{"BRK-B", "BRK-B", "BRK.B"},
		{"BRK/B", "BRK-B", "BRK.B"},
		{"BF.B", "BF-B", "BF.B"},
		{"BK", "BK", "BK"},
		{"BKNG", "BKNG", "BKNG"},
		{"aapl", "AAPL", "AAPL"},
		{"GOOG", "GOOG", "GOOG"},
		{"VOD.L", "VOD.L", "VOD.L"},
		// A joined iShares ticker is not guessed from the letters.
		{"BRKB", "BRKB", "BRKB"},
		{"BFB", "BFB", "BFB"},
	}
	for _, tc := range cases {
		yahoo, err := ProviderTicker("yahoo", tc.asked, "")
		require.NoError(t, err)
		require.Equal(t, tc.yahoo, yahoo, "yahoo %s", tc.asked)
		require.Equal(t, tc.yahoo, Yahoo(tc.asked))

		alpaca, err := ProviderTicker("alpaca", tc.asked, "")
		require.NoError(t, err)
		require.Equal(t, tc.alpaca, alpaca, "alpaca %s", tc.asked)
		require.Equal(t, tc.alpaca, Alpaca(tc.asked))
	}

	for asked, want := range map[string]string{"BRK.B": "brk-b.us", "BK": "bk.us", "AAPL": "aapl.us"} {
		code, canonical, err := Stooq(asked, "us")
		require.NoError(t, err)
		require.Equal(t, want, code, asked)
		require.Equal(t, asked, canonical)
		got, err := ProviderTicker("stooq", asked, "us")
		require.NoError(t, err)
		require.Equal(t, want, got, asked)
	}

	_, _, err := Stooq("VOD.L", "us")
	var refused *StooqSuffixError
	require.ErrorAs(t, err, &refused)
	require.Equal(t, "L", refused.Suffix)
	require.Equal(t, ".US", SupportedStooqSuffixes())

	_, _, err = Stooq("VOD.X", "us")
	var suffix *StooqSuffixError
	require.ErrorAs(t, err, &suffix)
	require.Equal(t, "X", suffix.Suffix)

	_, err = ProviderTicker("binance", "AAPL", "")
	require.ErrorContains(t, err, "valid providers: yahoo, stooq, alpaca")
}

func TestLaterCandidatesFollowAliasesThenSearch(t *testing.T) {
	t.Parallel()
	got := LaterCandidates("US30303M1027", "FB", []string{" FB ", "META", "not a ticker", "META"}, []SearchHit{
		{Symbol: "META", ISIN: "US30303M1027"},
		{Symbol: "OTHER", ISIN: "US9999999999"},
		{Symbol: "FUND", ISIN: ""},
		{Symbol: "NEW", ISIN: "US30303M1027"},
	})
	require.Equal(t, []string{"META", "NEW"}, got)

	require.Empty(t, LaterCandidates("US30303M1027", "FB", []string{"FB"}, nil))
}

func TestJoinedTickerComesFromTheISINListing(t *testing.T) {
	t.Parallel()
	classes := NewClasses()
	// No company list: BRKB is unchanged until a listing or a provider
	// symbol for that ISIN is the separated spelling.
	require.Equal(t, "BRKB", FormatYahoo(classes, "BRKB"))
	require.Equal(t, "BRKB", FormatAlpaca(classes, "BRKB"))

	classes.LearnIfJoined("BRKB", []string{"AAPL", "BRK-B"})
	classes.LearnIfJoined("BFB", []string{"BF.B"})
	classes.LearnIfJoined("META", []string{"META"})
	classes.LearnIfJoined("NVDA", []string{"NVDA"})
	classes.LearnIfJoined("PARA", []string{"PARA"})

	require.Equal(t, "BRK-B", FormatYahoo(classes, "BRKB"))
	require.Equal(t, "BRK.B", FormatAlpaca(classes, "brkb"))
	require.Equal(t, "BF-B", FormatYahoo(classes, "BFB"))
	require.Equal(t, "META", FormatYahoo(classes, "META"))
	require.Equal(t, "NVDA", FormatYahoo(classes, "NVDA"))
	require.Equal(t, "PARA", FormatYahoo(classes, "PARA"))

	code, canonical, err := FormatStooq(classes, "BRKB", "us")
	require.NoError(t, err)
	require.Equal(t, "brk-b.us", code)
	require.Equal(t, "BRKB", canonical)

	// Every spelling is one stored name, so the bars are one row.
	require.Equal(t, "BRKB", Stored("BRK.B"))
	require.Equal(t, "BRKB", Stored("BRK-B"))
	require.Equal(t, "BRKB", Stored("BRK/B"))
	require.Equal(t, "BRKB", Stored("BRK B"))
	require.Equal(t, "BRKB", Stored("BRKB"))
	require.Equal(t, "VOD.L", Stored("VOD.L"))
}
