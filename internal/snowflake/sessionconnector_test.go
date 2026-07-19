// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sf "github.com/snowflakedb/gosnowflake/v2"

	"thaw/internal/logger"
)

// recordingConnector is a fake driver.Connector that reports the peak number of
// Connect calls in flight simultaneously, so a test can assert whether the
// sessionConnector serializes the login handshake.
type recordingConnector struct {
	inFlight atomic.Int32
	peak     atomic.Int32
	calls    atomic.Int32
}

func (r *recordingConnector) Connect(context.Context) (driver.Conn, error) {
	r.calls.Add(1)
	n := r.inFlight.Add(1)
	for {
		p := r.peak.Load()
		if n <= p || r.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(2 * time.Millisecond) // widen the overlap window
	r.inFlight.Add(-1)
	return nil, nil // Connect returning (nil, nil) is fine: sessionConnector applies USE only when role/wh/db/schema are set
}

func (r *recordingConnector) Driver() driver.Driver { return nil }

func peakConcurrency(t *testing.T, serialLogin bool) int32 {
	t.Helper()
	rec := &recordingConnector{}
	conn := &sessionConnector{base: rec}
	if serialLogin {
		conn.loginGate = make(chan struct{}, 1)
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := conn.Connect(context.Background()); err != nil {
				t.Errorf("Connect: %v", err)
			}
		}()
	}
	wg.Wait()
	return rec.peak.Load()
}

// TestSessionConnector_SerializesMFALogin verifies the issue #804 fix: with
// serialLogin set (MFA auth), concurrent logins never overlap; without it, the
// pool logs in at full concurrency.
func TestSessionConnector_SerializesMFALogin(t *testing.T) {
	if peak := peakConcurrency(t, true); peak != 1 {
		t.Errorf("serialLogin: expected peak login concurrency 1, got %d", peak)
	}
	// The non-serial direction relies on the 2 ms Connect overlap across 16
	// goroutines actually overlapping; on a fully starved single-core runner it
	// could in theory schedule to 1. Practically reliable — noted in case of flakes.
	if peak := peakConcurrency(t, false); peak <= 1 {
		t.Errorf("non-serial: expected concurrent logins (peak > 1), got %d", peak)
	}
}

// TestSessionConnector_LoginGateHonorsContext verifies the issue #804 finding-2
// fix: a Connect queued behind a held login gate is released by ctx
// cancellation rather than blocking until the in-flight handshake finishes.
func TestSessionConnector_LoginGateHonorsContext(t *testing.T) {
	conn := &sessionConnector{base: &recordingConnector{}, loginGate: make(chan struct{}, 1)}
	conn.loginGate <- struct{}{} // hold the gate so the next acquire must wait

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	done := make(chan error, 1)
	go func() { _, err := conn.connectBase(ctx); done <- err }()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("connectBase did not observe context cancellation while waiting on the login gate")
	}
}

// TestSetPoolLimits_ClampsMFA verifies the issue #804 fix: a Client whose
// connector serializes MFA logins clamps a wide pool request (DDL export asks
// for 32/32) down to MFAMaxOpenConns, while a non-MFA client honors the request.
func TestSetPoolLimits_ClampsMFA(t *testing.T) {
	mfa := &Client{db: sql.OpenDB(&recordingConnector{}), connector: &sessionConnector{loginGate: make(chan struct{}, 1)}}
	defer mfa.db.Close() //nolint:errcheck
	mfa.SetPoolLimits(32, 32)
	if got := mfa.db.Stats().MaxOpenConnections; got != MFAMaxOpenConns {
		t.Errorf("MFA client: MaxOpenConnections = %d, want %d", got, MFAMaxOpenConns)
	}

	plain := &Client{db: sql.OpenDB(&recordingConnector{}), connector: &sessionConnector{}}
	defer plain.db.Close() //nolint:errcheck
	plain.SetPoolLimits(32, 32)
	if got := plain.db.Stats().MaxOpenConnections; got != 32 {
		t.Errorf("non-MFA client: MaxOpenConnections = %d, want 32", got)
	}
}

