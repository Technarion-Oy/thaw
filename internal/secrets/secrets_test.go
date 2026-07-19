// SPDX-License-Identifier: GPL-3.0-or-later

package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// newTempFileStore returns a file store rooted in a temp dir.
func newTempFileStore(t *testing.T) *fileStore {
	t.Helper()
	return &fileStore{path: filepath.Join(t.TempDir(), "secrets.json")}
}

func TestFileStoreSetGetDelete(t *testing.T) {
	s := newTempFileStore(t)

	if _, err := s.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) = %v, want ErrNotFound", err)
	}

	if err := s.Set("ai/api-key", "sk-secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("ai/api-key")
	if err != nil || got != "sk-secret" {
		t.Fatalf("Get = %q, %v; want sk-secret", got, err)
	}

	// Overwrite.
	if err := s.Set("ai/api-key", "sk-new"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	if got, _ := s.Get("ai/api-key"); got != "sk-new" {
		t.Fatalf("Get after overwrite = %q, want sk-new", got)
	}

	// Delete, then deleting again is not an error.
	if err := s.Delete("ai/api-key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete("ai/api-key"); err != nil {
		t.Fatalf("Delete missing should be nil, got %v", err)
	}
	if _, err := s.Get("ai/api-key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete = %v, want ErrNotFound", err)
	}
}

func TestFileStorePermissions(t *testing.T) {
	s := newTempFileStore(t)
	if err := s.Set("k", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	info, err := os.Stat(s.path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("secrets file perm = %o, want 600", perm)
	}
}

func TestFileStoreKeys(t *testing.T) {
	s := newTempFileStore(t)
	for _, k := range []string{"pip/credential/https://a", "pip/credential/https://b", "pip/proxy-password"} {
		if err := s.Set(k, "x"); err != nil {
			t.Fatalf("Set %s: %v", k, err)
		}
	}
	keys, err := s.Keys()
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("Keys = %v, want 3", keys)
	}
	// Keys are returned sorted.
	if keys[0] != "pip/credential/https://a" {
		t.Fatalf("Keys not sorted: %v", keys)
	}
}

func TestFileStoreInfo(t *testing.T) {
	s := newTempFileStore(t)
	info := s.Info()
	if info.Secure {
		t.Fatalf("file store Info.Secure = true, want false")
	}
	if info.Method != MethodFile {
		t.Fatalf("Info.Method = %q, want %q", info.Method, MethodFile)
	}
	if info.Detail != s.path {
		t.Fatalf("Info.Detail = %q, want %q", info.Detail, s.path)
	}
}

func TestKeyHelpers(t *testing.T) {
	k := PipCredentialKey("https://pypi.example.com/simple")
	if !IsPipCredentialKey(k) {
		t.Fatalf("IsPipCredentialKey(%q) = false", k)
	}
	if IsPipCredentialKey(KeyPipProxyPassword) {
		t.Fatalf("proxy password wrongly classified as pip credential")
	}
	if MCPTokenKey("session-1") != "mcp/token/session-1" {
		t.Fatalf("MCPTokenKey mismatch: %q", MCPTokenKey("session-1"))
	}
}

func TestDefaultStoreOverride(t *testing.T) {
	restore := SetDefaultForTesting(newTempFileStore(t))
	defer restore()

	if err := Set("k", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got, err := Get("k"); err != nil || got != "v" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if GetInfo().Method != MethodFile {
		t.Fatalf("GetInfo().Method = %q, want file", GetInfo().Method)
	}
}
