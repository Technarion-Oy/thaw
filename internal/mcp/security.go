// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package mcp

import (
	"net"
	"net/http"
	"net/url"
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
