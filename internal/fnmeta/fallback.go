// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package fnmeta

import _ "embed"

// fallbackData holds the embedded JSON catalogue of Snowflake built-in
// functions. It is bundled at compile time so the editor has instant
// completions even before a Snowflake connection is established.
//
//go:embed snowflake_builtin_fallback.json
var fallbackData []byte
