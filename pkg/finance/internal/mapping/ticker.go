// Package mapping is the one place a stored symbol becomes the ticker a
// provider is asked for, and the one place a later ticker of the same ISIN
// is chosen. The name stored on a bar stays the symbol the caller used.
package mapping

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/ticker"
	"github.com/rs/zerolog/log"
)

// separatedShareClass matches BRK.B, BRK-B, BRK/B and BRK B. The class
// letter is A or B, the US share-class letters. A venue suffix such as L
// or V is not a class. There is no list of companies: any root of that
// shape is a class, and a joined iShares ticker (BRKB) is one only after
// Classes learns the separated spelling from that ISIN's listing or from
// the symbol a provider returned for the ISIN.
var separatedShareClass = regexp.MustCompile(`^([A-Z]{1,5})[.\-/ ]([AB])$`)

// classID is one share class, company plus its letter.
type classID struct {
	company string
	class   string
}

// Classes remembers joined tickers taught by a listing or a provider.
// A nil Classes only knows the separated spelling.
type Classes struct {
	mu     sync.RWMutex
	joined map[string]classID
}

// Process is the Classes the broker's price clients and the listings file share.
func Process() *Classes {
	processOnce.Do(func() { processClasses = NewClasses() })
	return processClasses
}

var (
	processOnce    sync.Once
	processClasses *Classes
)

// NewClasses is an empty set. Learn teaches it.
func NewClasses() *Classes {
	return &Classes{joined: map[string]classID{}}
}

// Learn teaches the joined ticker of every separated share class in symbols.
// BRK-B teaches BRKB. A symbol that is not a separated class teaches nothing.
func (c *Classes) Learn(symbols ...string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, symbol := range symbols {
		company, class, ok := parseSeparated(symbol)
		if !ok {
			continue
		}
		c.joined[company+class] = classID{company: company, class: class}
	}
}

// LearnIfJoined teaches asked only when a hit is the separated spelling of
// that exact joined ticker. META does not become MET-A. A hit for another
// company is not this ticker.
func (c *Classes) LearnIfJoined(asked string, hits []string) {
	if c == nil {
		return
	}
	asked = strings.ToUpper(strings.TrimSpace(asked))
	var teach []string
	for _, hit := range hits {
		company, class, ok := parseSeparated(hit)
		if !ok {
			continue
		}
		if company+class == asked {
			teach = append(teach, hit)
		}
	}
	c.Learn(teach...)
}

