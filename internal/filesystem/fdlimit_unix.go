// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package filesystem

import (
	"fmt"
	"syscall"
)

// RaiseFDLimit raises this process's file-descriptor soft limit (RLIMIT_NOFILE)
// toward the hard limit and returns the resulting soft and hard limits. It is a
// mitigation for FD-hungry workloads: the recursive FS watcher needs far fewer
// descriptors than the old per-directory backend, but network drives, large
// tooling, and Linux/inotify can still push against a low soft limit.
//
// Behaviour:
//   - If the soft limit already meets the hard limit, it is a no-op.
//   - It first tries to set the soft limit equal to the hard limit.
//   - macOS refuses a soft limit of RLIM_INFINITY (or above kern.maxfilesperproc),
//     so on failure it retries with a bounded, widely-accepted value before
//     reporting an error.
//
// Raising the limit is idempotent and safe to call more than once.
func RaiseFDLimit() (soft, hard uint64, err error) {
	var lim syscall.Rlimit
	if e := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); e != nil {
		return 0, 0, fmt.Errorf("getrlimit RLIMIT_NOFILE: %w", e)
	}

	orig := uint64(lim.Cur)
	hardLimit := uint64(lim.Max)
	if orig >= hardLimit {
		return orig, hardLimit, nil // already at the hard limit; nothing to do
	}

	lim.Cur = lim.Max
	if e := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim); e != nil {
		// macOS rejects Cur == RLIM_INFINITY / above the per-process file cap.
		// Retry with a bounded value that comfortably covers a large tree.
		const fallback = 24576
		if hardLimit > fallback {
			lim.Cur = fallback
			if e2 := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim); e2 == nil {
				return fallback, hardLimit, nil
			}
		}
		return orig, hardLimit, fmt.Errorf("setrlimit RLIMIT_NOFILE: %w", e)
	}
	return hardLimit, hardLimit, nil
}
