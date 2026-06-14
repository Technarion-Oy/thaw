// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package fileformat

import (
	"fmt"
	"strings"
	"thaw/internal/snowflake"
)

// FileFormatConfig holds all parameters for a Snowflake FILE FORMAT object.
// Fields use their Snowflake default values as zero-values where possible;
// the SQL builder only emits a parameter when it differs from the Snowflake
// default, keeping the generated DDL concise.
type FileFormatConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Type          string `json:"type"` // CSV | JSON | AVRO | ORC | PARQUET | XML
	Comment       string `json:"comment"`

	// ── Common ──────────────────────────────────────────────────────────────
	// Compression applies to CSV, JSON, AVRO, PARQUET, XML. Default: AUTO.
	Compression    string `json:"compression"`
	TrimSpace      bool   `json:"trimSpace"`
	ReplaceInvalid bool   `json:"replaceInvalid"` // REPLACE_INVALID_CHARACTERS
	FileExtension  string `json:"fileExtension"`

	// ── CSV ─────────────────────────────────────────────────────────────────
	RecordDelimiter            string   `json:"recordDelimiter"`            // default: \n
	FieldDelimiter             string   `json:"fieldDelimiter"`             // default: ,
	MultiLine                  bool     `json:"multiLine"`                  // default: false
	ParseHeader                bool     `json:"parseHeader"`                // default: false
	SkipHeader                 int      `json:"skipHeader"`                 // default: 0
	SkipBlankLines             bool     `json:"skipBlankLines"`             // default: false
	DateFormat                 string   `json:"dateFormat"`                 // AUTO or pattern
	TimeFormat                 string   `json:"timeFormat"`                 // AUTO or pattern
	TimestampFormat            string   `json:"timestampFormat"`            // AUTO or pattern
	BinaryFormat               string   `json:"binaryFormat"`               // HEX | BASE64 | UTF8; default: HEX
	Escape                     string   `json:"escape"`                     // NONE or char; default: NONE
	EscapeUnenclosedField      string   `json:"escapeUnenclosedField"`      // NONE or char; default: \\
	FieldOptionallyEnclosedBy  string   `json:"fieldOptionallyEnclosedBy"`  // NONE | ' | "; default: NONE
	NullIf                     []string `json:"nullIf"`                     // default: (\N) for CSV
	ErrorOnColumnCountMismatch bool     `json:"errorOnColumnCountMismatch"` // default: true
	EmptyFieldAsNull           bool     `json:"emptyFieldAsNull"`           // default: true
	SkipByteOrderMark          bool     `json:"skipByteOrderMark"`          // default: true (CSV/JSON/XML)
	Encoding                   string   `json:"encoding"`                   // default: UTF8

	// ── JSON ────────────────────────────────────────────────────────────────
	EnableOctal      bool `json:"enableOctal"`
	AllowDuplicate   bool `json:"allowDuplicate"`
	StripOuterArray  bool `json:"stripOuterArray"`
	StripNullValues  bool `json:"stripNullValues"`
	IgnoreUTF8Errors bool `json:"ignoreUTF8Errors"`

	// ── XML ─────────────────────────────────────────────────────────────────
	PreserveSpace        bool `json:"preserveSpace"`
	StripOuterElement    bool `json:"stripOuterElement"`
	DisableSnowflakeData bool `json:"disableSnowflakeData"`
	DisableAutoConvert   bool `json:"disableAutoConvert"`

	// ── Parquet ─────────────────────────────────────────────────────────────
	BinaryAsText           bool `json:"binaryAsText"`           // default: true
	UseLogicalType         bool `json:"useLogicalType"`         // default: false
	SnappyCompression      bool `json:"snappyCompression"`      // default: false
	SnappyCompressionLevel int  `json:"snappyCompressionLevel"` // 0 = not set
	UseVectorizedScanner   bool `json:"useVectorizedScanner"`   // default: false
}

// PreviewResult holds a data preview (up to 50 rows) from a local or staged file.
type PreviewResult struct {
	Columns []string            `json:"columns"`
	Rows    []map[string]string `json:"rows"`
	Error   string              `json:"error"`
}

// ── SQL helpers ─────────────────────────────────────────────────────────────

// boolParam emits "  NAME = TRUE/FALSE" only when val differs from def.
func boolParam(sb *strings.Builder, name string, val, def bool) {
	if val == def {
		return
	}
	if val {
		fmt.Fprintf(sb, "\n  %s = TRUE", name)
	} else {
		fmt.Fprintf(sb, "\n  %s = FALSE", name)
	}
}

