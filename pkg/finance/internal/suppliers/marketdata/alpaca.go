package marketdata

import (
	"context"
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
	opts marketdata.ClientOpts
}

// NewAlpacaBars builds a bars client. An empty key or secret fails before any request.
func NewAlpacaBars(apiKey, apiSecret string) (*AlpacaBars, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiSecret) == "" {
		return nil, errors.New("alpaca price source requires an api key and secret")
	}
	return &AlpacaBars{opts: marketdata.ClientOpts{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}}, nil
}

// alpacaTimeout is the SDK's own default request timeout, kept for the
// per-call client below.
const alpacaTimeout = 10 * time.Second

// alpacaFetchBudget bounds one GetOHLC, retries and pages included. It stays
// below Sync's 90s idle-in-transaction timeout. Tests shorten it.
var alpacaFetchBudget = 60 * time.Second

// client returns an SDK client whose requests carry ctx. The SDK takes no
// context, so each call gets its own client with a transport that attaches
// ctx to every request: a cancelled download stops the Alpaca fetch.
func (a *AlpacaBars) client(ctx context.Context) *marketdata.Client {
	opts := a.opts
	base := http.DefaultTransport
	timeout := alpacaTimeout
	if opts.HTTPClient != nil {
		if opts.HTTPClient.Transport != nil {
			base = opts.HTTPClient.Transport
		}
		timeout = opts.HTTPClient.Timeout
	}
	opts.HTTPClient = &http.Client{Timeout: timeout, Transport: &ctxTransport{ctx: ctx, base: base}}
	return marketdata.NewClient(opts)
}

// ctxTransport sends each request with ctx, so cancelling ctx aborts it.
type ctxTransport struct {
	ctx  context.Context //nolint:containedctx // the SDK has no per-request ctx
	base http.RoundTripper
}

func (t *ctxTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.base.RoundTrip(req.WithContext(t.ctx)) //nolint:wrapcheck // passed through as is
}

func (a *AlpacaBars) GetAsset(context.Context, string) (*pb.Asset, error) {
	return nil, errors.New("alpaca price source does not serve asset metadata")
}

func (a *AlpacaBars) SearchAsset(context.Context, string) ([]*pb.Asset, error) {
	return nil, errors.New("alpaca price source does not serve asset search")
}

func (a *AlpacaBars) GetIndex(context.Context, string) ([]*pb.Asset, error) {
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

func (a *AlpacaBars) GetOHLC(ctx context.Context, symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error) {
	endAt := time.Now().UTC()
	if end != nil {
		endAt = *end
	}
	// Bars are returned for the asked ticker; only the request uses Alpaca's form.
	symbol = alpacaSymbol(symbol)
	// The SDK retries 429 and 500 up to ten times, a second apart, each try
	// with its own timeout, so one fetch could outlast Sync's idle timeout
	// when it runs under the symbol lock. A failed request isn't retried, so
	// ending ctx bounds the whole fetch.
	ctx, cancel := context.WithTimeout(ctx, alpacaFetchBudget)
	defer cancel()
	md := a.client(ctx)
	bars, err := md.GetBars(symbol, alpacaDailyBars(start, endAt))
	if err != nil {
		var apiErr *alpaca.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity {
			bars, err = md.GetBars(symbol, alpacaDailyBars(start, endAt.Add(-15*time.Minute)))
			if err != nil {
				return nil, fmt.Errorf("error getting bars: %w", err)
			}
		} else {
			return nil, fmt.Errorf("error getting bars: %w", err)
		}
	}

	ohlcs := make([]*pb.OHLC, 0, len(bars))
	for _, bar := range bars {
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
