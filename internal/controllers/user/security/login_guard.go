package security

import (
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	ErrRateLimitExceeded       = errors.New("muitas tentativas do mesmo IP")
	ErrEmailTemporarilyBlocked = errors.New("email temporariamente bloqueado por múltiplas falhas")
)

type LoginGuard struct {
	mu              sync.Mutex
	perIPAttempts   map[string][]time.Time
	perEmailFailure map[string]emailFailureState
}

type emailFailureState struct {
	failedCount  int
	blockedUntil time.Time
}

func NewLoginGuard() *LoginGuard {
	return &LoginGuard{
		perIPAttempts:   make(map[string][]time.Time),
		perEmailFailure: make(map[string]emailFailureState),
	}
}

func (g *LoginGuard) AllowAttempt(ip, email string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UTC()
	g.cleanupOldIPAttemptsLocked(now)

	normalizedIP := strings.TrimSpace(ip)
	if normalizedIP == "" {
		normalizedIP = "unknown"
	}

	attempts := g.perIPAttempts[normalizedIP]
	if len(attempts) >= 5 {
		return ErrRateLimitExceeded
	}
	g.perIPAttempts[normalizedIP] = append(attempts, now)

	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	state := g.perEmailFailure[normalizedEmail]
	if state.blockedUntil.After(now) {
		return ErrEmailTemporarilyBlocked
	}

	return nil
}

func (g *LoginGuard) RegisterFailure(email string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UTC()
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	state := g.perEmailFailure[normalizedEmail]
	state.failedCount++
	if state.failedCount >= 10 {
		state.blockedUntil = now.Add(15 * time.Minute)
		state.failedCount = 0
	}
	g.perEmailFailure[normalizedEmail] = state
}

func (g *LoginGuard) RegisterSuccess(email string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	delete(g.perEmailFailure, normalizedEmail)
}

func (g *LoginGuard) cleanupOldIPAttemptsLocked(now time.Time) {
	for ip, attempts := range g.perIPAttempts {
		var filtered []time.Time
		for _, at := range attempts {
			if now.Sub(at) < time.Minute {
				filtered = append(filtered, at)
			}
		}
		if len(filtered) == 0 {
			delete(g.perIPAttempts, ip)
			continue
		}
		g.perIPAttempts[ip] = filtered
	}
}
