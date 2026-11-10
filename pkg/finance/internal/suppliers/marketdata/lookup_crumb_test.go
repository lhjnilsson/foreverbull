package marketdata

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

func TestLookupTransportDoesNotLogYahooCrumb(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	err := fmt.Errorf("error making request: %w", &url.Error{
		Op:  "Get",
		URL: "https://query2.finance.yahoo.com/v1/finance/search?q=AAPL&crumb=s3cr3tCrumb",
		Err: errors.New("dial tcp: connection refused"),
	})
	got := lookupTransport("search", "AAPL", context.Background(), err)

	require.ErrorContains(t, got, "yahoo search for AAPL")
	require.NotContains(t, got.Error(), "crumb=")
	require.NotContains(t, got.Error(), "s3cr3tCrumb")
	require.Contains(t, buf.String(), "listing lookup request failed")
	require.Contains(t, buf.String(), "query2.finance.yahoo.com/v1/finance/search")
	require.NotContains(t, buf.String(), "crumb=")
	require.NotContains(t, buf.String(), "s3cr3tCrumb")
}
