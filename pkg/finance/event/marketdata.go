package event

import (
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
	return stream.NewMessage("finance", "marketdata", "download", entity)
}
