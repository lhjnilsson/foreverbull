// Package redact removes secrets from error text before it leaves the
// process: in a log line, an RPC error, or a NATS task state.
package redact

import (
	"errors"
	"net/url"
	"strings"
)

// URLError strips the query and fragment from a *url.Error's URL in err's
// text. Yahoo requests carry the session crumb as ?crumb=, and the HTTP
// client puts the full URL in its error, so without this the crumb reaches
// logs, RPC errors, and task states. An error with no *url.Error in its
// chain, and nil, are returned unchanged.
//
// The scrubbed error is built with errors.New from the redacted text, so it
// drops err's chain: errors.Is and errors.As on the result don't find the
// *url.Error, a sentinel, or a typed error that err wrapped. Check those on
// err first and scrub only the error that is logged or returned.
func URLError(err error) error {
	if err == nil {
		return nil
	}
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	redacted := *urlErr
	redacted.URL = URL(urlErr.URL)
	msg := strings.ReplaceAll(err.Error(), urlErr.Error(), redacted.Error())
	// Some wrappers format the URL on their own; drop any raw copy too.
	if urlErr.URL != redacted.URL {
		msg = strings.ReplaceAll(msg, urlErr.URL, redacted.URL)
	}
	return errors.New(msg)
}

// URL keeps scheme, host and path. An unparsable URL is cut at the first
// '?' or '#'.
func URL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		if i := strings.IndexAny(raw, "?#"); i >= 0 {
			return raw[:i]
		}
		return raw
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: parsed.Path}).String()
}
