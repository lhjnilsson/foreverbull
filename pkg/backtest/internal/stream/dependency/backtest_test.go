package dependency

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
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
		{"CASH_USD", "USD Cash", "-", "Cash", "", "", "", "", ""},
	}})
	require.NoError(t, err)

	var calls atomic.Int32
	var mu sync.Mutex
	var lastAsOf string
	asOfs := make(map[string]struct{})
	tabs := make(map[string]struct{})
	fileTypes := make(map[string]struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		mu.Lock()
		tabs[q.Get("tab")] = struct{}{}
		fileTypes[q.Get("fileType")] = struct{}{}
		asOf := q.Get("asOfDate")
		lastAsOf = asOf
		asOfs[asOf] = struct{}{}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(append([]byte("\xef\xbb\xbf"), body...))
	}))
	t.Cleanup(srv.Close)

	scraper := &ISharesETFLoopup{
		products: map[string]string{
			"iShares S&P 100": srv.URL,
		},
		cache:  &fsSnapshotCache{dir: t.TempDir()},
		client: srv.Client(),
	}

	assetMap, err := scraper.GetAssets("iShares S&P 100", "2022", "2023")
	require.NoError(t, err)
	mu.Lock()
	require.Equal(t, map[string]struct{}{"all": {}}, tabs)
	require.Equal(t, map[string]struct{}{"json": {}}, fileTypes)
	require.Equal(t, "20231231", lastAsOf)
	require.Len(t, asOfs, 24)
	mu.Unlock()
	require.Less(t, int(calls.Load()), 100)
	require.Equal(t, 24, int(calls.Load()))
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

	calls.Store(0)
	assetMap, err = scraper.GetAssets("iShares S&P 100", "2022", "2023")
	require.NoError(t, err)
	require.Equal(t, int32(0), calls.Load())
	require.Len(t, assetMap, 2)
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

func TestDefaultSnapshotCacheDir(t *testing.T) {
	t.Setenv(etfSnapshotCacheEnv, filepath.Join("tmp", "etf-cache"))
	require.Equal(t, filepath.Join("tmp", "etf-cache"), defaultSnapshotCacheDir())
}