// identParam emits "  NAME = VAL" (no quotes) only when val differs from def
// and is non-empty. Used for COMPRESSION, BINARY_FORMAT, ENCODING, etc.
// To prevent SQL injection, val must match a strict allowlist.
func identParam(sb *strings.Builder, name, val, def string) {
	if val == "" || strings.EqualFold(val, def) {
		return
	}

	valUpper := strings.ToUpper(val)

	// Strict allowlist for unquoted parameters to prevent SQL injection
	allowed := false
	switch valUpper {
	case "AUTO", "GZIP", "BZ2", "BROTLI", "ZSTD", "DEFLATE", "RAW_DEFLATE", "NONE", // COMPRESSION
		"LZO", "SNAPPY", // PARQUET COMPRESSION
		"HEX", "BASE64", "UTF8": // BINARY_FORMAT & ENCODING
		allowed = true
	}

	if allowed {
		fmt.Fprintf(sb, "\n  %s = %s", name, valUpper)
	}
}

// noneOrStrParam handles parameters that accept either NONE (keyword) or a
// quoted string literal. Used for ESCAPE and FIELD_OPTIONALLY_ENCLOSED_BY.
func noneOrStrParam(sb *strings.Builder, name, val, def string) {
	if val == "" || strings.EqualFold(val, def) {
		return
	}
	if strings.EqualFold(val, "NONE") {
		fmt.Fprintf(sb, "\n  %s = NONE", name)
	} else {
		fmt.Fprintf(sb, "\n  %s = '%s'", name, snowflake.EscapeStringLit(val))
	}
}

// dateTimeParam handles DATE_FORMAT / TIME_FORMAT / TIMESTAMP_FORMAT which can
// be the unquoted keyword AUTO or a quoted format pattern string.
func dateTimeParam(sb *strings.Builder, name, val, def string) {
	if val == "" || strings.EqualFold(val, def) {
		return
	}
	if strings.EqualFold(val, "AUTO") {
		fmt.Fprintf(sb, "\n  %s = AUTO", name)
	} else {
		fmt.Fprintf(sb, "\n  %s = '%s'", name, snowflake.EscapeStringLit(val))
	}
}

// nullIfParam emits NULL_IF = ('val1', 'val2', ...).
func nullIfParam(sb *strings.Builder, vals []string) {
	if len(vals) == 0 {
		return
	}
	quoted := make([]string, len(vals))
	for i, v := range vals {
		quoted[i] = "'" + snowflake.EscapeStringLit(v) + "'"
	}
	fmt.Fprintf(sb, "\n  NULL_IF = (%s)", strings.Join(quoted, ", "))
}

// ── SQL builder ──────────────────────────────────────────────────────────────

// BuildCreateFileFormatSql generates a CREATE FILE FORMAT SQL statement from cfg,
// emitting only the parameters that differ from Snowflake's defaults.
func BuildCreateFileFormatSql(db, schema string, cfg FileFormatConfig) string {
	var sb strings.Builder

	clause := "CREATE"
	if cfg.OrReplace {
		clause += " OR REPLACE"
	}
	clause += " FILE FORMAT"
	if cfg.IfNotExists && !cfg.OrReplace {
		clause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if strings.TrimSpace(cfg.Name) == "" {
		nameToken = "format_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", clause,
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	t := strings.ToUpper(strings.TrimSpace(cfg.Type))
	switch t {
	case "CSV", "JSON", "AVRO", "ORC", "PARQUET", "XML":
		// Valid type
	case "":
		t = "CSV"
	default:
		// Fallback to a safe default if somehow bypassed
		t = "CSV"
	}
	fmt.Fprintf(&sb, "\n  TYPE = %s", t)

	emitFormatParams(&sb, t, cfg)

	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", snowflake.EscapeStringLit(cfg.Comment))
	}

	sb.WriteString(";")
	return sb.String()
}

// BuildCreateTemporaryFileFormatSql generates a CREATE OR REPLACE TEMPORARY FILE_FORMAT
// statement with the given name.
func BuildCreateTemporaryFileFormatSql(name string, cfg FileFormatConfig) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "CREATE OR REPLACE TEMPORARY FILE_FORMAT %s", snowflake.QuoteIdent(name))

	t := strings.ToUpper(strings.TrimSpace(cfg.Type))
	switch t {
	case "CSV", "JSON", "AVRO", "ORC", "PARQUET", "XML":
		// Valid type
	case "":
		t = "CSV"
	default:
		// Fallback to a safe default if somehow bypassed
		t = "CSV"
	}
	fmt.Fprintf(&sb, "\n  TYPE = %s", t)

	emitFormatParams(&sb, t, cfg)

	sb.WriteString(";")
	return sb.String()
}

