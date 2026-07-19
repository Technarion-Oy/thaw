// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// mfaPromptTimeout bounds how long a re-login waits for the user to enter a fresh
// one-time code. Generous enough to open an authenticator app, short enough that
// a forgotten prompt doesn't pin the connection indefinitely.
const mfaPromptTimeout = 2 * time.Minute

// promptMFACode asks the frontend for a fresh one-time MFA passcode and blocks
// until the user submits one, cancels, the request times out, or ctx is
// canceled. It is wired into the snowflake client (via WithPasscodePrompt) and
// invoked when a passcode-authenticated session must re-login — its lone pooled
// connection was lost and the original single-use code is spent. Serialized by
// the client's login gate, so at most one prompt is outstanding per account+user.
func (a *App) promptMFACode(ctx context.Context) (string, error) {
	id := strconv.FormatUint(a.mfaPromptSeq.Add(1), 10)
	ch := make(chan string, 1)

	a.mfaPromptsMu.Lock()
	if a.mfaPrompts == nil {
		a.mfaPrompts = make(map[string]chan string)
	}
	a.mfaPrompts[id] = ch
	a.mfaPromptsMu.Unlock()
	defer func() {
		a.mfaPromptsMu.Lock()
		delete(a.mfaPrompts, id)
		a.mfaPromptsMu.Unlock()
	}()

	var user, account string
	if p := a.currentConnectParams(); p != nil {
		user, account = p.User, p.Account
	}
	wailsruntime.EventsEmit(a.ctx, "mfa:prompt-code", map[string]string{
		"requestId": id,
		"user":      user,
		"account":   account,
	})

	timer := time.NewTimer(mfaPromptTimeout)
	defer timer.Stop()
	select {
	case code := <-ch:
		if code == "" {
			return "", fmt.Errorf("MFA code entry canceled")
		}
		return code, nil
	case <-timer.C:
		wailsruntime.EventsEmit(a.ctx, "mfa:prompt-close", id)
		return "", fmt.Errorf("timed out waiting for a new MFA code")
	case <-ctx.Done():
		wailsruntime.EventsEmit(a.ctx, "mfa:prompt-close", id)
		return "", ctx.Err()
	}
}

// SubmitMFACode delivers a one-time MFA passcode the user entered in response to
// an "mfa:prompt-code" event. An empty code signals cancellation. Unknown or
// already-resolved request ids are ignored (e.g. a late submit after timeout).
func (a *App) SubmitMFACode(requestID, code string) {
	a.mfaPromptsMu.Lock()
	ch := a.mfaPrompts[requestID]
	a.mfaPromptsMu.Unlock()
	if ch == nil {
		return
	}
	// Non-blocking: the channel is buffered (cap 1) and each id is delivered once.
	select {
	case ch <- code:
	default:
	}
}