// TestShouldSerializeLogins covers the issue #804 review fix: the login-gate
// protection (serialization + pool clamp) applies to username_password_mfa and
// to plain password auth when a one-time TOTP passcode is supplied — but not to
// other authenticators or password auth without a passcode.
func TestShouldSerializeLogins(t *testing.T) {
	tests := []struct {
		name        string
		auth        sf.AuthType
		hasPasscode bool
		want        bool
	}{
		{"mfa", sf.AuthTypeUsernamePasswordMFA, false, true},
		{"mfa with passcode", sf.AuthTypeUsernamePasswordMFA, true, true},
		{"password with TOTP passcode", sf.AuthTypeSnowflake, true, true},
		{"password without passcode", sf.AuthTypeSnowflake, false, false},
		{"jwt", sf.AuthTypeJwt, false, false},
		{"external browser", sf.AuthTypeExternalBrowser, false, false},
		{"oauth with passcode ignored", sf.AuthTypeOAuth, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSerializeLogins(tt.auth, tt.hasPasscode); got != tt.want {
				t.Errorf("shouldSerializeLogins(%v, %v) = %v, want %v", tt.auth, tt.hasPasscode, got, tt.want)
			}
		})
	}
}

// TestSerializedLoginLogKeyRegistered verifies the package init registers the
// per-login log-scoping key with gosnowflake without clobbering the driver's own
// LOG_SESSION_ID / LOG_USER keys — the mechanism driverNoiseFilter relies on to
// scope auth-noise suppression per connection.
func TestSerializedLoginLogKeyRegistered(t *testing.T) {
	keys := sf.GetLogKeys()
	if !slices.Contains(keys, sf.ContextKey(logger.SerializedLoginLogKey)) {
		t.Errorf("serialized-login log key not registered; GetLogKeys()=%v", keys)
	}
	// The driver's own keys must be preserved (append, not replace).
	if !slices.Contains(keys, sf.SFSessionIDKey) {
		t.Errorf("driver LOG_SESSION_ID key was clobbered; GetLogKeys()=%v", keys)
	}
}

// TestConnectBaseRelogin covers the single-connection + re-prompt flow (issue
// #804): the first login uses the base connector (the credential captured at
// connect time); a re-login re-prompts for a fresh code (typed TOTP), errors
// with ErrMFAReauthRequired when no prompt is available, or re-triggers a push
// (no passcode) via the base connector.
func TestConnectBaseRelogin(t *testing.T) {
	t.Run("typed TOTP re-prompts on re-login", func(t *testing.T) {
		rec := &recordingConnector{}
		prompts := 0
		sc := &sessionConnector{
			base:          rec,
			loginGate:     make(chan struct{}, 1),
			needsPasscode: true,
			promptPasscode: func(context.Context) (string, error) {
				prompts++
				return "", errors.New("prompt refused") // avoid a real connector rebuild
			},
		}
		if _, err := sc.connectBase(context.Background()); err != nil {
			t.Fatalf("first login: %v", err)
		}
		if rec.calls.Load() != 1 || prompts != 0 {
			t.Fatalf("first login should use base once, no prompt (calls=%d prompts=%d)", rec.calls.Load(), prompts)
		}
		_, err := sc.connectBase(context.Background())
		if err == nil || !strings.Contains(err.Error(), "prompt refused") {
			t.Fatalf("re-login should surface the prompt error, got %v", err)
		}
		if rec.calls.Load() != 1 || prompts != 1 {
			t.Fatalf("re-login should prompt, not reuse base (calls=%d prompts=%d)", rec.calls.Load(), prompts)
		}
	})

	t.Run("typed TOTP without prompt returns ErrMFAReauthRequired", func(t *testing.T) {
		rec := &recordingConnector{}
		sc := &sessionConnector{base: rec, loginGate: make(chan struct{}, 1), needsPasscode: true}
		if _, err := sc.connectBase(context.Background()); err != nil {
			t.Fatalf("first login: %v", err)
		}
		_, err := sc.connectBase(context.Background())
		if !errors.Is(err, ErrMFAReauthRequired) {
			t.Fatalf("expected ErrMFAReauthRequired, got %v", err)
		}
		if rec.calls.Load() != 1 {
			t.Fatalf("base should not be reused with the spent passcode (calls=%d)", rec.calls.Load())
		}
	})

	t.Run("push MFA re-logs in via base", func(t *testing.T) {
		rec := &recordingConnector{}
		sc := &sessionConnector{base: rec, loginGate: make(chan struct{}, 1)} // needsPasscode=false
		if _, err := sc.connectBase(context.Background()); err != nil {
			t.Fatalf("first login: %v", err)
		}
		if _, err := sc.connectBase(context.Background()); err != nil {
			t.Fatalf("re-login: %v", err)
		}
		if rec.calls.Load() != 2 {
			t.Fatalf("push re-login should call base again (calls=%d)", rec.calls.Load())
		}
	})
}

