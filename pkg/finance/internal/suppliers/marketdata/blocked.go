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
	// Refused is true when the provider answered with a plain-text refusal
	// such as Stooq's "Access denied" rather than an HTML bot-check page.
	Refused bool
}

// Error is the sentence shown to the user, so it names the provider, the
// symbol, and the provider to retry with.
func (e *BlockedError) Error() string {
	reason := "it returned a bot-check page instead of data"
	if e.Refused {
		reason = "it refused the request instead of returning data"
	}
	return fmt.Sprintf("%s blocked the download for %s (%s). Retry with --provider %s.",
		providerDisplayName(e.Provider), e.Symbol, reason, alternativeProvider(e.Provider))
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

// blockKind says how a response blocked the client.
type blockKind int

const (
	notBlocked blockKind = iota
	blockedByBotCheck
	blockedByRefusal
)

// classifyBlock reports whether a response is a bot-check page or a
// plain-text refusal rather than data. Stooq answers a blocked client either
// with an HTML page or with HTTP 200 text/plain "Access denied", and the
// user sees a different reason for each.
func classifyBlock(contentType string, body []byte) blockKind {
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
			return blockedByBotCheck
		}
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return notBlocked
	}
	if trimmed[0] == '<' {
		return blockedByBotCheck
	}
	if strings.HasPrefix(strings.ToLower(string(trimmed)), "access denied") {
		return blockedByRefusal
	}
	return notBlocked
}

// blockedError returns a BlockedError for a blocked response, or nil.
func blockedError(provider, symbol, contentType string, body []byte) error {
	switch classifyBlock(contentType, body) {
	case blockedByBotCheck:
		return &BlockedError{Provider: provider, Symbol: symbol}
	case blockedByRefusal:
		return &BlockedError{Provider: provider, Symbol: symbol, Refused: true}
	case notBlocked:
		return nil
	default:
		return nil
	}
}
