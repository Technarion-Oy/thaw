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

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"thaw/internal/sqltok"
)

// reIdent matches a bare SQL identifier (letters, digits, underscore, dollar).
var reIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

// ── DataTypeKind ──────────────────────────────────────────────────────────────

// DataTypeKind identifies the parameter/validation family for a Snowflake type.
// It drives both autocompletion hints and the validation logic in ValidateDataType.
type DataTypeKind int

const (
	// KindNoParams — type accepts no parameters (e.g. INT, BOOLEAN, GEOGRAPHY).
	KindNoParams DataTypeKind = iota
	// KindPrecisionScale — optional (precision [, scale]) e.g. NUMBER, DECIMAL.
	KindPrecisionScale
	// KindLength — optional (length ≤ 16 777 216) e.g. VARCHAR, CHAR.
	KindLength
	// KindLengthBinary — optional (length ≤ 8 388 608) e.g. BINARY, VARBINARY.
	KindLengthBinary
	// KindFracSeconds — optional fractional-seconds scale 0–9 e.g. TIME, TIMESTAMP.
	KindFracSeconds
	// KindStructuredArray — bare ARRAY or typed ARRAY(<element_type>).
	KindStructuredArray
	// KindStructuredObject — bare OBJECT or typed OBJECT(<name> <type>, …).
	KindStructuredObject
	// KindMap — MAP(<key_type>, <value_type>); parameters are required.
	KindMap
	// KindVector — VECTOR(<element_type>, <dimension>); parameters are required.
	KindVector
)

// ── DataTypeCategory ──────────────────────────────────────────────────────────

// DataTypeCategory is the broad semantic family a Snowflake type belongs to.
// It is the single source of truth for category-driven logic such as
// IsNumeric (helpers.go) and the JOIN-compatibility buckets in sqleditor.
type DataTypeCategory string

const (
	// CategoryNumeric — exact and approximate numeric types (INT, NUMBER, FLOAT…).
	CategoryNumeric DataTypeCategory = "numeric"
	// CategoryString — character/text types (VARCHAR, CHAR, STRING, TEXT…).
	CategoryString DataTypeCategory = "string"
	// CategoryBinary — BINARY and VARBINARY.
	CategoryBinary DataTypeCategory = "binary"
	// CategoryBoolean — BOOLEAN.
	CategoryBoolean DataTypeCategory = "boolean"
	// CategoryDatetime — DATE/TIME/TIMESTAMP family.
	CategoryDatetime DataTypeCategory = "datetime"
	// CategorySemiStructured — VARIANT, OBJECT, ARRAY.
	CategorySemiStructured DataTypeCategory = "semi_structured"
	// CategoryStructured — MAP (and other strongly-typed structured types).
	CategoryStructured DataTypeCategory = "structured"
	// CategoryGeospatial — GEOGRAPHY, GEOMETRY.
	CategoryGeospatial DataTypeCategory = "geospatial"
	// CategoryVector — VECTOR.
	CategoryVector DataTypeCategory = "vector"
	// CategoryFile — FILE (references to unstructured-data files).
	CategoryFile DataTypeCategory = "file"
)

// ── DataTypeInfo ──────────────────────────────────────────────────────────────

// DataTypeInfo describes a single Snowflake data type.
// It is the authoritative record used by both AllDataTypes and ValidateDataType.
type DataTypeInfo struct {
	// Name is the canonical upper-case type keyword (e.g. "VARCHAR", "TIMESTAMP_LTZ").
	Name string
	// Kind determines which parameter syntax and constraints apply.
	Kind DataTypeKind
	// Category is the broad semantic family used by category-driven consumers.
	Category DataTypeCategory
	// ParamHint is a human-readable parameter synopsis shown in autocompletion
	// (e.g. "(precision, scale)").  Empty for types that take no parameters.
	ParamHint string
}

