package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

const (
	loginIPLimit     = 30
	loginWindow      = 15 * time.Minute
	loginBackoffBase = time.Second
	loginBackoffCap  = 15 * time.Minute
	// loginKeyCap bounds the in-memory maps for the life of the process.
	loginKeyCap = 1024
)

type pairState struct {
	fails int
	until time.Time
}

type failureLimiter struct {
	mu      sync.Mutex
	pairs   map[string]pairState
	byIP    map[string][]time.Time
	pending map[string]int
}

func newFailureLimiter() *failureLimiter {
	return &failureLimiter{
		pairs:   map[string]pairState{},
		byIP:    map[string][]time.Time{},
		pending: map[string]int{},
	}
}

// Begin reserves one in-flight login before the password is hashed.
// The address cap counts every attempt. A wrong password backs off that
// username on that address only.
func (l *failureLimiter) Begin(username, remote string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	if l.ipOver(remote) {
		return false
	}
	if state, ok := l.pairs[pairKey(username, remote)]; ok && now.Before(state.until) {
		return false
	}
	l.pending[remote]++
	return true
}

// End releases a Begin reservation. Every finished attempt counts toward the
// address cap. failed starts or lengthens the backoff for this username and
// address. A database error is not a failed password.
func (l *failureLimiter) End(username, remote string, failed bool, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	releasePending(l.pending, remote)
	l.rememberIP(remote, now)
	key := pairKey(username, remote)
	if !failed {
		delete(l.pairs, key)
		return
	}
	l.rememberFailure(key, now)
}

func (l *failureLimiter) userFailures(username string, now time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	total := 0
	prefix := username + "\x00"
	for key, state := range l.pairs {
		if strings.HasPrefix(key, prefix) && now.Before(state.until) {
			total += state.fails
		}
	}
	return total
}

func (l *failureLimiter) ipOver(remote string) bool {
	return len(l.byIP[remote])+l.pending[remote] >= loginIPLimit
}

func (l *failureLimiter) rememberIP(remote string, now time.Time) {
	if _, ok := l.byIP[remote]; !ok && l.size() >= loginKeyCap {
		return
	}
	l.byIP[remote] = append(l.byIP[remote], now)
}

func (l *failureLimiter) rememberFailure(key string, now time.Time) {
	state, ok := l.pairs[key]
	if !ok && l.size() >= loginKeyCap {
		return
	}
	state.fails++
	state.until = now.Add(backoff(state.fails))
	l.pairs[key] = state
}

func (l *failureLimiter) size() int {
	return len(l.pairs) + len(l.byIP)
}

func (l *failureLimiter) prune(now time.Time) {
	cutoff := now.Add(-loginWindow)
	l.byIP = pruneStamps(l.byIP, cutoff)
	for key, state := range l.pairs {
		// Keep the failure count until the window passes, so the next
		// wrong password waits longer instead of starting over.
		if !state.until.After(cutoff) {
			delete(l.pairs, key)
		}
	}
}

func pairKey(username, remote string) string {
	return username + "\x00" + remote
}

func backoff(fails int) time.Duration {
	if fails < 1 {
		fails = 1
	}
	shift := fails - 1
	if shift > 16 {
		shift = 16
	}
	delay := loginBackoffBase << shift
	if delay <= 0 || delay > loginBackoffCap {
		return loginBackoffCap
	}
	return delay
}

func releasePending(pending map[string]int, key string) {
	if pending[key] == 0 {
		return
	}
	pending[key]--
	if pending[key] == 0 {
		delete(pending, key)
	}
}

func pruneStamps(in map[string][]time.Time, cutoff time.Time) map[string][]time.Time {
	for key, stamps := range in {
		kept := stamps[:0]
		for _, stamp := range stamps {
			if stamp.After(cutoff) {
				kept = append(kept, stamp)
			}
		}
		if len(kept) == 0 {
			delete(in, key)
			continue
		}
		in[key] = kept
	}
	return in
}

func clientAddr(r *http.Request) string {
	remote := remoteHost(r)
	if r == nil || !environment.GetAuthTrustProxy() {
		return remote
	}
	// Setup refuses a bad list, so GetAuthTrustedProxyNets panics rather than
	// trusting a partial list or quietly falling back to the remote address.
	trusted := environment.GetAuthTrustedProxyNets()
	if host := rightmostUntrusted(r.Header.Get("X-Forwarded-For"), trusted); host != "" {
		return host
	}
	return remote
}

func remoteHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func rightmostUntrusted(forwarded string, trusted []*net.IPNet) string {
	parts := strings.Split(forwarded, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		host := trim(parts[i])
		if host == "" {
			continue
		}
		if ipInNets(net.ParseIP(unbracket(host)), trusted) {
			continue
		}
		return host
	}
	return ""
}

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, network := range nets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func unbracket(host string) string {
	if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		return host[1 : len(host)-1]
	}
	return host
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
