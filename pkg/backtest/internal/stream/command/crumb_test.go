package command

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

func TestFailedLookupDoesNotLogYahooCrumb(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	cause := fmt.Errorf("fail to get OHLC data for symbol AAPL: %w", &url.Error{
		Op:  "Get",
		URL: "https://query2.finance.yahoo.com/v8/finance/chart/AAPL?crumb=s3cr3tCrumb&interval=1d",
		Err: errors.New("dial tcp: connection refused"),
	})
	err := &finance.LookupError{Kind: finance.LookupNetwork, Provider: "yahoo", Symbol: "AAPL", Err: cause}

	miss, unexpected := failedLookup("yahoo", "AAPL", true, err)

	require.NoError(t, unexpected)
	require.NotNil(t, miss)
	require.Equal(t, "AAPL", miss.symbol)
	require.Contains(t, buf.String(), "listing or delisting lookup failed")
	require.Contains(t, buf.String(), "query2.finance.yahoo.com/v8/finance/chart/AAPL")
	require.NotContains(t, buf.String(), "crumb=")
	require.NotContains(t, buf.String(), "s3cr3tCrumb")
}
