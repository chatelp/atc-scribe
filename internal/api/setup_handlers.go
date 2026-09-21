package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/yegors/co-atc/pkg/logger"
)

// The first-run page. Upstream ships with no authentication at all and a README
// saying never to expose it; this is the other half of changing that -- a way to
// answer the question rather than a paragraph telling you to edit TOML.
//
// No setup token, deliberately. Bound to 127.0.0.1, reaching this page already
// means being on the machine, which is the same trust boundary as the terminal
// where you would have run -add-user. A token would protect nothing it does not
// already protect. Bound anywhere else, the server refuses to start without an
// account at all -- see cmd/server/main.go -- so this page is never served to a
// network.

type SetupState struct {
	Needed    bool   `json:"needed"`     // no account exists anywhere
	LocalOnly bool   `json:"local_only"` // the server is bound to loopback
	Host      string `json:"host"`
	File      string `json:"file"` // where an account would be written
}

// GetSetupStatus says whether anything needs setting up. Public: the page has to
// be able to ask before anyone can sign in.
func (h *Handler) GetSetupStatus(w http.ResponseWriter, r *http.Request) {
	st := SetupState{
		Needed:    h.auth.NeedsSetup(),
		LocalOnly: isLoopback(h.config.Server.Host),
		Host:      h.config.Server.Host,
		File:      h.auth.AccountsFile(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

// PostSetup answers the first-run question: local only, or reachable with an
// account. It works exactly once, and only while no account exists.
func (h *Handler) PostSetup(w http.ResponseWriter, r *http.Request) {
	if !h.auth.NeedsSetup() {
		http.Error(w, "already set up", http.StatusConflict)
		return
	}

	var body struct {
		Mode     string `json:"mode"` // "local" or "account"
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	switch body.Mode {
	case "local":
		// Nothing to write. Staying without authentication is a legitimate
		// answer on a loopback-bound server, and recording the choice is what
		// makes it a choice rather than an oversight.
		h.logger.Info("Setup: staying local-only, no authentication",
			logger.String("host", h.config.Server.Host))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "local"})

	case "account":
		if err := h.auth.AddUser(body.Username, body.Password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.logger.Info("Setup: account created, authentication is on",
			logger.String("user", body.Username),
			logger.String("file", h.auth.AccountsFile()))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "account", "user": body.Username})

	default:
		http.Error(w, `mode must be "local" or "account"`, http.StatusBadRequest)
	}
}

// isLoopback reports whether a bind address can only be reached from this
// machine. An empty host means every interface, which is the dangerous default
// to get wrong.
func isLoopback(host string) bool {
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return false
}
