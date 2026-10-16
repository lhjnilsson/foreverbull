package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	bufferSize = 128
)

// defaultYahooChartURL is the chart endpoint; the symbol is appended to it.
const defaultYahooChartURL = "https://query2.finance.yahoo.com/v8/finance/chart/"

type YahooClient struct {
	cookie string
	crumb  string
	// chartURL replaces defaultYahooChartURL so unit tests stay off the network.
	chartURL string
}

func NewYahooClient() (supplier.Marketdata, error) {
	yahoo := &YahooClient{}

	rsp, err := yahoo.doRequest("https://fc.yahoo.com")
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}

	defer rsp.Body.Close()

	yahoo.cookie = rsp.Header.Get("Set-Cookie")

	rsp, err = yahoo.doRequest("https://query2.finance.yahoo.com/v1/test/getcrumb")
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	buffer := make([]byte, bufferSize)

	length, err := rsp.Body.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	yahoo.crumb = string(buffer[:length])

	return yahoo, nil
}

func (y *YahooClient) doRequest(url string, params ...string) (*http.Response, error) {
	if y.crumb != "" {
		url += "?crumb=" + y.crumb
	}

	for _, param := range params {
		url += "&" + param
	}

	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	if y.cookie != "" {
		req.Header.Add("Cookie", y.cookie)
	}

	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	return rsp, nil
}

type AssetResponse struct {
	QuoteSummary struct {
		Result []struct {
			AssetProfile struct{} `json:"assetProfile"`
			QuoteType    struct {
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

func (y *YahooClient) getISIN(symbol string) (string, error) {
	url := "https://markets.businessinsider.com/ajax/markets-insider/search/" + symbol
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get index data for symbol %s: status %s", symbol, resp.Status)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	stocks, ok := data["stocks"]
	if !ok {
		return "", fmt.Errorf("response missing 'stocks' field")
	}
	stocksSlice, ok := stocks.([]interface{})
	if !ok {
		return "", fmt.Errorf("'stocks' field is not an array")
	}

	for _, stock := range stocksSlice {
		stockData, ok := stock.(map[string]interface{})
		if !ok {
			continue // Skip invalid stock entries
		}
		keywords, ok := stockData["keywords"].(string)
		if !ok {
			continue
		}

		parts := strings.Split(keywords, "|")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid keywords format for stock %s", symbol)
		}
		if strings.Compare(parts[0], symbol) == 0 {
			return parts[1], nil
		}
	}
	return "", fmt.Errorf("ISIN not found for symbol %s", symbol)
}

func (y *YahooClient) GetAsset(symbol string) (*pb.Asset, error) {
	url := "https://query2.finance.yahoo.com/v10/finance/quoteSummary/" + strings.ToUpper(symbol)

	resp, err := y.doRequest(url, "modules=quoteType")
	if err != nil {
		return nil, fmt.Errorf("error: %w", err)
	}
	defer resp.Body.Close()

	data := AssetResponse{}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if data.QuoteSummary.Error.Code != "" {
		return nil, fmt.Errorf("%v", data.QuoteSummary.Error.Description)
	}

	if len(data.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	asset := pb.Asset{
		Symbol: data.QuoteSummary.Result[0].QuoteType.Symbol,
		Name:   data.QuoteSummary.Result[0].QuoteType.LongName,
		ISIN:   "",
	}

	if data.QuoteSummary.Result[0].QuoteType.QuoteType == "EQUITY" {
		asset.ISIN, err = y.getISIN(asset.Symbol)
		if err != nil {
			log.Info().Msgf("error getting isin for symbol %s: %v", asset.Symbol, err)
		}
	}

	return &asset, nil
}

func (m *YahooClient) SearchAsset(q string) ([]*pb.Asset, error) {
	url := "https://query2.finance.yahoo.com/v1/finance/search?q=" + q + "&lang=en-US&region=US&quotesCount=6&newsCount=0&listsCount=0&quotesQueryId=tss_match_phrase_query"
	resp, err := m.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fail to search asset: %s", resp.Status)
	}
	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	assets := make([]*pb.Asset, 0)
	for _, quote := range data["quotes"].([]interface{}) {
		quoteMap := quote.(map[string]interface{})
		symbol := quoteMap["symbol"].(string)
		a, err := m.GetAsset(symbol)
		if err != nil {
			return nil, fmt.Errorf("error getting asset: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, nil
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
		return nil, fmt.Errorf("error: %w", err)
	}

	result := IndexResponse{}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fail to get index data for symbol %s: %s", symbol, resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	assets := make([]*pb.Asset, 0)
	assetChan := make(chan *pb.Asset)

	group, _ := errgroup.WithContext(context.Background())
	for _, component := range result.Summary.Result[0].Components.Components {
		group.Go(func() error {
			a, err := y.GetAsset(component)
			if err != nil {
				return fmt.Errorf("error getting asset: %w", err)
			}
			assetChan <- a

			return nil
		})
	}

	group.Go(func() error {
		a, err := y.GetAsset(symbol)
		if err != nil {
			return fmt.Errorf("error getting asset: %w", err)
		}
		assetChan <- a

		return nil
	})

	go func() {
		for a := range assetChan {
			assets = append(assets, a)
		}
	}()

	if err := group.Wait(); err != nil {
		return nil, fmt.Errorf("error getting assets: %w", err)
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
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []uint64  `json:"volume"`
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
	chartURL := y.chartURL
	if chartURL == "" {
		chartURL = defaultYahooChartURL
	}
	url := chartURL + strings.ToUpper(symbol)
	params := []string{}
	params = append(params, fmt.Sprintf("period1=%d", start.Unix()))

	if end != nil {
		params = append(params, fmt.Sprintf("period2=%d", end.Unix()))
	} else {
		params = append(params, fmt.Sprintf("period2=%d", time.Now().Unix()))
	}

	params = append(params, "interval=1d")

	resp, err := y.doRequest(url, params...)
	if err != nil {
		return nil, nil
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}
	if err := blockedError("yahoo", strings.ToUpper(symbol), resp.Header.Get("Content-Type"), body); err != nil {
		return nil, err
	}

	data := OHLCResponse{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if data.Chart.Error.Code != "" {
		return nil, fmt.Errorf("fail to get OHLC data for symbol %s: %v", symbol, data.Chart.Error.Description)
	}

	ohlcs := make([]*pb.OHLC, 0)
	for index, ts := range data.Chart.Result[0].Timestamp {
		ohlcs = append(ohlcs, &pb.OHLC{
			Timestamp: timestamppb.New(time.Unix(ts, 0)),
			Open:      data.Chart.Result[0].Indicators.Quote[0].Open[index],
			High:      data.Chart.Result[0].Indicators.Quote[0].High[index],
			Low:       data.Chart.Result[0].Indicators.Quote[0].Low[index],
			Close:     data.Chart.Result[0].Indicators.Quote[0].Close[index],
			Volume:    data.Chart.Result[0].Indicators.Quote[0].Volume[index],
		})
	}

	return ohlcs, nil
}
