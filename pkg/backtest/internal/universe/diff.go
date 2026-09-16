package universe

import (
	"sort"
	"strings"
	"time"
)

// Holding is one ETF constituent as of a snapshot. Identity is ISIN; ticker is a dated alias.
type Holding struct {
	ISIN      string
	Symbol    string
	Name      string
	AssetType string
}

// Snapshot is the Equity holdings of an ETF on one as-of date.
type Snapshot struct {
	AsOf     time.Time
	Holdings []Holding
}

// Member is a half-open [FromDate, ToDate) membership interval. Nil ToDate is still a member.
type Member struct {
	ISIN     string
	FromDate time.Time
	ToDate   *time.Time
}

// Alias is a half-open [FromDate, ToDate) ticker interval for one ISIN. Nil ToDate is current.
type Alias struct {
	ISIN     string
	Symbol   string
	FromDate time.Time
	ToDate   *time.Time
}

// Result is the consecutive as-of diff: membership by ISIN, tickers as aliases.
type Result struct {
	Members []Member
	Aliases []Alias
}

type openMember struct {
	from time.Time
}

type openAlias struct {
	symbol string
	from   time.Time
}

// Diff walks snapshots ordered by as-of date and emits ISIN membership intervals.
// Adds and drops change membership; a ticker change (FB then META) does not.
// Empty ISIN and non-Equity rows are skipped. Hysteresis is not applied: month-end
// sampling already drops unpublished days, and a two-as-of lag would delay a
// genuine reconstitution by a month.
func Diff(snapshots []Snapshot) Result {
	ordered := normalize(snapshots)
	openMembers := make(map[string]*openMember)
	openAliases := make(map[string]*openAlias)
	members := make([]Member, 0)
	aliases := make([]Alias, 0)
	var prev map[string]Holding

	for i, snap := range ordered {
		curr := holdingsByISIN(snap.Holdings)
		if i == 0 {
			for isin, h := range curr {
				openMembers[isin] = &openMember{from: snap.AsOf}
				openAliasFor(openAliases, isin, h.Symbol, snap.AsOf)
			}
			prev = curr
			continue
		}

		for isin, h := range curr {
			if _, was := prev[isin]; !was {
				openMembers[isin] = &openMember{from: snap.AsOf}
			}
			aliases = applyAlias(aliases, openAliases, isin, h.Symbol, snap.AsOf)
		}

		for isin := range prev {
			if _, still := curr[isin]; still {
				continue
			}
			opened := openMembers[isin]
			to := snap.AsOf
			members = append(members, Member{ISIN: isin, FromDate: opened.from, ToDate: &to})
			delete(openMembers, isin)
		}

		prev = curr
	}

	for isin, opened := range openMembers {
		members = append(members, Member{ISIN: isin, FromDate: opened.from, ToDate: nil})
	}
	for isin, opened := range openAliases {
		aliases = append(aliases, Alias{ISIN: isin, Symbol: opened.symbol, FromDate: opened.from, ToDate: nil})
	}

	sortMembers(members)
	sortAliases(aliases)
	return Result{Members: members, Aliases: aliases}
}

// MembersAt returns ISINs whose membership interval covers t ([from, to)).
func MembersAt(members []Member, t time.Time) []string {
	t = dateOnly(t)
	isins := make([]string, 0)
	for _, m := range members {
		if contains(m.FromDate, m.ToDate, t) {
			isins = append(isins, m.ISIN)
		}
	}
	sort.Strings(isins)
	return isins
}

// SymbolsAt returns the ticker of each member on t via dated aliases.
// Adds and drops change the list; a rename (FB then META) keeps one name.
// Members with no alias at t are omitted. Empty membership is an empty slice.
func SymbolsAt(members []Member, aliases []Alias, t time.Time) []string {
	t = dateOnly(t)
	wanted := make(map[string]struct{})
	for _, isin := range MembersAt(members, t) {
		wanted[isin] = struct{}{}
	}
	symbols := make([]string, 0)
	seen := make(map[string]struct{})
	for _, alias := range aliases {
		if _, ok := wanted[alias.ISIN]; !ok {
			continue
		}
		if alias.Symbol == "" || !contains(alias.FromDate, alias.ToDate, t) {
			continue
		}
		if _, dup := seen[alias.Symbol]; dup {
			continue
		}
		seen[alias.Symbol] = struct{}{}
		symbols = append(symbols, alias.Symbol)
	}
	sort.Strings(symbols)
	return symbols
}

