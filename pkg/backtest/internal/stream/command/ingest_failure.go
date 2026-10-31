package command

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ingestDetailsMarker separates the reason from the underlying exception in
// the data service's error. The CLI prints the part after it only with
// --verbose, so the marker is kept in the step message.
const ingestDetailsMarker = "\nDetails: "

const ingestFallbackReason = "the data service failed while ingesting prices"

// urlPassword matches the password in the userinfo of any scheme://user:pw@
// URL. The data service redacts its own text, but an older image or a
// wrapped driver error must not put a database password in the broker log or
// the CLI either.
var urlPassword = regexp.MustCompile(`(\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]*:)[^\s@]*@`)

func redactURLPasswords(text string) string {
	return urlPassword.ReplaceAllString(text, "${1}***@")
}

type ingestFailureKind int

const (
	// ingestFailed is any failure the data service did not classify. Its
	// text is never shown as the reason, only behind --verbose.
	ingestFailed ingestFailureKind = iota
	// ingestMissingDriver is the data service reporting that it is broken
	// itself, so the user's backtest and settings are not at fault.
	ingestMissingDriver
	// ingestUnreachable is either hop failing: the broker to the Zipline pod,
	// or the Zipline pod to Postgres. The user's next step is the same, so
	// the line is the same; the details say which hop failed.
	ingestUnreachable
)

// missingDriverReason is the data service's own sentence for a missing
// driver. Only the driver name is taken from it, and only when it matches.
var missingDriverReason = regexp.MustCompile(`^the data service is missing its database driver \(([A-Za-z0-9_]+)\)$`)

type ingestFailure struct {
	kind ingestFailureKind
	// driver is the missing driver's module name, empty when unknown.
	driver string
	// details is the redacted underlying error, shown only with --verbose.
	details string
	// sentence is the data service's own message for a refused ingest
	// (Aborted, no details marker). It is the user's whole message when it
	// names this backtest; see refusedSentence.
	sentence string
}

// notSavedSentence is how a refused ingest's sentence says that nothing was
// kept. It is replaced when the backtest couldn't be removed.
const notSavedSentence = "The backtest wasn't saved."

// notRemovedSentence replaces notSavedSentence when the backtest couldn't be
// removed after a refused ingest.
const notRemovedSentence = "The backtest was partly saved and couldn't be removed."

// readIngestFailure classifies the data service's error by its gRPC code.
// The data service uses FailedPrecondition only for a broken install and
// Unavailable for a database it cannot reach; gRPC itself uses Unavailable
// when the broker cannot reach the data service. The error text never becomes
// the reason: it goes to details, behind --verbose.
func readIngestFailure(cause error) ingestFailure {
	message := cause.Error()
	code := codes.Unknown
	var grpcErr interface{ GRPCStatus() *status.Status }
	if errors.As(cause, &grpcErr) {
		message = grpcErr.GRPCStatus().Message()
		code = grpcErr.GRPCStatus().Code()
	}
	reason, details, hasDetails := strings.Cut(message, ingestDetailsMarker)
	reason = strings.TrimSpace(reason)
	if !hasDetails {
		details = message
	}
	failure := ingestFailure{kind: ingestFailed, details: strings.TrimSpace(details)}
	if code == codes.FailedPrecondition {
		failure.kind = ingestMissingDriver
		if match := missingDriverReason.FindStringSubmatch(reason); match != nil {
			failure.driver = match[1]
		}
	}
	if code == codes.Unavailable {
		failure.kind = ingestUnreachable
		failure.details = unreachableHop(hasDetails, reason, failure.details)
	}
	if code == codes.Aborted && !hasDetails {
		// The data service refused the ingest with the user's sentence; its
		// cause, if any, stayed in the data service's log.
		failure.sentence = reason
	}
	return failure
}

// refusedSentence is the data service's sentence as the step's whole message,
// and false when there is none or it doesn't name this backtest (then the
// fixed lead is used, with the sentence behind --verbose). When the backtest
// couldn't be removed, "wasn't saved" would be wrong, so that sentence says it
// was partly saved, and a sentence to delete it first comes before the step.
func refusedSentence(name string, failure ingestFailure, removed bool) (string, bool) {
	if failure.sentence == "" || !strings.HasPrefix(failure.sentence, fmt.Sprintf("Couldn't load data for backtest '%s': ", name)) {
		return "", false
	}
	if removed {
		return failure.sentence, true
	}
	return deleteFirstSentence(name, failure.sentence)
}

