package entity

import "time"

type Image struct {
	ID           string            `json:"id"`
	Tags         []string          `json:"tags"`
	Architecture string            `json:"architecture"`
	CreatedAt    time.Time         `json:"created_at"`
	Size         int64             `json:"size"`
	Labels       map[string]string `json:"labels"`
}
