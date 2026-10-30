package api

import "time"

// SetDownloadTimeLimit shortens downloadTimeLimit for a test and returns the
// restore. Suite tests run one at a time, so nothing reads it meanwhile.
func SetDownloadTimeLimit(limit time.Duration) func() {
	previous := downloadTimeLimit
	downloadTimeLimit = limit
	return func() { downloadTimeLimit = previous }
}
