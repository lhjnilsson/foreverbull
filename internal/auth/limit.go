package auth

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

const (
	loginUserLimit = 5
	loginIPLimit   = 30
	loginWindow    = 15 * time.Minute
	// loginKeyCap bounds the in-memory failure map for the life of the process.
	loginKeyCap = 1024
)

type failureLimiter struct {
	mu     sync.Mutex
	byUser map[string][]time.Time
	byIP   map[string][]time.Time
}

func newFailureLimiter() *failureLimiter {
	return &failureLimiter{
		byUser: map[string][]time.Time{},
		byIP:   map[string][]time.Time{},
	}
}

// Limited reports whether this username or address is over the login failure cap.
// It does not record a failure. Over the limit, login skips argon2.
func (l *failureLimiter) Limited(username, remote string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	return len(l.byUser[username]) >= loginUserLimit || len(l.byIP[remote]) >= loginIPLimit
}

// Add records one failure. The maps drop expired stamps and stay within loginKeyCap.
func (l *failureLimiter) Add(username, remote string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	l.byUser[username] = append(l.byUser[username], now)
	l.byIP[remote] = append(l.byIP[remote], now)
	l.bound()
}

func (l *failureLimiter) prune(now time.Time) {
	cutoff := now.Add(-loginWindow)
	l.byUser = pruneStamps(l.byUser, cutoff)
	l.byIP = pruneStamps(l.byIP, cutoff)
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

func (l *failureLimiter) bound() {
	for len(l.byUser)+len(l.byIP) > loginKeyCap {
		for key := range l.byUser {
			delete(l.byUser, key)
			break
		}
		if len(l.byUser)+len(l.byIP) <= loginKeyCap {
			return
		}
		for key := range l.byIP {
			delete(l.byIP, key)
			break
		}
	}
}

func clientAddr(r *http.Request) string {
	if environment.GetAuthTrustProxy() {
		forwarded := r.Header.Get("X-Forwarded-For")
		if forwarded != "" {
			host := forwarded
			if comma := indexByte(forwarded, ','); comma >= 0 {
				host = forwarded[:comma]
			}
			return trim(host)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
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
