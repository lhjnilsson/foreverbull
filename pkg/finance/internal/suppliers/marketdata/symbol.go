package marketdata

import (
	"regexp"
	"strings"
)

// shareClassTickers lists share-class tickers that index holdings (iShares)
// write without a separator. They cannot be told apart from an ordinary
// ticker by their letters alone (BRKB, but also BKNG), so they are listed.
var shareClassTickers = map[string]shareClass{
	"BRKA": {company: "BRK", class: "A"},
	"BRKB": {company: "BRK", class: "B"},
	"BFA":  {company: "BF", class: "A"},
	"BFB":  {company: "BF", class: "B"},
	"LENB": {company: "LEN", class: "B"},
	"HEIA": {company: "HEI", class: "A"},
	"MOGA": {company: "MOG", class: "A"},
	"MOGB": {company: "MOG", class: "B"},
}

// separatedShareClass matches BRK.B, BRK-B, BRK/B and BRK B: a company code
// and a one-letter class. The match only counts when the joined ticker is in
// shareClassTickers, because a one-letter suffix can also be an exchange
// (VOD.L is London, not class L).
var separatedShareClass = regexp.MustCompile(`^([A-Z]{1,5})[.\-/ ]([A-Z])$`)

type shareClass struct {
	company string
	class   string
}

// parseShareClass reads a share-class ticker in any of the forms above.
func parseShareClass(symbol string) (shareClass, bool) {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	if sc, ok := shareClassTickers[upper]; ok {
		return sc, true
	}
	if m := separatedShareClass.FindStringSubmatch(upper); m != nil {
		if sc, ok := shareClassTickers[m[1]+m[2]]; ok {
			return sc, true
		}
	}
	return shareClass{}, false
}

// yahooSymbol is the ticker Yahoo knows the symbol by: share classes use a
// dash (BRK-B). Other tickers are only upper-cased.
func yahooSymbol(symbol string) string {
	if sc, ok := parseShareClass(symbol); ok {
		return sc.company + "-" + sc.class
	}
	return strings.ToUpper(strings.TrimSpace(symbol))
}

// alpacaSymbol is the ticker Alpaca knows the symbol by: share classes use a
// dot (BRK.B). Other tickers are only upper-cased.
func alpacaSymbol(symbol string) string {
	if sc, ok := parseShareClass(symbol); ok {
		return sc.company + "." + sc.class
	}
	return strings.ToUpper(strings.TrimSpace(symbol))
}

// stooqShareClassCode is Stooq's daily code for a share class (brk-b.us).
func stooqShareClassCode(symbol string) (string, bool) {
	sc, ok := parseShareClass(symbol)
	if !ok {
		return "", false
	}
	return strings.ToLower(sc.company+"-"+sc.class) + ".us", true
}
