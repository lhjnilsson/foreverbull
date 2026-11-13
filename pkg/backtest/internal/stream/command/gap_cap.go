package command

import "fmt"

// A missing-session error names at most this many symbols, and at most this
// many date ranges on each of them. A run of consecutive sessions is one
// range. The rest are counted in the sentence; the log keeps every one.
// A big universe used to put the whole list in the gRPC trailer.
const (
	maxGapSymbols = 20
	maxGapRanges  = 10
)

// gapStyle is how much of a gap list a sentence shows. Zero means the
// whole list: that is the log. The user sentence uses maxGapSymbols and
// maxGapRanges.
type gapStyle struct {
	symbols int
	ranges  int
}

var userGapStyle = gapStyle{symbols: maxGapSymbols, ranges: maxGapRanges}

// clipGaps keeps the first symbols and, on each, the first ranges.
// The order of gaps is the order the caller gives, which is the benchmark
// and then A to Z. omittedRanges lines up with the kept symbols.
func clipGaps(gaps []symbolGaps, symbolLimit, rangeLimit int) (shown []symbolGaps, omittedRanges []int, omittedSymbols int) {
	shown = gaps
	if symbolLimit > 0 && len(gaps) > symbolLimit {
		omittedSymbols = len(gaps) - symbolLimit
		shown = gaps[:symbolLimit]
	}
	out := make([]symbolGaps, len(shown))
	omittedRanges = make([]int, len(shown))
	for i, gap := range shown {
		out[i] = gap
		if rangeLimit > 0 {
			out[i].sessions, omittedRanges[i] = clipRangeIndexes(gap.sessions, rangeLimit)
		}
	}
	return out, omittedRanges, omittedSymbols
}

// clipRangeIndexes keeps the first limit runs of consecutive indexes.
// Indexes are positions in the session list, so a weekend inside a run
// does not start a new range.
func clipRangeIndexes(missing []int, limit int) ([]int, int) {
	if limit <= 0 || len(missing) == 0 {
		return missing, 0
	}
	ranges := 0
	for i := 0; i < len(missing); {
		j := i
		for j+1 < len(missing) && missing[j+1] == missing[j]+1 {
			j++
		}
		ranges++
		if ranges == limit {
			return missing[:j+1], countRanges(missing[j+1:])
		}
		i = j + 1
	}
	return missing, 0
}

func countRanges(missing []int) int {
	ranges := 0
	for i := 0; i < len(missing); {
		ranges++
		j := i
		for j+1 < len(missing) && missing[j+1] == missing[j]+1 {
			j++
		}
		i = j + 1
	}
	return ranges
}

// applyParsed keeps a count the caller already computed, when this list
// was not clipped further. Zipline's capped reason arrives that way: the
// days in hand are the ones to show, and the reason names how many ranges
// and symbols were left out.
func applyParsed(shown []symbolGaps, omittedRanges []int, omittedSymbols int, parsed map[string]int, parsedSymbols int) ([]int, int) {
	if omittedSymbols > 0 || omittedAny(omittedRanges) || (len(parsed) == 0 && parsedSymbols == 0) {
		return omittedRanges, omittedSymbols
	}
	out := make([]int, len(shown))
	for i, gap := range shown {
		out[i] = parsed[gap.symbol]
	}
	return out, parsedSymbols
}

func omittedAny(counts []int) bool {
	for _, count := range counts {
		if count > 0 {
			return true
		}
	}
	return false
}

func parsedRangesAny(parsed map[string]int) bool {
	for _, count := range parsed {
		if count > 0 {
			return true
		}
	}
	return false
}

func moreRanges(count int) string {
	if count == 1 {
		return "and 1 more range"
	}
	return fmt.Sprintf("and %d more ranges", count)
}

// dayCount is "N trading days" or "N days", singular when N is 1.
func dayCount(count int, trading bool) string {
	noun := "days"
	if count == 1 {
		noun = "day"
	}
	if trading {
		return fmt.Sprintf("%d trading %s", count, noun)
	}
	return fmt.Sprintf("%d %s", count, noun)
}

func moreSymbolsLine(count int) string {
	if count == 1 {
		return "  … and 1 more symbol"
	}
	return fmt.Sprintf("  … and %d more symbols", count)
}

func withMoreRanges(text string, count int) string {
	if count <= 0 {
		return text
	}
	return text + " … " + moreRanges(count)
}
