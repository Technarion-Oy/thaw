// SPDX-License-Identifier: GPL-3.0-or-later

package secrets

import (
	"errors"
	"os"
	"sync"
)

// serviceName is the identifier under which Thaw's secrets are grouped in the
// OS store (Keychain service, WinCred prefix scope, Secret Service attribute).
const serviceName = "thaw"

// ErrNotFound is returned by Get when no secret is stored for the given key.
var ErrNotFound = errors.New("secret not found")

// Method identifies which backend a Store is using. It is reported to the
// frontend so the Settings UI can show where a secret actually lives.
type Method string

// #nosec G101 -- these are backend identifiers reported to the UI, not credential values.
const (
	MethodKeychain          Method = "keychain"           // macOS Keychain
	MethodCredentialManager Method = "credential-manager" // Windows Credential Manager
	MethodSecretService     Method = "secret-service"     // Linux Secret Service (libsecret)
	MethodFile              Method = "file"               // plaintext 0600 fallback file
)

// Info describes the active storage backend. It carries no secret material and
// is safe to send to the frontend.
type Info struct {
	// Method is the backend identifier.
	Method Method `json:"method"`
	// Secure is true when secrets are held in an OS-native secure store, false
	// when the plaintext file fallback is active.
	Secure bool `json:"secure"`
	// Label is a human-readable name, e.g. "macOS Keychain".
	Label string `json:"label"`
	// Detail is the exact on-disk path when the file fallback is active, empty
	// otherwise. Lets the UI point the user at the file.
	Detail string `json:"detail,omitempty"`
}

// label returns the human-readable name for a storage method.
func (m Method) label() string {
	switch m {
	case MethodKeychain:
		return "macOS Keychain"
	case MethodCredentialManager:
		return "Windows Credential Manager"
	case MethodSecretService:
		return "Secret Service (libsecret)"
	default:
		return "a local file (no OS secure store available)"
	}
}

// Store is the credential-store abstraction. Implementations are safe for
// concurrent use.
type Store interface {
	// Get returns the secret for key, or ErrNotFound if none is stored.
	Get(key string) (string, error)
	// Set stores value under key, overwriting any existing value.
	Set(key, value string) error
	// Delete removes key. It is not an error if key does not exist, so callers
	// tolerate a secret deleted out-of-band from the OS store.
	Delete(key string) error
	// Keys lists all keys currently held by this store.
	Keys() ([]string, error)
	// Info reports the active backend.
	Info() Info
}

var (
	mu  sync.Mutex
	def Store
)

// defaultStore lazily builds and caches the process-wide store. Construction is
// deferred so a config Load/Save that touches no secrets never initializes an
// OS store (avoiding a keychain prompt on paths that don't need it).
func defaultStore() Store {
	mu.Lock()
	defer mu.Unlock()
	if def == nil {
		def = buildStore()
	}
	return def
}

// buildStore selects the best available backend: the OS-native secure store
// when it opens successfully, otherwise the plaintext 0600 file fallback.
// THAW_SECRETS_BACKEND=file forces the fallback (used by tests and headless
// deployments that must not touch a real keychain).
func buildStore() Store {
	if os.Getenv("THAW_SECRETS_BACKEND") == "file" {
		return newFileStore()
	}
	if s, err := newOSStore(); err == nil {
		return s
	}
	return newFileStore()
}

// SetDefaultForTesting swaps the process-wide store and returns a function that
// restores the previous one. Intended for tests only.
func SetDefaultForTesting(s Store) (restore func()) {
	mu.Lock()
	defer mu.Unlock()
	prev := def
	def = s
	return func() {
		mu.Lock()
		def = prev
		mu.Unlock()
	}
}

// Get returns the secret for key from the default store, or ErrNotFound.
func Get(key string) (string, error) { return defaultStore().Get(key) }

// Set stores value under key in the default store.
func Set(key, value string) error { return defaultStore().Set(key, value) }

// Delete removes key from the default store. Missing keys are not an error.
func Delete(key string) error { return defaultStore().Delete(key) }

// Keys lists all keys held by the default store.
func Keys() ([]string, error) { return defaultStore().Keys() }

// GetInfo reports the default store's active backend.
func GetInfo() Info { return defaultStore().Info() }
