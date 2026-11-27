package marketdata

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/mapping"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
)

// Global markets are not supported yet. A symbol listed outside the US, or
// priced in a currency other than USD, is refused before anything is stored.
//
// Each provider's signal:
//
//   - Yahoo reports the exchange (quoteType.exchange, or price.exchange when
//     that is set) and the currency (price.currency). A US exchange and USD
//     is accepted. A non-US exchange or any other currency is refused. The
//     quote is one lookup; it is not retried and it does not look up a later
//     ticker.
//   - Stooq reports neither an exchange nor a currency. The symbol suffix is
//     the signal. Any suffix other than .US is outside the US and is refused
//     before a download. A symbol with no suffix uses STOOQ_EXCHANGE, which
//     is only US, and is priced as USD because Stooq sends no currency.
//   - Alpaca reports the exchange, and the currency when the asset payload
//     has one, from GET /v2/assets/{symbol}. NASDAQ, NYSE, ARCA, AMEX, BATS,
//     OTC, and IEX are US. Any other exchange, or a currency other than USD,
//     is refused.
//
// A non-US suffix is enough to refuse, including when the provider lookup
// fails. A suffix is never enough to accept a listing the provider reports
// as outside the US or in another currency. A separated share class
// (BRK.B, class letter A or B) is not an exchange suffix.

// Quote is what a provider reported about one symbol. Reported is false when
// the provider does not send an exchange or a currency (Stooq).
type Quote struct {
	Exchange string
	Currency string
	Reported bool
}

// QuoteReader asks one provider about one symbol. Tests replace it.
type QuoteReader func(ctx context.Context, provider, profile, symbol string) (Quote, error)

// QuoteFailure is a provider lookup that did not answer. Symbol is the
// ticker that was asked. The cause is for the log; it is not the sentence.
type QuoteFailure struct {
	Provider string
	Symbol   string
	Err      error
}

func (e *QuoteFailure) Error() string {
	if e == nil {
		return "the listing lookup didn't respond"
	}
	return usercopy.ListingLookup(e.Provider, []string{e.Symbol})
}

// QuoteFailures is every listing lookup that did not answer. A refusal does
// not drop these, and a benchmark lookup does not replace one that came first.
type QuoteFailures struct {
	Items []*QuoteFailure
}

func (e *QuoteFailures) Error() string {
	if e == nil || len(e.Items) == 0 {
		return "the listing lookup didn't respond"
	}
	names := make([]string, 0, len(e.Items))
	provider := e.Items[0].Provider
	for _, item := range e.Items {
		if item == nil || item.Symbol == "" {
			continue
		}
		names = append(names, item.Symbol)
	}
	return usercopy.ListingLookup(provider, names)
}

func (e *QuoteFailures) Unwrap() []error {
	if e == nil {
		return nil
	}
	out := make([]error, 0, len(e.Items))
	for _, item := range e.Items {
		if item != nil {
			out = append(out, item)
		}
	}
	return out
}

