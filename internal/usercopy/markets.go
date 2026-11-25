package usercopy

import (
	"fmt"
	"sort"
	"strings"
)

// MarketSymbol is one symbol a global-markets refusal names.
// The fields pick the description:
//
//   - Place and Currency: "VOD.L (London, GBP)"
//   - CurrencyOnly: "XYZ (yahoo prices it in CAD)"
//   - Place only: "VOD.L (London)"
//   - Suffix, when Place is empty: "ABC.XX (the .XX suffix isn't a US exchange)"
//
// Provider is the lowercase price-source name used in the currency sentence.
type MarketSymbol struct {
	Symbol       string
	Place        string
	Currency     string
	Provider     string
	Suffix       string
	CurrencyOnly bool
}

// GlobalMarketsCreate is the create refusal. removed is false when the
// backtest row could not be deleted; the delete-first step then comes from
// BacktestSavedAndNext. benchmarkFixed is an ETF benchmark the user cannot
// edit: the step is the report sentence.
func GlobalMarketsCreate(name string, symbols []MarketSymbol, benchmark *MarketSymbol, removed, benchmarkFixed bool) string {
	symbols = sortedMarkets(symbols)
	saved, next := BacktestSavedAndNext(name, createMarketStep(symbols, benchmark, benchmarkFixed), removed)
	return fmt.Sprintf("Couldn't create backtest '%s': %s Global markets aren't supported yet. %s %s",
		name, marketBody(symbols, benchmark), saved, next)
}

// GlobalMarketsRun is the fbull backtest run refusal. Nothing was started.
func GlobalMarketsRun(name string, symbols []MarketSymbol, benchmark *MarketSymbol) string {
	symbols = sortedMarkets(symbols)
	return fmt.Sprintf("Couldn't run backtest '%s': %s Global markets aren't supported yet. Nothing was run. %s",
		name, marketBody(symbols, benchmark), runMarketStep(name, symbols, benchmark))
}

// GlobalMarketsTrading is the fbull trading run refusal. No order was sent.
func GlobalMarketsTrading(name string, symbols []MarketSymbol, benchmark *MarketSymbol) string {
	symbols = sortedMarkets(symbols)
	return fmt.Sprintf("Couldn't start trading with backtest '%s': %s Global markets aren't supported yet. No orders were sent. Pick another backtest, or %s",
		name, marketBody(symbols, benchmark), tradingMarketStep(name, symbols, benchmark))
}

// GlobalMarketsDeploy is the fbull strategy deploy refusal. The deployment
// is not stored and no order is sent. Deploy has no benchmark of its own.
func GlobalMarketsDeploy(name string, symbols []MarketSymbol) string {
	symbols = sortedMarkets(symbols)
	return fmt.Sprintf("Couldn't start deployment '%s': %s Global markets aren't supported yet. The deployment wasn't started. %s",
		name, marketBody(symbols, nil), deployMarketStep(name, symbols))
}

// ListingLookup is the clause for symbols whose listing lookup did not
// answer. One name uses "is listed". Two use "A and B are listed". Three or
// more use "A, B and C are listed". Names are sorted.
func ListingLookup(provider string, symbols []string) string {
	names := append([]string(nil), symbols...)
	sort.Strings(names)
	return provider + " didn't respond when checking where " + whereListed(names)
}

// LookupCreate is a create whose listing lookup did not answer. removed is
// false when the backtest row could not be deleted.
func LookupCreate(name, provider string, symbols []string, removed bool) string {
	saved, next := BacktestSavedAndNext(name, waitCreate, removed)
	return fmt.Sprintf("Couldn't create backtest '%s': %s. %s %s", name, ListingLookup(provider, symbols), saved, next)
}

// LookupRun is a backtest run whose listing lookup did not answer.
func LookupRun(name, provider string, symbols []string) string {
	return fmt.Sprintf("Couldn't run backtest '%s': %s. Nothing was run. %s", name, ListingLookup(provider, symbols), waitRun)
}

// LookupTrading is a trading run whose listing lookup did not answer.
func LookupTrading(name, provider string, symbols []string) string {
	return fmt.Sprintf("Couldn't start trading with backtest '%s': %s. No orders were sent. %s", name, ListingLookup(provider, symbols), waitTrading)
}

// LookupDeploy is a strategy deploy whose listing lookup did not answer.
func LookupDeploy(name, provider string, symbols []string) string {
	return fmt.Sprintf("Couldn't start deployment '%s': %s. The deployment wasn't started. %s",
		name, ListingLookup(provider, symbols), waitDeploy(name))
}

