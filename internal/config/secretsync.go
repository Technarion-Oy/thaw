// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"errors"

	"thaw/internal/secrets"
)

// This file keeps Thaw-owned secrets out of config.json. The secret-bearing
// struct fields (AI.APIKey, OAuth client secrets, pip passwords, MCP tokens) are
// treated as in-memory transport only: the authoritative copy lives in the OS
// secure store (see internal/secrets).
//
//   - save() writes what buildDiskConfig returns: each secret is blanked on disk
//     only once it is safely in the store. The store is NEVER overwritten from a
//     value read out of config.json, so a stale/synced/restored config.json can't
//     clobber a newer secret held in the OS store.
//   - Load() migrates a legacy plaintext config.json once (persist to store, then
//     scrub the file) and never hands back a secret the store doesn't own.
//   - Authoritative updates (the user changing a secret in Settings) go through
//     the owning IPC seam via secrets.Set/Delete, which DO overwrite. buildDiskConfig
//     is only the migration/first-write safety net, never the update path.
//
// After migration the fields are empty on disk, so the hot save/load paths perform
// zero secure-store access (buildDiskConfig only touches the store for a non-empty
// secret field, which occurs only right after a handler populated one).

// hasPlaintextSecret reports whether cfg carries any non-empty secret field —
// i.e. a legacy plaintext config.json that must be persisted to the store and
// scrubbed from disk on the next save.
func hasPlaintextSecret(cfg *AppConfig) bool {
	if cfg.AI.APIKey != "" ||
		cfg.OAuth.GithubClientSecret != "" ||
		cfg.OAuth.GitlabClientSecret != "" ||
		cfg.PipRegistry.ProxyPassword != "" {
		return true
	}
	for _, c := range cfg.PipRegistry.Credentials {
		if c.Password != "" {
			return true
		}
	}
	for _, cred := range cfg.MCPCredentials {
		if cred.Token != "" {
			return true
		}
	}
	return false
}

// blankSecrets zeroes every secret field on cfg in place. Load uses it so a
// caller never receives a secret value the store doesn't own: the store is
// authoritative, and a stale config.json must not shadow it.
func blankSecrets(cfg *AppConfig) {
	cfg.AI.APIKey = ""
	cfg.OAuth.GithubClientSecret = ""
	cfg.OAuth.GitlabClientSecret = ""
	cfg.PipRegistry.ProxyPassword = ""
	for i := range cfg.PipRegistry.Credentials {
		cfg.PipRegistry.Credentials[i].Password = ""
	}
	for label, cred := range cfg.MCPCredentials {
		if cred.Token != "" {
			cred.Token = ""
			cfg.MCPCredentials[label] = cred
		}
	}
}

// buildDiskConfig persists cfg's secrets into the OS store and returns the
// AppConfig to write to disk. A secret is blanked on disk ONLY once it is safely
// in the store — either already present (never overwritten, so a stale synced
// config.json can't clobber a newer store value) or written successfully now. A
// secret that could not be stored is left in the returned config so it is never
// lost; the 0600 file is the fallback and migration retries on the next save.
//
// The collection-valued secrets (pip credentials, MCP tokens) are copied before
// blanking so the caller's in-memory config is never mutated.
func buildDiskConfig(cfg *AppConfig) AppConfig {
	c := *cfg

	// stored reports whether it is safe to blank this secret on disk: true when
	// the value is empty, already in the store, or written to the store now. It
	// never overwrites a value the store already holds — the anti-clobber guard
	// for secrets that have an authoritative app write seam (SaveAIConfig,
	// SavePipRegistryConfig, saveMCPCredential), so a stale/synced config.json
	// can't overwrite a newer value set through the app.
	//
	// A transient Get error (dbus hiccup, keychain glitch — anything other than a
	// genuine ErrNotFound) must NOT be treated as "absent": overwriting the store
	// with a disk value on a read glitch is the very clobber this guard prevents.
	// On such an error, keep the plaintext on disk (return false) and retry next
	// save rather than risk clobbering a real, newer secret.
	stored := func(key, val string) bool {
		if val == "" {
			return true
		}
		switch _, err := secrets.Get(key); {
		case err == nil:
			return true // present already — do NOT overwrite from disk
		case errors.Is(err, secrets.ErrNotFound):
			return secrets.Set(key, val) == nil // genuinely absent — migrate it in
		default:
			return false // unknown store error — leave plaintext on disk, retry later
		}
	}

	// storedFromDisk is for secrets whose ONLY writer is config.json — the OAuth
	// client secrets have no UI/app write seam, so a hand-edit to config.json IS
	// the authoritative update. Set-if-changed (overwriting any prior store
	// value) so rotating the secret via config.json is honored, not silently
	// dropped by the anti-clobber guard above. An empty value is left untouched
	// in the store: after the first migration config.json is scrubbed, so empty
	// means "already migrated", not "cleared" — deleting on empty would wipe the
	// secret on the very next load.
	storedFromDisk := func(key, val string) bool {
		if val == "" {
			return true
		}
		if cur, err := secrets.Get(key); err == nil && cur == val {
			return true // already up to date
		}
		return secrets.Set(key, val) == nil
	}

	if stored(secrets.KeyAIAPIKey, cfg.AI.APIKey) {
		c.AI.APIKey = ""
	}
	if storedFromDisk(secrets.KeyGitHubClientSecret, cfg.OAuth.GithubClientSecret) {
		c.OAuth.GithubClientSecret = ""
	}
	if storedFromDisk(secrets.KeyGitLabClientSecret, cfg.OAuth.GitlabClientSecret) {
		c.OAuth.GitlabClientSecret = ""
	}
	if stored(secrets.KeyPipProxyPassword, cfg.PipRegistry.ProxyPassword) {
		c.PipRegistry.ProxyPassword = ""
	}
	if len(cfg.PipRegistry.Credentials) > 0 {
		creds := make([]PipRegistryCredential, len(cfg.PipRegistry.Credentials))
		copy(creds, cfg.PipRegistry.Credentials)
		for i := range creds {
			if stored(secrets.PipCredentialKey(creds[i].Registry), creds[i].Password) {
				creds[i].Password = ""
			}
		}
		c.PipRegistry.Credentials = creds
	}
	if len(cfg.MCPCredentials) > 0 {
		m := make(map[string]MCPSessionCredential, len(cfg.MCPCredentials))
		for label, cred := range cfg.MCPCredentials {
			if stored(secrets.MCPTokenKey(label), cred.Token) {
				cred.Token = ""
			}
			m[label] = cred
		}
		c.MCPCredentials = m
	}
	return c
}
