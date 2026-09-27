package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidSecret = errors.New("invalid operator secret")
	ErrRateLimited   = errors.New("operator login rate limited")
	ErrSessionLimit  = errors.New("operator session limit reached")
)

const (
	defaultSessionTTL = 8 * time.Hour
	maxSessions       = 256
	maxLoginBuckets   = 1024
	loginWindow       = time.Minute
	maxLoginAttempts  = 10
)

type sessionEntry struct {
	expiresAt time.Time
	done      chan struct{}
	closed    bool
}

type loginBucket struct {
	start time.Time
	count int
}

type Watch struct {
	ExpiresAt time.Time
	Done      <-chan struct{}
}

type Manager struct {
	secretHash     [32]byte
	cookieName     string
	allowedOrigins map[string]bool
	ttl            time.Duration

	mu       sync.Mutex
	sessions map[[32]byte]*sessionEntry
	logins   map[string]loginBucket
}

func New(secret, cookieName string, allowedOrigins []string, allowInsecureHTTP bool) (*Manager, error) {
	if len(secret) < 32 || len(secret) > 1024 || strings.TrimSpace(secret) != secret {
		return nil, errors.New("OPERATOR_ACCESS_SECRET must be 32 to 1024 characters without surrounding whitespace")
	}
	if !validCookieName(cookieName) {
		return nil, errors.New("OPERATOR_SESSION_COOKIE must use 1 to 64 letters, digits, '_' or '-'")
	}
	if len(allowedOrigins) == 0 {
		return nil, errors.New("OPERATOR_ALLOWED_ORIGINS must contain at least one origin")
	}
	origins := make(map[string]bool, len(allowedOrigins))
	for _, raw := range allowedOrigins {
		origin, secure, _, err := canonicalOrigin(raw)
		if err != nil {
			return nil, err
		}
		if !secure && !allowInsecureHTTP {
			return nil, errors.New("plain HTTP operator origins require OPERATOR_ALLOW_INSECURE_HTTP=true")
		}
		origins[origin] = secure
	}
	return &Manager{
		secretHash:     sha256.Sum256([]byte(secret)),
		cookieName:     cookieName,
		allowedOrigins: origins,
		ttl:            defaultSessionTTL,
		sessions:       make(map[[32]byte]*sessionEntry),
		logins:         make(map[string]loginBucket),
	}, nil
}

func validCookieName(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func canonicalOrigin(raw string) (string, bool, bool, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.User != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false, false, errors.New("OPERATOR_ALLOWED_ORIGINS entries must be absolute http(s) origins")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", false, false, errors.New("OPERATOR_ALLOWED_ORIGINS entries must not contain paths, query strings or fragments")
	}
	host := parsed.Hostname()
	loopback := strings.EqualFold(host, "localhost")
	if ip := net.ParseIP(host); ip != nil {
		loopback = ip.IsLoopback()
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), parsed.Scheme == "https", loopback, nil
}

func (m *Manager) CookieName() string { return m.cookieName }
func (m *Manager) TTL() time.Duration { return m.ttl }

func (m *Manager) OriginAllowed(origin string) bool {
	canonical, _, _, err := canonicalOrigin(origin)
	if err != nil {
		return false
	}
	_, ok := m.allowedOrigins[canonical]
	return ok
}

func (m *Manager) CookieSecureForOrigin(origin string) bool {
	canonical, _, _, err := canonicalOrigin(origin)
	if err != nil {
		return true
	}
	secure, ok := m.allowedOrigins[canonical]
	return !ok || secure
}

func (m *Manager) allowLogin(remoteAddr string, now time.Time) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = strings.TrimSpace(remoteAddr)
	}
	if host == "" {
		host = "unknown"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.logins) >= maxLoginBuckets {
		for key, bucket := range m.logins {
			if now.Sub(bucket.start) >= loginWindow {
				delete(m.logins, key)
			}
		}
	}
	if len(m.logins) >= maxLoginBuckets {
		return false
	}
	bucket := m.logins[host]
	if bucket.start.IsZero() || now.Sub(bucket.start) >= loginWindow {
		bucket = loginBucket{start: now}
	}
	bucket.count++
	m.logins[host] = bucket
	return bucket.count <= maxLoginAttempts
}

func (m *Manager) CreateSession(secret, remoteAddr string, now time.Time) (string, Watch, error) {
	candidate := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare(candidate[:], m.secretHash[:]) != 1 {
		if !m.allowLogin(remoteAddr, now) {
			return "", Watch{}, ErrRateLimited
		}
		return "", Watch{}, ErrInvalidSecret
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", Watch{}, err
	}
	token := "ps_" + base64.RawURLEncoding.EncodeToString(random)
	key := sha256.Sum256([]byte(token))
	expiresAt := now.Add(m.ttl)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpiredLocked(now)
	if len(m.sessions) >= maxSessions {
		return "", Watch{}, ErrSessionLimit
	}
	entry := &sessionEntry{expiresAt: expiresAt, done: make(chan struct{})}
	m.sessions[key] = entry
	return token, Watch{ExpiresAt: expiresAt, Done: entry.done}, nil
}

func (m *Manager) Validate(token string, now time.Time) (Watch, bool) {
	if len(token) < 4 || len(token) > 256 {
		return Watch{}, false
	}
	key := sha256.Sum256([]byte(token))
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.sessions[key]
	if !ok {
		return Watch{}, false
	}
	if !now.Before(entry.expiresAt) {
		m.closeEntryLocked(key, entry)
		return Watch{}, false
	}
	return Watch{ExpiresAt: entry.expiresAt, Done: entry.done}, true
}

func (m *Manager) Revoke(token string) {
	key := sha256.Sum256([]byte(token))
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.sessions[key]; ok {
		m.closeEntryLocked(key, entry)
	}
}

func (m *Manager) pruneExpiredLocked(now time.Time) {
	for key, entry := range m.sessions {
		if !now.Before(entry.expiresAt) {
			m.closeEntryLocked(key, entry)
		}
	}
}

func (m *Manager) closeEntryLocked(key [32]byte, entry *sessionEntry) {
	if !entry.closed {
		close(entry.done)
		entry.closed = true
	}
	delete(m.sessions, key)
}
