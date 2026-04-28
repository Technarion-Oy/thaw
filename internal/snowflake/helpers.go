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

var (
	reScale = regexp.MustCompile(`\(.*\)$`)
	reNumeric = regexp.MustCompile(`^(NUMBER|INT|INTEGER|BIGINT|SMALLINT|TINYINT|BYTEINT|FLOAT|DOUBLE|DECIMAL|NUMERIC|REAL)$`)
)

// IsBoolean reports whether the given Snowflake data type is a boolean.
func IsBoolean(dataType string) bool {
	base := strings.ToUpper(strings.TrimSpace(reScale.ReplaceAllString(dataType, "")))
	return base == "BOOLEAN" || base == "BOOL"
}

// IsNumeric reports whether the given Snowflake data type is a numeric type.
func IsNumeric(dataType string) bool {
	base := strings.ToUpper(strings.TrimSpace(reScale.ReplaceAllString(dataType, "")))
	return reNumeric.MatchString(base)
}

// NeedsQuotes reports whether a value of the given data type should be quoted in SQL.
// Boolean and numeric values are typically not quoted.
func NeedsQuotes(dataType string) bool {
	return !IsBoolean(dataType) && !IsNumeric(dataType)
}
