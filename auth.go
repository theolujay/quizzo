package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	sessionCookie   = "quizzo_session"
	sessionDuration = 24 * time.Hour
)

// AuthManager manages authentication, persistence, and active sessions.
type AuthManager struct {
	mu       sync.RWMutex
	sessions map[string]time.Time
}

// NewAuthManager initializes AuthManager.
func NewAuthManager() *AuthManager {
	return &AuthManager{
		sessions: make(map[string]time.Time),
	}
}

// Authenticate verifies the username and password against the environment variables.
func (am *AuthManager) Authenticate(username, password string) bool {
	envUser := os.Getenv("QUIZZO_USERNAME")
	envPass := os.Getenv("QUIZZO_PASSWORD")

	if envUser == "" || envPass == "" {
		return false
	}

	return username == envUser && password == envPass
}

// CreateSession generates a new secure session token.
func (am *AuthManager) CreateSession() (string, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	token := hex.EncodeToString(b)
	am.sessions[token] = time.Now().Add(sessionDuration)
	return token, nil
}

// IsValidSession checks if the session is valid and not expired.
func (am *AuthManager) IsValidSession(token string) bool {
	if token == "" {
		return false
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	expiry, exists := am.sessions[token]
	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		delete(am.sessions, token)
		return false
	}

	// Extend the session duration on activity
	am.sessions[token] = time.Now().Add(sessionDuration)
	return true
}

// DeleteSession destroys a session.
func (am *AuthManager) DeleteSession(token string) {
	if token == "" {
		return
	}
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.sessions, token)
}

// RequireAuthentication restricts access to logged-in host users.
func (am *AuthManager) RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || !am.IsValidSession(cookie.Value) {
			http.Redirect(w, r, "/host/login", http.StatusSeeOther)
			return
		}

		// Ensure pages that require authentication are not stored in cache
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
