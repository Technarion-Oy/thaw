// SPDX-License-Identifier: GPL-3.0-or-later

package filesystem

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteBinaryFile decodes the base64-encoded content and writes the raw bytes
// to path, creating parent directories as needed. Used for binary export
// formats such as Excel (.xlsx).
func WriteBinaryFile(path, base64Content string) error {
	data, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		return fmt.Errorf("base64 decode: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// SanitizeFilename replaces characters that are invalid in file names with an
// underscore. Returns "_" when the result would otherwise be empty.
func SanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}