// TestCloneConfigWithPasscode verifies the rebuilt config gets the fresh code and
// a private Params map (so re-login can't mutate the base connector's map).
func TestCloneConfigWithPasscode(t *testing.T) {
	v := "true"
	base := sf.Config{Passcode: "old", Params: map[string]*string{"client_session_keep_alive": &v}}
	got := cloneConfigWithPasscode(base, "new")
	if got.Passcode != "new" {
		t.Errorf("Passcode = %q, want new", got.Passcode)
	}
	if base.Passcode != "old" {
		t.Errorf("base mutated: Passcode = %q, want old", base.Passcode)
	}
	got.Params["extra"] = &v
	if _, ok := base.Params["extra"]; ok {
		t.Error("clone shares Params map with base")
	}
}

// TestUsesSingleUseMFACredential covers the unified classification (issue #804
// review): it resolves the authenticator exactly as NewClient does, so an
// unrecognized string folds to the "snowflake" default — the edge where the old
// app-side copy diverged from shouldSerializeLogins.
func TestUsesSingleUseMFACredential(t *testing.T) {
	tests := []struct {
		name string
		auth string
		pass string
		want bool
	}{
		{"mfa push", "username_password_mfa", "", true},
		{"mfa push mixed case", "Username_Password_MFA", "", true},
		{"password with TOTP", "snowflake", "123456", true},
		{"empty auth with TOTP", "", "123456", true},
		{"unrecognized auth with TOTP folds to snowflake", "password", "123456", true},
		{"password without TOTP", "snowflake", "", false},
		{"key-pair", "snowflake_jwt", "", false},
		{"external browser", "externalbrowser", "", false},
		{"oauth with stray passcode", "oauth", "123456", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UsesSingleUseMFACredential(ConnectParams{Authenticator: tt.auth, Passcode: tt.pass})
			if got != tt.want {
				t.Errorf("UsesSingleUseMFACredential(auth=%q pass=%q) = %v, want %v", tt.auth, tt.pass, got, tt.want)
			}
		})
	}
}

// TestIsMFAAuthenticator locks the dedicated-MFA-authenticator predicate used to
// gate the ALLOW_CLIENT_MFA_CACHING hint.
func TestIsMFAAuthenticator(t *testing.T) {
	for _, tt := range []struct {
		auth string
		want bool
	}{
		{"username_password_mfa", true},
		{"USERNAME_PASSWORD_MFA", true},
		{"snowflake", false},
		{"", false},
		{"snowflake_jwt", false},
		{"password", false},
	} {
		if got := IsMFAAuthenticator(tt.auth); got != tt.want {
			t.Errorf("IsMFAAuthenticator(%q) = %v, want %v", tt.auth, got, tt.want)
		}
	}
}
