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
	// ingestDatabaseUnreachable is the data service failing to reach Postgres.
	ingestDatabaseUnreachable
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
// Unavailable for a database it cannot reach. The error text never becomes
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
		failure.kind = ingestDatabaseUnreachable
	}
	return failure
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
	case ingestDatabaseUnreachable:
		return "the Foreverbull database isn't reachable. Run `fbenv status` to check it's running.",
			"Nothing was saved, so run `fbull backtest create` again after that."
	case ingestFailed:
		return ingestFallbackReason + ".",
			"Nothing was saved, so run `fbull backtest create` again once it's fixed."
	default:
		panic(fmt.Sprintf("unknown ingest failure kind %d", failure.kind))
	}
}

// ingestFailureMessage is the step's whole message. "Nothing was saved" is
// only true when the backtest was removed; otherwise the user has to delete
// it before the same create can run again.
func ingestFailureMessage(name string, failure ingestFailure, removed bool) string {
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
func discardFailedIngest(ctx context.Context, backtests repository.Backtest, name string, cause error) error {
	failure := readIngestFailure(cause)
	err := backtests.Delete(ctx, name)
	removed := err == nil || errors.Is(err, pgx.ErrNoRows)
	if !removed {
		log.Warn().Err(err).Str("backtest", name).Msg("error deleting backtest after failed ingest")
	}
	return plainSentence(ingestFailureMessage(name, failure, removed))
}