// MightBeJoinedClass reports an iShares-style ticker that might be a share
// class written without a separator. The letters alone are not enough, so
// this only decides whether to ask the ISIN's listing. BK and BKNG are not.
func MightBeJoinedClass(symbol string) bool {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	if len(upper) < 3 || len(upper) > 6 {
		return false
	}
	if strings.ContainsAny(upper, ".-/ ") {
		return false
	}
	for _, r := range upper {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	last := upper[len(upper)-1]
	return last == 'A' || last == 'B'
}

func parseSeparated(symbol string) (company, class string, ok bool) {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	m := separatedShareClass.FindStringSubmatch(upper)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func resolve(c *Classes, symbol string) (company, class string, ok bool) {
	if company, class, ok = parseSeparated(symbol); ok {
		return company, class, true
	}
	if c == nil {
		return "", "", false
	}
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	c.mu.RLock()
	got, found := c.joined[upper]
	c.mu.RUnlock()
	if !found {
		return "", "", false
	}
	return got.company, got.class, true
}

// ParseShareClass reads a separated share class, or a joined ticker this
// Classes was taught. A nil Classes does not guess a joined ticker.
func ParseShareClass(symbol string) (company, class string, ok bool) {
	return resolve(nil, symbol)
}

// Stored is the one name a share class is stored under: company plus class,
// so BRK.B, BRK-B, BRK/B and BRK B are BRKB. A joined ticker is that name
// already. Any other symbol, including an exchange suffix, is returned as typed.
func Stored(symbol string) string {
	if company, class, ok := parseSeparated(symbol); ok {
		return company + class
	}
	return symbol
}

// Yahoo is the ticker Yahoo is asked for. A separated share class uses a
// dash (BRK-B). A joined ticker uses a dash only when c was taught it.
// Other tickers are only upper-cased.
func Yahoo(symbol string) string { return formatYahoo(nil, symbol) }

// FormatYahoo is Yahoo with the joined tickers c has learned.
func FormatYahoo(c *Classes, symbol string) string { return formatYahoo(c, symbol) }

func formatYahoo(c *Classes, symbol string) string {
	if company, class, ok := resolve(c, symbol); ok {
		return company + "-" + class
	}
	return strings.ToUpper(strings.TrimSpace(symbol))
}

// Alpaca is the ticker Alpaca is asked for. A separated share class uses a
// dot (BRK.B). Other tickers are only upper-cased.
func Alpaca(symbol string) string { return formatAlpaca(nil, symbol) }

// FormatAlpaca is Alpaca with the joined tickers c has learned.
func FormatAlpaca(c *Classes, symbol string) string { return formatAlpaca(c, symbol) }

func formatAlpaca(c *Classes, symbol string) string {
	if company, class, ok := resolve(c, symbol); ok {
		return company + "." + class
	}
	return strings.ToUpper(strings.TrimSpace(symbol))
}

// stooqExchangeSuffix maps a suffix Stooq may be asked for. Only US is
// accepted. A non-US suffix used to be downloaded and priced as USD on the
// NYSE calendar; it is refused before any request. A suffix that is not
// here is refused. The keys are the suffixes the refusal lists as supported.
var stooqExchangeSuffix = map[string]string{
	"US": "us",
}

// stooqTicker bounds a suffixed ticker before it is echoed in an error.
var stooqTicker = regexp.MustCompile(`^[A-Z0-9^=_-]{1,32}\.[A-Z]{1,8}$`)

// ErrStooqTicker is a suffixed ticker outside the shape Stooq can be asked
// for. The stooq client replaces it with the sentence a person reads.
var ErrStooqTicker = errors.New("stooq ticker is not valid")

// StooqSuffixError is a suffixed ticker Stooq has no code for. Symbol is the
// upper-case ticker including the suffix. The stooq client writes the sentence.
type StooqSuffixError struct {
	Symbol string
	Suffix string
}

func (e *StooqSuffixError) Error() string { return e.Symbol }

// SupportedStooqSuffixes lists the supported suffixes. Only .US is accepted.
func SupportedStooqSuffixes() string {
	suffixes := make([]string, 0, len(stooqExchangeSuffix))
	for suffix := range stooqExchangeSuffix {
		suffixes = append(suffixes, "."+suffix)
	}
	sort.Strings(suffixes)
	return strings.Join(suffixes, ", ")
}

func stooqSuffixFor(suffix string) (string, bool) {
	if code, ok := stooqExchangeSuffix[strings.ToUpper(suffix)]; ok {
		return code, true
	}
	lower := strings.ToLower(suffix)
	for _, code := range stooqExchangeSuffix {
		if code == lower {
			return code, true
		}
	}
	return "", false
}

// StooqShareClass is Stooq's daily code for a separated share class.
// exchange is Stooq's suffix, so US is brk-b.us. An empty exchange is not
// a code. A joined ticker needs FormatStooqShareClass.
func StooqShareClass(symbol, exchange string) (string, bool) {
	return FormatStooqShareClass(nil, symbol, exchange)
}

// FormatStooqShareClass is StooqShareClass with the joined tickers c has learned.
func FormatStooqShareClass(c *Classes, symbol, exchange string) (string, bool) {
	company, class, ok := resolve(c, symbol)
	if !ok || strings.TrimSpace(exchange) == "" {
		return "", false
	}
	return strings.ToLower(company+"-"+class) + "." + exchange, true
}

// Stooq maps a stored symbol to the Stooq code and the canonical ticker,
// after a caller's own symbol file has missed. A listed share class is asked
// for with a dash. A symbol that already names an exchange uses that
// exchange. Anything else uses exchange, which the caller has already
// required. There is no implied market.
func Stooq(symbol, exchange string) (code, canonical string, err error) {
	return FormatStooq(nil, symbol, exchange)
}

// FormatStooq is Stooq with the joined tickers c has learned.
func FormatStooq(c *Classes, symbol, exchange string) (code, canonical string, err error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return "", "", errors.New("missing symbol")
	}
	if _, _, ok := resolve(c, symbol); ok {
		if strings.TrimSpace(exchange) == "" {
			return "", "", errors.New("stooq exchange is empty")
		}
		classCode, _ := FormatStooqShareClass(c, symbol, exchange)
		return classCode, strings.ToUpper(symbol), nil
	}
	if i := strings.LastIndexByte(symbol, '.'); i > 0 {
		return stooqSuffixed(symbol[:i], symbol[i+1:])
	}
	if strings.TrimSpace(exchange) == "" {
		return "", "", errors.New("stooq exchange is empty")
	}
	return strings.ToLower(symbol) + "." + exchange, strings.ToUpper(symbol), nil
}

func stooqSuffixed(name, suffix string) (code, canonical string, err error) {
	upper := strings.ToUpper(name + "." + suffix)
	if !stooqTicker.MatchString(upper) {
		return "", "", ErrStooqTicker
	}
	stooqSuffix, ok := stooqSuffixFor(suffix)
	if !ok {
		return "", "", &StooqSuffixError{Symbol: upper, Suffix: strings.ToUpper(suffix)}
	}
	code = strings.ToLower(name) + "." + stooqSuffix
	if stooqSuffix == "us" {
		return code, strings.ToUpper(name), nil
	}
	return code, upper, nil
}

// ProviderTicker is the ticker provider is asked for. stooqExchange is
// Stooq's suffix and is used only for stooq. An unknown provider is an error.
func ProviderTicker(provider, symbol, stooqExchange string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "yahoo":
		return FormatYahoo(nil, symbol), nil
	case "alpaca":
		return FormatAlpaca(nil, symbol), nil
	case "stooq":
		code, _, err := FormatStooq(nil, symbol, stooqExchange)
		return code, err
	default:
		return "", fmt.Errorf("unknown price provider %q; valid providers: yahoo, stooq, alpaca", provider)
	}
}

