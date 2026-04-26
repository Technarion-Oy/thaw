// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

//go:build darwin

package gitrepo

import (
	"bytes"
	"os/exec"
	"strings"
)

// lookupOSCredentials queries the macOS Keychain using the built-in `security`
// command-line tool.  No dependency on system git is required.
func lookupOSCredentials(host string) *storedCreds {
	// Attempt 1: internet password with explicit protocol (htps = HTTPS)
	password := runSecurityPassword(host, "-r", "htps")
	if password == "" {
		// Attempt 2: without protocol filter — matches any scheme
		password = runSecurityPassword(host)
	}
	if password == "" {
		return nil
	}

	// Retrieve the account name from verbose output
	username := extractKeychainUsername(host)

	return &storedCreds{
		username: username,
		password: password,
		source:   "keychain",
	}
}

// runSecurityPassword calls `security find-internet-password -s host [extraArgs…] -w`
// and returns the trimmed password, or "" on failure.
func runSecurityPassword(host string, extraArgs ...string) string {
	args := append([]string{"find-internet-password", "-s", host}, extraArgs...)
	args = append(args, "-w")
	var out bytes.Buffer
	cmd := exec.Command("security", args...)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// extractKeychainUsername retrieves the "acct" field from a verbose
// `security find-internet-password` lookup.
func extractKeychainUsername(host string) string {
	var out bytes.Buffer
	cmd := exec.Command("security", "find-internet-password", "-s", host)
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run()

	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		// Match: "acct"<blob>="username"
		if strings.HasPrefix(line, `"acct"`) && strings.Contains(line, `="`) {
			parts := strings.SplitN(line, `="`, 2)
			if len(parts) == 2 {
				return strings.TrimSuffix(parts[1], `"`)
			}
		}
	}
	return ""
}
