// SPDX-License-Identifier: GPL-3.0-or-later

package sysinfo

import (
	"fmt"
	"os/exec"
	"strings"
)

// MemoryGB returns the total physical RAM in gigabytes (rounded down).
// Returns 0 if the value cannot be determined (e.g. unsupported platform).
// Used by the frontend to suggest a sensible Ollama context-window size.
func MemoryGB() int {
	// macOS / Linux: sysctl -n hw.memsize (macOS) or hw.physmem (some BSDs)
	for _, key := range []string{"hw.memsize", "hw.physmem"} {
		out, err := exec.Command("sysctl", "-n", key).Output()
		if err != nil {
			continue
		}
		var bytes uint64
		if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &bytes); err != nil {
			continue
		}
		if bytes > 0 {
			return int(bytes / (1024 * 1024 * 1024))
		}
	}
	return 0
}