// ── Per-type parameter emitters ──────────────────────────────────────────────

func emitFormatParams(sb *strings.Builder, t string, cfg FileFormatConfig) {
	switch t {
	case "CSV":
		emitCSVParams(sb, cfg)
	case "JSON":
		emitJSONParams(sb, cfg)
	case "AVRO":
		emitAVROParams(sb, cfg)
	case "ORC":
		emitORCParams(sb, cfg)
	case "PARQUET":
		emitParquetParams(sb, cfg)
	case "XML":
		emitXMLParams(sb, cfg)
	}
}

func emitCSVParams(sb *strings.Builder, cfg FileFormatConfig) {
	identParam(sb, "COMPRESSION", cfg.Compression, "AUTO")
	noneOrStrParam(sb, "RECORD_DELIMITER", cfg.RecordDelimiter, "\n")
	noneOrStrParam(sb, "FIELD_DELIMITER", cfg.FieldDelimiter, ",")
	boolParam(sb, "MULTI_LINE", cfg.MultiLine, false)
	if cfg.FileExtension != "" {
		fmt.Fprintf(sb, "\n  FILE_EXTENSION = '%s'", snowflake.EscapeStringLit(cfg.FileExtension))
	}
	boolParam(sb, "PARSE_HEADER", cfg.ParseHeader, false)
	if cfg.SkipHeader > 0 {
		fmt.Fprintf(sb, "\n  SKIP_HEADER = %d", cfg.SkipHeader)
	}
	boolParam(sb, "SKIP_BLANK_LINES", cfg.SkipBlankLines, false)
	dateTimeParam(sb, "DATE_FORMAT", cfg.DateFormat, "AUTO")
	dateTimeParam(sb, "TIME_FORMAT", cfg.TimeFormat, "AUTO")
	dateTimeParam(sb, "TIMESTAMP_FORMAT", cfg.TimestampFormat, "AUTO")
	identParam(sb, "BINARY_FORMAT", cfg.BinaryFormat, "HEX")
	noneOrStrParam(sb, "ESCAPE", cfg.Escape, "NONE")
	noneOrStrParam(sb, "ESCAPE_UNENCLOSED_FIELD", cfg.EscapeUnenclosedField, "\\")
	boolParam(sb, "TRIM_SPACE", cfg.TrimSpace, false)
	noneOrStrParam(sb, "FIELD_OPTIONALLY_ENCLOSED_BY", cfg.FieldOptionallyEnclosedBy, "NONE")
	nullIfParam(sb, cfg.NullIf)
	boolParam(sb, "ERROR_ON_COLUMN_COUNT_MISMATCH", cfg.ErrorOnColumnCountMismatch, true)
	boolParam(sb, "REPLACE_INVALID_CHARACTERS", cfg.ReplaceInvalid, false)
	boolParam(sb, "EMPTY_FIELD_AS_NULL", cfg.EmptyFieldAsNull, true)
	boolParam(sb, "SKIP_BYTE_ORDER_MARK", cfg.SkipByteOrderMark, true)
	identParam(sb, "ENCODING", cfg.Encoding, "UTF8")
}

func emitJSONParams(sb *strings.Builder, cfg FileFormatConfig) {
	identParam(sb, "COMPRESSION", cfg.Compression, "AUTO")
	if cfg.FileExtension != "" {
		fmt.Fprintf(sb, "\n  FILE_EXTENSION = '%s'", snowflake.EscapeStringLit(cfg.FileExtension))
	}
	dateTimeParam(sb, "DATE_FORMAT", cfg.DateFormat, "AUTO")
	dateTimeParam(sb, "TIME_FORMAT", cfg.TimeFormat, "AUTO")
	dateTimeParam(sb, "TIMESTAMP_FORMAT", cfg.TimestampFormat, "AUTO")
	identParam(sb, "BINARY_FORMAT", cfg.BinaryFormat, "HEX")
	boolParam(sb, "TRIM_SPACE", cfg.TrimSpace, false)
	boolParam(sb, "MULTI_LINE", cfg.MultiLine, false)
	nullIfParam(sb, cfg.NullIf)
	boolParam(sb, "ENABLE_OCTAL", cfg.EnableOctal, false)
	boolParam(sb, "ALLOW_DUPLICATE", cfg.AllowDuplicate, false)
	boolParam(sb, "STRIP_OUTER_ARRAY", cfg.StripOuterArray, false)
	boolParam(sb, "STRIP_NULL_VALUES", cfg.StripNullValues, false)
	boolParam(sb, "REPLACE_INVALID_CHARACTERS", cfg.ReplaceInvalid, false)
	boolParam(sb, "IGNORE_UTF8_ERRORS", cfg.IgnoreUTF8Errors, false)
	boolParam(sb, "SKIP_BYTE_ORDER_MARK", cfg.SkipByteOrderMark, true)
}

