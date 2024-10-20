package marketdata

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AlpacaClient struct {
	client   *alpaca.Client
	mdclient *marketdata.Client
}

func NewAlpacaClient() (*AlpacaClient, error) {
	client := alpaca.NewClient(alpaca.ClientOpts{
		BaseURL:   environment.GetAlpacaBaseURL(),
		APIKey:    environment.GetAlpacaAPIKey(),
		APISecret: environment.GetAlpacaAPISecret(),
	})
	mdclient := marketdata.NewClient(marketdata.ClientOpts{
		APIKey:    environment.GetAlpacaAPIKey(),
		APISecret: environment.GetAlpacaAPISecret(),
	})

	return &AlpacaClient{client: client, mdclient: mdclient}, nil
}

func (a *AlpacaClient) GetAsset(symbol string) (*pb.Asset, error) {
	asset, err := a.client.GetAsset(symbol)
	if err != nil {
		return nil, fmt.Errorf("error getting asset: %w", err)
	}

	storeAsset := pb.Asset{
		Symbol: asset.Symbol,
		Name:   asset.Name,
	}

	return &storeAsset, nil
}

func (a *AlpacaClient) GetIndex(_ string) ([]*pb.Asset, error) {
	var storeAssets []*pb.Asset
	return storeAssets, nil
}

func (a *AlpacaClient) GetOHLC(symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error) {
	ohlcs := make([]*pb.OHLC, 0)

	ohlc, err := a.mdclient.GetBars(symbol, marketdata.GetBarsRequest{
		Start: start,
		End:   *end,
	})
	if err != nil {
		alpacaErr := alpaca.APIError{}
		if errors.Is(err, &alpacaErr) {
			if alpacaErr.StatusCode == http.StatusUnprocessableEntity {
				var innerErr error

				e := end.Add(-15 * time.Minute)

				ohlc, innerErr = a.mdclient.GetBars(symbol, marketdata.GetBarsRequest{
					Start: start,
					End:   e,
				})
				if innerErr != nil {
					return nil, fmt.Errorf("error getting bars: %w", innerErr)
				}
			}
		}
	}

	for _, bar := range ohlc {
		ohlc := pb.OHLC{
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    int32(bar.Volume),
			Timestamp: timestamppb.New(bar.Timestamp),
		}

		ohlcs = append(ohlcs, &ohlc)
	}

	return ohlcs, nil
}
