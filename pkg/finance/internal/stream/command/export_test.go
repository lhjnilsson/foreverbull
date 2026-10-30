package command

import "time"

// SetIngestTimeLimit shortens ingestTimeLimit for a test and returns the
// restore. Suite tests run one at a time, so nothing reads it meanwhile.
func SetIngestTimeLimit(limit time.Duration) func() {
	previous := ingestTimeLimit
	ingestTimeLimit = limit
	return func() { ingestTimeLimit = previous }
}
