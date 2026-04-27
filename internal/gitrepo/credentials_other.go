// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

//go:build !darwin && !windows

package gitrepo

// lookupOSCredentials is a no-op on non-macOS, non-Windows platforms.
// Credentials are still resolved from ~/.git-credentials and ~/.netrc.
func lookupOSCredentials(_ string) *storedCreds {
	return nil
}
