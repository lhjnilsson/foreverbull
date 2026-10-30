package redact_test

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/redact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const crumbURL = "https://query2.finance.yahoo.com/v1/finance/quoteType/AAPL?crumb=s3cr3tCrumb#frag"

func yahooURLError() error {
	return fmt.Errorf("yahoo get asset: %w", &url.Error{
		Op:  "Get",
		URL: crumbURL,
		Err: errors.New("dial tcp: connection refused"),
	})
}

func TestURLErrorStripsQueryAndFragment(t *testing.T) {
	got := redact.URLError(yahooURLError())

	assert.NotContains(t, got.Error(), "crumb")
	assert.NotContains(t, got.Error(), "s3cr3tCrumb")
	assert.NotContains(t, got.Error(), "frag")
	assert.Equal(t,
		`yahoo get asset: Get "https://query2.finance.yahoo.com/v1/finance/quoteType/AAPL": dial tcp: connection refused`,
		got.Error())
}

func TestURLErrorLeavesOtherErrors(t *testing.T) {
	plain := errors.New("boom")
	assert.Same(t, plain, redact.URLError(plain))
	assert.NoError(t, redact.URLError(nil))
}

// The scrubbed error is new text: its chain is gone, so callers check
// sentinels and types on the original error.
func TestURLErrorDropsTheChain(t *testing.T) {
	sentinel := errors.New("sentinel")
	err := fmt.Errorf("%w: %w", sentinel, yahooURLError())

	got := redact.URLError(err)

	var urlErr *url.Error
	require.NotErrorIs(t, got, sentinel)
	require.NotErrorAs(t, got, &urlErr)
}

func TestURLCutsUnparsableURL(t *testing.T) {
	assert.Equal(t, "https://x.example/a", redact.URL("https://x.example/a?crumb=s3cr3t"))
	assert.Equal(t, "%zz", redact.URL("%zz?crumb=s3cr3t"))
}
