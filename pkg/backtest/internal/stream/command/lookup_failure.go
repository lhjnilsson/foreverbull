package command

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance"
)

// lookupMiss is one symbol whose listing or delisting lookup failed.
// cause is classified. Its Error text is for the log, not this sentence.
type lookupMiss struct {
	symbol  string
	listing bool
	cause   *finance.LookupError
}

// stepKind is one next step. Several failures join their steps in this order.
type stepKind int

const (
	stepNetwork stepKind = iota
	stepServer
	stepRate
	stepAlpacaKey
	stepRefused
	stepParse
)

const (
	stepNetworkText = "Check your internet connection, then run `fbull backtest create` again."
	stepServerText  = "Run `fbull backtest create` again in a few minutes, or try another provider with --provider."
	stepRateText    = "Wait a few minutes, then run `fbull backtest create` again, or try another provider with --provider."
	stepAlpacaText  = "Check your alpaca API key and secret, or try another provider with --provider, then run `fbull backtest create` again."
	stepRefusedText = "Try another provider with --provider, then run `fbull backtest create` again."
	stepParseText   = "Try another provider with --provider, then run `fbull backtest create` again. " +
		"If it keeps happening, please report it at https://github.com/lhjnilsson/foreverbull/issues with this message."
)

// lookupCause is the phrase after the colon, and which step it offers.
// Only network, a server error, and a rate limit offer a retry.
func lookupCause(provider string, err *finance.LookupError) (string, stepKind) {
	name := strings.ToLower(strings.TrimSpace(provider))
	switch err.Kind {
	case finance.LookupNetwork:
		return name + " didn't answer", stepNetwork
	case finance.LookupBot:
		return name + " answered with a bot check instead of data", stepRefused
	case finance.LookupParse:
		return name + "'s answer couldn't be read", stepParse
	case finance.LookupHTTP:
		switch {
		case err.Status == http.StatusTooManyRequests:
			return name + " limited how many requests foreverbull can make", stepRate
		case err.Status >= 500 && err.Status <= 599:
			return fmt.Sprintf("%s had a server error (HTTP %d)", name, err.Status), stepServer
		case name == "alpaca" && (err.Status == http.StatusUnauthorized || err.Status == http.StatusForbidden):
			return fmt.Sprintf("alpaca didn't accept foreverbull's API key (HTTP %d)", err.Status), stepAlpacaKey
		default:
			return fmt.Sprintf("%s refused the request (HTTP %d)", name, err.Status), stepRefused
		}
	default:
		return name + "'s answer couldn't be read", stepParse
	}
}

func stepText(kind stepKind) string {
	switch kind {
	case stepNetwork:
		return stepNetworkText
	case stepServer:
		return stepServerText
	case stepRate:
		return stepRateText
	case stepAlpacaKey:
		return stepAlpacaText
	case stepRefused:
		return stepRefusedText
	case stepParse:
		return stepParseText
	default:
		return stepParseText
	}
}

// joinSteps lists each distinct step once, in stepKind order, then extra
// when that sentence is not already one of them. Steps are joined with " Or ".
func joinSteps(kinds []stepKind, extra string) string {
	seen := make(map[stepKind]bool, len(kinds))
	steps := make([]string, 0, len(kinds)+1)
	for _, kind := range []stepKind{stepNetwork, stepServer, stepRate, stepAlpacaKey, stepRefused, stepParse} {
		if !seen[kind] && containsKind(kinds, kind) {
			seen[kind] = true
			steps = append(steps, stepText(kind))
		}
	}
	if extra != "" && !slicesContains(steps, extra) {
		steps = append(steps, extra)
	}
	return strings.Join(steps, " Or ")
}

func containsKind(kinds []stepKind, kind stepKind) bool {
	for _, item := range kinds {
		if item == kind {
			return true
		}
	}
	return false
}

func slicesContains(steps []string, extra string) bool {
	for _, step := range steps {
		if step == extra {
			return true
		}
	}
	return false
}

func missKinds(provider string, misses []lookupMiss) []stepKind {
	kinds := make([]stepKind, 0, len(misses))
	for _, miss := range misses {
		_, kind := lookupCause(provider, miss.cause)
		kinds = append(kinds, kind)
	}
	return kinds
}

