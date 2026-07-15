// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "strings"

// IsPrivilegeError reports whether err reads as a Snowflake access-control
// failure (as opposed to a transient/network one). Callers use it to decide
// when an optional supplemental query may silently degrade instead of failing
// the request (e.g. the DESCRIBE USER merge in internal/objects). Keep the
// phrase list here — the single shared matcher — rather than re-deriving it
// at call sites.
func IsPrivilegeError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "insufficient privileges") ||
		strings.Contains(s, "not authorized") ||
		strings.Contains(s, "access control")
}
