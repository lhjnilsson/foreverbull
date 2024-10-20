package stream

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
)

type IngestCommand struct {
	Symbols []string `json:"symbols"`
	Start   string   `json:"start"`
	End     *string  `json:"end"`
}

func NewIngestCommand(symbols []string, start string, end *string) (stream.Message, error) {
	entity := &IngestCommand{
		Symbols: symbols,
		Start:   start,
		End:     end,
	}

	msg, err := stream.NewMessage("finance", "marketdata", "ingest", entity)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	return msg, nil
}
