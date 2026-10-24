package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"sync"
)

const (
	listenerSecretLen = 32
	// maxListenerSecrets is above the 27000-27015 port range, with room for
	// a session that has been minted and has not stopped yet.
	maxListenerSecrets = 64
	sessionMetadata    = "x-foreverbull-session"
)

// SessionMetadata is the gRPC metadata key for a listener secret. It has no -bin suffix.
func SessionMetadata() string {
	return sessionMetadata
}

type secretMap struct {
	mu sync.Mutex
	m  map[string]string
}

var (
	backtestSecrets = &secretMap{}
	tradingSecrets  = &secretMap{}
)

// MintBacktestSecret stores a new secret for a backtest session. The value
// stays in this process. It is not a column and it is not published.
func MintBacktestSecret(sessionID string) (string, error) {
	return backtestSecrets.mint(sessionID)
}

// LookupBacktestSecret returns the secret minted for sessionID.
func LookupBacktestSecret(sessionID string) (string, bool) {
	return backtestSecrets.lookup(sessionID)
}

// ForgetBacktestSecret drops the secret when the listener stops.
func ForgetBacktestSecret(sessionID string) {
	backtestSecrets.forget(sessionID)
}

// MintTradingSecret stores a new secret for a trading session.
func MintTradingSecret(sessionID string) (string, error) {
	return tradingSecrets.mint(sessionID)
}

// LookupTradingSecret returns the secret minted for sessionID.
func LookupTradingSecret(sessionID string) (string, bool) {
	return tradingSecrets.lookup(sessionID)
}

// ForgetTradingSecret drops the secret when the listener stops.
func ForgetTradingSecret(sessionID string) {
	tradingSecrets.forget(sessionID)
}

func (s *secretMap) mint(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("%s: session id is empty", sessionMetadata)
	}
	raw := make([]byte, listenerSecretLen)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("%s: %w", sessionMetadata, err)
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	for i := range raw {
		raw[i] = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]string{}
	}
	if _, exists := s.m[id]; !exists && len(s.m) >= maxListenerSecrets {
		return "", fmt.Errorf("%s: too many open sessions", sessionMetadata)
	}
	s.m[id] = secret
	return secret, nil
}

func (s *secretMap) lookup(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	secret, ok := s.m[id]
	return secret, ok && secret != ""
}

func (s *secretMap) forget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
}

// SecretMatches compares a presented listener secret with the minted value.
// A different length still spends a compare on the expected secret.
func SecretMatches(got, want string) bool {
	gb := []byte(got)
	wb := []byte(want)
	if len(gb) != len(wb) || len(wb) == 0 {
		if len(wb) > 0 {
			subtle.ConstantTimeCompare(wb, wb)
		}
		return false
	}
	return subtle.ConstantTimeCompare(gb, wb) == 1
}
