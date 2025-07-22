package dependency

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/stream"
)

const (
	GetETFScraper       stream.Dependency = "get_etf_scraper"
	GetMarketdataClient stream.Dependency = "get_marketdata_client"
)

type ETFScraper interface {
	GetAssets(name string, years ...string) (map[string][]Asset, error)
}

func trapBOM(fileBytes []byte) []byte {
	trimmedBytes := bytes.Trim(fileBytes, "\xef\xbb\xbf")
	return trimmedBytes
}

type Response struct {
	Data [][]interface{} `json:"aaData"`
}

type Asset struct {
	Symbol    string
	Name      string
	Sector    string
	AssetType string
	ISIN      string
}

func NewETFScaper() ETFScraper {
	return &ISharesETFLoopup{
		products: map[string]string{
			"iShares S&P 100": "https://www.ishares.com/us/products/239723/ishares-sp-100-etf/1467271812596.ajax",
		},
	}
}

type ISharesETFLoopup struct {
	products map[string]string
}

func (e *ISharesETFLoopup) GetAssets(etf string, years ...string) (map[string][]Asset, error) {
	baseurl, found := e.products[etf]
	if !found {
		return nil, fmt.Errorf("ETF not found: %s", etf)
	}
	req, err := http.NewRequest("GET", baseurl, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	q := req.URL.Query()
	q.Add("tab", "all")
	q.Add("fileType", "json")
	retMap := make(map[string][]Asset)

	var data Response
	for _, year := range years {
		for day := 0; day <= 365; day++ {
			date, err := time.Parse("2006-01-02", fmt.Sprintf("%s-01-01", year))
			if err != nil {
				return nil, fmt.Errorf("error parsing date: %w", err)
			}
			date = date.AddDate(0, 0, day)
			q.Set("asOfDate", date.Format("20060102"))
			req.URL.RawQuery = q.Encode()
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("error doing request: %w", err)
			}
			if resp.StatusCode != 200 {
				return nil, fmt.Errorf("error status code: %d", resp.StatusCode)
			}

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("error reading body: %w", err)
			}
			body = trapBOM(body)
			err = json.Unmarshal(body, &data)
			if err != nil {
				return nil, fmt.Errorf("error unmarshalling json: %w", err)
			}
			if len(data.Data) == 0 {
				continue
			}
			var assets []Asset
			for _, row := range data.Data {
				asset := Asset{
					Symbol:    row[0].(string),
					Name:      row[1].(string),
					Sector:    row[2].(string),
					AssetType: row[3].(string),
					ISIN:      row[8].(string),
				}
				assets = append(assets, asset)
			}
			retMap[year] = assets
			break
		}
	}
	return retMap, nil
}
