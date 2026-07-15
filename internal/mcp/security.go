// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// loopbackGuard wraps an HTTP handler and rejects any request whose Host or
// Origin header does not resolve to the loopback interface. The listener
// already binds 127.0.0.1, but a loopback listener is still reachable from the
// user's browser, so a malicious page could target http://localhost:<port>/sse
// and — via DNS rebinding — read Snowflake schema metadata from the active
// connection. Validating Host (reject rebound hostnames) and Origin (reject
// cross-origin browser requests) is the standard mitigation for local
// MCP/HTTP servers.
func loopbackGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden: non-loopback Host header", http.StatusForbidden)
			return
		}
		// Origin is only sent by browsers. A non-loopback Origin means the
		// request originates from a web page, which must never reach the SSE
		// transport (DNS-rebinding defense). Non-browser MCP clients omit it.
		if origin := r.Header.Get("Origin"); origin != "" && !isLoopbackOrigin(origin) {
			http.Error(w, "forbidden: cross-origin request", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newSessionToken returns a cryptographically random, URL-safe per-session
// token. The token is required to open the session-creating SSE GET (see
// tokenGuard) so a co-resident local process cannot read schema metadata over
// the loopback port without it.
func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// RawURLEncoding is URL-safe (no '+', '/', or '=' padding), so the token
	// can be embedded directly in a query string without escaping.
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// tokenGuard enforces the per-session token on session-creating GET requests.
//
// Only the GET is checked. The SSE transport has the SDK generate an
// unguessable sessionid (crypto-random, delivered only over this authenticated
// GET stream) that authorizes the client's follow-up message POSTs, so the
// POSTs are not separately token-checked. Requiring a token on them would in
// fact break the transport: the go-sdk builds the message endpoint via
// req.URL.Parse("?sessionid=…"), a query-only relative reference that replaces
// the entire query string and therefore drops any ?token=… from the original
// GET. A local process that cannot pass the GET token never learns a valid
// sessionid, so it can neither open a session nor post into one.
//
// The token may be presented as an "Authorization: Bearer <token>" header or a
// "token" query parameter, so both header-capable and URL-only MCP clients
// work.
func tokenGuard(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && !validToken(token, r) {
			http.Error(w, "unauthorized: missing or invalid session token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// validToken reports whether the request presents the expected session token,
// via either the Authorization bearer header or the "token" query parameter.
// The comparison is constant-time to avoid leaking the token byte-by-byte.
func validToken(want string, r *http.Request) bool {
	got := bearerToken(r)
	if got == "" {
		got = r.URL.Query().Get("token")
	}
	if want == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// bearerToken extracts the token from an "Authorization: Bearer <token>"
// header, or returns "" if the header is absent or not a bearer credential.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return h[len(prefix):]
	}
	return ""
}

// isLoopbackHost reports whether a Host header (which may include a port)
// refers to the loopback interface: "localhost", or any loopback IP literal.
func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	h := stripPort(host)
	if h == "localhost" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// isLoopbackOrigin reports whether an Origin header value is a loopback origin.
func isLoopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return isLoopbackHost(u.Host)
}

// stripPort removes a trailing :port from a host, tolerating IPv6 brackets and
// bare hostnames without a port.
func stripPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
