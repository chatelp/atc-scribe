package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CookieName is the session cookie. Prefixed so it is obvious in a browser's
// storage inspector which application it belongs to.
const CookieName = "coatc_session"

// User is an account that may sign in.
type User struct {
	Name         string
	PasswordHash string
}

// Service answers "who is asking", and is the only place that says yes.
type Service struct {
	enabled bool
	users   map[string]string // name -> password hash
	store   *Store
	proxies *TrustedProxies
	limiter *Limiter
}

// Config is what the service needs from the configuration file.
type Config struct {
	Enabled        bool
	Users          []User
	SessionTTL     time.Duration
	TrustedProxies []string
	MaxAttempts    int
	AttemptWindow  time.Duration
}

func NewService(cfg Config) (*Service, error) {
	proxies, err := NewTrustedProxies(cfg.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("invalid trusted_proxies: %w", err)
	}
	users := map[string]string{}
	for _, u := range cfg.Users {
		name := strings.TrimSpace(u.Name)
		if name == "" || u.PasswordHash == "" {
			continue
		}
		users[name] = u.PasswordHash
	}
	if cfg.Enabled && len(users) == 0 {
		return nil, fmt.Errorf("auth.enabled is true but no user has a password hash; run: co-atc -add-user <name>")
	}
	max := cfg.MaxAttempts
	if max <= 0 {
		max = 8
	}
	window := cfg.AttemptWindow
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &Service{
		enabled: cfg.Enabled,
		users:   users,
		store:   NewStore(cfg.SessionTTL),
		proxies: proxies,
		limiter: NewLimiter(max, window),
	}, nil
}

func (s *Service) Enabled() bool            { return s != nil && s.enabled }
func (s *Service) Sessions() *Store         { return s.store }
func (s *Service) Proxies() *TrustedProxies { return s.proxies }

// SignIn checks a password and issues a session.
//
// A wrong user name and a wrong password are answered identically, and both pay the
// same Argon2id cost: telling them apart would turn the login form into a way to
// enumerate accounts.
func (s *Service) SignIn(r *http.Request, name, password string) (*Session, error) {
	ip := s.proxies.ClientIP(r)
	if !s.limiter.Allow(ip) {
		return nil, fmt.Errorf("too many attempts")
	}

	hash, known := s.users[strings.TrimSpace(name)]
	if !known {
		// Verify against a throwaway hash so an unknown name costs the same time as
		// a known one.
		VerifyPassword(password, decoyHash)
		return nil, fmt.Errorf("invalid credentials")
	}
	if !VerifyPassword(password, hash) {
		return nil, fmt.Errorf("invalid credentials")
	}

	s.limiter.Succeeded(ip)
	return s.store.Create(strings.TrimSpace(name))
}

// decoyHash is a real Argon2id hash of a value nobody knows, used only to spend the
// same time on an unknown user as on a known one.
const decoyHash = "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$kFYJ0kYOQ0aH7VbTlSJ8YZm1LcAqvJLHhWpFkYhqQ2M"

// SessionFrom returns the session a request carries, if any.
func (s *Service) SessionFrom(r *http.Request) (*Session, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return nil, false
	}
	return s.store.Lookup(c.Value)
}

// SetCookie installs the session cookie.
//
// HttpOnly so no script can read it, SameSite=Strict so no other site can cause it
// to be sent, and Secure whenever the browser is actually on HTTPS -- which behind
// a proxy is known only from a forwarded header, and only from a trusted one.
func (s *Service) SetCookie(w http.ResponseWriter, r *http.Request, sess *Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.proxies.Scheme(r) == "https",
		Expires:  sess.ExpiresAt,
	})
}

// ClearCookie removes it.
func (s *Service) ClearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.proxies.Scheme(r) == "https",
		MaxAge:   -1,
	})
}

// Require rejects a request that carries no valid session. It is a no-op when
// authentication is disabled, which is the default of the published configuration:
// someone cloning the repository must not find themselves locked out of their own
// receiver.
func (s *Service) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := s.SessionFrom(r); !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"authentication required"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