// snowflakeDataTypes is the single source of truth for every Snowflake type
// recognized by this package.  AllDataTypes returns a copy; ValidateDataType
// dispatches via dataTypeMap which is built from this slice at init time.
var snowflakeDataTypes = []DataTypeInfo{
	// ── Numeric — exact ──────────────────────────────────────────────────
	{Name: "NUMBER", Kind: KindPrecisionScale, Category: CategoryNumeric, ParamHint: "(precision, scale)"},
	{Name: "DECIMAL", Kind: KindPrecisionScale, Category: CategoryNumeric, ParamHint: "(precision, scale)"},
	{Name: "NUMERIC", Kind: KindPrecisionScale, Category: CategoryNumeric, ParamHint: "(precision, scale)"},
	{Name: "INT", Kind: KindNoParams, Category: CategoryNumeric},
	{Name: "INTEGER", Kind: KindNoParams, Category: CategoryNumeric},
	{Name: "BIGINT", Kind: KindNoParams, Category: CategoryNumeric},
	{Name: "SMALLINT", Kind: KindNoParams, Category: CategoryNumeric},
	{Name: "TINYINT", Kind: KindNoParams, Category: CategoryNumeric},
	{Name: "BYTEINT", Kind: KindNoParams, Category: CategoryNumeric},
	// ── Numeric — approximate ────────────────────────────────────────────
	{Name: "FLOAT", Kind: KindNoParams, Category: CategoryNumeric},
	{Name: "FLOAT4", Kind: KindNoParams, Category: CategoryNumeric},
	{Name: "FLOAT8", Kind: KindNoParams, Category: CategoryNumeric},
	{Name: "DOUBLE", Kind: KindNoParams, Category: CategoryNumeric},
	{Name: "DOUBLE PRECISION", Kind: KindNoParams, Category: CategoryNumeric},
	{Name: "REAL", Kind: KindNoParams, Category: CategoryNumeric},
	// ── String ───────────────────────────────────────────────────────────
	{Name: "VARCHAR", Kind: KindLength, Category: CategoryString, ParamHint: "(length)"},
	{Name: "CHAR", Kind: KindLength, Category: CategoryString, ParamHint: "(length)"},
	{Name: "CHARACTER", Kind: KindLength, Category: CategoryString, ParamHint: "(length)"},
	{Name: "STRING", Kind: KindNoParams, Category: CategoryString},
	{Name: "TEXT", Kind: KindNoParams, Category: CategoryString},
	// ── Binary ───────────────────────────────────────────────────────────
	{Name: "BINARY", Kind: KindLengthBinary, Category: CategoryBinary, ParamHint: "(length)"},
	{Name: "VARBINARY", Kind: KindLengthBinary, Category: CategoryBinary, ParamHint: "(length)"},
	// ── Logical ──────────────────────────────────────────────────────────
	{Name: "BOOLEAN", Kind: KindNoParams, Category: CategoryBoolean},
	// ── Date & Time ──────────────────────────────────────────────────────
	{Name: "DATE", Kind: KindNoParams, Category: CategoryDatetime},
	{Name: "DATETIME", Kind: KindNoParams, Category: CategoryDatetime},
	{Name: "TIME", Kind: KindFracSeconds, Category: CategoryDatetime, ParamHint: "(scale)"},
	{Name: "TIMESTAMP", Kind: KindFracSeconds, Category: CategoryDatetime, ParamHint: "(scale)"},
	{Name: "TIMESTAMP_LTZ", Kind: KindFracSeconds, Category: CategoryDatetime, ParamHint: "(scale)"},
	{Name: "TIMESTAMP_NTZ", Kind: KindFracSeconds, Category: CategoryDatetime, ParamHint: "(scale)"},
	{Name: "TIMESTAMP_TZ", Kind: KindFracSeconds, Category: CategoryDatetime, ParamHint: "(scale)"},
	// No-underscore TIMESTAMP synonyms (documented official forms).
	{Name: "TIMESTAMPLTZ", Kind: KindFracSeconds, Category: CategoryDatetime, ParamHint: "(scale)"},
	{Name: "TIMESTAMPNTZ", Kind: KindFracSeconds, Category: CategoryDatetime, ParamHint: "(scale)"},
	{Name: "TIMESTAMPTZ", Kind: KindFracSeconds, Category: CategoryDatetime, ParamHint: "(scale)"},
	// ── Semi-structured ──────────────────────────────────────────────────
	{Name: "VARIANT", Kind: KindNoParams, Category: CategorySemiStructured},
	{Name: "OBJECT", Kind: KindStructuredObject, Category: CategorySemiStructured, ParamHint: "(name type, ...)"},
	{Name: "ARRAY", Kind: KindStructuredArray, Category: CategorySemiStructured, ParamHint: "(element_type)"},
	// ── Structured ───────────────────────────────────────────────────────
	{Name: "MAP", Kind: KindMap, Category: CategoryStructured, ParamHint: "(key_type, value_type)"},
	// ── Geospatial ───────────────────────────────────────────────────────
	{Name: "GEOGRAPHY", Kind: KindNoParams, Category: CategoryGeospatial},
	{Name: "GEOMETRY", Kind: KindNoParams, Category: CategoryGeospatial},
	// ── Vector ───────────────────────────────────────────────────────────
	{Name: "VECTOR", Kind: KindVector, Category: CategoryVector, ParamHint: "(element_type, dimension)"},
	// ── File ─────────────────────────────────────────────────────────────
	{Name: "FILE", Kind: KindNoParams, Category: CategoryFile},
}

