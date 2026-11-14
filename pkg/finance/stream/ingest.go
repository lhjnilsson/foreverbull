package stream

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
)

type IngestCommand struct {
	Calendar string   `json:"calendar"`
	Symbols  []string `json:"symbols"`
	Start    string   `json:"start"`
	End      *string  `json:"end"`
	Provider string   `json:"provider,omitempty"`
	Profile  string   `json:"profile,omitempty"`
	// Trading is set by a trading session. The provider is then required:
	// an empty name is not read as yahoo.
	Trading bool `json:"trading,omitempty"`
}

func NewIngestCommand(calendar string, symbols []string, start string, end *string) (stream.Message, error) {
	entity := &IngestCommand{
		Calendar: calendar,
		Symbols:  symbols,
		Start:    start,
		End:      end,
	}

	msg, err := stream.NewMessage("finance", "marketdata", "ingest", entity)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %w", err)
	}
	return msg, nil
}
