// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package externalfunction

// These are the fixed choice lists the CREATE EXTERNAL FUNCTION grammar accepts.
// They live in the backend (rather than hardcoded in the React modal) so the SQL
// grammar and its UI options stay defined in one place; the create modal fetches
// them via App.GetExternalFunctionOptions and renders them as dropdowns.
var (
	// compressionOptions are the COMPRESSION values.
	compressionOptions = []string{"NONE", "AUTO", "GZIP", "DEFLATE"}

	// nullHandlingOptions are the NULL-input handling modifiers.
	nullHandlingOptions = []string{"CALLED ON NULL INPUT", "RETURNS NULL ON NULL INPUT", "STRICT"}

	// volatilityOptions are the volatility modifiers.
	volatilityOptions = []string{"VOLATILE", "IMMUTABLE"}

	// contextHeaderFunctions are the Snowflake context functions whose values can
	// be bound to HTTP headers via CONTEXT_HEADERS. Stored as the bare function
	// name (Snowflake's `CONTEXT_HEADERS = (current_timestamp)` form).
	contextHeaderFunctions = []string{
		"CURRENT_ACCOUNT", "CURRENT_CLIENT", "CURRENT_DATABASE", "CURRENT_DATE",
		"CURRENT_IP_ADDRESS", "CURRENT_REGION", "CURRENT_ROLE", "CURRENT_SCHEMA",
		"CURRENT_SCHEMAS", "CURRENT_SESSION", "CURRENT_STATEMENT", "CURRENT_TIME",
		"CURRENT_TIMESTAMP", "CURRENT_TRANSACTION", "CURRENT_USER", "CURRENT_VERSION",
		"CURRENT_WAREHOUSE", "LAST_QUERY_ID", "LAST_TRANSACTION", "LOCALTIME",
		"LOCALTIMESTAMP",
	}
)

// BuilderOptions bundles the static choice lists for the create-external-function
// UI so the frontend can populate its dropdowns without duplicating the SQL
// grammar's allowed values.
type BuilderOptions struct {
	Compression    []string `json:"compression"`
	NullHandling   []string `json:"nullHandling"`
	Volatility     []string `json:"volatility"`
	ContextHeaders []string `json:"contextHeaders"`
}

// GetBuilderOptions returns the fixed choice lists for the external function
// builder. Each slice is copied so callers can't mutate the package-level lists.
func GetBuilderOptions() BuilderOptions {
	clone := func(s []string) []string { return append([]string(nil), s...) }
	return BuilderOptions{
		Compression:    clone(compressionOptions),
		NullHandling:   clone(nullHandlingOptions),
		Volatility:     clone(volatilityOptions),
		ContextHeaders: clone(contextHeaderFunctions),
	}
}