// dataTypeMap is a fast lookup by canonical upper-case name, built once at init.
var dataTypeMap map[string]DataTypeInfo

func init() {
	dataTypeMap = make(map[string]DataTypeInfo, len(snowflakeDataTypes))
	names := make([]string, 0, len(snowflakeDataTypes))
	for _, dt := range snowflakeDataTypes {
		dataTypeMap[dt.Name] = dt
		names = append(names, dt.Name)
	}
	// Inject the authoritative type names into the lower-level tokenizer's
	// keyword set so it classifies data types as keywords without duplicating
	// the list.  snowflake → sqltok is the only allowed direction (sqltok is a
	// leaf), so registration via init keeps datatypes.go the single source of
	// truth.  Go guarantees this init runs before any snowflake-importing
	// package's code, so consumers observe the registered names.
	sqltok.RegisterDataTypeKeywords(names)
}

// AllDataTypes returns a copy of the complete list of supported Snowflake data
// types.  Callers may use it to populate autocompletion lists; the Name and
// ParamHint fields are intended to be displayed directly in editor UI.
func AllDataTypes() []DataTypeInfo {
	result := make([]DataTypeInfo, len(snowflakeDataTypes))
	copy(result, snowflakeDataTypes)
	return result
}

// BaseType reduces a Snowflake data-type string to its bare, upper-cased base
// type name, dropping any parameter list and normalizing the TIMESTAMPTZ
// synonym to TIMESTAMP_TZ:
//
//	"VARCHAR(256)"        → "VARCHAR"
//	"NUMBER(38,0)"        → "NUMBER"
//	"TIMESTAMP_TZ(9)"     → "TIMESTAMP_TZ"
//	"VECTOR(FLOAT, 256)"  → "VECTOR"
//	"timestamptz"         → "TIMESTAMP_TZ"
//
// It is a lenient, best-effort classifier (it never errors and ignores trailing
// tokens) intended for type-family checks such as index-eligibility filters; use
// ValidateDataType when strict validation/normalization is required.
func BaseType(dataType string) string {
	base, _, _ := strings.Cut(strings.TrimSpace(dataType), "(")
	base = strings.ToUpper(strings.TrimSpace(base))
	if base == "TIMESTAMPTZ" {
		return "TIMESTAMP_TZ"
	}
	return base
}

// ── DataTypeError ─────────────────────────────────────────────────────────────

// DataTypeError is returned by ValidateDataType when the input is not a
// recognized or syntactically valid Snowflake data type.
type DataTypeError struct {
	Input   string
	Message string
}

func (e *DataTypeError) Error() string {
	return fmt.Sprintf("invalid Snowflake data type %q: %s", e.Input, e.Message)
}

// ── ValidateDataType ──────────────────────────────────────────────────────────

