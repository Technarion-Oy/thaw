// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"thaw/internal/config"
	"thaw/internal/secrets"
)

// failingSetStore is an in-memory secrets.Store whose Set always fails,
// simulating a locked keychain / unavailable Secret Service.
type failingSetStore struct{ m map[string]string }

func (s *failingSetStore) Get(k string) (string, error) {
	if v, ok := s.m[k]; ok {
		return v, nil
	}
	return "", secrets.ErrNotFound
}
func (s *failingSetStore) Set(string, string) error { return errors.New("secure store unavailable") }
func (s *failingSetStore) Delete(k string) error    { delete(s.m, k); return nil }
func (s *failingSetStore) Keys() ([]string, error) {
	keys := make([]string, 0, len(s.m))
	for k := range s.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}
func (s *failingSetStore) Info() secrets.Info {
	return secrets.Info{Method: secrets.MethodKeychain, Secure: true, Label: "test"}
}

func rawConfigJSON(t *testing.T) string {
	t.Helper()
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "thaw", "config.json"))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	return string(data)
}

// TestSaveAIConfig_KeepsKeyWhenStoreWriteFails guards the failure mode flagged
// in review: if the secure-store write fails, the API key must not vanish. It
// stays on disk (0600) rather than being blanked before it is safely stored.
func TestSaveAIConfig_KeepsKeyWhenStoreWriteFails(t *testing.T) {
	isolateConfig(t)
	restore := secrets.SetDefaultForTesting(&failingSetStore{m: map[string]string{}})
	defer restore()

	a := &App{}
	if err := a.SaveAIConfig(config.AIConfig{Provider: "openai", APIKey: "sk-keepme", Enabled: true}); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	// The store write failed, so buildDiskConfig must have kept the key on disk
	// (never lost) instead of scrubbing an unstored secret.
	if !strings.Contains(rawConfigJSON(t), "sk-keepme") {
		t.Fatalf("API key lost: not in the store (write failed) and scrubbed from config.json")
	}
}

// TestSaveAIConfig_ScrubsKeyWhenStoreWriteSucceeds is the happy path: the key
// goes to the store and is scrubbed from config.json.
func TestSaveAIConfig_ScrubsKeyWhenStoreWriteSucceeds(t *testing.T) {
	isolateConfig(t)
	t.Setenv("THAW_SECRETS_BACKEND", "file")
	t.Setenv("THAW_SECRETS_DIR", t.TempDir())
	restore := secrets.SetDefaultForTesting(nil) // rebuild from env on next use
	defer restore()

	a := &App{}
	if err := a.SaveAIConfig(config.AIConfig{Provider: "openai", APIKey: "sk-secret", Enabled: true}); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	if strings.Contains(rawConfigJSON(t), "sk-secret") {
		t.Fatalf("API key leaked into config.json")
	}
	if got := a.GetAIConfig(); got.APIKey != "sk-secret" {
		t.Fatalf("GetAIConfig APIKey = %q, want sk-secret (hydrated from store)", got.APIKey)
	}
	// Sanity: config.json is still valid JSON with the non-secret provider field.
	var m map[string]any
	if err := json.Unmarshal([]byte(rawConfigJSON(t)), &m); err != nil {
		t.Fatalf("config.json invalid: %v", err)
	}
}
