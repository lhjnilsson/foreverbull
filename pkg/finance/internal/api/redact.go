package api

import (
	"errors"
	"net/url"
	"strings"
)

// redactURLError strips the query and fragment from a *url.Error's URL in
// err's text. Yahoo requests carry the session crumb as ?crumb=, and the
// HTTP client puts the full URL in its error, so without this the crumb
// reaches logs and RPC errors. Other errors are returned unchanged.
func redactURLError(err error) error {
	if err == nil {
		return nil
	}
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	redacted := *urlErr
	redacted.URL = redactURL(urlErr.URL)
	msg := strings.ReplaceAll(err.Error(), urlErr.Error(), redacted.Error())
	// Some wrappers format the URL on their own; drop any raw copy too.
	if urlErr.URL != redacted.URL {
		msg = strings.ReplaceAll(msg, urlErr.URL, redacted.URL)
	}
	return errors.New(msg)
}

// redactURL keeps scheme, host and path. An unparsable URL is cut at the
// first '?' or '#'.
func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		if i := strings.IndexAny(raw, "?#"); i >= 0 {
			return raw[:i]
		}
		return raw
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: parsed.Path}).String()
}
