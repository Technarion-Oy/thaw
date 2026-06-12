// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package sfconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// profileNameRe matches valid profile names: alphanumerics, hyphens, underscores.
var profileNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// sectionHeaderRe matches TOML section headers like [connections.myprofile].
// Uses [^\[\]]+ to avoid matching TOML array-of-tables ([[...]]) or malformed brackets.
var sectionHeaderRe = regexp.MustCompile(`^\s*\[([^\[\]]+)\]\s*$`)

// sectionSpan marks the start and end line indices of a TOML section.
type sectionSpan struct {
	name  string // e.g. "connections.myprofile"
	start int    // line index of the [section] header
	end   int    // exclusive end (first line of next section or len(lines))
}

// ValidateProfileName checks that a profile name is non-empty and contains
// only alphanumerics, hyphens, and underscores.
func ValidateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name must not be empty")
	}
	if !profileNameRe.MatchString(name) {
		return fmt.Errorf("profile name %q contains invalid characters (allowed: A-Z, a-z, 0-9, _ , -)", name)
	}
	return nil
}

// resolvePath returns the effective config file path, matching Load() behavior.
func resolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".snowflake", "config.toml"), nil
}

// parseSections finds all [section] boundaries in the given lines.
func parseSections(lines []string) []sectionSpan {
	var spans []sectionSpan
	for i, line := range lines {
		m := sectionHeaderRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if len(spans) > 0 {
			spans[len(spans)-1].end = i
		}
		spans = append(spans, sectionSpan{name: m[1], start: i, end: len(lines)})
	}
	return spans
}

// tomlEscape escapes a string for use as a TOML basic quoted value.
func tomlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// connectionFieldOrder defines the canonical order for rendering connection fields.
var connectionFieldOrder = []struct {
	tomlKey string
	getter  func(*Connection) string
}{
	{"account", func(c *Connection) string { return c.Account }},
	{"user", func(c *Connection) string { return c.User }},
	{"password", func(c *Connection) string { return c.Password }},
	{"role", func(c *Connection) string { return c.Role }},
	{"warehouse", func(c *Connection) string { return c.Warehouse }},
	{"database", func(c *Connection) string { return c.Database }},
	{"schema", func(c *Connection) string { return c.Schema }},
	{"authenticator", func(c *Connection) string { return c.Authenticator }},
	{"passcode", func(c *Connection) string { return c.Passcode }},
	{"okta_url", func(c *Connection) string { return c.OktaURL }},
	{"private_key_path", func(c *Connection) string { return c.PrivateKeyPath }},
	{"private_key_passphrase", func(c *Connection) string { return c.PrivateKeyPassphrase }},
	{"token", func(c *Connection) string { return c.Token }},
	{"token_file_path", func(c *Connection) string { return c.TokenFilePath }},
	{"oauth_client_id", func(c *Connection) string { return c.OAuthClientID }},
	{"oauth_client_secret", func(c *Connection) string { return c.OAuthClientSecret }},
	{"oauth_token_request_url", func(c *Connection) string { return c.OAuthTokenRequestURL }},
	{"oauth_authorization_url", func(c *Connection) string { return c.OAuthAuthorizationURL }},
	{"oauth_redirect_uri", func(c *Connection) string { return c.OAuthRedirectURI }},
	{"oauth_scope", func(c *Connection) string { return c.OAuthScope }},
	{"workload_identity_provider", func(c *Connection) string { return c.WorkloadIdentityProvider }},
	{"workload_identity_entra_resource", func(c *Connection) string { return c.WorkloadIdentityEntraResource }},
	{"workload_identity_impersonation_path", func(c *Connection) string { return c.WorkloadIdentityImpersonationPath }},
}

// connectionBoolFieldOrder defines the canonical order for boolean connection
// fields. These render as unquoted TOML booleans (e.g. `key = true`) rather
// than quoted strings, and are only emitted when true.
var connectionBoolFieldOrder = []struct {
	tomlKey string
	getter  func(*Connection) bool
}{
	{"enable_single_use_refresh_tokens", func(c *Connection) bool { return c.EnableSingleUseRefreshTokens }},
}

