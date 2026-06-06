// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

// Package ddl provides tools for parsing Snowflake DDL strings and
// extracting per-object metadata so that each statement can be written
// to an appropriately named file.  Statement splitting is provided by
// the sqlutil package.
package ddl

// isIdentRune reports whether r can appear inside a dollar-quote tag.
// Kept for tests and any callers that operate on rune values.
func isIdentRune(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '_'
}

// runesEqual reports whether two rune slices have identical contents.
// Kept for tests.
func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