func (e *QuoteFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

var (
	readQuote  QuoteReader = defaultQuote
	quoteMu    sync.Mutex
	quoteYahoo *YahooClient
	// quoteCache remembers a listing that answered, and a symbol the provider
	// said it does not know. A failed lookup is not cached, so a later create
	// can ask again. An unknown symbol is not a pass. SetQuoteReader clears it.
	quoteCache sync.Map
)

// cachedListing is one symbol's listing answer. unknown is a provider that
// said the ticker has no quote. That is not a refusal and not a failed
// lookup, and it is not a symbol that answered.
type cachedListing struct {
	quote   Quote
	unknown bool
}

// SetQuoteReader replaces the provider lookup and returns the previous one.
func SetQuoteReader(fn QuoteReader) func() {
	quoteMu.Lock()
	prev := readQuote
	if fn == nil {
		fn = defaultQuote
	}
	readQuote = fn
	quoteCache = sync.Map{}
	quoteMu.Unlock()
	return func() {
		quoteMu.Lock()
		readQuote = prev
		quoteCache = sync.Map{}
		quoteMu.Unlock()
	}
}

func currentQuoteReader() QuoteReader {
	quoteMu.Lock()
	defer quoteMu.Unlock()
	return readQuote
}

// usExchanges are the exchange codes Yahoo and Alpaca use for a US listing.
// The comparison is case-insensitive.
var usExchanges = map[string]struct{}{
	"NMS": {}, "NYQ": {}, "PCX": {}, "ASE": {}, "NGM": {}, "NCM": {}, "BTS": {},
	"NGMS": {}, "NYS": {}, "NASDAQ": {}, "NASDAQGS": {}, "NASDAQGM": {}, "NASDAQCM": {},
	"NYSE": {}, "NYSEARCA": {}, "NYSEAMERICAN": {}, "NYSEMKT": {}, "AMEX": {},
	"BATS": {}, "CBOE": {}, "IEX": {}, "ARCA": {}, "OTC": {}, "PNK": {}, "OTCMKTS": {},
	// US index codes Yahoo reports. ^NDX is NIM, ^GSPC is SNP, ^DJI is DJI,
	// ^RUT is WCB. CXI is another US index venue. An unknown code is refused.
	"NIM": {}, "SNP": {}, "DJI": {}, "WCB": {}, "CXI": {},
	// US OTC venues. PNK is already above.
	"OQB": {}, "OQX": {}, "OEM": {},
}

// exchangeNames turns a provider exchange code into the name a sentence uses.
var exchangeNames = map[string]string{
	"LSE": "London", "L": "London", "LON": "London",
	"GER": "Xetra", "XETRA": "Xetra", "DE": "Xetra",
	"TOR": "Toronto", "TO": "Toronto", "TSE": "Toronto", "TSX": "Toronto",
}

// suffixNames is the exchange a stored suffix names. A suffix that is not
// here and is not .US still refuses; the sentence says the suffix is not a
// US exchange.
var suffixNames = map[string]string{
	"L":  "London",
	"UK": "London",
	"DE": "Xetra",
}

var usSuffixes = map[string]struct{}{
	"US": {},
}

var suffixCode = regexp.MustCompile(`^[A-Z]{1,8}$`)

// RefuseListed asks the provider about every symbol and returns each one
// that is listed outside the US or priced in a currency other than USD.
// benchmark is checked the same way and is not also returned in members.
// An empty benchmark is skipped. A lookup that fails does not hide a
// suffix refusal. Every failed lookup is returned, including when a symbol
// or the benchmark is refused, and a later failure does not replace an
// earlier one. A symbol that already answered is not asked again.
func RefuseListed(ctx context.Context, provider, profile string, members []string, benchmark string) ([]usercopy.MarketSymbol, *usercopy.MarketSymbol, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "yahoo"
	}
	benchKey := strings.ToUpper(strings.TrimSpace(benchmark))
	seen := map[string]struct{}{}
	if benchKey != "" {
		seen[benchKey] = struct{}{}
	}
	var (
		refusals []usercopy.MarketSymbol
		failed   []*QuoteFailure
	)
	note := func(symbol string, refusal usercopy.MarketSymbol, refused bool, classErr error) {
		if refused {
			refusals = append(refusals, refusal)
			return
		}
		var one *QuoteFailure
		if errors.As(classErr, &one) && one != nil {
			failed = append(failed, one)
		}
	}
	consider := func(symbol string) {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			return
		}
		key := strings.ToUpper(symbol)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		quote, err := readListing(ctx, provider, profile, symbol)
		refusal, ok, classErr := Classify(provider, symbol, quote, err)
		note(symbol, refusal, ok, classErr)
	}
	var benchRefusal *usercopy.MarketSymbol
	if benchKey != "" {
		quote, err := readListing(ctx, provider, profile, strings.TrimSpace(benchmark))
		refusal, ok, classErr := Classify(provider, strings.TrimSpace(benchmark), quote, err)
		if ok {
			benchRefusal = &refusal
		} else {
			var one *QuoteFailure
			if errors.As(classErr, &one) && one != nil {
				failed = append(failed, one)
			}
		}
	}
	for _, symbol := range members {
		consider(symbol)
	}
	if len(failed) == 0 {
		return refusals, benchRefusal, nil
	}
	return refusals, benchRefusal, &QuoteFailures{Items: failed}
}

