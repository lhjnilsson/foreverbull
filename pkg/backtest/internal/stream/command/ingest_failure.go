package command

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
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
	return head + middle + deleteFirst(name, next), true
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

// notSavedSentence, notRemovedSentence, deleteFirst, and savedAndNext live
// in usercopy, so the API's sentence for a create that failed on a full task
// queue reads the same.
const (
	notSavedSentence   = usercopy.BacktestNotSaved
	notRemovedSentence = usercopy.BacktestNotRemoved
)

func deleteFirst(name, next string) string { return usercopy.BacktestDeleteFirst(name, next) }

func savedAndNext(name, next string, removed bool) (string, string) {
	return usercopy.BacktestSavedAndNext(name, next, removed)
}

// ingestFailureMessage is the step's whole message: the data service's own
// sentence for a refused ingest, otherwise a fixed lead. "Nothing was saved"
// is only true when the backtest was removed; otherwise it was partly saved,
// and the user deletes it first.
func ingestFailureMessage(name string, failure ingestFailure, removed bool) string {
	if sentence, ok := refusedSentence(name, failure, removed); ok {
		return redactURLPasswords(sentence)
	}
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
// every other canceled create and never as a data service failure, when the
// backtest was removed. The delete runs on a context that outlives the
// cancel. When that delete fails, the name is still taken, so the message
// says to delete the backtest first instead of reporting only the cancel.
func discardFailedIngest(ctx context.Context, backtests repository.Backtest, name, id string, cause error) error {
	err := deleteAfterFailure(ctx, backtests, name, id)
	removed := err == nil || errors.Is(err, pgx.ErrNoRows)
	if !removed {
		log.Warn().Err(err).Str("backtest", name).Msg("error deleting backtest after failed ingest")
	}
	var result error
	if ctxErr := ctx.Err(); ctxErr != nil {
		if removed {
			return ctxErr
		}
		result = plainSentence(endedIngestMessage(name, ctxErr))
	} else {
		result = plainSentence(ingestFailureMessage(name, readIngestFailure(cause), removed))
	}
	return markKept(removed, result)
}

// endedIngestNextStep is the step after a canceled or timed-out ingest whose
// backtest could not be removed. Neither is a data-service failure, so the
// step is to run create again once the leftover backtest is deleted.
const endedIngestNextStep = "Run `fbull backtest create` again."

// endedIngestMessage is the step when a canceled or timed-out ingest could
// not remove the backtest. The lead names which one, then the partly-saved
// sentence, then the step to delete it first. There is no details line.
func endedIngestMessage(name string, cause error) string {
	return endedIngestLead(name, cause) + notRemovedSentence + " " + deleteFirst(name, endedIngestNextStep)
}

// endedIngestPricesLeft is the same lead when the backtest row was removed
// but the stored prices were not. The create step stays the one this ending
// already uses.
func endedIngestPricesLeft(name string, cause error) string {
	return endedIngestLead(name, cause) + usercopy.BacktestPricesLeft() + " " + endedIngestNextStep
}

func endedIngestLead(name string, cause error) string {
	reason := "the create was canceled"
	if errors.Is(cause, context.DeadlineExceeded) {
		reason = "the create timed out"
	}
	return fmt.Sprintf("Couldn't load data for backtest '%s': %s. ", name, reason)
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

// deleteIngestionObject removes the object CreateObject wrote for an ingest
// that then failed. The delete uses the same deadline as the backtest
// delete, including after the ingest context is canceled.
func deleteIngestionObject(ctx context.Context, store storage.Storage, key string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	if err := store.DeleteObject(cleanupCtx, storage.IngestionsBucket, key); err != nil {
		return fmt.Errorf("error deleting ingestion %s: %w", key, err)
	}
	return nil
}

// pricesLeftError is the user-facing failure when stored prices could not be
// removed. Error is that sentence. Unwrap keeps the row error and the storage
// error. A canceled create whose row was removed is still
// errors.Is(context.Canceled), so the task is stored as canceled. A row that
// stayed is not, or the subscriber would drop the delete-first sentence.
// The storage text is logged, not printed.
type pricesLeftError struct {
	text   string
	rowErr error
	err    error
}

func (e *pricesLeftError) Error() string { return e.text }

func (e *pricesLeftError) Unwrap() []error { return []error{e.rowErr, e.err} }

// finishFailedIngest removes the stored prices before the backtest row.
// A same-name create in the other order could have its new prices deleted.
// When the prices stay, the returned sentence does not say nothing was saved.
func finishFailedIngest(ctx context.Context, store storage.Storage, key string, backtests repository.Backtest, name, id string, cause error) error {
	return cleanupStoredPrices(ctx, store, key, name, func() error {
		return discardFailedIngest(ctx, backtests, name, id, cause)
	})
}

// cleanupStoredPrices deletes the prices, then runs remove, which deletes the
// backtest row and builds its sentence. A failed price delete is logged and
// joined as BacktestPricesLeft, with the storage error still wrapped.
func cleanupStoredPrices(ctx context.Context, store storage.Storage, key, name string, remove func() error) error {
	var delErr error
	if store != nil && key != "" {
		delErr = deleteIngestionObject(ctx, store, key)
		if delErr != nil {
			log.Warn().Err(delErr).Str("backtest", name).Str("key", key).Msg("could not delete stored prices after failed ingest")
		}
	}
	rowErr := remove()
	if delErr == nil {
		return rowErr
	}
	text, shapeErr := pricesLeftMessage(name, rowErr)
	if shapeErr != nil {
		return &pricesLeftError{
			text:   text,
			rowErr: rowErr,
			err:    errors.Join(shapeErr, delErr),
		}
	}
	return &pricesLeftError{
		text:   text,
		rowErr: rowErr,
		err:    delErr,
	}
}

// rowKept marks a failure whose backtest row is still stored. Error is the
// sentence the row delete already built.
type rowKept struct{ error }

func (e *rowKept) Unwrap() error { return e.error }

func markKept(removed bool, err error) error {
	if removed || err == nil {
		return err
	}
	return &rowKept{error: err}
}

func rowStayed(err error) bool {
	var kept *rowKept
	return errors.As(err, &kept)
}

// pricesLeftMessage is the user text when the price delete failed.
// A removed row whose context ended uses the cancel lead or the timeout lead.
// A row that stayed keeps its sentence, with the not-removed clause naming
// the prices as well, before the steps. A nil row error has no surrounding
// sentence, so rowErr.Error is not called.
func pricesLeftMessage(name string, rowErr error) (string, error) {
	if rowErr == nil {
		return usercopy.BacktestPricesLeft() + " Run `fbull backtest create` again once it's fixed.", nil
	}
	if rowStayed(rowErr) {
		// The text of a context error is the sentinel. Use the cancel or
		// timeout sentence so the prices clause has a not-removed sentence
		// to join. The cut-off paths return that sentence themselves, with
		// the context error off the chain.
		message := rowErr.Error()
		if errors.Is(rowErr, context.Canceled) || errors.Is(rowErr, context.DeadlineExceeded) {
			message = endedIngestMessage(name, rowErr)
		}
		text, err := withPricesKept(message)
		if err != nil {
			// The row's own text stays in the log. The user gets the
			// reportable sentence, and err stays on the unwrap chain.
			log.Error().Str("backtest", name).Str("message", rowErr.Error()).Msg("row-kept message has no not-removed sentence")
			return usercopy.BacktestCleanupInternal(name, pricesKeptClause()), err
		}
		return text, nil
	}
	if errors.Is(rowErr, context.Canceled) || errors.Is(rowErr, context.DeadlineExceeded) {
		return endedIngestPricesLeft(name, rowErr), nil
	}
	return withoutSavedClaim(rowErr.Error(), usercopy.BacktestPricesLeft()), nil
}

// pricesKeptClause is the not-removed sentence when the row stayed and the
// stored prices could not be removed either. The period of the row sentence
// becomes the join, so the clause stays where that sentence was, before the
// delete-first step.
func pricesKeptClause() string {
	return strings.TrimSuffix(usercopy.BacktestNotRemoved, ".") +
		", and some of its stored prices couldn't be removed either."
}

// errRowKeptShape is a row-kept message with no not-removed sentence, so the
// prices clause has nowhere to go. The create fails with this error.
var errRowKeptShape = errors.New("row-kept message has no not-removed sentence")

// withPricesKept rewrites the not-removed sentence. A details marker stays
// where the message already put it. A message without that sentence is
// errRowKeptShape.
func withPricesKept(message string) (string, error) {
	if !strings.Contains(message, usercopy.BacktestNotRemoved) {
		return "", errRowKeptShape
	}
	return strings.Replace(message, usercopy.BacktestNotRemoved, pricesKeptClause(), 1), nil
}

// withoutSavedClaim replaces only the "nothing was saved" claim.
// BacktestPricesLeft has no step of its own, so a run step that followed
// the claim stays, and is not written twice.
func withoutSavedClaim(message, leftover string) string {
	const onceFixed = "Nothing was saved, so run `fbull backtest create` again once it's fixed."
	const afterThat = "Nothing was saved, so run `fbull backtest create` again after that."
	switch {
	case strings.Contains(message, onceFixed):
		return strings.Replace(message, onceFixed, leftover+" Run `fbull backtest create` again once it's fixed.", 1)
	case strings.Contains(message, afterThat):
		return strings.Replace(message, afterThat, leftover+" Run `fbull backtest create` again after that.", 1)
	case strings.Contains(message, usercopy.BacktestNotSaved):
		return strings.Replace(message, usercopy.BacktestNotSaved, leftover, 1)
	}
	head, tail, hasDetails := strings.Cut(message, ingestDetailsMarker)
	if hasDetails {
		return strings.TrimRight(head, " ") + " " + leftover + ingestDetailsMarker + tail
	}
	return strings.TrimRight(message, " ") + " " + leftover
}
