package auth

import (
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestManagerSessionLifecycle(t *testing.T) {
	manager, err := New(testSecret, "phmon_operator", []string{"http://127.0.0.1:3005", "https://phmon.example.test"}, true)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0).UTC()
	token, watch, err := manager.CreateSession(testSecret, "127.0.0.1:1234", now)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || !watch.ExpiresAt.Equal(now.Add(8*time.Hour)) {
		t.Fatal("unexpected session")
	}
	if _, ok := manager.Validate(token, now.Add(time.Hour)); !ok {
		t.Fatal("valid session rejected")
	}
	if manager.CookieSecureForOrigin("http://127.0.0.1:3005") {
		t.Fatal("loopback development cookie unexpectedly secure")
	}
	if !manager.CookieSecureForOrigin("https://phmon.example.test") {
		t.Fatal("HTTPS cookie must be secure")
	}
	manager.Revoke(token)
	if _, ok := manager.Validate(token, now.Add(time.Hour)); ok {
		t.Fatal("revoked session accepted")
	}
	select {
	case <-watch.Done:
	default:
		t.Fatal("revocation did not notify watcher")
	}
}

func TestManagerRejectsInvalidConfigurationAndSecret(t *testing.T) {
	if _, err := New("short", "phmon_operator", []string{"https://example.test"}, false); err == nil {
		t.Fatal("short secret accepted")
	}
	if _, err := New(testSecret, "bad cookie", []string{"https://example.test"}, false); err == nil {
		t.Fatal("invalid cookie name accepted")
	}
	if _, err := New(testSecret, "phmon_operator", []string{"http://example.test"}, true); err == nil {
		t.Fatal("non-loopback insecure origin accepted")
	}
	manager, err := New(testSecret, "phmon_operator", []string{"https://example.test"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.CreateSession("wrong", "127.0.0.1:1", time.Now()); err != ErrInvalidSecret {
		t.Fatalf("wrong secret error = %v", err)
	}
	if !manager.OriginAllowed("https://example.test") || manager.OriginAllowed("https://evil.test") {
		t.Fatal("origin allowlist mismatch")
	}
}

func TestManagerRateLimitsLoginAttempts(t *testing.T) {
	manager, err := New(testSecret, "phmon_operator", []string{"https://example.test"}, false)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0).UTC()
	for i := 0; i < maxLoginAttempts; i++ {
		_, _, _ = manager.CreateSession("wrong", "192.0.2.1:1234", now)
	}
	if _, _, err := manager.CreateSession(testSecret, "192.0.2.1:1234", now); err != ErrRateLimited {
		t.Fatalf("expected rate limit, got %v", err)
	}
}
