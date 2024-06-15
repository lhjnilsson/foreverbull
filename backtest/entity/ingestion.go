package entity

import "time"

type IngestionStatusType string

const (
	IngestionStatusCreated   IngestionStatusType = "CREATED"
	IngestionStatusIngesting IngestionStatusType = "INGESTING"
	IngestionStatusCompleted IngestionStatusType = "COMPLETED"
	IngestionStatusError     IngestionStatusType = "ERROR"
)

type Ingestion struct {
	Name string `json:"name" mapstructure:"name"`

	Calendar string    `json:"calendar" mapstructure:"calendar" required:"true"`
	Start    time.Time `json:"start" mapstructure:"start" required:"true"`
	End      time.Time `json:"end" mapstructure:"end" required:"true"`
	Symbols  []string  `json:"symbols" mapstructure:"symbols" required:"true"`

	Statuses []IngestionStatus `json:"statuses"`
}

type IngestionStatus struct {
	Status     IngestionStatusType `json:"status"`
	Error      *string             `json:"message"`
	OccurredAt time.Time           `json:"occurred_at"`
}
