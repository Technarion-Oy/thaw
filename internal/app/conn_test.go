// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"sync"
	"testing"

	"thaw/internal/snowflake"
)

// TestConnStateConcurrentAccess exercises the connMu guard around the shared
// connection state (client / connectParams). Connect/Disconnect flip these
// pointers while many IPC methods read them on concurrent Wails goroutines, so
// this hammers both sides at once. It is a no-op unless run under -race, whose
// job is to fail if a reader ever bypasses currentClient()/currentConnectParams()
// or a writer forgets to take the write lock (the #351 regression).
func TestConnStateConcurrentAccess(t *testing.T) {
	a := &App{}
	const goroutines = 8
	const iterations = 2000

	var wg sync.WaitGroup

	// Writers: mirror Connect/Disconnect flipping the connection triple under the
	// write lock. Uses non-nil sentinels so a torn read would be observable.
	connected := &snowflake.Client{}
	params := &snowflake.ConnectParams{}
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iterations {
				a.connMu.Lock()
				if i%2 == 0 {
					a.client = connected
					a.connectParams = params
				} else {
					a.client = nil
					a.connectParams = nil
				}
				a.connMu.Unlock()
			}
		}()
	}

	// Readers: route through the accessors, exactly as the IPC methods now do.
	// (The accessors only copy the pointer, so the zero-value sentinel client is
	// never dereferenced.)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				_ = a.currentClient()
				_ = a.currentConnectParams()
			}
		}()
	}

	wg.Wait()
}