// readListing asks the provider once per symbol. A quote that answered is
// remembered, and so is a symbol the provider does not know. A lookup that
// failed is not, so the next create can ask again.
func readListing(ctx context.Context, provider, profile, symbol string) (Quote, error) {
	key := listingKey(provider, symbol)
	quoteMu.Lock()
	if value, ok := quoteCache.Load(key); ok {
		quoteMu.Unlock()
		cached, _ := value.(cachedListing)
		if cached.unknown {
			return Quote{}, ErrUnknownSymbol
		}
		return cached.quote, nil
	}
	quoteMu.Unlock()
	quote, err := currentQuoteReader()(ctx, provider, profile, symbol)
	if err != nil {
		if errors.Is(err, ErrUnknownSymbol) {
			quoteMu.Lock()
			quoteCache.Store(key, cachedListing{unknown: true})
			quoteMu.Unlock()
		}
		return Quote{}, err
	}
	quoteMu.Lock()
	quoteCache.Store(key, cachedListing{quote: quote})
	quoteMu.Unlock()
	return quote, nil
}

func listingKey(provider, symbol string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "yahoo"
	}
	return provider + "\x00" + strings.ToUpper(strings.TrimSpace(symbol))
}

// ListingsAnswered reports whether every symbol was answered by a listing
// lookup. An unknown symbol is not an answer, and a symbol that was not
// asked is not an answer. An empty list is answered: there was nothing to ask.
func ListingsAnswered(provider string, symbols []string) bool {
	seen := map[string]struct{}{}
	quoteMu.Lock()
	defer quoteMu.Unlock()
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		key := listingKey(provider, symbol)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		value, ok := quoteCache.Load(key)
		if !ok {
			return false
		}
		cached, _ := value.(cachedListing)
		if cached.unknown {
			return false
		}
	}
	return true
}

// Unanswered are the symbols that were not answered. An unknown symbol is
// included. The order is the order they were asked, without repeats.
func Unanswered(provider string, symbols []string) []string {
	seen := map[string]struct{}{}
	var out []string
	quoteMu.Lock()
	defer quoteMu.Unlock()
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		key := listingKey(provider, symbol)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		value, ok := quoteCache.Load(key)
		if !ok {
			out = append(out, symbol)
			continue
		}
		cached, _ := value.(cachedListing)
		if cached.unknown {
			out = append(out, symbol)
		}
	}
	return out
}

// Classify decides one symbol from a quote. ok is false when the symbol is
// not refused. A non-US suffix refuses even when qerr is set; the suffix
// does not accept a quote that says otherwise.
func Classify(provider, symbol string, quote Quote, qerr error) (usercopy.MarketSymbol, bool, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "yahoo"
	}
	suffix, suffixPlace, badSuffix := exchangeSuffix(symbol)
	if qerr != nil && !badSuffix {
		if errors.Is(qerr, ErrUnknownSymbol) {
			return usercopy.MarketSymbol{}, false, nil
		}
		return usercopy.MarketSymbol{}, false, &QuoteFailure{Provider: provider, Symbol: symbol, Err: qerr}
	}
	if qerr != nil {
		quote = Quote{}
	}
	place, currency, nonUS, nonUSD := quoteFacts(quote)
	if badSuffix {
		nonUS = true
		if place == "" {
			place = suffixPlace
		}
	}
	if !nonUS && !nonUSD {
		return usercopy.MarketSymbol{}, false, nil
	}
	out := usercopy.MarketSymbol{
		Symbol:   symbol,
		Place:    place,
		Currency: currency,
		Provider: provider,
		Suffix:   suffix,
	}
	// A symbol with no non-US listing, refused only because of the currency.
	if !badSuffix && place == "" && nonUSD {
		out.CurrencyOnly = true
		out.Place = ""
	}
	if badSuffix && place == "" {
		out.Suffix = suffix
	}
	return out, true, nil
}

