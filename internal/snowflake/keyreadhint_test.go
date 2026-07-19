// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestKeyReadHint(t *testing.T) {
	permErr := &fs.PathError{Op: "open", Path: "/Users/x/Documents/rsa_key.p8", Err: os.ErrPermission}
	notFound := &fs.PathError{Op: "open", Path: "/Users/x/Documents/rsa_key.p8", Err: os.ErrNotExist}

	t.Run("darwin permission error gets an actionable hint", func(t *testing.T) {
		got := keyReadHint("darwin", "/Users/x/Documents/rsa_key.p8", permErr)
		if got == nil {
			t.Fatal("expected an error, got nil")
		}
		msg := got.Error()
		if !strings.Contains(msg, "macOS blocked access") {
			t.Errorf("hint missing from message: %q", msg)
		}
		if !strings.Contains(msg, "/Users/x/Documents/rsa_key.p8") {
			t.Errorf("path missing from message: %q", msg)
		}
		// The original error must remain wrapped so callers can still match it.
		if !errors.Is(got, os.ErrPermission) {
			t.Errorf("wrapped error no longer matches os.ErrPermission: %q", msg)
		}
	})

	t.Run("non-darwin permission error is returned unchanged", func(t *testing.T) {
		got := keyReadHint("linux", "/home/x/Documents/rsa_key.p8", permErr)
		if got != error(permErr) {
			t.Errorf("expected the original error unchanged, got %v", got)
		}
	})

	t.Run("darwin non-permission error is returned unchanged", func(t *testing.T) {
		got := keyReadHint("darwin", "/Users/x/Documents/rsa_key.p8", notFound)
		if got != error(notFound) {
			t.Errorf("expected the original error unchanged, got %v", got)
		}
	})
}
