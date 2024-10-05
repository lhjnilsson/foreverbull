package stream

import (
	"github.com/lhjnilsson/foreverbull/internal/stream"
)

type IngestCommand struct {
	Symbols []string `json:"symbols"`
	Start   string   `json:"start"`
	End     string   `json:"end"`
}

func NewIngestCommand(symbols []string, start, end string) (stream.Message, error) {
	entity := &IngestCommand{
		Symbols: symbols,
		Start:   start,
		End:     end,
	}
	return stream.NewMessage("finance", "marketdata", "ingest", entity)
}
