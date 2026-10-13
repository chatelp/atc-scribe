package auth

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Origins says which web pages may call this server from a browser with the
// signed-in user's cookie: CORS on the API, and the WebSocket handshake.
//
// Until 13/10 the answer was "any", echoed back with credentials allowed, which
// lets any page the user happens to open read the receiver's data in their name.
// It is now the server itself -- whatever name the browser reached it by, on any
// of its own ports, in http or https -- and the pages listed in
// server.allowed_origins. The page reaches the audio on another of the server's
// ports (www/audio-client.js), which is a different origin to a browser: that
// is why the ports are listed rather than only the exact origin of the request.
type Origins struct {
	ports map[string]bool // the server's own ports, as they appear in an origin
	names map[string]bool // names that always mean this machine
	extra map[string]bool // server.allowed_origins, normalised
}

// NewOrigins builds the policy from the bind address, the ports the server
// listens on, and the extra origins the configuration allows. An extra origin
// that does not parse is an error: a page someone meant to allow and that is
// silently refused is a support question nobody can answer.
func NewOrigins(host string, ports []int, allowed []string) (*Origins, error) {
	o := &Origins{ports: map[string]bool{}, names: map[string]bool{}, extra: map[string]bool{}}
	for _, p := range ports {
		if p > 0 {
			o.ports[strconv.Itoa(p)] = true
		}
	}
	for _, n := range []string{"localhost", "127.0.0.1", "::1"} {
		o.names[n] = true
	}
	if h := strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]")); h != "" && h != "0.0.0.0" && h != "::" {
		o.names[h] = true
	}
	for _, a := range allowed {
		n, err := NormalizeOrigin(a)
		if err != nil {
			return nil, err
		}
		o.extra[n] = true
	}
	return o, nil
}

// NormalizeOrigin checks that s is an origin as a browser sends it --
// scheme://host[:port], http or https, nothing after -- and returns it in the
// form compared against: lower case, default port left out, no trailing slash.
// "*" is refused: with credentials allowed, it is exactly the hole this closes.
func NormalizeOrigin(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "*" {
		return "", fmt.Errorf("allowed origin %q: \"*\" would let any page use the signed-in session; list the pages instead", s)
	}
	u, err := url.Parse(strings.TrimSuffix(s, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("allowed origin %q: want scheme://host[:port], http or https, e.g. http://100.101.102.103:8000", s)
	}
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if port == defaultPort(u.Scheme) {
		port = ""
	}
	return u.Scheme + "://" + joinHostPort(host, port), nil
}

// Allowed says whether a browser request may be answered for its page: false
// when it carries no Origin, which is what a same-origin GET and every client
// that is not a browser send, and which needs no CORS header at all.
func (o *Origins) Allowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if o == nil || origin == "" {
		return false
	}
	return o.allows(origin, r.Host)
}

// AllowedOrAbsent is the WebSocket handshake's rule: a client that sends no
// Origin is not a browser -- a browser always sends one there -- and nothing
// can be borrowed from it; one that does must be an allowed page.
func (o *Origins) AllowedOrAbsent(r *http.Request) bool {
	if r.Header.Get("Origin") == "" {
		return true
	}
	return o.Allowed(r)
}

func (o *Origins) allows(origin, requestHost string) bool {
	norm, err := NormalizeOrigin(origin)
	if err != nil {
		return false // "null", a file:// page, garbage
	}
	if o.extra[norm] {
		return true
	}
	u, _ := url.Parse(norm)
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = defaultPort(u.Scheme)
	}

	// The exact origin of the request, whatever port it came to: what a
	// reverse proxy that keeps the Host header looks like.
	reqHost, reqPort := splitHost(requestHost)
	if host == reqHost && (port == reqPort || (reqPort == "" && port == defaultPort(u.Scheme))) {
		return true
	}
	// The server under the name the browser used or one of its own names, on
	// one of its ports: the page on 8000 reaching the audio on 8001.
	return o.ports[port] && (host == reqHost || o.names[host])
}

// splitHost splits a Host header into a lower-case name and its port, if any.
func splitHost(h string) (string, string) {
	h = strings.ToLower(strings.TrimSpace(h))
	if host, port, err := net.SplitHostPort(h); err == nil {
		return strings.Trim(host, "[]"), port
	}
	return strings.Trim(h, "[]"), ""
}

func joinHostPort(host, port string) string {
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port == "" {
		return host
	}
	return host + ":" + port
}

func defaultPort(scheme string) string {
	if scheme == "https" {
		return "443"
	}
	return "80"
}
