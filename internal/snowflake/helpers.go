// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package snowflake

import (
	"regexp"
	"strings"
)

var reScale = regexp.MustCompile(`\(.*\)$`)

// numericTypeNames is the set of canonical numeric type names, derived from the
// authoritative registry (CategoryNumeric) so adding a numeric type to
// snowflakeDataTypes automatically makes it numeric here too.
var numericTypeNames = func() map[string]struct{} {
	m := make(map[string]struct{})
	for _, dt := range snowflakeDataTypes {
		if dt.Category == CategoryNumeric {
			m[dt.Name] = struct{}{}
		}
	}
	return m
}()

// IsBoolean reports whether the given Snowflake data type is a boolean.
func IsBoolean(dataType string) bool {
	base := strings.ToUpper(strings.TrimSpace(reScale.ReplaceAllString(dataType, "")))
	return base == "BOOLEAN" || base == "BOOL"
}

// IsNumeric reports whether the given Snowflake data type is a numeric type.
func IsNumeric(dataType string) bool {
	base := strings.ToUpper(strings.TrimSpace(reScale.ReplaceAllString(dataType, "")))
	_, ok := numericTypeNames[base]
	return ok
}

// NeedsQuotes reports whether a value of the given data type should be quoted in SQL.
// Boolean and numeric values are typically not quoted.
func NeedsQuotes(dataType string) bool {
	return !IsBoolean(dataType) && !IsNumeric(dataType)
}
