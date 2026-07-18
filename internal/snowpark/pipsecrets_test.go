// SPDX-License-Identifier: GPL-3.0-or-later

package snowpark

import (
	"errors"
	"sort"
	"testing"

	"thaw/internal/config"
	"thaw/internal/secrets"
)

// fileBackedTempStore points the default secrets store at a temp file backend.
func fileBackedTempStore(t *testing.T) func() {
	t.Helper()
	t.Setenv("THAW_SECRETS_BACKEND", "file")
	t.Setenv("THAW_SECRETS_DIR", t.TempDir())
	return secrets.SetDefaultForTesting(nil) // rebuild from env on next use
}

func TestStorePipSecrets_StoresHydratesAndPrunes(t *testing.T) {
	restore := fileBackedTempStore(t)
	defer restore()

	rc := config.PipRegistryConfig{
		Credentials: []config.PipRegistryCredential{
			{Registry: "https://a.example/simple", Username: "u", Password: "pw-a"},
			{Registry: "https://b.example/simple", Username: "u", Password: "pw-b"},
		},
		ProxyPassword: "proxy-pw",
	}
	if err := storePipSecrets(rc); err != nil {
		t.Fatalf("storePipSecrets: %v", err)
	}

	// hydratePipSecrets fills passwords back from the store.
	got := config.PipRegistryConfig{
		Credentials: []config.PipRegistryCredential{
			{Registry: "https://a.example/simple"},
			{Registry: "https://b.example/simple"},
		},
	}
	hydratePipSecrets(&got)
	if got.Credentials[0].Password != "pw-a" || got.Credentials[1].Password != "pw-b" {
		t.Fatalf("hydrate mismatch: %+v", got.Credentials)
	}
	if got.ProxyPassword != "proxy-pw" {
		t.Fatalf("proxy password not hydrated: %q", got.ProxyPassword)
	}

	// Remove registry b: its secret must be pruned; a must survive.
	rc2 := config.PipRegistryConfig{
		Credentials:   []config.PipRegistryCredential{{Registry: "https://a.example/simple", Username: "u", Password: "pw-a"}},
		ProxyPassword: "proxy-pw",
	}
	if err := storePipSecrets(rc2); err != nil {
		t.Fatalf("storePipSecrets (prune): %v", err)
	}
	if _, err := secrets.Get(secrets.PipCredentialKey("https://b.example/simple")); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("orphaned credential for removed registry not pruned: %v", err)
	}
	if v, err := secrets.Get(secrets.PipCredentialKey("https://a.example/simple")); err != nil || v != "pw-a" {
		t.Fatalf("kept credential lost during prune: %q, %v", v, err)
	}
}

func TestStorePipSecrets_ClearingPasswordDeletesSecret(t *testing.T) {
	restore := fileBackedTempStore(t)
	defer restore()

	reg := "https://a.example/simple"
	if err := storePipSecrets(config.PipRegistryConfig{
		Credentials: []config.PipRegistryCredential{{Registry: reg, Password: "pw"}},
	}); err != nil {
		t.Fatalf("store: %v", err)
	}
	// Save the same registry with an empty password → the stored secret is deleted.
	if err := storePipSecrets(config.PipRegistryConfig{
		Credentials: []config.PipRegistryCredential{{Registry: reg, Password: ""}},
	}); err != nil {
		t.Fatalf("store (clear): %v", err)
	}
	if _, err := secrets.Get(secrets.PipCredentialKey(reg)); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("cleared credential not deleted: %v", err)
	}
}

// pipFailSetStore fails every Set but serves Get/Keys/Delete from its map.
type pipFailSetStore struct{ m map[string]string }

func (s *pipFailSetStore) Get(k string) (string, error) {
	if v, ok := s.m[k]; ok {
		return v, nil
	}
	return "", secrets.ErrNotFound
}
func (s *pipFailSetStore) Set(string, string) error { return errors.New("secure store unavailable") }
func (s *pipFailSetStore) Delete(k string) error    { delete(s.m, k); return nil }
func (s *pipFailSetStore) Keys() ([]string, error) {
	keys := make([]string, 0, len(s.m))
	for k := range s.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}
func (s *pipFailSetStore) Info() secrets.Info {
	return secrets.Info{Method: secrets.MethodKeychain, Secure: true}
}

// TestStorePipSecrets_FailsFastAndDoesNotPrune verifies that a store-write
// failure aborts before any pruning, so a failed save never also mutates the
// store by deleting an existing (orphan) secret.
func TestStorePipSecrets_FailsFastAndDoesNotPrune(t *testing.T) {
	ms := &pipFailSetStore{m: map[string]string{
		// A pre-existing secret for a registry that would be pruned on success.
		secrets.PipCredentialKey("https://old.example/simple"): "old-pw",
	}}
	restore := secrets.SetDefaultForTesting(ms)
	defer restore()

	err := storePipSecrets(config.PipRegistryConfig{
		Credentials: []config.PipRegistryCredential{{Registry: "https://new.example/simple", Password: "new-pw"}},
	})
	if err == nil {
		t.Fatalf("storePipSecrets returned nil despite a failed store write")
	}
	// The pre-existing (would-be-orphan) secret must NOT have been pruned, since
	// pruning happens only after all writes succeed.
	if _, ok := ms.m[secrets.PipCredentialKey("https://old.example/simple")]; !ok {
		t.Fatalf("orphan pruned despite a failed write (pruning must not run on failure)")
	}
}

// TestSavePipRegistryConfig_FailsOnStoreError verifies the write-seam contract:
// a store-write failure fails the whole save (surfaced error) rather than
// silently persisting a scrubbed config while the password is lost.
func TestSavePipRegistryConfig_FailsOnStoreError(t *testing.T) {
	restore := secrets.SetDefaultForTesting(&pipFailSetStore{m: map[string]string{}})
	defer restore()

	s := &Service{}
	err := s.SavePipRegistryConfig(config.PipRegistryConfig{
		Credentials: []config.PipRegistryCredential{{Registry: "https://a.example/simple", Password: "pw"}},
	})
	if err == nil {
		t.Fatalf("SavePipRegistryConfig returned nil despite a failed store write")
	}
}