func emitAVROParams(sb *strings.Builder, cfg FileFormatConfig) {
	identParam(sb, "COMPRESSION", cfg.Compression, "AUTO")
	boolParam(sb, "TRIM_SPACE", cfg.TrimSpace, false)
	boolParam(sb, "REPLACE_INVALID_CHARACTERS", cfg.ReplaceInvalid, false)
	nullIfParam(sb, cfg.NullIf)
}

func emitORCParams(sb *strings.Builder, cfg FileFormatConfig) {
	boolParam(sb, "TRIM_SPACE", cfg.TrimSpace, false)
	boolParam(sb, "REPLACE_INVALID_CHARACTERS", cfg.ReplaceInvalid, false)
	nullIfParam(sb, cfg.NullIf)
}

func emitParquetParams(sb *strings.Builder, cfg FileFormatConfig) {
	identParam(sb, "COMPRESSION", cfg.Compression, "AUTO")
	boolParam(sb, "SNAPPY_COMPRESSION", cfg.SnappyCompression, false)
	boolParam(sb, "BINARY_AS_TEXT", cfg.BinaryAsText, true)
	boolParam(sb, "USE_LOGICAL_TYPE", cfg.UseLogicalType, false)
	boolParam(sb, "TRIM_SPACE", cfg.TrimSpace, false)
	boolParam(sb, "USE_VECTORIZED_SCANNER", cfg.UseVectorizedScanner, false)
	if cfg.SnappyCompressionLevel > 0 {
		fmt.Fprintf(sb, "\n  SNAPPY_COMPRESSION_LEVEL = %d", cfg.SnappyCompressionLevel)
	}
	boolParam(sb, "REPLACE_INVALID_CHARACTERS", cfg.ReplaceInvalid, false)
	nullIfParam(sb, cfg.NullIf)
}

func emitXMLParams(sb *strings.Builder, cfg FileFormatConfig) {
	identParam(sb, "COMPRESSION", cfg.Compression, "AUTO")
	boolParam(sb, "IGNORE_UTF8_ERRORS", cfg.IgnoreUTF8Errors, false)
	boolParam(sb, "PRESERVE_SPACE", cfg.PreserveSpace, false)
	boolParam(sb, "STRIP_OUTER_ELEMENT", cfg.StripOuterElement, false)
	boolParam(sb, "DISABLE_SNOWFLAKE_DATA", cfg.DisableSnowflakeData, false)
	boolParam(sb, "DISABLE_AUTO_CONVERT", cfg.DisableAutoConvert, false)
	boolParam(sb, "REPLACE_INVALID_CHARACTERS", cfg.ReplaceInvalid, false)
	boolParam(sb, "SKIP_BYTE_ORDER_MARK", cfg.SkipByteOrderMark, true)
}

// ── Inline FILE_FORMAT clause builder ────────────────────────────────────────

// BuildInlineFileFormat returns a Snowflake inline FILE_FORMAT clause suitable
// for use inside a stage query:
//
//	SELECT * FROM @stage/path (FILE_FORMAT => (<inline>)) LIMIT 50
func BuildInlineFileFormat(cfg FileFormatConfig) string {
	t := strings.ToUpper(strings.TrimSpace(cfg.Type))
	switch t {
	case "CSV", "JSON", "AVRO", "ORC", "PARQUET", "XML":
		// Valid type
	case "":
		t = "CSV"
	default:
		// Fallback to a safe default if somehow bypassed
		t = "CSV"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "TYPE = %s", t)

	// Reuse per-type emitters on a blank builder, then strip the leading newline+spaces.
	var paramSb strings.Builder
	emitFormatParams(&paramSb, t, cfg)

	if paramSb.Len() > 0 {
		// Each line begins with "\n  "; replace with ", " to produce an inline clause.
		params := strings.TrimPrefix(paramSb.String(), "\n  ")
		params = strings.ReplaceAll(params, "\n  ", ", ")
		fmt.Fprintf(&sb, ", %s", params)
	}

	return sb.String()
}
