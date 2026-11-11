package backtest

import (
	"slices"
	"strconv"
	"strings"
	"time"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// autoCloseOnUniverseDrop is the create-time option. An omitted field is on.
func autoCloseOnUniverseDrop(item *pb.Backtest) bool {
	if item == nil || item.AutoCloseOnUniverseDrop == nil {
		return true
	}
	return *item.AutoCloseOnUniverseDrop
}

// universeDrops records symbols foreverbull closed because they left the
// universe. The runner writes it and RunNotices reads it after the period
// channel has closed.
type universeDrops struct {
	byDate map[time.Time][]string
}

func newUniverseDrops() *universeDrops {
	return &universeDrops{byDate: make(map[time.Time][]string)}
}

func (u *universeDrops) add(day time.Time, symbols []string) {
	if u == nil || len(symbols) == 0 {
		return
	}
	day = dayOf(day)
	seen := make(map[string]struct{}, len(u.byDate[day]))
	for _, symbol := range u.byDate[day] {
		seen[symbol] = struct{}{}
	}
	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		u.byDate[day] = append(u.byDate[day], symbol)
	}
}

func (u *universeDrops) notices() []string {
	if u == nil || len(u.byDate) == 0 {
		return nil
	}
	dates := make([]time.Time, 0, len(u.byDate))
	for day := range u.byDate {
		dates = append(dates, day)
	}
	slices.SortFunc(dates, func(a, b time.Time) int { return a.Compare(b) })
	lines := make([]string, 0, len(dates))
	for _, day := range dates {
		lines = append(lines, universeDropLine(day, u.byDate[day]))
	}
	return lines
}

// universeDropSymbolsShown is how many names a stored line lists before
// "and N more".
const universeDropSymbolsShown = 5

// universeDropLine is one stored notice for the symbols closed on day.
func universeDropLine(day time.Time, symbols []string) string {
	names := append([]string(nil), symbols...)
	slices.Sort(names)
	shown := names
	extra := ""
	if len(names) > universeDropSymbolsShown {
		extra = " and " + strconv.Itoa(len(names)-universeDropSymbolsShown) + " more"
		shown = names[:universeDropSymbolsShown]
	}
	list := strings.Join(shown, ", ") + extra
	date := day.UTC().Format(time.DateOnly)
	if len(names) == 1 {
		return "1 symbol left the universe on " + date + " and a held position was closed at that session's close: " + list + "."
	}
	return strconv.Itoa(len(names)) + " symbols left the universe on " + date + " and held positions were closed at that session's close: " + list + "."
}

// heldNonMembers adds portfolio symbols that are not members, so a strategy
// that only walks the symbol list can flatten them. members is the universe
// at t. process is what Process would otherwise receive.
func heldNonMembers(members, process []string, portfolio *finance.Portfolio) []string {
	member := make(map[string]struct{}, len(members))
	for _, symbol := range members {
		member[symbol] = struct{}{}
	}
	out := append([]string{}, process...)
	seen := make(map[string]struct{}, len(out))
	for _, symbol := range out {
		seen[symbol] = struct{}{}
	}
	if portfolio == nil {
		return out
	}
	for _, position := range portfolio.GetPositions() {
		if position.GetAmount() == 0 {
			continue
		}
		symbol := position.GetSymbol()
		if symbol == "" {
			continue
		}
		if _, ok := member[symbol]; ok {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		out = append(out, symbol)
	}
	slices.Sort(out)
	return out
}