// deleteFirstSentence rewrites a refusal for a backtest that couldn't be
// removed. notSavedSentence becomes notRemovedSentence, and "Run `fbull
// backtest delete <name>` first." goes as its own sentence before the
// refusal's own next step, which follows it on the same line or is the last
// line and stays exactly as written. It returns false when the sentence
// doesn't have that shape.
func deleteFirstSentence(name, sentence string) (string, bool) {
	saved := strings.Index(sentence, notSavedSentence)
	if saved < 0 {
		return "", false
	}
	head := sentence[:saved] + notRemovedSentence
	rest := sentence[saved+len(notSavedSentence):]
	var middle, next string
	switch {
	case strings.HasPrefix(rest, " ") && !strings.Contains(rest, "\n"):
		next = strings.TrimPrefix(rest, " ")
	case strings.HasPrefix(rest, "\n"):
		cut := strings.LastIndex(rest, "\n")
		middle, next = rest[:cut+1], rest[cut+1:]
	}
	if next == "" {
		return "", false
	}
	if middle == "" {
		middle = " "
	}
	return head + middle + fmt.Sprintf("Run `fbull backtest delete %s` first. ", name) + next, true
}

// unreachableHop names the hop that failed for the --verbose details. Only
// the data service sends the details marker, so a status with the marker
// is the Zipline pod failing to reach Postgres, and one without it is the
// broker's own call to the Zipline pod failing.
func unreachableHop(fromDataService bool, reason, details string) string {
	if !fromDataService {
		return joinDetail("the broker couldn't reach the Zipline pod", details)
	}
	return joinDetail(joinDetail("the Zipline pod couldn't reach Postgres", reason), details)
}

func joinDetail(lead, detail string) string {
	if detail == "" {
		return lead
	}
	return lead + ": " + detail
}

// ingestFailureLead is the fixed plain sentence for the failure, then the
// sentence that ends the message when the backtest was removed.
func ingestFailureLead(failure ingestFailure) (string, string) {
	switch failure.kind {
	case ingestMissingDriver:
		reason := "the data service is missing its database driver"
		if failure.driver != "" {
			reason += " (" + failure.driver + ")"
		}
		return reason + ". This is a bug in the Foreverbull install, not in your strategy or settings.",
			"Nothing was saved, so run `fbull backtest create` again once it's fixed."
	case ingestUnreachable:
		return "Foreverbull's data service isn't reachable. Run `fbenv status` to check it's running.",
			"Nothing was saved, so run `fbull backtest create` again after that."
	case ingestFailed:
		return ingestFallbackReason + ".",
			"Nothing was saved, so run `fbull backtest create` again once it's fixed."
	default:
		panic(fmt.Sprintf("unknown ingest failure kind %d", failure.kind))
	}
}

// ingestFailureMessage is the step's whole message: the data service's own
// sentence for a refused ingest, otherwise a fixed lead. "Nothing was saved" is
// only true when the backtest was removed; otherwise the user has to delete
// it before the same create can run again.
func ingestFailureMessage(name string, failure ingestFailure, removed bool) string {
	if sentence, ok := refusedSentence(name, failure, removed); ok {
		return redactURLPasswords(sentence)
	}
	lead, saved := ingestFailureLead(failure)
	var text strings.Builder
	fmt.Fprintf(&text, "Couldn't load data for backtest '%s': %s ", name, lead)
	if removed {
		text.WriteString(saved)
	} else {
		fmt.Fprintf(&text, "The backtest couldn't be removed, so run `fbull backtest delete %s` "+
			"before you run `fbull backtest create` again.", name)
	}
	if failure.details != "" {
		text.WriteString(ingestDetailsMarker)
		text.WriteString(failure.details)
	}
	return redactURLPasswords(text.String())
}

// discardFailedIngest deletes the backtest after its ingest failed. The
// INGESTING step only runs inside a create, right after CREATING stored the
// backtest, so the record is this create's own. The backtest record, its
// periods and its assets are removed so a retry under the same name works.
// Bars are stored per provider and symbol, not per backtest, so they are
// kept. The returned error is the step's whole message.
func discardFailedIngest(ctx context.Context, backtests repository.Backtest, name, id string, cause error) error {
	failure := readIngestFailure(cause)
	err := deleteStoredBacktest(ctx, backtests, name, id)
	removed := err == nil || errors.Is(err, pgx.ErrNoRows)
	if !removed {
		log.Warn().Err(err).Str("backtest", name).Msg("error deleting backtest after failed ingest")
	}
	return plainSentence(ingestFailureMessage(name, failure, removed))
}
