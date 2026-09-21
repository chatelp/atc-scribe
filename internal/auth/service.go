package auth

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
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
	mu      sync.RWMutex
	enabled bool
	users   map[string]string // name -> password hash
	file    *UserFile
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

	// Accounts created by the first-run page. config.toml is never rewritten by
	// the program -- it carries the measured tables this project reasons from,
	// in comments -- so a page that creates an account writes here instead, and
	// the two sources are merged.
	UserFile *UserFile
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
	// The file comes second so an account created from the page can replace one
	// of the same name in the configuration -- the page is the thing someone
	// just used, and surprising them is worse than surprising the file.
	//
	// And an account in the file turns authentication on, whatever the
	// configuration says. Creating one through the setup page *is* the act of
	// turning it on; without this, a restart left the account in place, the
	// setup page satisfied, and the server open -- configured-looking and
	// unprotected, which is worse than either.
	//
	// Accounts written by hand into config.toml keep obeying auth.enabled: a
	// deliberate `enabled = false` there is someone's decision, not an oversight.
	enabled := cfg.Enabled
	if cfg.UserFile != nil {
		for _, u := range cfg.UserFile.Users {
			name := strings.TrimSpace(u.Name)
			if name == "" || u.PasswordHash == "" {
				continue
			}
			users[name] = u.PasswordHash
			enabled = true
		}
	}
	if cfg.Enabled && len(users) == 0 {
		return nil, fmt.Errorf("auth.enabled is true but no account exists; create one from the setup page, or run: co-atc -add-user <name>")
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
		enabled: enabled,
		users:   users,
		file:    cfg.UserFile,
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

	s.mu.RLock()
	hash, known := s.users[strings.TrimSpace(name)]
	s.mu.RUnlock()
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

// NeedsSetup reports that no account exists anywhere -- neither in the
// configuration nor in the accounts file. It is the question the first-run page
// asks, and it stays true until someone answers it.
func (s *Service) NeedsSetup() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users) == 0
}

// AddUser hashes a password, writes the account to the accounts file, and turns
// authentication on. The password is never stored, logged or returned.
//
// It refuses once an account exists: this is the first-run path, and a page that
// creates accounts without being signed in must stop being able to the moment
// there is someone to sign in as.
func (s *Service) AddUser(name, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.users) > 0 {
		return fmt.Errorf("an account already exists; sign in, or use co-atc -add-user")
	}
	if s.file == nil {
		return fmt.Errorf("no accounts file configured")
	}
	if len(strings.TrimSpace(name)) == 0 {
		return fmt.Errorf("choose a user name")
	}
	if len([]rune(password)) < 10 {
		// Ten because this may end up reachable from outside a LAN, and the
		// rate limiter buys time rather than safety.
		return fmt.Errorf("the password must be at least 10 characters")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.file.Add(strings.TrimSpace(name), hash); err != nil {
		return err
	}
	s.users[strings.TrimSpace(name)] = hash
	s.enabled = true
	return nil
}

// AccountsFile is where accounts created from the page are kept.
func (s *Service) AccountsFile() string {
	if s.file == nil {
		return ""
	}
	return s.file.Path()
}
