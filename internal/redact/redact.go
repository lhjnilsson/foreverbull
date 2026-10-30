// Package redact removes secrets from error text before it leaves the
// process: in a log line, an RPC error, or a NATS task state.
package redact

import (
	"errors"
	"net/url"
	"strings"
)

// URLError strips the query and fragment from every *url.Error's URL in
// err's text, including each error joined with errors.Join. Yahoo requests carry the session crumb as ?crumb=, and the HTTP
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
	urlErrs := urlErrors(err, nil)
	if len(urlErrs) == 0 {
		return err
	}
	msg := err.Error()
	for _, urlErr := range urlErrs {
		redacted := *urlErr
		redacted.URL = URL(urlErr.URL)
		msg = strings.ReplaceAll(msg, urlErr.Error(), redacted.Error())
		// Some wrappers format the URL on their own; drop any raw copy too.
		if urlErr.URL != redacted.URL {
			msg = strings.ReplaceAll(msg, urlErr.URL, redacted.URL)
		}
	}
	return errors.New(msg)
}

// urlErrors appends every *url.Error in err's tree to found.
func urlErrors(err error, found []*url.Error) []*url.Error {
	if err == nil {
		return found
	}
	if urlErr, ok := err.(*url.Error); ok { //nolint:errorlint // walking the tree by hand
		found = append(found, urlErr)
	}
	switch wrapped := err.(type) { //nolint:errorlint // walking the tree by hand
	case interface{ Unwrap() []error }:
		for _, e := range wrapped.Unwrap() {
			found = urlErrors(e, found)
		}
	case interface{ Unwrap() error }:
		found = urlErrors(wrapped.Unwrap(), found)
	}
	return found
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