// ValidateDataType checks whether s is a syntactically valid Snowflake data
// type string and returns a normalised upper-case representation on success.
// Validation rules (parameter ranges, required vs optional params, etc.) are
// driven by the Kind field in snowflakeDataTypes so the type registry and the
// validator stay in sync automatically.
func ValidateDataType(s string) (string, error) {
	norm := strings.TrimSpace(s)
	if norm == "" {
		return "", &DataTypeError{Input: s, Message: "empty string"}
	}

	baseName, params, err := splitBaseAndParams(norm)
	if err != nil {
		return "", &DataTypeError{Input: s, Message: err.Error()}
	}
	upperBase := strings.ToUpper(baseName)

	info, ok := dataTypeMap[upperBase]
	if !ok {
		return "", &DataTypeError{Input: s, Message: "unrecognized data type"}
	}

	switch info.Kind {

	case KindNoParams:
		if params != "" {
			return "", &DataTypeError{Input: s, Message: upperBase + " takes no parameters"}
		}
		return upperBase, nil

	case KindPrecisionScale:
		if params == "" {
			return upperBase, nil
		}
		p, sc, err := parseTwoOptionalInts(params)
		if err != nil {
			return "", &DataTypeError{Input: s, Message: err.Error()}
		}
		if p < 1 || p > 38 {
			return "", &DataTypeError{Input: s, Message: "precision must be 1–38"}
		}
		if sc < 0 || sc > p {
			return "", &DataTypeError{Input: s, Message: "scale must be 0–precision"}
		}
		return fmt.Sprintf("%s(%d, %d)", upperBase, p, sc), nil

	case KindLength:
		if params == "" {
			return upperBase, nil
		}
		n, err := parseSingleInt(params)
		if err != nil {
			return "", &DataTypeError{Input: s, Message: err.Error()}
		}
		if n < 1 || n > 16_777_216 {
			return "", &DataTypeError{Input: s, Message: "length must be 1–16777216"}
		}
		return fmt.Sprintf("%s(%d)", upperBase, n), nil

	case KindLengthBinary:
		if params == "" {
			return upperBase, nil
		}
		n, err := parseSingleInt(params)
		if err != nil {
			return "", &DataTypeError{Input: s, Message: err.Error()}
		}
		if n < 1 || n > 8_388_608 {
			return "", &DataTypeError{Input: s, Message: "length must be 1–8388608"}
		}
		return fmt.Sprintf("%s(%d)", upperBase, n), nil

	case KindFracSeconds:
		if params == "" {
			return upperBase, nil
		}
		sc, err := parseSingleInt(params)
		if err != nil {
			return "", &DataTypeError{Input: s, Message: err.Error()}
		}
		if sc < 0 || sc > 9 {
			return "", &DataTypeError{Input: s, Message: "fractional seconds precision must be 0–9"}
		}
		return fmt.Sprintf("%s(%d)", upperBase, sc), nil

	case KindStructuredArray:
		if params == "" {
			return upperBase, nil // bare ARRAY → semi-structured
		}
		elemNorm, err := ValidateDataType(params)
		if err != nil {
			return "", &DataTypeError{Input: s, Message: "invalid element type: " + err.Error()}
		}
		return fmt.Sprintf("ARRAY(%s)", elemNorm), nil

	case KindStructuredObject:
		if params == "" {
			return upperBase, nil // bare OBJECT → semi-structured
		}
		if err := validateObjectFields(params); err != nil {
			return "", &DataTypeError{Input: s, Message: err.Error()}
		}
		return fmt.Sprintf("OBJECT(%s)", normaliseObjectFields(params)), nil

	case KindMap:
		if params == "" {
			return "", &DataTypeError{Input: s, Message: "MAP requires (key_type, value_type)"}
		}
		kNorm, vNorm, err := validateMapParams(params)
		if err != nil {
			return "", &DataTypeError{Input: s, Message: err.Error()}
		}
		return fmt.Sprintf("MAP(%s, %s)", kNorm, vNorm), nil

	case KindVector:
		if params == "" {
			return "", &DataTypeError{Input: s, Message: "VECTOR requires (element_type, dimension)"}
		}
		elemNorm, dim, err := validateVectorParams(params)
		if err != nil {
			return "", &DataTypeError{Input: s, Message: err.Error()}
		}
		return fmt.Sprintf("VECTOR(%s, %d)", elemNorm, dim), nil
	}

	return "", &DataTypeError{Input: s, Message: "unrecognized data type"}
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// splitBaseAndParams separates the base type name from the parenthesised
// parameter list.  It handles multi-word base names such as "DOUBLE PRECISION"
// and balanced nested parentheses inside the parameter list.
//
// Examples:
//
//	"VARCHAR(255)"          → "VARCHAR", "255", nil
//	"OBJECT(name VARCHAR)"  → "OBJECT", "name VARCHAR", nil
//	"DOUBLE PRECISION"      → "DOUBLE PRECISION", "", nil
func splitBaseAndParams(s string) (base, params string, err error) {
	before, after, hasParen := strings.Cut(s, "(")
	if !hasParen {
		upper := strings.ToUpper(strings.TrimSpace(s))
		if upper == "DOUBLE PRECISION" {
			return "DOUBLE PRECISION", "", nil
		}
		if len(strings.Fields(s)) != 1 {
			return "", "", fmt.Errorf("unexpected tokens after type name")
		}
		return strings.TrimSpace(s), "", nil
	}

	base = strings.TrimSpace(before)
	rest := after

	// Find the matching closing parenthesis (depth-aware).
	depth := 1
	end := -1
	for i, ch := range rest {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		return "", "", fmt.Errorf("unmatched opening parenthesis")
	}
	if strings.TrimSpace(rest[end+1:]) != "" {
		return "", "", fmt.Errorf("unexpected tokens after closing parenthesis")
	}

	params = strings.TrimSpace(rest[:end])
	return base, params, nil
}

// parseSingleInt parses a parameter list that must contain exactly one integer.
func parseSingleInt(params string) (int, error) {
	v := strings.TrimSpace(params)
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("expected a single integer, got %q", v)
	}
	return n, nil
}

