package marketdata

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AlpacaBars downloads daily bars with a profile's key. Asset metadata stays on Yahoo.
type AlpacaBars struct {
	md *marketdata.Client
	// now replaces the clock, so tests can fix today's session.
	now func() time.Time
}

// NewAlpacaBars builds a bars client. An empty key or secret fails before any request.
func NewAlpacaBars(apiKey, apiSecret string) (*AlpacaBars, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiSecret) == "" {
		return nil, errors.New("alpaca price source requires an api key and secret")
	}
	return &AlpacaBars{md: marketdata.NewClient(marketdata.ClientOpts{
		APIKey:    apiKey,
		APISecret: apiSecret,
	})}, nil
}

func (a *AlpacaBars) GetAsset(string) (*pb.Asset, error) {
	return nil, errors.New("alpaca price source does not serve asset metadata")
}

func (a *AlpacaBars) SearchAsset(string) ([]*pb.Asset, error) {
	return nil, errors.New("alpaca price source does not serve asset search")
}

func (a *AlpacaBars) GetIndex(string) ([]*pb.Asset, error) {
	return nil, errors.New("alpaca price source does not serve index membership")
}

// alpacaAdjustment is the corporate-action basis of stored Alpaca bars.
// The SDK sends adjustment=raw when the field is empty, so a 4:1 split
// shows as a -75% day. Split-adjusted matches Yahoo's close (split, no
// dividends), the default provider; dividends are not adjusted, so a run
// stays a price return as on Yahoo (pkg/finance/profiles.md, adjustment).
const alpacaAdjustment = marketdata.AdjustmentSplit

// alpacaDailyBars names every field that changes a stored price: daily
// bars and the split adjustment. Neither is left to an SDK default.
func alpacaDailyBars(start, end time.Time) marketdata.GetBarsRequest {
	return marketdata.GetBarsRequest{
		TimeFrame:  marketdata.OneDay,
		Adjustment: alpacaAdjustment,
		Start:      start,
		End:        end,
	}
}

func (a *AlpacaBars) GetOHLC(symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error) {
	now := clockNow(a.now)
	endAt := now.UTC()
	if end != nil {
		endAt = *end
	}
	// Today's unfinished session is never asked for: while the market is
	// open, Alpaca's bar for today has prices and would be stored as final.
	// Alpaca's end is inclusive and a daily bar is stamped at midnight New
	// York, so a clamped fetch stops just before the limit.
	limit, ok := completedFetchEnd(start, endAt, now)
	if !ok {
		return []*pb.OHLC{}, nil
	}
	if limit.Before(endAt) {
		endAt = limit.Add(-time.Nanosecond)
	}
	// Bars are returned for the asked ticker; only the request uses Alpaca's form.
	symbol = alpacaSymbol(symbol)
	bars, err := a.md.GetBars(symbol, alpacaDailyBars(start, endAt))
	if err != nil {
		var apiErr *alpaca.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity {
			bars, err = a.md.GetBars(symbol, alpacaDailyBars(start, endAt.Add(-15*time.Minute)))
			if err != nil {
				return nil, fmt.Errorf("error getting bars: %w", err)
			}
		} else {
			return nil, fmt.Errorf("error getting bars: %w", err)
		}
	}

	last := lastCompletedSession(now)
	ohlcs := make([]*pb.OHLC, 0, len(bars))
	for _, bar := range bars {
		// Alpaca could send today's live bar even past the clamped end.
		if afterLastSession("alpaca", symbol, bar.Timestamp.In(xnysZone), last) {
			continue
		}
		ohlcs = append(ohlcs, &pb.OHLC{
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			Timestamp: timestamppb.New(bar.Timestamp),
		})
	}
	return ohlcs, nil
}

var _ supplier.Marketdata = (*AlpacaBars)(nil)
