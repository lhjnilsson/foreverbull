package marketdata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/entity"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
)

type YahooClient struct {
	cookie string
	crumb  string
}

func NewYahooClient() (supplier.Marketdata, error) {
	yc := &YahooClient{}
	rsp, err := yc.doRequest("https://fc.yahoo.com")
	if err != nil {
		return nil, err
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
		return nil, err
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
		return nil, err
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

func (y *YahooClient) GetAsset(symbol string) (*entity.Asset, error) {
	url := "https://query2.finance.yahoo.com/v10/finance/quoteSummary/" + strings.ToUpper(symbol)
	resp, err := y.doRequest(url, "modules=quoteType")
	if err != nil {
		return nil, fmt.Errorf("error: %v", err)
	}
	defer resp.Body.Close()

	data := AssetResponse{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}
	if data.QuoteSummary.Error.Code != "" {

		return nil, fmt.Errorf("%v", data.QuoteSummary.Error.Description)
	}

	asset := entity.Asset{
		Symbol: data.QuoteSummary.Result[0].QuoteType.Symbol,
		Name:   data.QuoteSummary.Result[0].QuoteType.LongName,
		Title:  data.QuoteSummary.Result[0].QuoteType.ShortName,
		Type:   data.QuoteSummary.Result[0].QuoteType.QuoteType,
	}
	return &asset, nil
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

func (y *YahooClient) GetOHLC(symbol string, start, end time.Time) (*[]entity.OHLC, error) {
	startUnix := start.Unix()
	endUnix := end.Unix()

	url := "https://query2.finance.yahoo.com/v8/finance/chart/" + strings.ToUpper(symbol)
	resp, err := y.doRequest(url, fmt.Sprintf("period1=%d", startUnix), fmt.Sprintf("period2=%d", endUnix), fmt.Sprintf("interval=%s", "1d"))
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()

	data := OHLCResponse{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}
	if data.Chart.Error.Code != "" {
		return nil, fmt.Errorf("fail to get OHLC data for symbol %s: %v", symbol, data.Chart.Error.Description)
	}

	ohlcs := make([]entity.OHLC, 0)
	for i, ts := range data.Chart.Result[0].Timestamp {
		ohlcs = append(ohlcs, entity.OHLC{
			Time:  time.Unix(ts, 0),
			Open:  data.Chart.Result[0].Indicators.Quote[0].Open[i],
			High:  data.Chart.Result[0].Indicators.Quote[0].High[i],
			Low:   data.Chart.Result[0].Indicators.Quote[0].Low[i],
			Close: data.Chart.Result[0].Indicators.Quote[0].Close[i],
		})
	}
	return &ohlcs, nil

}