// parseTwoOptionalInts parses "precision" or "precision, scale".
func parseTwoOptionalInts(params string) (precision, scale int, err error) {
	parts := strings.SplitN(params, ",", 2)
	p, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid precision %q", parts[0])
	}
	if len(parts) == 1 {
		return p, 0, nil
	}
	sc, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid scale %q", parts[1])
	}
	return p, sc, nil
}

// splitTopLevelCommas splits s on commas that are NOT inside nested
// parentheses, returning the trimmed segments.
func splitTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, ch := range s {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}

// validateObjectFields checks that params is a comma-separated list of
// "<identifier> <type>" pairs.
func validateObjectFields(params string) error {
	fields := splitTopLevelCommas(params)
	if len(fields) == 0 {
		return fmt.Errorf("OBJECT requires at least one field definition")
	}
	for _, f := range fields {
		parts := strings.SplitN(f, " ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("expected \"<name> <type>\" in OBJECT field, got %q", f)
		}
		name := strings.TrimSpace(parts[0])
		if !reIdent.MatchString(name) {
			return fmt.Errorf("invalid field name %q in OBJECT", name)
		}
		if _, err := ValidateDataType(strings.TrimSpace(parts[1])); err != nil {
			return fmt.Errorf("invalid type for field %q: %w", name, err)
		}
	}
	return nil
}

// normaliseObjectFields returns a normalised, upper-cased representation of
// the OBJECT field list.
func normaliseObjectFields(params string) string {
	fields := splitTopLevelCommas(params)
	norm := make([]string, 0, len(fields))
	for _, f := range fields {
		parts := strings.SplitN(f, " ", 2)
		name := strings.ToUpper(strings.TrimSpace(parts[0]))
		typNorm, _ := ValidateDataType(strings.TrimSpace(parts[1]))
		norm = append(norm, name+" "+typNorm)
	}
	return strings.Join(norm, ", ")
}

// validateMapParams validates MAP(key_type, value_type) and returns the
// normalised type strings.
func validateMapParams(params string) (keyNorm, valNorm string, err error) {
	parts := splitTopLevelCommas(params)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("MAP requires exactly two type arguments (key_type, value_type)")
	}
	kNorm, err := ValidateDataType(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("invalid MAP key type: %w", err)
	}
	vNorm, err := ValidateDataType(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid MAP value type: %w", err)
	}
	return kNorm, vNorm, nil
}

// validateVectorParams validates VECTOR(element_type, dimension) and returns
// the normalised element type string and dimension integer.
func validateVectorParams(params string) (elemNorm string, dim int, err error) {
	parts := splitTopLevelCommas(params)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("VECTOR requires exactly two arguments (element_type, dimension)")
	}
	elemNorm, err = ValidateDataType(parts[0])
	if err != nil {
		return "", 0, fmt.Errorf("invalid VECTOR element type: %w", err)
	}
	dim, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || dim < 1 {
		return "", 0, fmt.Errorf("VECTOR dimension must be a positive integer")
	}
	return elemNorm, dim, nil
}
