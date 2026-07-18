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

func configJSONPath(t *testing.T) string {
	t.Helper()
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	return filepath.Join(dir, "thaw", "config.json")
}

func rawConfigJSON(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(configJSONPath(t))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	return string(data)
}

// TestSaveAIConfig_FailsWhenStoreWriteFails guards the failure mode: if the
// secure-store write fails, the save must fail loudly (surfaced error) instead
// of silently reporting success, and must not leak the key into config.json.
func TestSaveAIConfig_FailsWhenStoreWriteFails(t *testing.T) {
	isolateConfig(t)
	restore := secrets.SetDefaultForTesting(&failingSetStore{m: map[string]string{}})
	defer restore()

	a := &App{}
	err := a.SaveAIConfig(config.AIConfig{Provider: "openai", APIKey: "sk-new", Enabled: true})
	if err == nil {
		t.Fatalf("SaveAIConfig returned nil despite a failed store write")
	}
	// The key must not have leaked to config.json (which would be a plaintext
	// secret on disk — exactly what this feature exists to prevent).
	if _, statErr := os.Stat(configJSONPath(t)); statErr == nil {
		if strings.Contains(rawConfigJSON(t), "sk-new") {
			t.Fatalf("API key leaked into config.json after a failed store write")
		}
	}
}

// TestSaveAIConfig_DoesNotDropNewKeyWhenUpdateFailsOverOldValue is the exact
// scenario from review: the store already holds an old key, the user changes it,
// and the store write for the new value fails. The save must fail (surfaced) and
// must NOT silently scrub the new value while leaving the stale old one in the
// store reported as success.
func TestSaveAIConfig_DoesNotDropNewKeyWhenUpdateFailsOverOldValue(t *testing.T) {
	isolateConfig(t)
	restore := secrets.SetDefaultForTesting(&failingSetStore{m: map[string]string{
		secrets.KeyAIAPIKey: "sk-old",
	}})
	defer restore()

	a := &App{}
	err := a.SaveAIConfig(config.AIConfig{Provider: "openai", APIKey: "sk-new", Enabled: true})
	if err == nil {
		t.Fatalf("SaveAIConfig returned nil despite a failed store write over an existing value")
	}
	// The store is left in its prior consistent state (old value intact, not
	// corrupted), and the new plaintext never lands on disk.
	if got, _ := secrets.Get(secrets.KeyAIAPIKey); got != "sk-old" {
		t.Fatalf("store value changed on a failed write: %q", got)
	}
	if _, statErr := os.Stat(configJSONPath(t)); statErr == nil {
		if strings.Contains(rawConfigJSON(t), "sk-new") {
			t.Fatalf("new API key leaked into config.json after a failed store write")
		}
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
