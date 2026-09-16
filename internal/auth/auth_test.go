package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("un mot de passe correct")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("un mot de passe correct", h) {
		t.Error("the right password was refused")
	}
	if VerifyPassword("un mot de passe incorrect", h) {
		t.Error("the wrong password was accepted")
	}
}

// Two hashes of the same password must differ: a salt that repeated would let one
// stolen file show which accounts share a password.
func TestHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("le meme mot de passe")
	b, _ := HashPassword("le meme mot de passe")
	if a == b {
		t.Error("two hashes of one password are identical: the salt is not random")
	}
}

func TestShortPasswordRefused(t *testing.T) {
	if _, err := HashPassword("court"); err == nil {
		t.Error("a five-character password was accepted")
	}
}

// A malformed hash must be refused, not crash and not accidentally match.
func TestGarbageHashRefused(t *testing.T) {
	for _, bad := range []string{"", "$argon2id$", "not a hash", "$argon2id$v=19$m=x,t=y,p=z$AA$BB"} {
		if VerifyPassword("anything", bad) {
			t.Errorf("garbage hash %q was accepted", bad)
		}
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := NewStore(time.Hour)
	sess, err := s.Create("pierre")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Lookup(sess.Token); !ok {
		t.Fatal("a fresh session was not found")
	}
	s.Revoke(sess.Token)
	if _, ok := s.Lookup(sess.Token); ok {
		t.Error("a revoked session is still valid")
	}
}

func TestExpiredSessionRefused(t *testing.T) {
	s := NewStore(time.Millisecond)
	sess, _ := s.Create("pierre")
	time.Sleep(5 * time.Millisecond)
	if _, ok := s.Lookup(sess.Token); ok {
		t.Error("an expired session is still valid")
	}
}

// The trap this guards: a forwarded header believed from anyone lets any client
// declare itself already on HTTPS, or wear someone else's address.
func TestForwardedHeadersIgnoredFromUntrustedSource(t *testing.T) {
	p, _ := NewTrustedProxies(nil)
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.99:5000"
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-For", "10.0.0.1")

	if got := p.Scheme(r); got != "http" {
		t.Errorf("scheme = %q from an untrusted source, want http", got)
	}
	if got := p.ClientIP(r); got != "192.168.1.99" {
		t.Errorf("client IP = %q from an untrusted source, want the real peer", got)
	}
}

func TestForwardedHeadersHonouredFromTrustedProxy(t *testing.T) {
	p, err := NewTrustedProxies([]string{"172.18.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "172.18.0.4:5000"
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-For", "10.0.0.1")

	if got := p.Scheme(r); got != "https" {
		t.Errorf("scheme = %q from a trusted proxy, want https", got)
	}
	if got := p.ClientIP(r); got != "10.0.0.1" {
		t.Errorf("client IP = %q from a trusted proxy, want the forwarded one", got)
	}
}

func TestLimiterStopsGuessing(t *testing.T) {
	l := NewLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("attempt %d refused too early", i+1)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Error("a fourth attempt was allowed")
	}
	if !l.Allow("5.6.7.8") {
		t.Error("another address was caught by the first one's limit")
	}
	l.Succeeded("1.2.3.4")
	if !l.Allow("1.2.3.4") {
		t.Error("a correct password did not clear the count")
	}
}

func TestDisabledServiceLetsEverythingThrough(t *testing.T) {
	s, err := NewService(Config{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	reached := false
	h := s.Require(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/aircraft", nil))
	if !reached {
		t.Error("authentication is disabled but the request was blocked")
	}
}

func TestEnabledServiceBlocksAnonymous(t *testing.T) {
	hash, _ := HashPassword("un mot de passe correct")
	s, err := NewService(Config{Enabled: true, Users: []User{{Name: "pierre", PasswordHash: hash}}})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.Require(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("an anonymous request reached the handler")
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/aircraft", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// Enabling authentication without an account would lock the owner out of their own
// receiver, so it is refused at startup rather than at the first request.
func TestEnabledWithoutUsersIsRefused(t *testing.T) {
	if _, err := NewService(Config{Enabled: true}); err == nil {
		t.Error("auth.enabled with no user was accepted")
	}
}

func TestSignInIssuesASession(t *testing.T) {
	hash, _ := HashPassword("un mot de passe correct")
	s, _ := NewService(Config{Enabled: true, Users: []User{{Name: "pierre", PasswordHash: hash}}})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	r.RemoteAddr = "192.168.1.28:5000"

	if _, err := s.SignIn(r, "pierre", "un mauvais mot de passe"); err == nil {
		t.Error("a wrong password was accepted")
	}
	sess, err := s.SignIn(r, "pierre", "un mot de passe correct")
	if err != nil {
		t.Fatalf("the right password was refused: %v", err)
	}
	rec := httptest.NewRecorder()
	s.SetCookie(rec, r, sess)
	c := rec.Result().Cookies()[0]
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie is not HttpOnly+Strict: %+v", c)
	}
	if c.Secure {
		t.Error("cookie marked Secure on a plain HTTP request; the browser would never send it back")
	}
}