// GlobalMarketsCreateMixed is a create that both refuses symbols and could
// not look some up. The refusal body comes first, then the global-markets
// sentence, then the lookup, then the saved sentence, then one step.
// benchmarkFixed is an ETF benchmark the user cannot edit: the step is the
// report sentence alone.
func GlobalMarketsCreateMixed(name string, symbols []MarketSymbol, benchmark *MarketSymbol, provider string, lookups []string, removed, benchmarkFixed bool) string {
	symbols = sortedMarkets(symbols)
	saved, next := BacktestSavedAndNext(name, mixedCreateStep(symbols, benchmark, benchmarkFixed), removed)
	return fmt.Sprintf("Couldn't create backtest '%s': %s Global markets aren't supported yet. %s. %s %s",
		name, marketBody(symbols, benchmark), ListingLookup(provider, lookups), saved, next)
}

// GlobalMarketsRunMixed is a run that both refuses symbols and could not
// look some up. The run step stands alone; there is no second wait step.
func GlobalMarketsRunMixed(name string, symbols []MarketSymbol, benchmark *MarketSymbol, provider string, lookups []string) string {
	symbols = sortedMarkets(symbols)
	return fmt.Sprintf("Couldn't run backtest '%s': %s Global markets aren't supported yet. %s. Nothing was run. %s",
		name, marketBody(symbols, benchmark), ListingLookup(provider, lookups), runMarketStep(name, symbols, benchmark))
}

// GlobalMarketsTradingMixed is a trading run that both refuses symbols and
// could not look some up. The trading step stands alone.
func GlobalMarketsTradingMixed(name string, symbols []MarketSymbol, benchmark *MarketSymbol, provider string, lookups []string) string {
	symbols = sortedMarkets(symbols)
	return fmt.Sprintf("Couldn't start trading with backtest '%s': %s Global markets aren't supported yet. %s. No orders were sent. Pick another backtest, or %s",
		name, marketBody(symbols, benchmark), ListingLookup(provider, lookups), tradingMarketStep(name, symbols, benchmark))
}

// GlobalMarketsDeployMixed is a deploy that both refuses symbols and could
// not look some up. The remove step and the wait are one sentence.
func GlobalMarketsDeployMixed(name string, symbols []MarketSymbol, provider string, lookups []string) string {
	symbols = sortedMarkets(symbols)
	return fmt.Sprintf("Couldn't start deployment '%s': %s Global markets aren't supported yet. %s. The deployment wasn't started. %s",
		name, marketBody(symbols, nil), ListingLookup(provider, lookups), mixedDeployStep(name, symbols))
}

const (
	waitCreate  = "Wait a few minutes, then run `fbull backtest create` again."
	waitRun     = "Wait a few minutes, then run `fbull backtest run` again."
	waitTrading = "Wait a few minutes, then run `fbull trading run` again."
)

func waitDeploy(name string) string {
	return "Wait a few minutes, then run `fbull strategy deploy " + name + "` again with the same options."
}

func whereListed(symbols []string) string {
	switch len(symbols) {
	case 0:
		return "the symbol is listed"
	case 1:
		return symbols[0] + " is listed"
	case 2:
		return symbols[0] + " and " + symbols[1] + " are listed"
	default:
		return strings.Join(symbols[:len(symbols)-1], ", ") + " and " + symbols[len(symbols)-1] + " are listed"
	}
}

// GlobalMarketsLead names every refused symbol, without the command or the
// next step. Download uses it when it has no backtest name.
func GlobalMarketsLead(symbols []MarketSymbol, benchmark *MarketSymbol) string {
	return marketBody(sortedMarkets(symbols), benchmark)
}

// GlobalMarketsSuffix is a download refused from the ticker suffix alone.
// A known London or Xetra suffix names that place and its currency. The
// global-markets sentence follows the listing.
func GlobalMarketsSuffix(s MarketSymbol) string {
	if s.Currency == "" {
		switch strings.ToUpper(s.Suffix) {
		case "L", "UK":
			s.Currency = "GBP"
		case "DE":
			s.Currency = "EUR"
		}
	}
	return s.Symbol + " is listed outside the US (" + s.detail() + "). Global markets aren't supported yet."
}

