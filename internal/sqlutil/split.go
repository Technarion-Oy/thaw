// SPDX-License-Identifier: GPL-3.0-or-later

package sqlutil

import "thaw/internal/sqltok"

// Deprecated: Split is superseded by [thaw/internal/sqltok.Split], which
// uses a single-pass tokenizer instead of a hand-rolled state machine.
// All new callers should use sqltok.Split directly. This wrapper exists
// only for backward compatibility during migration; it delegates to the
// new implementation and will be removed once all callers have migrated.
func Split(src string) []string {
	return sqltok.Split(src)
}
