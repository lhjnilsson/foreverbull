package marketdata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Request rates per provider, shared by every download and ingest in the
// process. One 10,000-symbol request must not get the broker throttled and
// stop the download for everyone. Alpaca's free market-data plan allows 200
// requests a minute, so 3 a second stays under it. Yahoo and Stooq publish
// no limit; at these rates a 10,000-symbol download takes about an hour on
// Yahoo or Alpaca and about 85 minutes on Stooq, under downloadTimeLimit.
const (
	yahooRequestsPerSecond  = 5
	stooqRequestsPerSecond  = 2
	alpacaRequestsPerSecond = 3
)

// Backoff after a 429. Retry-After wins when it is a number of seconds up to
// maxRateLimitBackoff; otherwise the wait doubles from firstRateLimitBackoff.
const (
	firstRateLimitBackoff = 2 * time.Second
	maxRateLimitBackoff   = 30 * time.Second
)

// rateLimitRetries is how many times a Yahoo or Stooq request is sent again
// after a 429. Alpaca's SDK already retries a 429 itself, so its transport
// sends each request once and only slows the shared limiter down.
const rateLimitRetries = 3

// requestLimiter spaces requests to one provider at least interval apart,
// across every goroutine that holds it. A 429 pushes the next slot back for
// everyone, not only for the request that got it.
type requestLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newRequestLimiter(requestsPerSecond int) *requestLimiter {
	return &requestLimiter{interval: time.Second / time.Duration(requestsPerSecond)}
}

// providerLimiters holds one limiter per provider for the life of the
// process. The keys are the NormalizeProvider allowlist, so it can't grow.
var providerLimiters = map[string]*requestLimiter{
	"yahoo":  newRequestLimiter(yahooRequestsPerSecond),
	"stooq":  newRequestLimiter(stooqRequestsPerSecond),
	"alpaca": newRequestLimiter(alpacaRequestsPerSecond),
}

// limiterFor is the shared limiter of provider. An unknown provider is a
// programmer error: every client is built for one of the three.
func limiterFor(provider string) *requestLimiter {
	limiter, ok := providerLimiters[provider]
	if !ok {
		panic(fmt.Sprintf("no request limiter for provider %q", provider))
	}
	return limiter
}

// wait blocks until the caller's slot, or until ctx ends. The slot is taken
// when wait is called, so callers are served in arrival order.
func (l *requestLimiter) wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	now := time.Now()
	slot := l.next
	if slot.Before(now) {
		slot = now
	}
	l.next = slot.Add(l.interval)
	l.mu.Unlock()

	delay := time.Until(slot)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// pause keeps every caller from sending before now plus backoff.
func (l *requestLimiter) pause(backoff time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	until := time.Now().Add(backoff)
	if l.next.Before(until) {
		l.next = until
	}
}

// rateLimitBackoff is the wait after the attempt-th 429 in a row (0-based).
func rateLimitBackoff(header http.Header, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header.Get("Retry-After")); err == nil {
		retryAfter := time.Duration(seconds) * time.Second
		if retryAfter > 0 && retryAfter <= maxRateLimitBackoff {
			return retryAfter
		}
	}
	backoff := firstRateLimitBackoff << attempt
	if backoff > maxRateLimitBackoff {
		return maxRateLimitBackoff
	}
	return backoff
}

// limitedTransport sends each request in a slot of the provider's shared
// limiter. The wait uses the request ctx, so the client timeout and the
// caller's cancel bound it like the request itself.
type limitedTransport struct {
	provider string
	limiter  *requestLimiter
	base     http.RoundTripper
	// retries is how many times a 429 is sent again; the last 429 is
	// returned to the caller as it came.
	retries int
}

func newLimitedTransport(provider string, limiter *requestLimiter, base http.RoundTripper, retries int) *limitedTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &limitedTransport{provider: provider, limiter: limiter, base: base, retries: retries}
}

func (t *limitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only a request without a body is sent again; the price requests are
	// all GETs.
	retries := t.retries
	if req.Body != nil && req.Body != http.NoBody {
		retries = 0
	}
	for attempt := 0; ; attempt++ {
		if err := t.limiter.wait(req.Context()); err != nil {
			return nil, err
		}
		resp, err := t.base.RoundTrip(req) //nolint:bodyclose // returned to the caller or closed below
		if err != nil {
			return nil, err //nolint:wrapcheck // passed through as is
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}
		backoff := rateLimitBackoff(resp.Header, attempt)
		t.limiter.pause(backoff)
		log.Warn().Str("provider", t.provider).Dur("backoff", backoff).Int("attempt", attempt+1).
			Msg("price provider answered 429, slowing every download to it")
		if attempt < retries {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
			_ = resp.Body.Close()
			continue
		}
		return resp, nil
	}
}
