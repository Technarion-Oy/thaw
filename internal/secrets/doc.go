// SPDX-License-Identifier: GPL-3.0-or-later

// Package secrets provides an OS-native credential store for Thaw-owned
// secrets (AI API keys, Git OAuth client secrets, pip registry passwords, MCP
// session tokens).
//
// Secrets are kept out of ~/.config/thaw/config.json and stored in the
// platform secure store instead: the macOS Keychain, the Windows Credential
// Manager, or the Linux Secret Service (libsecret) via github.com/99designs/keyring.
// When no OS store is available (headless Linux, unsupported platforms) it
// falls back to a plaintext 0600 file (~/.config/thaw/secrets.json), preserving
// the app's previous on-disk behavior without ever writing secrets to config.json.
//
// The active backend is reported over IPC (see App.GetSecretStorageInfo) so the
// Settings UI can show where a secret actually lives, including the fallback.
package secrets

// thaw:domain: Core IPC & App Lifecycle
