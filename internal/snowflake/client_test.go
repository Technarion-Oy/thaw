// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

// TestIsTruthyFlag covers the SHOW-column flag spellings that mark a dropped
// row as an iceberg table. "Y"/"N" is what SHOW TABLES HISTORY emits today;
// "true"/"false" guards against version drift. Anything else is false.
func TestIsTruthyFlag(t *testing.T) {
	truthy := []string{"Y", "y", "YES", "true", "TRUE", "t", "1", " Y "}
	for _, s := range truthy {
		if !isTruthyFlag(s) {
			t.Errorf("isTruthyFlag(%q) = false, want true", s)
		}
	}
	falsy := []string{"N", "n", "NO", "false", "", "0", "<nil>", "iceberg"}
	for _, s := range falsy {
		if isTruthyFlag(s) {
			t.Errorf("isTruthyFlag(%q) = true, want false", s)
		}
	}
}
