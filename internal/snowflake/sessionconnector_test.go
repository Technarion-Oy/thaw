// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"context"
	"database/sql/driver"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingConnector is a fake driver.Connector that reports the peak number of
// Connect calls in flight simultaneously, so a test can assert whether the
// sessionConnector serializes the login handshake.
type recordingConnector struct {
	inFlight atomic.Int32
	peak     atomic.Int32
}

func (r *recordingConnector) Connect(context.Context) (driver.Conn, error) {
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
