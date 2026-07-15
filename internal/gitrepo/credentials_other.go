// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !darwin && !windows

package gitrepo

// lookupOSCredentials is a no-op on non-macOS, non-Windows platforms.
// Credentials are still resolved from ~/.git-credentials and ~/.netrc.
func lookupOSCredentials(_ string) *storedCreds {
	return nil
}
