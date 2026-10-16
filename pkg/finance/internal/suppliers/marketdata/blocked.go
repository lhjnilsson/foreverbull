package marketdata

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"strings"
)

// ErrProviderBlocked matches every BlockedError with errors.Is.
var ErrProviderBlocked = errors.New("price provider blocked the download")

// BlockedError is returned when a provider answers with a bot-check or
// access-denied page instead of data. The download stops at the first one,
// because every later symbol would hit the same block.
type BlockedError struct {
	Provider string
	Symbol   string
}

// Error is the sentence shown to the user, so it names the provider, the
// symbol, and the provider to retry with.
func (e *BlockedError) Error() string {
	return fmt.Sprintf("%s blocked the download for %s (it returned a bot-check page instead of data). Retry with --provider %s.",
		providerDisplayName(e.Provider), e.Symbol, alternativeProvider(e.Provider))
}

func (e *BlockedError) Is(target error) bool {
	return target == ErrProviderBlocked
}

func providerDisplayName(provider string) string {
	switch provider {
	case "stooq":
		return "Stooq"
	case "yahoo":
		return "Yahoo"
	case "alpaca":
		return "Alpaca"
	default:
		return provider
	}
}

// alternativeProvider is the price source to suggest after a block. Alpaca
// needs a profile, so a blocked Stooq or Alpaca points at Yahoo.
func alternativeProvider(provider string) string {
	if provider == "yahoo" {
		return "stooq"
	}
	return "yahoo"
}

// isBlockedPage reports whether a response is a bot-check or access-denied
// page rather than data. Stooq answers a blocked client either with an HTML
// page or with HTTP 200 text/plain "Access denied", so both count.
func isBlockedPage(contentType string, body []byte) bool {
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
			return true
		}
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] == '<' {
		return true
	}
	return strings.HasPrefix(strings.ToLower(string(trimmed)), "access denied")
}
