package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yegors/co-atc/internal/auth"
	"github.com/yegors/co-atc/pkg/logger"
)

type loginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// GetAuthStatus tells the page whether anyone needs to sign in, and who is signed
// in. It is deliberately reachable without a session: the login form has to know
// whether to show itself.
func (h *Handler) GetAuthStatus(w http.ResponseWriter, r *http.Request) {
	out := map[string]interface{}{"enabled": h.auth.Enabled(), "authenticated": !h.auth.Enabled()}
	if h.auth.Enabled() {
		if sess, ok := h.auth.SessionFrom(r); ok {
			out["authenticated"] = true
			out["user"] = sess.User
			out["expires_at"] = sess.ExpiresAt
		}
	}
	WriteJSON(w, http.StatusOK, out)
}

// PostLogin exchanges a password for a session cookie.
func (h *Handler) PostLogin(w http.ResponseWriter, r *http.Request) {
	if !h.auth.Enabled() {
		WriteJSON(w, http.StatusOK, map[string]interface{}{"authenticated": true})
		return
	}
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	sess, err := h.auth.SignIn(r, req.Name, req.Password)
	if err != nil {
		// The client is told only that it failed. Which of the name or the password
		// was wrong, and whether the account exists at all, stay unsaid: a login
		// form that distinguishes them is a way to enumerate accounts.
		h.logger.Warn("Failed sign-in",
			logger.String("name", req.Name),
			logger.String("from", h.auth.Proxies().ClientIP(r)))
		time.Sleep(250 * time.Millisecond)
		WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	h.auth.SetCookie(w, r, sess)
	h.logger.Info("Signed in",
		logger.String("user", sess.User),
		logger.String("from", h.auth.Proxies().ClientIP(r)))
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true, "user": sess.User, "expires_at": sess.ExpiresAt,
	})
}

// PostLogout ends the session that made the request.
func (h *Handler) PostLogout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := h.auth.SessionFrom(r); ok {
		h.auth.Sessions().Revoke(sess.Token)
		h.logger.Info("Signed out", logger.String("user", sess.User))
	}
	h.auth.ClearCookie(w, r)
	WriteJSON(w, http.StatusOK, map[string]interface{}{"authenticated": false})
}

// Auth exposes the service so the router can hang its middleware on it. The
// service is never nil: a disabled one is built when no account is configured, and
// it lets every request through.
func (h *Handler) Auth() *auth.Service { return h.auth }
