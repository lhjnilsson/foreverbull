package command

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// missingSessionsReasonLead is the start of Zipline's MissingSessionsError
// text, which the ingest sends as the DATA_LOSS reason.
const missingSessionsReasonLead = "missing prices on trading days: "

// gapReasonByteBound is the most bytes of a DATA_LOSS status: the capped
// reason plus the details line. Zipline folds whole trailing symbol entries
// into the leftover symbol count until that text fits, and the details line
// is the type name only, so the reason is not sent twice. A longer message
// is not parsed. 8 KiB is the header list grpc-go enforces when its
// experimental limit is on.
const gapReasonByteBound = 8 << 10

// gapEntry matches one symbol. A field is YYYY-MM-DD or YYYY-MM-DD..YYYY-MM-DD.
// The second form is one run of consecutive sessions, not every day of it.
var gapEntry = regexp.MustCompile(
	`^([A-Za-z0-9.^=_-]{1,32}) (` + gapFields + `)(?: and (\d+) more ranges?)?$`)

const gapDate = `\d{4}-\d{2}-\d{2}`

const gapFields = gapDate + `(?:\.\.` + gapDate + `)?(?:,` + gapDate + `(?:\.\.` + gapDate + `)?)*`

var moreSymbols = regexp.MustCompile(`^(\d+) more symbols?$`)

// daySpan is one listed field. from and to are inclusive. A single day has
// from equal to to.
type daySpan struct {
	from, to time.Time
}

// ingestGapReason reads the gaps from Zipline's MissingSessionsError.
// A capped reason still matches, including a run written as A..B. A reason
// that is not this sentence is false, and the caller uses the ingest copy.
func ingestGapReason(reason string) (map[string][]daySpan, map[string]int, int, bool) {
	if !strings.HasPrefix(reason, missingSessionsReasonLead) {
		return nil, nil, 0, false
	}
	body := strings.TrimPrefix(reason, missingSessionsReasonLead)
	if body == "" {
		return nil, nil, 0, false
	}
	extraSymbols := 0
	if rest, tail, found := strings.Cut(body, "; and "); found {
		count, ok := moreCount(tail, moreSymbols)
		if !ok {
			return nil, nil, 0, false
		}
		extraSymbols = count
		body = rest
	}
	entries := strings.Split(body, "; ")
	spans := make(map[string][]daySpan, len(entries))
	extraRanges := make(map[string]int, len(entries))
	for _, entry := range entries {
		match := gapEntry.FindStringSubmatch(entry)
		if match == nil {
			return nil, nil, 0, false
		}
		symbol := match[1]
		if _, dup := spans[symbol]; dup {
			return nil, nil, 0, false
		}
		for _, field := range strings.Split(match[2], ",") {
			span, ok := parseDaySpan(field)
			if !ok {
				return nil, nil, 0, false
			}
			spans[symbol] = append(spans[symbol], span)
		}
		if match[3] != "" {
			count, ok := positiveCount(match[3])
			if !ok {
				return nil, nil, 0, false
			}
			extraRanges[symbol] = count
		}
	}
	return spans, extraRanges, extraSymbols, true
}

func parseDaySpan(field string) (daySpan, bool) {
	fromText, toText, ranged := strings.Cut(field, "..")
	from, err := time.Parse(time.DateOnly, fromText)
	if err != nil {
		return daySpan{}, false
	}
	if !ranged {
		return daySpan{from: from, to: from}, true
	}
	to, err := time.Parse(time.DateOnly, toText)
	if err != nil || to.Before(from) {
		return daySpan{}, false
	}
	return daySpan{from: from, to: to}, true
}

// expandDaySpans fills each A..B with the sessions from A through B. Both
// ends have to be sessions. A day that is not a session is left out of the
// fill, so a holiday inside a run does not fail the reason. false means an
// end is not a session or the range is backwards.
func expandDaySpans(sessions []time.Time, spans map[string][]daySpan) (map[string][]time.Time, bool) {
	isSession := make(map[time.Time]bool, len(sessions))
	for _, session := range sessions {
		isSession[session] = true
	}
	out := make(map[string][]time.Time, len(spans))
	for symbol, list := range spans {
		var days []time.Time
		seen := make(map[time.Time]bool)
		for _, span := range list {
			if !isSession[span.from] || !isSession[span.to] {
				return nil, false
			}
			for _, session := range sessions {
				if session.Before(span.from) || session.After(span.to) || seen[session] {
					continue
				}
				seen[session] = true
				days = append(days, session)
			}
		}
		out[symbol] = days
	}
	return out, true
}

func moreCount(text string, pattern *regexp.Regexp) (int, bool) {
	match := pattern.FindStringSubmatch(text)
	if match == nil {
		return 0, false
	}
	return positiveCount(match[1])
}

func positiveCount(text string) (int, bool) {
	count, err := strconv.Atoi(text)
	if err != nil || count < 1 {
		return 0, false
	}
	return count, true
}
