// SPDX-License-Identifier: GPL-3.0-or-later

package app

import "thaw/internal/secrets"

// GetSecretStorageInfo reports where Thaw stores its secrets on this OS, so the
// Settings UI can show an accurate indicator ("Stored in macOS Keychain", or a
// warning when the plaintext file fallback is active) driven by the backend
// rather than frontend platform guessing.
func (a *App) GetSecretStorageInfo() secrets.Info {
	return secrets.GetInfo()
}

// storeOrDelete writes value under key, or deletes the key when value is empty,
// so clearing a secret in the UI removes it from the OS store rather than
// leaving a stale entry.
func storeOrDelete(key, value string) error {
	if value == "" {
		return secrets.Delete(key)
	}
	return secrets.Set(key, value)
}
