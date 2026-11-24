package marketdata

import (
	"errors"
	"net/http"
)

// ErrUnknownSymbol matches a GetAsset error when Yahoo answered that it
// has no quote for the ticker: a quoteSummary error code of "Not Found",
// or HTTP 404. A 200 with no error code and an empty result is not that
// answer, and neither is a transport error, another status, a bot check,
// or a timeout. The provider's own text stays on the error for the log.
// Callers that show a sentence use their own wording.
var ErrUnknownSymbol = errors.New("yahoo does not know the symbol")

// unknownSymbolError keeps Yahoo's description and matches ErrUnknownSymbol.
type unknownSymbolError struct{ text string }

func (e *unknownSymbolError) Error() string { return e.text }

func (e *unknownSymbolError) Is(target error) bool { return target == ErrUnknownSymbol }

func unknownSymbol(text string) error {
	if text == "" {
		text = "no results found"
	}
	return &unknownSymbolError{text: text}
}

// unknownQuoteSummary reports Yahoo's answer that the ticker has no quote.
// That is an explicit "Not Found" code, or any HTTP 404 that is not a bot
// check. A 200 with no error code and an empty result is not that answer:
// it used to be a failed download, and treating it as unknown would tell
// the user to remove a symbol Yahoo did not reject.
func unknownQuoteSummary(status int, data AssetResponse) (bool, string) {
	if status == http.StatusNotFound || data.QuoteSummary.Error.Code == yahooChartNotFound {
		return true, data.QuoteSummary.Error.Description
	}
	return false, ""
}
