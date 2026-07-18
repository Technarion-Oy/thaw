// SPDX-License-Identifier: GPL-3.0-or-later

package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"thaw/internal/filesystem"
)

// fileStore is the fallback backend: a plaintext JSON file written with mode
// 0600. It preserves Thaw's previous on-disk behavior on platforms without an
// OS secure store, while still keeping secrets out of config.json.
type fileStore struct {
	mu   sync.Mutex
	path string
}

// newFileStore returns a file-backed store at ~/.config/thaw/secrets.json.
// THAW_SECRETS_DIR overrides the directory (used by tests).
func newFileStore() *fileStore {
	return &fileStore{path: fileStorePath()}
}

func fileStorePath() string {
	if dir := os.Getenv("THAW_SECRETS_DIR"); dir != "" {
		return filepath.Join(dir, "secrets.json")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		// Last resort: current directory. UserConfigDir only fails when neither
		// HOME nor the platform config env is set, which is unusual.
		return "secrets.json"
	}
	return filepath.Join(dir, "thaw", "secrets.json")
}

// read loads the on-disk map; a missing file yields an empty map.
func (s *fileStore) read() (map[string]string, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// write atomically persists the map with mode 0600.
func (s *fileStore) write(m map[string]string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return filesystem.WriteFileAtomic(s.path, data, 0o600)
}

func (s *fileStore) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.read()
	if err != nil {
		return "", err
	}
	v, ok := m[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (s *fileStore) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.read()
	if err != nil {
		return err
	}
	m[key] = value
	return s.write(m)
}

func (s *fileStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.read()
	if err != nil {
		return err
	}
	if _, ok := m[key]; !ok {
		return nil
	}
	delete(m, key)
	return s.write(m)
}

func (s *fileStore) Keys() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.read()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (s *fileStore) Info() Info {
	return Info{Method: MethodFile, Secure: false, Label: MethodFile.label(), Detail: s.path}
}
