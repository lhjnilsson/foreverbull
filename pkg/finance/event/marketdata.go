package event

import (
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/stream"
)

const MarketdataDownloadTopic = "foreverbull.finance.marketdata.download"

type MarketdataDownloadMessage struct {
	Symbols []string  `json:"symbols"`
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
}

func NewMarketdataDownloadCommand(entity MarketdataDownloadMessage) (stream.Message, error) {
	msg, err := stream.NewMessage("finance", "marketdata", "download", entity)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	return msg, nil
}
