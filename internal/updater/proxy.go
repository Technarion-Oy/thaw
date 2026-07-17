// SPDX-License-Identifier: GPL-3.0-or-later

package updater

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"time"

	"thaw/internal/logger"

	"github.com/mattn/go-ieproxy"
)

// httpTimeout bounds each individual attempt. A GUI app must never hang on a
// dead proxy, so keep this short.
const httpTimeout = 10 * time.Second

// proxyAttempt is one connection strategy tried in order until one yields an
// HTTP response (any status — a 4xx/5xx still means we reached GitHub).
type proxyAttempt struct {
	name  string
	proxy func(*http.Request) (*url.URL, error)
}

// doWithProxyFallback issues req, trying the detected OS/system proxy and a
// direct connection so a stale or misconfigured proxy can't hard-fail the check.
//
// Requirement (issue #568): a GUI app launched from Finder/Dock/Start menu does
// not inherit shell environment variables, so HTTP_PROXY/HTTPS_PROXY alone miss
// most corporate desktop users. go-ieproxy reads the actual OS proxy settings
// (WinHTTP/IE registry + PAC on Windows, CFNetworkCopySystemProxySettings on
// macOS) and falls back to environment variables on Linux, with explicit env
// vars taking precedence.
//
// Attempt order:
//   - When a proxy is resolved for this URL (system or env): [proxy, direct].
//   - When none is resolved: [direct], plus an explicit env-var proxy attempt if
//     HTTP(S)_PROXY is set but system detection somehow missed it.
//
// The path that succeeds is logged so corporate-network issues are diagnosable.
func doWithProxyFallback(ctx context.Context, req *http.Request) (*http.Response, error) {
	systemProxy := ieproxy.GetProxyFunc()

	// Does the system/env proxy resolve an actual proxy for this request?
	resolved, _ := systemProxy(req)

	var attempts []proxyAttempt
	if resolved != nil {
		attempts = []proxyAttempt{
			{name: "system proxy", proxy: systemProxy},
			{name: "direct", proxy: nil},
		}
	} else {
		attempts = []proxyAttempt{{name: "direct", proxy: nil}}
		// Belt-and-suspenders: if env vars are set but system detection returned
		// nothing, try the env-var proxy explicitly as a fallback.
		if envProxySet() {
			attempts = append(attempts, proxyAttempt{name: "env proxy", proxy: http.ProxyFromEnvironment})
		}
	}

	var lastErr error
	for _, a := range attempts {
		client := &http.Client{
			Timeout:   httpTimeout,
			Transport: &http.Transport{Proxy: a.proxy},
		}
		// Clone the request per attempt so a partially-consumed request from a
		// failed attempt can't corrupt the next one.
		resp, err := client.Do(req.Clone(ctx))
		if err == nil {
			logger.L.Info("update check connected", "path", a.name)
			return resp, nil
		}
		logger.L.Warn("update check attempt failed", "path", a.name, "err", err)
		lastErr = err
	}
	return nil, lastErr
}

// envProxySet reports whether any of the standard proxy environment variables
// are set (upper or lower case, as Go's ProxyFromEnvironment honors both).
func envProxySet() bool {
	for _, k := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}