// connectionToTOMLLines renders a Connection as TOML key=value lines (without
// the section header). Only non-empty / true fields are included.
func connectionToTOMLLines(c Connection) []string {
	var out []string
	for _, f := range connectionFieldOrder {
		v := f.getter(&c)
		if v == "" {
			continue
		}
		out = append(out, fmt.Sprintf(`%s = "%s"`, f.tomlKey, tomlEscape(v)))
	}
	for _, f := range connectionBoolFieldOrder {
		if f.getter(&c) {
			out = append(out, fmt.Sprintf("%s = true", f.tomlKey))
		}
	}
	return out
}

// kvLineRe matches TOML key = value lines to extract the key name.
var kvLineRe = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=`)

// knownConnectionKeys is the set of TOML keys that Thaw models on Connection.
var knownConnectionKeys map[string]bool

func init() {
	knownConnectionKeys = make(map[string]bool, len(connectionFieldOrder)+len(connectionBoolFieldOrder))
	for _, f := range connectionFieldOrder {
		knownConnectionKeys[f.tomlKey] = true
	}
	for _, f := range connectionBoolFieldOrder {
		knownConnectionKeys[f.tomlKey] = true
	}
}

// atomicWriteFile writes data to a temp file and renames it into place.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".thaw-sfconfig-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// Clean up on failure.
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// readFileLines reads a file and splits it into lines.
// Returns empty slice (not error) if the file doesn't exist.
func readFileLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	if content == "" {
		return nil, nil
	}
	return strings.Split(content, "\n"), nil
}

// writeLines joins lines and writes them atomically.
func writeLines(path string, lines []string) error {
	return atomicWriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}

// findConnectionSpan finds the sectionSpan for connections.<name>.
func findConnectionSpan(spans []sectionSpan, name string) *sectionSpan {
	target := "connections." + name
	for i := range spans {
		if spans[i].name == target {
			return &spans[i]
		}
	}
	return nil
}

// sectionBodyEnd returns the end of the meaningful body of a section, excluding
// trailing blank lines and comment-only lines that visually belong to the next
// section. This prevents "eating" comments that precede the next [section] header.
func sectionBodyEnd(lines []string, span *sectionSpan) int {
	end := span.end
	for end > span.start+1 {
		line := strings.TrimSpace(lines[end-1])
		if line == "" || strings.HasPrefix(line, "#") {
			end--
		} else {
			break
		}
	}
	return end
}

// extractPreservedLines returns lines from an existing section that should
// survive a SaveProfile update: intra-section comments, blank lines, and
// key=value lines for keys Thaw doesn't model.
//
// Note: SaveProfile appends preserved lines after the Thaw-modeled fields,
// so intra-section content may be reordered relative to the original layout.
func extractPreservedLines(lines []string, span *sectionSpan) []string {
	var preserved []string
	bodyEnd := sectionBodyEnd(lines, span)
	for i := span.start + 1; i < bodyEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			preserved = append(preserved, lines[i])
			continue
		}
		m := kvLineRe.FindStringSubmatch(lines[i])
		if m != nil && !knownConnectionKeys[m[1]] {
			preserved = append(preserved, lines[i])
		}
	}
	return preserved
}

// SaveProfile creates or updates a named connection profile in the config file.
func SaveProfile(path string, profile Connection) error {
	if err := ValidateProfileName(profile.Name); err != nil {
		return err
	}
	resolved, err := resolvePath(path)
	if err != nil {
		return err
	}

	lines, err := readFileLines(resolved)
	if err != nil {
		return err
	}

	spans := parseSections(lines)
	existing := findConnectionSpan(spans, profile.Name)

	header := fmt.Sprintf("[connections.%s]", profile.Name)
	newKVs := connectionToTOMLLines(profile)

	if existing != nil {
		// Update: replace only the meaningful body (header + key/value lines),
		// leaving trailing blanks/comments that belong to the next section.
		bodyEnd := sectionBodyEnd(lines, existing)
		extra := extractPreservedLines(lines, existing)
		var replacement []string
		replacement = append(replacement, header)
		replacement = append(replacement, newKVs...)
		replacement = append(replacement, extra...)

		// Build new file content.
		var result []string
		result = append(result, lines[:existing.start]...)
		result = append(result, replacement...)
		result = append(result, lines[bodyEnd:]...)
		return writeLines(resolved, result)
	}

	// New profile: append at end.
	if lines == nil {
		lines = []string{}
	}
	// Ensure a blank line before the new section if file isn't empty.
	if len(lines) > 0 {
		last := lines[len(lines)-1]
		if strings.TrimSpace(last) != "" {
			lines = append(lines, "")
		}
	}
	lines = append(lines, header)
	lines = append(lines, newKVs...)
	lines = append(lines, "") // trailing newline
	return writeLines(resolved, lines)
}

// DeleteProfile removes a named connection profile. If the deleted profile was
// the default, default_connection_name is cleared.
func DeleteProfile(path string, name string) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	resolved, err := resolvePath(path)
	if err != nil {
		return err
	}

	lines, err := readFileLines(resolved)
	if err != nil {
		return err
	}
	if lines == nil {
		return fmt.Errorf("profile %q not found", name)
	}

	spans := parseSections(lines)
	span := findConnectionSpan(spans, name)
	if span == nil {
		return fmt.Errorf("profile %q not found", name)
	}

	// Remove the section, plus at most one trailing blank line.
	end := span.end
	if end < len(lines) && strings.TrimSpace(lines[end]) == "" {
		end++
	}

	var result []string
	result = append(result, lines[:span.start]...)
	result = append(result, lines[end:]...)

	// Clear default_connection_name if it pointed to the deleted profile.
	result = clearDefaultIfMatches(result, name)

	return writeLines(resolved, result)
}

// CloneProfile duplicates a profile under a new name.
func CloneProfile(path string, sourceName, newName string) error {
	if err := ValidateProfileName(sourceName); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if err := ValidateProfileName(newName); err != nil {
		return fmt.Errorf("new name: %w", err)
	}
	resolved, err := resolvePath(path)
	if err != nil {
		return err
	}

	lines, err := readFileLines(resolved)
	if err != nil {
		return err
	}
	if lines == nil {
		return fmt.Errorf("source profile %q not found", sourceName)
	}

	spans := parseSections(lines)

	// Check source exists.
	src := findConnectionSpan(spans, sourceName)
	if src == nil {
		return fmt.Errorf("source profile %q not found", sourceName)
	}

	// Check destination doesn't exist.
	if findConnectionSpan(spans, newName) != nil {
		return fmt.Errorf("profile %q already exists", newName)
	}

	// Copy source section body (everything except the header), excluding
	// trailing blanks/comments that visually belong to the next section.
	bodyEnd := sectionBodyEnd(lines, src)
	var cloned []string
	cloned = append(cloned, fmt.Sprintf("[connections.%s]", newName))
	for i := src.start + 1; i < bodyEnd; i++ {
		cloned = append(cloned, lines[i])
	}

	// Append the cloned section at end.
	if len(lines) > 0 {
		last := lines[len(lines)-1]
		if strings.TrimSpace(last) != "" {
			lines = append(lines, "")
		}
	}
	lines = append(lines, cloned...)
	lines = append(lines, "") // trailing newline
	return writeLines(resolved, lines)
}

// defaultConnRe matches the default_connection_name line with double-quoted,
// single-quoted, or bare (unquoted) values.
var defaultConnRe = regexp.MustCompile(`^(\s*)default_connection_name\s*=\s*(?:"([^"]*)"|'([^']*)'|(\S+))`)

