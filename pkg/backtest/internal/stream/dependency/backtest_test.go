package dependency

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAssetsUnknownETF(t *testing.T) {
	t.Parallel()

	_, err := NewETFScaper().GetAssets("unknown")
	require.ErrorContains(t, err, "ETF not found")
}

func TestGetAssetsInvalidYear(t *testing.T) {
	t.Parallel()

	_, err := NewETFScaper().GetAssets("iShares S&P 100", "not-a-year")
	require.ErrorContains(t, err, "error parsing date")
}

func TestGetAssets(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(Response{Data: [][]interface{}{
		{"AAPL", "Apple Inc.", "Technology", "Equity", "", "", "", "", "US0378331005"},
	}})
	require.NoError(t, err)

	var gotTab, gotFileType, gotAsOfDate string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotTab = q.Get("tab")
		gotFileType = q.Get("fileType")
		gotAsOfDate = q.Get("asOfDate")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(append([]byte("\xef\xbb\xbf"), body...))
	}))
	t.Cleanup(srv.Close)

	scraper := &ISharesETFLoopup{
		products: map[string]string{
			"iShares S&P 100": srv.URL,
		},
	}

	assetMap, err := scraper.GetAssets("iShares S&P 100", "2022", "2023")
	require.NoError(t, err)
	require.Equal(t, "all", gotTab)
	require.Equal(t, "json", gotFileType)
	require.NotEmpty(t, gotAsOfDate)
	require.Len(t, assetMap, 2)

	for _, year := range []string{"2022", "2023"} {
		assets, ok := assetMap[year]
		require.True(t, ok)
		require.Equal(t, []Asset{{
			Symbol:    "AAPL",
			Name:      "Apple Inc.",
			Sector:    "Technology",
			AssetType: "Equity",
			ISIN:      "US0378331005",
		}}, assets)
	}
}

func TestGetBenchmark(t *testing.T) {
	t.Parallel()

	uut := NewETFScaper()

	benchmark, err := uut.GetBenchmark("iShares S&P 100")
	require.NoError(t, err)
	require.Equal(t, "OEF", benchmark)

	_, err = uut.GetBenchmark("unknown")
	require.ErrorContains(t, err, "ETF not found")
}