// LaterTickerPrices is a download whose prices would come from a later
// ticker of the same company that is listed outside the US or priced in a
// currency other than USD. asked is the symbol in the backtest. candidate
// is that later ticker.
func LaterTickerPrices(asked string, candidate MarketSymbol) string {
	which := "which is listed outside the US (" + candidate.detail() + ")"
	if candidate.CurrencyOnly {
		which = fmt.Sprintf("which %s prices in %s, not USD", candidate.Provider, candidate.Currency)
	}
	return fmt.Sprintf("Prices for %s come from %s, the company's later ticker, %s. Global markets aren't supported yet.",
		asked, candidate.Symbol, which)
}

// GlobalMarketsOutside is a trading session call for a symbol that was not
// in the set checked when the session was created. A legacy session has an
// empty set, so every symbol is outside it. The symbol is not described as
// a listing: it was not checked. The call still fails.
func GlobalMarketsOutside(symbols []string) string {
	names := append([]string(nil), symbols...)
	const tail = " No orders were sent. Stop trading, then run `fbull trading run` again with a backtest that includes "
	switch len(names) {
	case 0:
		return "A symbol wasn't checked when this trading session started, so it can't be traded." +
			tail + "every symbol your algo trades."
	case 1:
		return names[0] + " wasn't checked when this trading session started, so it can't be traded." +
			tail + names[0] + "."
	default:
		return joinedNames(names) + " weren't checked when this trading session started, so they can't be traded." +
			tail + "them."
	}
}

func joinedNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
}

// DeploymentSymbolsUnknown is a deployment whose listing check did not get
// an answer for every symbol. provider is the source that was asked, such
// as yahoo. symbols are the ones that did not answer.
func DeploymentSymbolsUnknown(name, provider string, symbols []string) string {
	names := append([]string(nil), symbols...)
	sort.Strings(names)
	var known, step string
	switch len(names) {
	case 0:
		known = provider + " didn't answer for every symbol"
		step = "Check the symbols, or remove them from --symbols, then run `fbull strategy deploy " + name + "` again with the same options."
	case 1:
		known = provider + " doesn't know the symbol " + names[0]
		step = "Check " + names[0] + ", or remove it from --symbols, then run `fbull strategy deploy " + name + "` again with the same options."
	case 2:
		known = provider + " doesn't know the symbols " + names[0] + " and " + names[1]
		step = "Check them, or remove them from --symbols, then run `fbull strategy deploy " + name + "` again with the same options."
	default:
		known = provider + " doesn't know the symbols " + strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
		step = "Check them, or remove them from --symbols, then run `fbull strategy deploy " + name + "` again with the same options."
	}
	return fmt.Sprintf("Couldn't start deployment '%s': %s. The deployment wasn't started. %s", name, known, step)
}

// DeploymentProfileUnchecked is a deployment whose trading profile cannot
// be read, so its symbols cannot be checked. fileMissing is a profiles file
// that is absent or unreadable. Otherwise the name is not in the file.
func DeploymentProfileUnchecked(name, profile string, fileMissing bool) string {
	if fileMissing {
		return fmt.Sprintf("Couldn't start deployment '%s': this broker has no trading profiles set up, so foreverbull can't check where the symbols for '%s' are listed. The deployment wasn't started. Set up the trading profile '%s', then run `fbull strategy deploy %s` again with the same options.",
			name, profile, profile, name)
	}
	return fmt.Sprintf("Couldn't start deployment '%s': there's no trading profile named '%s', so foreverbull can't check where its symbols are listed. The deployment wasn't started. Check the name against the trading profiles set up for foreverbull, then run `fbull strategy deploy %s` again with the same options.",
		name, profile, name)
}

// DeploymentListingUnexpected is a listing failure that names no symbol.
// The user cannot fix it from the message, so the step is to report it.
func DeploymentListingUnexpected(strategy string) string {
	name := strings.TrimSpace(strategy)
	if name == "" {
		return "Couldn't start the deployment: foreverbull couldn't tell where its symbols are listed. The deployment wasn't started. " + ReportIt
	}
	return fmt.Sprintf("Couldn't start deployment '%s': foreverbull couldn't tell where its symbols are listed. The deployment wasn't started. %s",
		name, ReportIt)
}