func distinctMisses(misses []lookupMiss) int {
	seen := make(map[string]bool, len(misses))
	for _, miss := range misses {
		seen[miss.symbol] = true
	}
	return len(seen)
}

func symbolNoun(n int) string {
	if n == 1 {
		return "symbol"
	}
	return "symbols"
}

// lookupCheck is "whether it was delisted" or "when it was listed".
func lookupCheck(listing bool) string {
	if listing {
		return "when it was listed"
	}
	return "whether it was delisted"
}

// lookupFailureMessage is the create error when every failure is a lookup.
// One symbol is one sentence. Two or more are a block. removed selects the
// partly-saved sentence.
func lookupFailureMessage(name, provider string, misses []lookupMiss, removed bool) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	saved, next := savedAndNext(name, joinSteps(missKinds(provider, misses), ""), removed)
	if len(misses) == 1 {
		miss := misses[0]
		cause, _ := lookupCause(provider, miss.cause)
		verb := "whether"
		what := "delisted"
		if miss.listing {
			verb = "when"
			what = "listed"
		}
		return fmt.Sprintf("Couldn't load data for backtest '%s': foreverbull couldn't check with %s %s %s was %s: %s. %s %s",
			name, provider, verb, miss.symbol, what, cause, saved, next)
	}
	n := distinctMisses(misses)
	lines := []string{fmt.Sprintf("Couldn't load data for backtest '%s': foreverbull couldn't check %d %s with %s. %s",
		name, n, symbolNoun(n), provider, saved)}
	for _, miss := range misses {
		cause, _ := lookupCause(provider, miss.cause)
		lines = append(lines, fmt.Sprintf("  %s: couldn't check %s: %s.", miss.symbol, lookupCheck(miss.listing), cause))
	}
	return strings.Join(append(lines, next), "\n")
}

// mixedCoverageMessage is the create error when lookup failures share the
// message with missing prices or an unconfirmed edge. The step is the union
// of both.
func mixedCoverageMessage(name, provider string, days []time.Time, gaps []symbolGaps, misses []lookupMiss, removed bool) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	saved, _ := savedAndNext(name, "", removed)
	// savedAndNext's next is rebuilt below so the union, not one step, follows
	// the delete-first sentence when the backtest couldn't be removed.
	next := joinSteps(missKinds(provider, misses), coverageStep(days, gaps))
	if !removed {
		next = deleteFirst(name, next)
	}
	order, lines := mixedLines(provider, days, gaps, misses)
	n := len(order)
	verb := "are"
	if n == 1 {
		verb = "is"
	}
	lead := fmt.Sprintf("Couldn't load data for backtest '%s': %d %s %s missing prices or couldn't be checked. %s",
		name, n, symbolNoun(n), verb, saved)
	before, after := false, false
	for _, gap := range gaps {
		before = before || !gap.noneBefore.IsZero()
		after = after || !gap.noneAfter.IsZero()
	}
	reason := ""
	if before || after {
		reason = unconfirmedReason(provider, before, after) + " "
	}
	return strings.Join(append(append([]string{lead}, lines...), reason+next), "\n")
}

func mixedLines(provider string, days []time.Time, gaps []symbolGaps, misses []lookupMiss) ([]string, []string) {
	gapBy := make(map[string]symbolGaps, len(gaps))
	var order []string
	seen := make(map[string]bool)
	add := func(symbol string) {
		if seen[symbol] {
			return
		}
		seen[symbol] = true
		order = append(order, symbol)
	}
	for _, gap := range gaps {
		add(gap.symbol)
		gapBy[gap.symbol] = gap
	}
	missBy := make(map[string][]lookupMiss)
	for _, miss := range misses {
		add(miss.symbol)
		missBy[miss.symbol] = append(missBy[miss.symbol], miss)
	}
	lines := make([]string, 0, len(order))
	for _, symbol := range order {
		if gap, ok := gapBy[symbol]; ok {
			lines = append(lines, coverageLine(days, gap))
		}
		for _, miss := range missBy[symbol] {
			cause, _ := lookupCause(provider, miss.cause)
			lines = append(lines, fmt.Sprintf("  %s: couldn't check %s: %s.", miss.symbol, lookupCheck(miss.listing), cause))
		}
	}
	return order, lines
}
