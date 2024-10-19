package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type YahooClient struct {
	cookie string
	crumb  string
}

func NewYahooClient() (supplier.Marketdata, error) {
	yc := &YahooClient{}

	rsp, err := yc.doRequest("https://fc.yahoo.com")
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}

	defer rsp.Body.Close()

	yc.cookie = rsp.Header.Get("Set-Cookie")

	rsp, err = yc.doRequest("https://query2.finance.yahoo.com/v1/test/getcrumb")
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	buffer := make([]byte, 128)

	n, err := rsp.Body.Read([]byte(buffer))
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	yc.crumb = string(buffer[:n])

	return yc, nil
}

func (y *YahooClient) doRequest(url string, params ...string) (*http.Response, error) {
	if y.crumb != "" {
		url += "?crumb=" + y.crumb
	}

	for _, param := range params {
		url += "&" + param
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.95 Safari/537.36'")

	if y.cookie != "" {
		req.Header.Add("Cookie", y.cookie)
	}

	return http.DefaultClient.Do(req)
}

type AssetResponse struct {
	QuoteSummary struct {
		Result []struct {
			AssetProfile struct {
			} `json:"assetProfile"`
			QuoteType struct {
				Exchange  string `json:"exchange"`
				Symbol    string `json:"symbol"`
				QuoteType string `json:"quoteType"`
				ShortName string `json:"shortName"`
				LongName  string `json:"longName"`
			} `json:"quoteType"`
		} `json:"result"`
		Error struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"quoteSummary"`
}

func (y *YahooClient) GetAsset(symbol string) (*pb.Asset, error) {
	url := "https://query2.finance.yahoo.com/v10/finance/quoteSummary/" + strings.ToUpper(symbol)

	resp, err := y.doRequest(url, "modules=quoteType")
	if err != nil {
		return nil, fmt.Errorf("error: %v", err)
	}

	defer resp.Body.Close()

	data := AssetResponse{}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	if data.QuoteSummary.Error.Code != "" {
		return nil, fmt.Errorf("%v", data.QuoteSummary.Error.Description)
	}

	asset := pb.Asset{
		Symbol: data.QuoteSummary.Result[0].QuoteType.Symbol,
		Name:   data.QuoteSummary.Result[0].QuoteType.LongName,
	}

	return &asset, nil
}

type IndexResponse struct {
	Summary struct {
		Result []struct {
			Components struct {
				Components []string `json:"components"`
			} `json:"components"`
		} `json:"result"`
	} `json:"quoteSummary"`
}

func (y *YahooClient) GetIndex(symbol string) ([]*pb.Asset, error) {
	url := "https://query2.finance.yahoo.com/v10/finance/quoteSummary/" + strings.ToUpper(symbol)

	resp, err := y.doRequest(url, "modules=components%2CsummaryDetail")
	if err != nil {
		return nil, fmt.Errorf("error: %v", err)
	}

	result := IndexResponse{}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	assets := make([]*pb.Asset, 0)
	assetChan := make(chan *pb.Asset)

	g, _ := errgroup.WithContext(context.Background())
	for _, component := range result.Summary.Result[0].Components.Components {
		g.Go(func() error {
			a, err := y.GetAsset(component)
			if err != nil {
				return fmt.Errorf("error getting asset: %v", err)
			}
			assetChan <- a

			return nil
		})
	}

	g.Go(func() error {
		a, err := y.GetAsset(symbol)
		if err != nil {
			return fmt.Errorf("error getting asset: %v", err)
		}
		assetChan <- a

		return nil
	})

	go func() {
		for a := range assetChan {
			assets = append(assets, a)
		}
	}()

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("error getting assets: %v", err)
	}

	return assets, nil
}

type OHLCResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency string `json:"currency"`
				Symbol   string `json:"symbol"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open  []float64 `json:"open"`
					High  []float64 `json:"high"`
					Low   []float64 `json:"low"`
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		}
	}
}

func (y *YahooClient) GetOHLC(symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error) {
	url := "https://query2.finance.yahoo.com/v8/finance/chart/" + strings.ToUpper(symbol)
	params := []string{}
	params = append(params, fmt.Sprintf("period1=%d", start.Unix()))

	if end != nil {
		params = append(params, fmt.Sprintf("period2=%d", end.Unix()))
	} else {
		params = append(params, fmt.Sprintf("period2=%d", time.Now().Unix()))
	}

	params = append(params, fmt.Sprintf("interval=%s", "1d"))

	resp, err := y.doRequest(url, params...)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()

	data := OHLCResponse{}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	if data.Chart.Error.Code != "" {
		return nil, fmt.Errorf("fail to get OHLC data for symbol %s: %v", symbol, data.Chart.Error.Description)
	}

	ohlcs := make([]*pb.OHLC, 0)
	for i, ts := range data.Chart.Result[0].Timestamp {
		ohlcs = append(ohlcs, &pb.OHLC{
			Timestamp: timestamppb.New(time.Unix(ts, 0)),
			Open:      data.Chart.Result[0].Indicators.Quote[0].Open[i],
			High:      data.Chart.Result[0].Indicators.Quote[0].High[i],
			Low:       data.Chart.Result[0].Indicators.Quote[0].Low[i],
			Close:     data.Chart.Result[0].Indicators.Quote[0].Close[i],
		})
	}

	return ohlcs, nil
}
