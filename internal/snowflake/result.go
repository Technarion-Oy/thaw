// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ColIdx returns the index of the first column whose lowercase name matches any
// of the given alternatives, or -1 if none match.
func ColIdx(cols []string, names ...string) int {
	for i, c := range cols {
		if slices.Contains(names, strings.ToLower(c)) {
			return i
		}
	}
	return -1
}

// Cell safely returns the string value of row[idx], or "" when idx is out of
// range. It folds the recurring `idx >= 0 && idx < len(row)` bounds-guard (paired
// with a ColIdx lookup that may return -1) and the CellString conversion into one
// call, so a SHOW/DESCRIBE row reader can write `Cell(row, ColIdx(cols, "key"))`
// without a separate guard — a missing column (ColIdx returns -1) safely yields "".
func Cell(row []any, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return CellString(row[idx])
}

// CellString converts a query-result cell value to a string. []byte is decoded
// as UTF-8, time.Time is formatted as RFC3339, nil becomes "", and all other
// types fall back to fmt.Sprintf("%v").
func CellString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case time.Time:
		return t.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// CellFloat converts a query-result cell value to a float64, returning 0 when
// the value is nil or cannot be parsed.
func CellFloat(v any) float64 {
	switch t := v.(type) {
	case nil:
		return 0
	case float64:
		return t
	case float32:
		return float64(t)
	case []byte:
		f, _ := strconv.ParseFloat(string(t), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		f, _ := strconv.ParseFloat(CellString(v), 64)
		return f
	}
}

// CellInt64 converts a query-result cell value to an int64. It accepts both
// integer and float string representations (e.g. "1234" or "1234.00") and
// returns 0 when the value is nil or cannot be parsed.
func CellInt64(v any) int64 {
	s := CellString(v)
	if s == "" {
		return 0
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f)
	}
	return 0
}

// CellBool interprets a query-result cell value as a boolean. It returns true
// for the common Snowflake truthy spellings: TRUE, YES, Y, ON, 1 (any case).
func CellBool(v any) bool {
	s := strings.ToUpper(strings.TrimSpace(CellString(v)))
	return s == "TRUE" || s == "YES" || s == "Y" || s == "ON" || s == "1"
}

// PropertyPair is a single key/value property row, used by SHOW/DESCRIBE
// projections (object properties, integration params, warehouse params, etc.).
type PropertyPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ResultToPairs projects the first row of a result as key/value pairs, one per
// column. It returns an empty (non-nil) slice when res is nil or has no rows.
func ResultToPairs(res *QueryResult) []PropertyPair {
	if res == nil || len(res.Rows) == 0 {
		return []PropertyPair{}
	}
	pairs := make([]PropertyPair, 0, len(res.Columns))
	row := res.Rows[0]
	for i, col := range res.Columns {
		val := ""
		if i < len(row) {
			val = CellString(row[i])
		}
		pairs = append(pairs, PropertyPair{Key: col, Value: val})
	}
	return pairs
}

// ResultPropertyValueRows projects a property/value-shaped result — one row per
// property with separate "property" and "value" columns, as DESCRIBE
// <object> returns for many object types — into key/value pairs (one per row).
// This is the multi-row counterpart of ResultToPairs (which projects a single
// row's columns). Column matching is case-insensitive; it returns an empty
// (non-nil) slice when res is nil, empty, or lacks the two columns.
func ResultPropertyValueRows(res *QueryResult) []PropertyPair {
	if res == nil || len(res.Rows) == 0 {
		return []PropertyPair{}
	}
	pi, vi := -1, -1
	for i, col := range res.Columns {
		switch strings.ToLower(col) {
		case "property":
			pi = i
		case "value":
			vi = i
		}
	}
	if pi < 0 || vi < 0 {
		return []PropertyPair{}
	}
	pairs := make([]PropertyPair, 0, len(res.Rows))
	for _, row := range res.Rows {
		if pi >= len(row) {
			continue
		}
		val := ""
		if vi < len(row) {
			val = CellString(row[vi])
		}
		pairs = append(pairs, PropertyPair{Key: CellString(row[pi]), Value: val})
	}
	return pairs
}