func sortedMarkets(symbols []MarketSymbol) []MarketSymbol {
	out := append([]MarketSymbol(nil), symbols...)
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

func marketBody(symbols []MarketSymbol, benchmark *MarketSymbol) string {
	var b strings.Builder
	switch len(symbols) {
	case 0:
	case 1:
		b.WriteString(symbols[0].clause(false, true))
	default:
		items := make([]string, len(symbols))
		for i, symbol := range symbols {
			items[i] = symbol.item()
		}
		fmt.Fprintf(&b, "%d symbols aren't US-listed in USD: %s.", len(symbols), strings.Join(items, ", "))
	}
	if benchmark != nil {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(benchmark.clause(true, b.Len() == 0))
	}
	return b.String()
}

func (s MarketSymbol) clause(benchmark, lower bool) string {
	if s.CurrencyOnly {
		who := s.Symbol
		if benchmark {
			who = "the benchmark " + s.Symbol
		}
		return fmt.Sprintf("%s prices %s in %s, not USD.", s.Provider, who, s.Currency)
	}
	subject := s.Symbol
	if benchmark {
		subject = "the benchmark " + s.Symbol
		if !lower {
			subject = "The benchmark " + s.Symbol
		}
	}
	return subject + " is listed outside the US (" + s.detail() + ")."
}

func (s MarketSymbol) item() string {
	if s.CurrencyOnly {
		return fmt.Sprintf("%s (%s prices it in %s)", s.Symbol, s.Provider, s.Currency)
	}
	return s.Symbol + " (" + s.detail() + ")"
}

func (s MarketSymbol) detail() string {
	switch {
	case s.Place != "" && s.Currency != "":
		return s.Place + ", " + s.Currency
	case s.Place != "":
		return s.Place
	default:
		return fmt.Sprintf("the .%s suffix isn't a US exchange", s.Suffix)
	}
}

func createMarketStep(symbols []MarketSymbol, benchmark *MarketSymbol, benchmarkFixed bool) string {
	if benchmarkFixed && benchmark != nil {
		return ReportIt
	}
	var action string
	switch {
	case len(symbols) == 1 && benchmark != nil:
		action = fmt.Sprintf("Remove %s and set `benchmark` in the backtest file to a US-listed symbol such as SPY", symbols[0].Symbol)
	case len(symbols) > 1 && benchmark != nil:
		action = "Remove them and set `benchmark` in the backtest file to a US-listed symbol such as SPY"
	case len(symbols) == 1:
		action = "Remove " + symbols[0].Symbol
	case len(symbols) > 1:
		action = "Remove them"
	default:
		action = "Set `benchmark` in the backtest file to a US-listed symbol such as SPY"
	}
	return action + ", then run `fbull backtest create` again."
}

func runMarketStep(name string, symbols []MarketSymbol, benchmark *MarketSymbol) string {
	return fmt.Sprintf("Delete it with `fbull backtest delete %s`, %s, then create it again with `fbull backtest create`.",
		name, marketFileStep(symbols, benchmark))
}

func tradingMarketStep(name string, symbols []MarketSymbol, benchmark *MarketSymbol) string {
	return fmt.Sprintf("delete it with `fbull backtest delete %s`, %s, then create it again with `fbull backtest create`.",
		name, marketFileStep(symbols, benchmark))
}

func deployMarketStep(name string, symbols []MarketSymbol) string {
	action := "Remove them from --symbols"
	if len(symbols) == 1 {
		action = "Remove " + symbols[0].Symbol + " from --symbols"
	}
	return action + ", then run `fbull strategy deploy " + name + "` again with the same options."
}

func mixedCreateStep(symbols []MarketSymbol, benchmark *MarketSymbol, benchmarkFixed bool) string {
	if benchmarkFixed {
		return ReportIt
	}
	step := createMarketStep(symbols, benchmark, false)
	const tail = ", then run `fbull backtest create` again."
	const mixed = ", wait a few minutes, then run `fbull backtest create` again."
	if strings.HasSuffix(step, tail) {
		return strings.TrimSuffix(step, tail) + mixed
	}
	return step
}

func mixedDeployStep(name string, symbols []MarketSymbol) string {
	action := "Remove them from --symbols"
	if len(symbols) == 1 {
		action = "Remove " + symbols[0].Symbol + " from --symbols"
	}
	return action + ", wait a few minutes, then run `fbull strategy deploy " + name + "` again with the same options."
}

func marketFileStep(symbols []MarketSymbol, benchmark *MarketSymbol) string {
	switch {
	case len(symbols) == 1 && benchmark != nil:
		return fmt.Sprintf("remove %s and set `benchmark` to a US-listed symbol such as SPY in the backtest file", symbols[0].Symbol)
	case len(symbols) > 1 && benchmark != nil:
		return "remove them and set `benchmark` to a US-listed symbol such as SPY in the backtest file"
	case len(symbols) == 1:
		return "remove " + symbols[0].Symbol + " from the backtest file"
	case len(symbols) > 1:
		return "remove them from the backtest file"
	default:
		return "set `benchmark` in the backtest file to a US-listed symbol such as SPY"
	}
}
