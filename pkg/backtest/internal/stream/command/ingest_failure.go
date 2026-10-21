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

type ingestFailure struct {
	reason  string
	details string
	// installBug is set when the data service reports that it is broken
	// itself, for example a missing database driver, so the user's backtest
	// and settings are not at fault.
	installBug bool
}

// readIngestFailure splits the data service's error into the reason and the
// details after ingestDetailsMarker. FailedPrecondition is the code the data
// service uses only for a broken install.
func readIngestFailure(cause error) ingestFailure {
	message := cause.Error()
	installBug := false
	var grpcErr interface{ GRPCStatus() *status.Status }
	if errors.As(cause, &grpcErr) {
		message = grpcErr.GRPCStatus().Message()
		installBug = grpcErr.GRPCStatus().Code() == codes.FailedPrecondition
	}
	reason, details, _ := strings.Cut(message, ingestDetailsMarker)
	reason = strings.TrimRight(strings.TrimSpace(reason), ".")
	if reason == "" {
		reason = ingestFallbackReason
	}
	return ingestFailure{
		reason:     reason,
		details:    strings.TrimSpace(details),
		installBug: installBug,
	}
}

// ingestFailureMessage is the step's whole message. "Nothing was saved" is
// only true when the backtest was removed; otherwise the user has to delete
// it before the same create can run again.
func ingestFailureMessage(name string, failure ingestFailure, removed bool) string {
	var text strings.Builder
	fmt.Fprintf(&text, "Couldn't load data for backtest '%s': %s.", name, failure.reason)
	if failure.installBug {
		text.WriteString(" This is a bug in the Foreverbull install, not in your strategy or settings.")
	}
	if removed {
		text.WriteString(" Nothing was saved, so run `fbull backtest create` again once it's fixed.")
	} else {
		fmt.Fprintf(&text, " The backtest could not be removed, so run `fbull backtest delete %s` "+
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
