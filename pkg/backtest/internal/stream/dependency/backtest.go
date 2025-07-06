package dependency

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/rs/zerolog/log"
)

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

type ETFLookup struct {
	FinanceAPI financeServicePB.MarketdataClient
}

func (e *ETFLookup) Get(baseurl string, years ...string) error {
	req, err := http.NewRequest("GET", baseurl, nil)
	if err != nil {
		panic(err)
	}
	q := req.URL.Query()
	q.Add("tab", "all")
	q.Add("fileType", "json")
	q.Add("asOfDate", "20220104")
	req.URL.RawQuery = q.Encode()

	var assets []Asset
	for _, _ = range years {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		if resp.StatusCode != 200 {
			panic(resp.Status)
		}

		defer resp.Body.Close()
		var data Response

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		body = trapBOM(body)
		err = json.Unmarshal(body, &data)
		if err != nil {
			panic(err)
		}

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
	}

	for _, asset := range assets {
		if asset.AssetType != "Equity" {
			continue
		}
		rsp, err := e.FinanceAPI.GetAsset(context.Background(), &financeServicePB.GetAssetRequest{
			Identifier: &financeServicePB.GetAssetRequest_Symbol{
				Symbol: asset.Symbol,
			},
		})
		if err != nil {
			return err
		}
		if rsp.Asset.ISIN != asset.ISIN {
			log.Warn().Msgf("NAME: %s, ISIN: %s", rsp.Asset.Name, asset.Name)
		}
	}
	return nil
}
