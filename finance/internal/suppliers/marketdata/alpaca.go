package marketdata

import (
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/internal/environment"
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

func (a *AlpacaClient) GetAsset(symbol string) (*entity.Asset, error) {
	asset, err := a.client.GetAsset(symbol)
	if err != nil {
		return nil, err
	}
	storeAsset := entity.Asset{
		Symbol:      asset.Symbol,
		Name:        asset.Name,
		Title:       asset.Name,
		Type:        string(asset.Class),
		LastUpdated: time.Now(),
	}
	return &storeAsset, nil
}

func (a *AlpacaClient) GetOHLC(symbol string, start, end time.Time) (*[]entity.OHLC, error) {
	var ohlcs []entity.OHLC
	ohlc, err := a.mdclient.GetBars(symbol, marketdata.GetBarsRequest{
		Start: start,
		End:   end,
	})
	if err != nil {
		if err, ok := err.(*alpaca.APIError); ok {
			if err.StatusCode == 422 {
				var innerErr error
				end = end.Add(-15 * time.Minute)
				ohlc, innerErr = a.mdclient.GetBars(symbol, marketdata.GetBarsRequest{
					Start: start,
					End:   end,
				})
				if innerErr != nil {
					return nil, innerErr
				}
			}
		}
	}
	for _, bar := range ohlc {
		o := entity.OHLC{
			Open:   bar.Open,
			High:   bar.High,
			Low:    bar.Low,
			Close:  bar.Close,
			Volume: int(bar.Volume),
			Time:   time.Date(bar.Timestamp.Year(), bar.Timestamp.Month(), bar.Timestamp.Day(), 0, 0, 0, 0, time.UTC),
		}

		ohlcs = append(ohlcs, o)
	}
	return &ohlcs, nil
}