func contains(from time.Time, to *time.Time, t time.Time) bool {
	if from.After(t) {
		return false
	}
	return to == nil || t.Before(dateOnly(*to))
}

func normalize(snapshots []Snapshot) []Snapshot {
	type dated struct {
		asOf     time.Time
		holdings map[string]Holding
	}
	byDate := make(map[string]*dated)
	keys := make([]string, 0)
	for _, snap := range snapshots {
		asOf := dateOnly(snap.AsOf)
		key := asOf.Format("20060102")
		entry, ok := byDate[key]
		if !ok {
			entry = &dated{asOf: asOf, holdings: make(map[string]Holding)}
			byDate[key] = entry
			keys = append(keys, key)
		}
		for isin, h := range holdingsByISIN(snap.Holdings) {
			entry.holdings[isin] = h
		}
	}
	sort.Strings(keys)
	out := make([]Snapshot, 0, len(keys))
	for _, key := range keys {
		entry := byDate[key]
		holdings := make([]Holding, 0, len(entry.holdings))
		for _, h := range entry.holdings {
			holdings = append(holdings, h)
		}
		sort.Slice(holdings, func(i, j int) bool { return holdings[i].ISIN < holdings[j].ISIN })
		out = append(out, Snapshot{AsOf: entry.asOf, Holdings: holdings})
	}
	return out
}

func holdingsByISIN(holdings []Holding) map[string]Holding {
	out := make(map[string]Holding)
	for _, h := range holdings {
		h.ISIN = strings.TrimSpace(h.ISIN)
		h.Symbol = strings.TrimSpace(h.Symbol)
		h.AssetType = strings.TrimSpace(h.AssetType)
		if h.ISIN == "" || h.AssetType != "Equity" {
			continue
		}
		out[h.ISIN] = h
	}
	return out
}

func openAliasFor(openAliases map[string]*openAlias, isin, symbol string, asOf time.Time) {
	if symbol == "" {
		return
	}
	openAliases[isin] = &openAlias{symbol: symbol, from: asOf}
}

func applyAlias(aliases []Alias, openAliases map[string]*openAlias, isin, symbol string, asOf time.Time) []Alias {
	if symbol == "" {
		return aliases
	}
	cur, ok := openAliases[isin]
	if !ok {
		openAliases[isin] = &openAlias{symbol: symbol, from: asOf}
		return aliases
	}
	if cur.symbol == symbol {
		return aliases
	}
	if dateOnly(cur.from).Equal(asOf) {
		cur.symbol = symbol
		return aliases
	}
	to := asOf
	aliases = append(aliases, Alias{ISIN: isin, Symbol: cur.symbol, FromDate: cur.from, ToDate: &to})
	openAliases[isin] = &openAlias{symbol: symbol, from: asOf}
	return aliases
}

func sortMembers(members []Member) {
	sort.Slice(members, func(i, j int) bool {
		if members[i].ISIN != members[j].ISIN {
			return members[i].ISIN < members[j].ISIN
		}
		return members[i].FromDate.Before(members[j].FromDate)
	})
}

func sortAliases(aliases []Alias) {
	sort.Slice(aliases, func(i, j int) bool {
		if aliases[i].ISIN != aliases[j].ISIN {
			return aliases[i].ISIN < aliases[j].ISIN
		}
		if !aliases[i].FromDate.Equal(aliases[j].FromDate) {
			return aliases[i].FromDate.Before(aliases[j].FromDate)
		}
		return aliases[i].Symbol < aliases[j].Symbol
	})
}

func dateOnly(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