// SearchHit is one symbol a provider's search returned for an ISIN.
type SearchHit struct {
	Symbol string
	ISIN   string
}

// LaterCandidates lists the other tickers of isin: the stored aliases first,
// then search hits that carry this exact ISIN. asked is not a candidate.
// A symbol outside the ticker pattern is skipped. hits from a failed search
// are not passed: the caller fails instead of continuing with a partial list.
func LaterCandidates(isin, asked string, aliases []string, hits []SearchHit) []string {
	asked = strings.TrimSpace(asked)
	seen := map[string]struct{}{asked: {}}
	candidates := make([]string, 0)
	for _, symbol := range aliases {
		symbol = strings.TrimSpace(symbol)
		if !candidateSymbol(isin, symbol) {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		candidates = append(candidates, symbol)
	}
	for _, hit := range hits {
		symbol := strings.TrimSpace(hit.Symbol)
		if !candidateSymbol(isin, symbol) {
			continue
		}
		// Only a hit with this exact ISIN is the same instrument. A hit
		// without an ISIN could be another company.
		if hit.ISIN != isin {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		candidates = append(candidates, symbol)
	}
	return candidates
}

func candidateSymbol(isin, symbol string) bool {
	if symbol == "" {
		return false
	}
	if ticker.Valid(symbol) {
		return true
	}
	log.Warn().Str("isin", isin).Str("candidate", ticker.Logged(symbol)).Msg("skipping a later ticker outside the symbol pattern")
	return false
}