// defaultConnValue extracts the matched value from defaultConnRe submatches.
// Group 2 = double-quoted, group 3 = single-quoted, group 4 = bare.
func defaultConnValue(m []string) string {
	if m[2] != "" {
		return m[2]
	}
	if m[3] != "" {
		return m[3]
	}
	return m[4]
}

// SetDefaultProfile updates or inserts the default_connection_name line.
func SetDefaultProfile(path string, name string) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	resolved, err := resolvePath(path)
	if err != nil {
		return err
	}

	lines, err := readFileLines(resolved)
	if err != nil {
		return err
	}
	if lines == nil {
		return fmt.Errorf("config file does not exist")
	}

	// Verify the profile exists.
	spans := parseSections(lines)
	if findConnectionSpan(spans, name) == nil {
		return fmt.Errorf("profile %q not found", name)
	}

	// Find and replace existing default_connection_name line.
	for i, line := range lines {
		m := defaultConnRe.FindStringSubmatch(line)
		if m != nil {
			lines[i] = fmt.Sprintf(`%sdefault_connection_name = "%s"`, m[1], tomlEscape(name))
			return writeLines(resolved, lines)
		}
	}

	// No existing line — insert before the first [section].
	newLine := fmt.Sprintf(`default_connection_name = "%s"`, tomlEscape(name))
	if len(spans) > 0 {
		insertAt := spans[0].start
		var result []string
		result = append(result, lines[:insertAt]...)
		result = append(result, newLine, "")
		result = append(result, lines[insertAt:]...)
		return writeLines(resolved, result)
	}

	// No sections at all — append.
	lines = append(lines, newLine, "")
	return writeLines(resolved, lines)
}

