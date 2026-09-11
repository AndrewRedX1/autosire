package session

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const cookieName = "autosire_session"

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]time.Time
}

func NewManager() *Manager {
	return &Manager{
		sessions: map[string]time.Time{},
	}
}

func (m *Manager) Create(w http.ResponseWriter, remember bool) error {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	expiresAt := time.Now().Add(12 * time.Hour)
	maxAge := 0
	if remember {
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
		maxAge = int((30 * 24 * time.Hour).Seconds())
	}

	m.mu.Lock()
	m.sessions[token] = expiresAt
	m.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
		Expires:  expiresAt,
	})
	return nil
}

func (m *Manager) Valid(r *http.Request) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return false
	}

	m.mu.RLock()
	expiresAt, ok := m.sessions[cookie.Value]
	m.mu.RUnlock()
	if !ok || time.Now().After(expiresAt) {
		if ok {
			m.mu.Lock()
			delete(m.sessions, cookie.Value)
			m.mu.Unlock()
		}
		return false
	}
	return true
}

func (m *Manager) Delete(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(cookieName); err == nil {
		m.mu.Lock()
		delete(m.sessions, cookie.Value)
		m.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
