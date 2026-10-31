package marketdata

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
)

// yahooSplitFixture is a real Yahoo chart response for AAPL from 2020-08-27
// to 2020-09-01, recorded with events=split. AAPL split 4:1 on 2020-08-31.
// Only the volatile meta fields were trimmed. Yahoo's close is already
// split-adjusted, which is the basis pkg/finance/profiles.md names.
const yahooSplitFixture = "testdata/yahoo_chart_aapl_split.json"

// yahooLargestDailyMove is the largest one-day close-to-close move, as a
// fraction, across the bars in order.
func yahooLargestDailyMove(ohlcs []*pb.OHLC) float64 {
	largest := 0.0
	for index := 1; index < len(ohlcs); index++ {
		move := math.Abs(ohlcs[index].Close/ohlcs[index-1].Close - 1)
		if move > largest {
			largest = move
		}
	}
	return largest
}

// Quant's AAPL 2020-08-31 golden: no one-day move beyond ±20%.
func TestYahooGetOHLCKeepsTheAAPLSplitSmooth(t *testing.T) {
	t.Parallel()
	client := yahooFixtureServer(t, yahooSplitFixture)
	start := time.Date(2020, 8, 27, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, 9, 2, 0, 0, 0, 0, time.UTC)

	ohlcs, err := client.GetOHLC(context.Background(), "aapl", start, &end)
	require.NoError(t, err)

	require.Len(t, ohlcs, 4)
	require.Less(t, yahooLargestDailyMove(ohlcs), 0.20)
	// The same split-adjusted closes Alpaca gives in public #210.
	require.InDelta(t, 124.8075, ohlcs[1].Close, 1e-4)
	require.InDelta(t, 129.04, ohlcs[2].Close, 1e-4)
	require.Equal(t, "2020-08-31", ohlcs[2].Timestamp.AsTime().UTC().Format("2006-01-02"))
	require.Equal(t, uint64(187630000), ohlcs[1].Volume)
}

// The golden check above must be able to fail. Undoing the recorded split on
// the bars before it gives the raw closes, where the split is a fake drop of
// about 75%.
func TestYahooRawBarsShowTheSplitAsAFakeDrop(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(yahooSplitFixture)
	require.NoError(t, err)
	var recorded struct {
		Chart struct {
			Result []struct {
				Timestamp []int64 `json:"timestamp"`
				Events    struct {
					Splits map[string]struct {
						Date        int64   `json:"date"`
						Numerator   float64 `json:"numerator"`
						Denominator float64 `json:"denominator"`
					} `json:"splits"`
				} `json:"events"`
				Indicators struct {
					Quote []struct {
						Close []float64 `json:"close"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	require.NoError(t, json.Unmarshal(body, &recorded))
	result := recorded.Chart.Result[0]
	require.Len(t, result.Events.Splits, 1)

	ohlcs := make([]*pb.OHLC, 0, len(result.Timestamp))
	for index, ts := range result.Timestamp {
		closePx := result.Indicators.Quote[0].Close[index]
		for _, split := range result.Events.Splits {
			if ts < split.Date {
				closePx *= split.Numerator / split.Denominator
			}
		}
		ohlcs = append(ohlcs, &pb.OHLC{Close: closePx})
	}
	require.InDelta(t, 0.74, yahooLargestDailyMove(ohlcs), 0.01)
}
