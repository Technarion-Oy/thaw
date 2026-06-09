// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import "strings"

// Collation support in Snowflake is defined by a hyphen-separated specification
// string: an optional locale (which, when present, must come first) followed by
// zero or more specifiers in any order. The pseudo-locale "utf8" selects raw
// UTF-8 byte-order sorting.
//
// See https://docs.snowflake.com/en/sql-reference/collation#label-collation-specification
//
// This file is the single source of truth for the locales and specifiers Thaw
// surfaces in its UI. Collations returns a curated, ready-to-use list of common
// collation strings; CollationLocales and CollationSpecifiers expose the
// building blocks so callers can assemble arbitrary valid specifications.

// ── Types ─────────────────────────────────────────────────────────────────────

// CollationOption is a ready-to-display collation choice. The JSON field names
// match the antd Select option shape ({value, label}) so the frontend can bind
// the result directly to a dropdown without remapping.
type CollationOption struct {
	// Value is the collation specification string passed to COLLATE '<value>'.
	Value string `json:"value"`
	// Label is the human-readable description shown in the dropdown.
	Label string `json:"label"`
}

// CollationLocale is a single locale recognized by Snowflake collation.
type CollationLocale struct {
	// Code is the locale token used in a collation spec (e.g. "en", "en_US").
	Code string `json:"code"`
	// Name is the English display name (e.g. "English (United States)").
	Name string `json:"name"`
}

// CollationSpecifier is a single collation specifier (e.g. case- or
// accent-sensitivity) that may be appended to a collation specification.
type CollationSpecifier struct {
	// Code is the specifier token (e.g. "ci", "as", "trim").
	Code string `json:"code"`
	// Name is the human-readable description of the specifier's effect.
	Name string `json:"name"`
	// Category groups related specifiers (e.g. "Case", "Accent", "Punctuation").
	Category string `json:"category"`
}

// ── Source of truth ───────────────────────────────────────────────────────────

// collationLocales is the curated list of locales Thaw offers. Snowflake accepts
// many ISO/Unicode locale codes; this list covers the commonly used ones.
var collationLocales = []CollationLocale{
	{Code: "en", Name: "English"},
	{Code: "en_US", Name: "English (United States)"},
	{Code: "en_GB", Name: "English (United Kingdom)"},
	{Code: "de", Name: "German"},
	{Code: "fr", Name: "French"},
	{Code: "fr_CA", Name: "French (Canada)"},
	{Code: "es", Name: "Spanish"},
	{Code: "it", Name: "Italian"},
	{Code: "pt", Name: "Portuguese"},
	{Code: "pt_BR", Name: "Portuguese (Brazil)"},
	{Code: "nl", Name: "Dutch"},
	{Code: "sv", Name: "Swedish"},
	{Code: "da", Name: "Danish"},
	{Code: "nb", Name: "Norwegian Bokmål"},
	{Code: "fi", Name: "Finnish"},
	{Code: "pl", Name: "Polish"},
	{Code: "cs", Name: "Czech"},
	{Code: "hu", Name: "Hungarian"},
	{Code: "ro", Name: "Romanian"},
	{Code: "el", Name: "Greek"},
	{Code: "ru", Name: "Russian"},
	{Code: "uk", Name: "Ukrainian"},
	{Code: "tr", Name: "Turkish"},
	{Code: "he", Name: "Hebrew"},
	{Code: "ar", Name: "Arabic"},
	{Code: "th", Name: "Thai"},
	{Code: "vi", Name: "Vietnamese"},
	{Code: "hi", Name: "Hindi"},
	{Code: "ja", Name: "Japanese"},
	{Code: "ko", Name: "Korean"},
	{Code: "zh", Name: "Chinese"},
	{Code: "zh_TW", Name: "Chinese (Taiwan)"},
}

