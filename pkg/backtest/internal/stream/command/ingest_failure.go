package command

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

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
}

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
	return failure
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

// ingestFailureLead is the fixed plain sentence for the failure, the
// sentence that ends the message when the backtest was removed, and the next
// step that follows the delete step when it couldn't be removed. Each is
// written out as it reads.
func ingestFailureLead(failure ingestFailure) (string, string, string) {
	switch failure.kind {
	case ingestMissingDriver:
		reason := "the data service is missing its database driver"
		if failure.driver != "" {
			reason += " (" + failure.driver + ")"
		}
		return reason + ". This is a bug in the Foreverbull install, not in your strategy or settings.",
			"Nothing was saved, so run `fbull backtest create` again once it's fixed.",
			"Run `fbull backtest create` again once it's fixed."
	case ingestUnreachable:
		return "Foreverbull's data service isn't reachable. Run `fbenv status` to check it's running.",
			"Nothing was saved, so run `fbull backtest create` again after that.",
			"Run `fbull backtest create` again after that."
	case ingestFailed:
		return ingestFallbackReason + ".",
			"Nothing was saved, so run `fbull backtest create` again once it's fixed.",
			"Run `fbull backtest create` again once it's fixed."
	default:
		panic(fmt.Sprintf("unknown ingest failure kind %d", failure.kind))
	}
}

// notSavedSentence says a failed create kept nothing. It is only true when
// the backtest was removed.
const notSavedSentence = "The backtest wasn't saved."

// notRemovedSentence replaces notSavedSentence when the backtest couldn't be
// removed after a failed create.
const notRemovedSentence = "The backtest was partly saved and couldn't be removed."

// deleteFirst is the next step when the backtest couldn't be removed: its
// own sentence to delete it first, then the step the failure gives when it
// was removed, exactly as that step reads.
func deleteFirst(name, next string) string {
	return fmt.Sprintf("Run `fbull backtest delete %s` first. %s", name, next)
}

// savedAndNext is the saved sentence and the next step of a failed create:
// notSavedSentence and next when the backtest was removed, otherwise
// notRemovedSentence and next behind the delete-first step.
func savedAndNext(name, next string, removed bool) (string, string) {
	if removed {
		return notSavedSentence, next
	}
	return notRemovedSentence, deleteFirst(name, next)
}

// ingestFailureMessage is the step's whole message. "Nothing was saved" is
// only true when the backtest was removed; otherwise it was partly saved,
// and the user deletes it first.
func ingestFailureMessage(name string, failure ingestFailure, removed bool) string {
	lead, saved, next := ingestFailureLead(failure)
	var text strings.Builder
	fmt.Fprintf(&text, "Couldn't load data for backtest '%s': %s ", name, lead)
	if removed {
		text.WriteString(saved)
	} else {
		text.WriteString(notRemovedSentence + " " + deleteFirst(name, next))
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
//
// A canceled create returns ctx.Err() itself, unwrapped, so it reads as
// every other canceled create and never as a data service failure. The
// backtest is still removed, on a context that outlives the cancel.
func discardFailedIngest(ctx context.Context, backtests repository.Backtest, name, id string, cause error) error {
	err := deleteAfterFailure(ctx, backtests, name, id)
	removed := err == nil || errors.Is(err, pgx.ErrNoRows)
	if !removed {
		log.Warn().Err(err).Str("backtest", name).Msg("error deleting backtest after failed ingest")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return plainSentence(ingestFailureMessage(name, readIngestFailure(cause), removed))
}

// cleanupTimeout bounds the delete that runs after a failed or canceled
// create.
const cleanupTimeout = 10 * time.Second

// deleteAfterFailure deletes this create's backtest on ctx without its
// cancel, so a canceled create still leaves nothing behind, within
// cleanupTimeout.
func deleteAfterFailure(ctx context.Context, backtests repository.Backtest, name, id string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	return deleteStoredBacktest(cleanupCtx, backtests, name, id)
}
