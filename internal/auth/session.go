package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// Session is one signed-in browser.
type Session struct {
	Token     string
	User      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Store keeps the sessions in force.
//
// In memory, on purpose. Persisting them would mean a restart no longer ends every
// session, which is the one thing an operator can always do to lock everyone out.
// The cost is that a restart asks for the password again -- on a station that
// restarts when its owner decides to, that is a fair trade and it is documented.
type Store struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
}

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	return &Store{sessions: map[string]*Session{}, ttl: ttl}
}

// Create issues a session for a user. The token is 256 bits of randomness and
// nothing else: it carries no claims, so there is nothing in it to forge and
// revoking it is a map delete rather than a blocklist.
func (s *Store) Create(user string) (*Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	now := time.Now()
	sess := &Session{
		Token:     base64.RawURLEncoding.EncodeToString(raw),
		User:      user,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}
	s.mu.Lock()
	s.sessions[sess.Token] = sess
	s.pruneLocked(now)
	s.mu.Unlock()
	return sess, nil
}

// Lookup returns the session for a token and slides its expiry forward. A session
// in daily use should not expire under its user; one abandoned for a month should.
func (s *Store) Lookup(token string) (*Session, bool) {
	if token == "" {
		return nil, false
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok {
		return nil, false
	}
	if now.After(sess.ExpiresAt) {
		delete(s.sessions, token)
		return nil, false
	}
	sess.ExpiresAt = now.Add(s.ttl)
	return sess, true
}

// Revoke ends one session.
func (s *Store) Revoke(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// RevokeUser ends every session of one user -- what you reach for when a device is
// lost rather than merely closed.
func (s *Store) RevokeUser(user string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for token, sess := range s.sessions {
		if sess.User == user {
			delete(s.sessions, token)
			n++
		}
	}
	return n
}

// Count reports how many sessions are in force.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(time.Now())
	return len(s.sessions)
}

func (s *Store) pruneLocked(now time.Time) {
	for token, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
}
