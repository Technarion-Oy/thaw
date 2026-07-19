// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package filesystem

// RaiseFDLimit is a no-op on Windows: the C runtime file-handle limit is not a
// setrlimit-style process resource, and the recursive ReadDirectoryChangesW
// backend uses a single handle for the whole tree regardless. Returns zero
// limits and no error so callers can treat it uniformly across platforms.
func RaiseFDLimit() (soft, hard uint64, err error) {
	return 0, 0, nil
}
