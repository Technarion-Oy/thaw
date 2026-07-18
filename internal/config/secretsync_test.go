// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"thaw/internal/secrets"
)

// isolate points os.UserConfigDir at a temp dir and swaps in an in-memory-ish
// (temp file) secret store so tests never touch the real config or keychain.
func isolate(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("THAW_SECRETS_BACKEND", "file")
	t.Setenv("THAW_SECRETS_DIR", t.TempDir())
	restore := secrets.SetDefaultForTesting(nil) // force rebuild from env on next use
	t.Cleanup(restore)
}

// rawConfig reads config.json as a generic map to assert what actually hit disk.
func rawConfig(t *testing.T) map[string]any {
	t.Helper()
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "thaw", "config.json"))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestSaveScrubsSecretsFromDisk(t *testing.T) {
	isolate(t)

	err := Update(func(cfg *AppConfig) error {
		cfg.AI.APIKey = "sk-plain"
		cfg.OAuth.GithubClientSecret = "gh-secret"
		cfg.PipRegistry.ProxyPassword = "proxypw"
		cfg.PipRegistry.Credentials = []PipRegistryCredential{
			{Registry: "https://pypi.example.com", Username: "u", Password: "credpw"},
		}
		cfg.MCPCredentials = map[string]MCPSessionCredential{
			"sess": {Port: 9000, Token: "tok"},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	// The raw JSON must contain none of the secret material.
	blob, _ := json.Marshal(rawConfig(t))
	for _, secret := range []string{"sk-plain", "gh-secret", "proxypw", "credpw", `"tok"`} {
		if strings.Contains(string(blob), secret) {
			t.Fatalf("config.json leaked secret %q: %s", secret, blob)
		}
	}

	// Non-secret companion data must survive (the MCP port).
	m := rawConfig(t)
	mcp := m["mcpCredentials"].(map[string]any)["sess"].(map[string]any)
	if mcp["port"].(float64) != 9000 {
		t.Fatalf("mcp port not persisted: %v", mcp)
	}

	// The in-memory struct passed to Update keeps its values (scrub is copy-only).
	// The caller's cfg is local to Update, so verify via the store instead.
	if got, _ := secrets.Get(secrets.KeyAIAPIKey); got != "sk-plain" {
		t.Fatalf("AI key not in store: %q", got)
	}
	if got, _ := secrets.Get(secrets.PipCredentialKey("https://pypi.example.com")); got != "credpw" {
		t.Fatalf("pip credential not in store: %q", got)
	}
	if got, _ := secrets.Get(secrets.KeyPipProxyPassword); got != "proxypw" {
		t.Fatalf("proxy password not in store: %q", got)
	}
	if got, _ := secrets.Get(secrets.MCPTokenKey("sess")); got != "tok" {
		t.Fatalf("mcp token not in store: %q", got)
	}
}

func TestMigratesLegacyPlaintextConfig(t *testing.T) {
	isolate(t)

	// Simulate an older config.json that still has plaintext secrets.
	dir, _ := os.UserConfigDir()
	cfgDir := filepath.Join(dir, "thaw")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := `{
	  "ai": {"provider": "openai", "apiKey": "sk-legacy", "enabled": true},
	  "oauth": {"gitlabClientSecret": "gl-legacy"}
	}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	// Load triggers migration + scrubbing re-save.
	if _, err := Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Secrets moved to the store.
	if got, _ := secrets.Get(secrets.KeyAIAPIKey); got != "sk-legacy" {
		t.Fatalf("AI key not migrated: %q", got)
	}
	if got, _ := secrets.Get(secrets.KeyGitLabClientSecret); got != "gl-legacy" {
		t.Fatalf("gitlab secret not migrated: %q", got)
	}

	// Plaintext scrubbed from disk.
	blob, _ := json.Marshal(rawConfig(t))
	if strings.Contains(string(blob), "sk-legacy") || strings.Contains(string(blob), "gl-legacy") {
		t.Fatalf("legacy secrets not scrubbed: %s", blob)
	}

	// Non-secret AI fields preserved.
	if ai := rawConfig(t)["ai"].(map[string]any); ai["provider"] != "openai" {
		t.Fatalf("provider lost during migration: %v", ai)
	}
}

// writeConfigJSON writes raw bytes to the isolated config.json path.
func writeConfigJSON(t *testing.T, body string) {
	t.Helper()
	dir, _ := os.UserConfigDir()
	cfgDir := filepath.Join(dir, "thaw")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config.json: %v", err)
	}
}

// TestOAuthSecretEditIsHonored covers the one secret with no app write seam:
// the GitHub/GitLab OAuth client secret is set only by hand-editing config.json,
// so config.json is authoritative for it. A later edit (rotation) must reach the
// store, not be dropped by the anti-clobber guard used for UI-backed secrets.
func TestOAuthSecretEditIsHonored(t *testing.T) {
	isolate(t)

	// First provisioning: migrate v1 into the store, scrub disk.
	writeConfigJSON(t, `{"oauth": {"githubClientSecret": "gh-secret-v1"}}`)
	if _, err := Load(); err != nil {
		t.Fatalf("Load v1: %v", err)
	}
	if got, _ := secrets.Get(secrets.KeyGitHubClientSecret); got != "gh-secret-v1" {
		t.Fatalf("v1 not migrated: %q", got)
	}

	// Admin rotates the secret by hand-editing config.json again.
	writeConfigJSON(t, `{"oauth": {"githubClientSecret": "gh-secret-v2"}}`)
	if _, err := Load(); err != nil {
		t.Fatalf("Load v2: %v", err)
	}

	// The rotation must have reached the store (not silently dropped).
	if got, _ := secrets.Get(secrets.KeyGitHubClientSecret); got != "gh-secret-v2" {
		t.Fatalf("rotated OAuth secret lost: store has %q, want gh-secret-v2", got)
	}
	// ...and scrubbed from disk.
	blob, _ := json.Marshal(rawConfig(t))
	if strings.Contains(string(blob), "gh-secret-v2") {
		t.Fatalf("OAuth secret not scrubbed from disk: %s", blob)
	}
}

func TestDeleteClearsSecretOnEmptySave(t *testing.T) {
	isolate(t)

	if err := secrets.Set(secrets.KeyAIAPIKey, "sk-old"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// A save with no plaintext must not resurrect or lose anything unexpectedly;
	// the store value is independent of config.json scrubbing.
	if err := Update(func(cfg *AppConfig) error { return nil }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got, _ := secrets.Get(secrets.KeyAIAPIKey); got != "sk-old" {
		t.Fatalf("store value clobbered by unrelated save: %q", got)
	}
}

// TestDoesNotClobberStoreWithStalePlaintext guards the dotfile-sync / restore
// case: a config.json carrying a stale plaintext secret must NOT overwrite a
// newer value already held in the OS store. Only the disk copy is scrubbed.
func TestDoesNotClobberStoreWithStalePlaintext(t *testing.T) {
	isolate(t)

	// The store already holds the current (newer) secret, e.g. set on this
	// machine, while config.json below carries a stale value from another host.
	if err := secrets.Set(secrets.KeyAIAPIKey, "sk-current"); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := secrets.Set(secrets.PipCredentialKey("https://pypi.example.com"), "pw-current"); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	dir, _ := os.UserConfigDir()
	cfgDir := filepath.Join(dir, "thaw")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stale := `{
	  "ai": {"provider": "openai", "apiKey": "sk-stale", "enabled": true},
	  "pipRegistry": {"credentials": [{"registry": "https://pypi.example.com", "username": "u", "password": "pw-stale"}]}
	}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(stale), 0o600); err != nil {
		t.Fatalf("write stale config: %v", err)
	}

	if _, err := Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The store keeps the newer values — never overwritten from the stale disk.
	if got, _ := secrets.Get(secrets.KeyAIAPIKey); got != "sk-current" {
		t.Fatalf("AI key clobbered by stale config.json: %q, want sk-current", got)
	}
	if got, _ := secrets.Get(secrets.PipCredentialKey("https://pypi.example.com")); got != "pw-current" {
		t.Fatalf("pip credential clobbered by stale config.json: %q, want pw-current", got)
	}

	// The stale plaintext is still scrubbed from disk.
	blob, _ := json.Marshal(rawConfig(t))
	if strings.Contains(string(blob), "sk-stale") || strings.Contains(string(blob), "pw-stale") {
		t.Fatalf("stale secrets not scrubbed from disk: %s", blob)
	}
}

// TestLoadDoesNotLeakSecretFields verifies Load never hands back a secret value
// in the config struct — consumers must hydrate from the store instead.
func TestLoadDoesNotLeakSecretFields(t *testing.T) {
	isolate(t)

	if err := secrets.Set(secrets.KeyAIAPIKey, "sk-live"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AI.APIKey != "" {
		t.Fatalf("Load leaked a secret field: %q", cfg.AI.APIKey)
	}
}
