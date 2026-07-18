// SPDX-License-Identifier: GPL-3.0-or-later

package secrets

import (
	"errors"
	"runtime"

	"github.com/99designs/keyring"
)

// osStore wraps github.com/99designs/keyring, restricted to the single
// OS-native secure backend for the current platform.
type osStore struct {
	ring   keyring.Keyring
	method Method
}

// newOSStore opens the OS-native secure store. It returns an error when no such
// store is available (e.g. headless Linux with no Secret Service), signalling
// the caller to fall back to the file store.
func newOSStore() (*osStore, error) {
	var allowed []keyring.BackendType
	var method Method
	switch runtime.GOOS {
	case "darwin":
		allowed = []keyring.BackendType{keyring.KeychainBackend}
		method = MethodKeychain
	case "windows":
		allowed = []keyring.BackendType{keyring.WinCredBackend}
		method = MethodCredentialManager
	default:
		allowed = []keyring.BackendType{keyring.SecretServiceBackend}
		method = MethodSecretService
	}

	ring, err := keyring.Open(keyring.Config{
		ServiceName:     serviceName,
		AllowedBackends: allowed,
		// macOS: trust the calling app so reads don't prompt on every access,
		// and keep items out of iCloud sync.
		KeychainTrustApplication:       true,
		KeychainAccessibleWhenUnlocked: true,
		KeychainSynchronizable:         false,
		// Linux Secret Service: use the default (login) collection.
		LibSecretCollectionName: "login",
		// Windows Credential Manager: namespace targets under thaw:.
		WinCredPrefix: serviceName + ":",
	})
	if err != nil {
		return nil, err
	}
	return &osStore{ring: ring, method: method}, nil
}

func (s *osStore) Get(key string) (string, error) {
	item, err := s.ring.Get(key)
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return string(item.Data), nil
}

func (s *osStore) Set(key, value string) error {
	return s.ring.Set(keyring.Item{
		Key:   key,
		Data:  []byte(value),
		Label: serviceName + " · " + key,
	})
}

func (s *osStore) Delete(key string) error {
	err := s.ring.Remove(key)
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return nil
	}
	return err
}

func (s *osStore) Keys() ([]string, error) {
	return s.ring.Keys()
}

func (s *osStore) Info() Info {
	return Info{Method: s.method, Secure: true, Label: s.method.label()}
}