func quoteFacts(quote Quote) (place, currency string, nonUS, nonUSD bool) {
	if !quote.Reported {
		return "", "", false, false
	}
	code := strings.ToUpper(strings.TrimSpace(quote.Exchange))
	currency = strings.ToUpper(strings.TrimSpace(quote.Currency))
	if currency == "GBX" {
		currency = "GBP"
	}
	if code != "" {
		if _, us := usExchanges[code]; us {
			nonUS = false
		} else {
			nonUS = true
			if name, ok := exchangeNames[code]; ok {
				place = name
			} else {
				place = code
			}
		}
	}
	nonUSD = currency != "" && currency != "USD"
	if !nonUS && !nonUSD {
		currency = ""
		place = ""
	}
	if !nonUS {
		place = ""
	}
	if currency == "USD" && !nonUS {
		currency = ""
	}
	return place, currency, nonUS, nonUSD
}

// exchangeSuffix reports a non-US suffix. A share class (BRK.B) and the US
// suffix (.US) are not one. bad is false when there is no refusing suffix.
func exchangeSuffix(symbol string) (suffix, place string, bad bool) {
	if parseShareClass(symbol) {
		return "", "", false
	}
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	dot := strings.LastIndexByte(upper, '.')
	if dot <= 0 || dot == len(upper)-1 {
		return "", "", false
	}
	suffix = upper[dot+1:]
	if !suffixCode.MatchString(suffix) {
		return "", "", false
	}
	if _, us := usSuffixes[suffix]; us {
		return suffix, "", false
	}
	place = suffixNames[suffix]
	return suffix, place, true
}

func defaultQuote(ctx context.Context, provider, profile, symbol string) (Quote, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "stooq":
		// Stooq's CSV has no exchange and no currency. The suffix decides.
		return Quote{}, nil
	case "alpaca":
		return alpacaQuote(ctx, profile, symbol)
	default:
		return fetchYahooQuote(ctx, symbol)
	}
}

func fetchYahooQuote(ctx context.Context, symbol string) (Quote, error) {
	client, err := sharedListingYahoo()
	if err != nil {
		return Quote{}, err
	}
	exchange, currency, err := client.Listing(ctx, symbol)
	if err != nil {
		return Quote{}, err
	}
	return Quote{Exchange: exchange, Currency: currency, Reported: true}, nil
}

func sharedListingYahoo() (*YahooClient, error) {
	quoteMu.Lock()
	defer quoteMu.Unlock()
	if quoteYahoo != nil {
		return quoteYahoo, nil
	}
	client, err := NewYahooClient()
	if err != nil {
		return nil, err
	}
	yahoo, ok := client.(*YahooClient)
	if !ok {
		return nil, errors.New("yahoo client has the wrong type")
	}
	quoteYahoo = yahoo
	return quoteYahoo, nil
}

func alpacaQuote(ctx context.Context, profile, symbol string) (Quote, error) {
	key, secret, err := trading.AlpacaPriceCredentials(environment.GetTradingProfilesFile(), profile)
	if err != nil {
		return Quote{}, err
	}
	client, err := NewAlpacaBars(key, secret)
	if err != nil {
		return Quote{}, err
	}
	client.UseShareClasses(mapping.Process())
	exchange, currency, err := client.Listing(ctx, symbol)
	if err != nil {
		return Quote{}, err
	}
	return Quote{Exchange: exchange, Currency: currency, Reported: true}, nil
}
