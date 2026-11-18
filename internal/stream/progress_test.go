package stream

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The progress message carries the provider and the wait, and a later
// report without them does not.
func TestProgressCarriesRateLimit(t *testing.T) {
	payload, err := marshalProgress(Progress{
		Text:        "Downloading prices: 1,240 of 10,000 symbols",
		Provider:    "yahoo",
		WaitSeconds: 30,
	})
	require.NoError(t, err)
	got, err := unmarshalProgress(payload)
	require.NoError(t, err)
	require.Equal(t, Progress{
		Text:        "Downloading prices: 1,240 of 10,000 symbols",
		Provider:    "yahoo",
		WaitSeconds: 30,
	}, got)

	cleared, err := marshalProgress(Progress{Text: "Downloading prices: 1,241 of 10,000 symbols"})
	require.NoError(t, err)
	got, err = unmarshalProgress(cleared)
	require.NoError(t, err)
	require.Equal(t, Progress{Text: "Downloading prices: 1,241 of 10,000 symbols"}, got)
}

func TestProgressRejectsAPayloadThatIsNotProgress(t *testing.T) {
	_, err := unmarshalProgress([]byte{0xff, 0xff, 0xff})
	require.ErrorContains(t, err, "error reading progress")
}