// ClearDefaultProfile removes the default_connection_name value (sets it to "").
func ClearDefaultProfile(path string) error {
	resolved, err := resolvePath(path)
	if err != nil {
		return err
	}

	lines, err := readFileLines(resolved)
	if err != nil {
		return err
	}
	if lines == nil {
		return nil
	}

	for i, line := range lines {
		m := defaultConnRe.FindStringSubmatch(line)
		if m != nil {
			lines[i] = fmt.Sprintf(`%sdefault_connection_name = ""`, m[1])
			return writeLines(resolved, lines)
		}
	}
	return nil
}

// RenameProfile renames a connection profile. The old section header is
// replaced, and default_connection_name is updated if it matched the old name.
// Returns an error if the new name already exists.
func RenameProfile(path string, oldName, newName string) error {
	if err := ValidateProfileName(oldName); err != nil {
		return fmt.Errorf("old name: %w", err)
	}
	if err := ValidateProfileName(newName); err != nil {
		return fmt.Errorf("new name: %w", err)
	}
	if oldName == newName {
		return nil
	}
	resolved, err := resolvePath(path)
	if err != nil {
		return err
	}

	lines, err := readFileLines(resolved)
	if err != nil {
		return err
	}
	if lines == nil {
		return fmt.Errorf("profile %q not found", oldName)
	}

	spans := parseSections(lines)

	src := findConnectionSpan(spans, oldName)
	if src == nil {
		return fmt.Errorf("profile %q not found", oldName)
	}
	if findConnectionSpan(spans, newName) != nil {
		return fmt.Errorf("profile %q already exists", newName)
	}

	// Replace the section header.
	lines[src.start] = fmt.Sprintf("[connections.%s]", newName)

	// Update default_connection_name if it pointed to the old name.
	for i, line := range lines {
		m := defaultConnRe.FindStringSubmatch(line)
		if m != nil && defaultConnValue(m) == oldName {
			lines[i] = fmt.Sprintf(`%sdefault_connection_name = "%s"`, m[1], tomlEscape(newName))
			break
		}
	}

	return writeLines(resolved, lines)
}

// clearDefaultIfMatches scans lines for default_connection_name and clears it
// if it matches the given name.
func clearDefaultIfMatches(lines []string, name string) []string {
	for i, line := range lines {
		m := defaultConnRe.FindStringSubmatch(line)
		if m != nil && defaultConnValue(m) == name {
			lines[i] = fmt.Sprintf(`%sdefault_connection_name = ""`, m[1])
			break
		}
	}
	return lines
}
