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

// GetIndex joins one error per failed member; every crumb is redacted,
// not only the first.
func TestURLErrorRedactsEveryJoinedURLError(t *testing.T) {
	joined := fmt.Errorf("error getting assets: %w", errors.Join(
		fmt.Errorf("component AAPL: %w", yahooURLError()),
		fmt.Errorf("component MSFT: %w", &url.Error{
			Op: "Get", URL: "https://query2.finance.yahoo.com/v10/finance/quoteSummary/MSFT?crumb=0therCrumb",
			Err: errors.New("EOF"),
		}),
	))
	got := redact.URLError(joined).Error()
	for _, raw := range []string{"crumb", "s3cr3tCrumb", "0therCrumb", "frag"} {
		assert.NotContains(t, got, raw)
	}
	assert.Contains(t, got, "component AAPL: ")
	assert.Contains(t, got, `component MSFT: Get "https://query2.finance.yahoo.com/v10/finance/quoteSummary/MSFT": EOF`)
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
