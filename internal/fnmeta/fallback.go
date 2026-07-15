// SPDX-License-Identifier: GPL-3.0-or-later

package fnmeta

import _ "embed"

// fallbackData holds the embedded JSON catalog of Snowflake built-in
// functions. It is bundled at compile time so the editor has instant
// completions even before a Snowflake connection is established.
//
//go:embed snowflake_builtin_fallback.json
var fallbackData []byte
