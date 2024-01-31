package stream

import (
	"time"

	"github.com/lhjnilsson/foreverbull/internal/stream"
)

type IngestCommand struct {
	Symbols []string  `json:"symbols"`
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
}

func NewIngestCommand(symbols []string, start, end time.Time) (stream.Message, error) {
	entity := &IngestCommand{
		Symbols: symbols,
		Start:   start,
		End:     end,
	}
	return stream.NewMessage("finance", "marketdata", "ingest", entity)
}
