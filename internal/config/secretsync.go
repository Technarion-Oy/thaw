// SPDX-License-Identifier: GPL-3.0-or-later

package config

import "thaw/internal/secrets"

// This file keeps Thaw-owned secrets out of config.json. The secret-bearing
// struct fields (AI.APIKey, OAuth client secrets, pip passwords, MCP tokens) are
// treated as in-memory transport only: they are never written to disk, and the
// authoritative copy lives in the OS secure store (see internal/secrets).
//
//   - save() marshals a scrubbed copy, so a secret can never leak to config.json
//     even if a caller sets the field and saves without going through the store.
//   - load() runs a one-time migration: any plaintext secret still present in an
//     older config.json is moved into the OS store, and Load() then re-saves the
//     scrubbed file. After migration the fields are empty on disk, so the hot
//     load path performs zero secure-store access.
//
// Reads and writes of the actual secret values are routed through internal/secrets
// at each consumer/IPC seam, not here — this file only guarantees the disk file
// stays clean.

// persistSecrets writes every non-empty secret field in cfg to the OS secure
// store, so a secret set on the config struct is never lost when save() scrubs
// it from disk. It performs no store access for empty fields, so an unrelated
// save (feature flags, editor prefs, …) — whose secret fields are always empty
// after load's scrub — touches the store zero times.
//
// It intentionally never deletes: clearing a secret (empty value) is done
// explicitly by the owning IPC handler via secrets.Delete, because save() has
// no view of the previous value to diff against.
func persistSecrets(cfg *AppConfig) {
	set := func(key, val string) {
		if val == "" {
			return
		}
		if err := secrets.Set(key, val); err != nil {
			// Keep the plaintext scrub anyway; the handler-level store write is
			// the authoritative path and will have surfaced the error already.
			_ = err
		}
	}
	set(secrets.KeyAIAPIKey, cfg.AI.APIKey)
	set(secrets.KeyGitHubClientSecret, cfg.OAuth.GithubClientSecret)
	set(secrets.KeyGitLabClientSecret, cfg.OAuth.GitlabClientSecret)
	set(secrets.KeyPipProxyPassword, cfg.PipRegistry.ProxyPassword)
	for i := range cfg.PipRegistry.Credentials {
		c := cfg.PipRegistry.Credentials[i]
		set(secrets.PipCredentialKey(c.Registry), c.Password)
	}
	for label, cred := range cfg.MCPCredentials {
		set(secrets.MCPTokenKey(label), cred.Token)
	}
}

// scrubbedCopy returns a value copy of cfg with every secret field blanked. The
// collection-valued secrets (pip credentials, MCP tokens) are copied before
// blanking so the caller's in-memory config keeps its values.
func scrubbedCopy(cfg *AppConfig) AppConfig {
	c := *cfg
	c.AI.APIKey = ""
	c.OAuth.GithubClientSecret = ""
	c.OAuth.GitlabClientSecret = ""
	c.PipRegistry.ProxyPassword = ""
	if len(cfg.PipRegistry.Credentials) > 0 {
		creds := make([]PipRegistryCredential, len(cfg.PipRegistry.Credentials))
		copy(creds, cfg.PipRegistry.Credentials)
		for i := range creds {
			creds[i].Password = ""
		}
		c.PipRegistry.Credentials = creds
	}
	if len(cfg.MCPCredentials) > 0 {
		m := make(map[string]MCPSessionCredential, len(cfg.MCPCredentials))
		for k, v := range cfg.MCPCredentials {
			v.Token = ""
			m[k] = v
		}
		c.MCPCredentials = m
	}
	return c
}

// migrateSecretsToStore moves any plaintext secret still embedded in cfg into
// the OS secure store. It returns true when the disk file needs a scrubbing
// re-save — either because a value was newly migrated, or because a stale
// plaintext copy remains on disk while the store already holds the secret.
//
// It performs no secure-store access for empty fields, so once migration has
// run and config.json is scrubbed, subsequent loads are free of store calls.
func migrateSecretsToStore(cfg *AppConfig) (needsScrub bool) {
	move := func(key, val string) {
		if val == "" {
			return
		}
		// A stale plaintext value on disk but already present in the store: no
		// write needed, but the disk copy must be scrubbed.
		if _, err := secrets.Get(key); err == nil {
			needsScrub = true
			return
		}
		if err := secrets.Set(key, val); err != nil {
			// Leave the plaintext in place rather than lose the secret; it will
			// be retried on the next load.
			return
		}
		needsScrub = true
	}

	move(secrets.KeyAIAPIKey, cfg.AI.APIKey)
	move(secrets.KeyGitHubClientSecret, cfg.OAuth.GithubClientSecret)
	move(secrets.KeyGitLabClientSecret, cfg.OAuth.GitlabClientSecret)
	move(secrets.KeyPipProxyPassword, cfg.PipRegistry.ProxyPassword)
	for i := range cfg.PipRegistry.Credentials {
		c := cfg.PipRegistry.Credentials[i]
		move(secrets.PipCredentialKey(c.Registry), c.Password)
	}
	for label, cred := range cfg.MCPCredentials {
		move(secrets.MCPTokenKey(label), cred.Token)
	}
	return needsScrub
}