// collationSpecifiers is the complete set of specifiers Snowflake supports, per
// the collation specification reference.
var collationSpecifiers = []CollationSpecifier{
	{Code: "ci", Name: "Case-insensitive", Category: "Case sensitivity"},
	{Code: "cs", Name: "Case-sensitive (default)", Category: "Case sensitivity"},
	{Code: "ai", Name: "Accent-insensitive", Category: "Accent sensitivity"},
	{Code: "as", Name: "Accent-sensitive (default)", Category: "Accent sensitivity"},
	{Code: "pi", Name: "Punctuation-insensitive", Category: "Punctuation sensitivity"},
	{Code: "ps", Name: "Punctuation-sensitive (default)", Category: "Punctuation sensitivity"},
	{Code: "fl", Name: "Lower-case first", Category: "First-letter preference"},
	{Code: "fu", Name: "Upper-case first", Category: "First-letter preference"},
	{Code: "lower", Name: "Normalize to lower-case before comparison", Category: "Case conversion"},
	{Code: "upper", Name: "Normalize to upper-case before comparison", Category: "Case conversion"},
	{Code: "trim", Name: "Trim leading and trailing spaces", Category: "Space trimming"},
	{Code: "ltrim", Name: "Trim leading spaces", Category: "Space trimming"},
	{Code: "rtrim", Name: "Trim trailing spaces", Category: "Space trimming"},
}

// commonCollationVariants enumerates the specifier suffixes appended to each
// locale to form the curated Collations list. The bare locale (no suffix) is
// also emitted. Each entry's desc is appended to the locale name in the label.
var commonCollationVariants = []struct {
	suffix string // appended to the locale code (without leading hyphen), "" for bare
	desc   string // human description appended to the label
}{
	{suffix: "", desc: ""},
	{suffix: "ci", desc: "case-insensitive"},
	{suffix: "ci-ai", desc: "case- & accent-insensitive"},
	{suffix: "cs", desc: "case-sensitive"},
}

// ── Accessors ─────────────────────────────────────────────────────────────────

// Collations returns a curated list of ready-to-use collation specifications,
// suitable for populating a UI dropdown. It always begins with the empty
// "default" choice followed by the "utf8" pseudo-locale, then the bare and
// common case/accent variants of every locale in collationLocales.
func Collations() []CollationOption {
	opts := make([]CollationOption, 0, len(collationLocales)*len(commonCollationVariants)+1)
	opts = append(opts, CollationOption{Value: "utf8", Label: "utf8 — UTF-8 byte order"})
	for _, loc := range collationLocales {
		for _, v := range commonCollationVariants {
			value := loc.Code
			if v.suffix != "" {
				value = loc.Code + "-" + v.suffix
			}
			label := value + " — " + loc.Name
			if v.desc != "" {
				label += ", " + v.desc
			}
			opts = append(opts, CollationOption{Value: value, Label: label})
		}
	}
	return opts
}

// CollationLocales returns a copy of the supported collation locales.
func CollationLocales() []CollationLocale {
	result := make([]CollationLocale, len(collationLocales))
	copy(result, collationLocales)
	return result
}

// CollationSpecifiers returns a copy of the supported collation specifiers.
func CollationSpecifiers() []CollationSpecifier {
	result := make([]CollationSpecifier, len(collationSpecifiers))
	copy(result, collationSpecifiers)
	return result
}

// BuildCollation assembles a collation specification string from an optional
// locale and an ordered list of specifier codes, joining them with hyphens.
// The locale, when non-empty, is always placed first as Snowflake requires.
// Empty segments are skipped; the result is suitable for COLLATE '<result>'.
func BuildCollation(locale string, specifiers ...string) string {
	segments := make([]string, 0, len(specifiers)+1)
	if s := strings.TrimSpace(locale); s != "" {
		segments = append(segments, s)
	}
	for _, sp := range specifiers {
		if s := strings.TrimSpace(sp); s != "" {
			segments = append(segments, s)
		}
	}
	return strings.Join(segments, "-")
}
