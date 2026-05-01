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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	maxPreviewRows  = 50
	maxPreviewBytes = 1 << 20 // 1 MiB — prevent memory spikes for large files
)

// GetLocalFilePreview reads a local file and returns up to 50 rows shaped the
// same way Snowflake would present them after applying the file format options:
//   - CSV/TSV: field delimiter, skip-header rows, and parse-header are applied;
//     columns are named $1/$2/… unless PARSE_HEADER = true.
//   - JSON: JSON array or NDJSON; key names become column headers.
//   - Other types: returns an error directing the user to Stage Preview.
func GetLocalFilePreview(path string, cfg FileFormatConfig) PreviewResult {
	t := strings.ToUpper(strings.TrimSpace(cfg.Type))
	switch t {
	case "CSV":
		return readCSVPreview(path, cfg)
	case "JSON":
		return readJSONPreview(path)
	default:
		return PreviewResult{
			Columns: []string{},
			Rows:    []map[string]string{},
			Error: fmt.Sprintf(
				"Local preview is not supported for %s files. "+
					"Select Snowflake Stage as the source to preview %s files via Snowflake's compute engine.",
				t, t,
			),
		}
	}
}

// ── CSV ──────────────────────────────────────────────────────────────────────

func readCSVPreview(path string, cfg FileFormatConfig) PreviewResult {
	f, err := os.Open(path)
	if err != nil {
		return PreviewResult{Columns: []string{}, Rows: []map[string]string{}, Error: err.Error()}
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(io.LimitReader(f, maxPreviewBytes))
	r.LazyQuotes = true
	r.TrimLeadingSpace = cfg.TrimSpace
	// -1 disables field-count enforcement so rows with a different number of
	// columns than the first row don't cause csv.ErrFieldCount — which, when
	// returned on every data row, previously caused an infinite loop in the
	// error-continuation path.
	r.FieldsPerRecord = -1

	// Apply FIELD_DELIMITER. Go's csv.Reader always uses a single rune.
	if d := cfg.FieldDelimiter; d != "" && d != "," && strings.ToUpper(d) != "NONE" {
		if runes := []rune(d); len(runes) == 1 {
			r.Comma = runes[0]
		}
	}

	// SKIP_HEADER: discard the first N rows unconditionally.
	for i := 0; i < cfg.SkipHeader; i++ {
		if _, rdErr := r.Read(); rdErr != nil {
			break
		}
	}

	// PARSE_HEADER semantics mirror Snowflake:
	//   PARSE_HEADER = true  → first non-skipped row is the header; use its
	//                          values as column names.
	//   PARSE_HEADER = false → columns are $1, $2, … (Snowflake default).
	var headers []string
	var firstData []string // when PARSE_HEADER = false we buffer the first data row

	if cfg.ParseHeader {
		headers, err = r.Read()
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return PreviewResult{Columns: []string{}, Rows: []map[string]string{}}
			}
			return PreviewResult{Columns: []string{}, Rows: []map[string]string{}, Error: err.Error()}
		}
	} else {
		// Peek at the first data row to determine the column count.
		firstData, err = r.Read()
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return PreviewResult{Columns: []string{}, Rows: []map[string]string{}}
			}
			return PreviewResult{Columns: []string{}, Rows: []map[string]string{}, Error: err.Error()}
		}
		headers = make([]string, len(firstData))
		for i := range headers {
			headers[i] = fmt.Sprintf("$%d", i+1)
		}
	}

	rows := make([]map[string]string, 0, maxPreviewRows)

	// When PARSE_HEADER = false the first data row was consumed above; add it.
	if firstData != nil {
		rows = append(rows, recordToMap(headers, firstData))
	}

	// Read remaining rows. Break on EOF or after too many consecutive errors to
	// prevent hanging on pathological files.
	consecutiveErrors := 0
	for len(rows) < maxPreviewRows {
		record, rdErr := r.Read()
		if rdErr == io.EOF || rdErr == io.ErrUnexpectedEOF {
			break
		}
		if rdErr != nil {
			consecutiveErrors++
			if consecutiveErrors > 20 {
				break
			}
			continue
		}
		consecutiveErrors = 0
		rows = append(rows, recordToMap(headers, record))
	}

	return PreviewResult{Columns: headers, Rows: rows}
}

func recordToMap(headers, record []string) map[string]string {
	row := make(map[string]string, len(headers))
	for i, h := range headers {
		if i < len(record) {
			row[h] = record[i]
		} else {
			row[h] = ""
		}
	}
	return row
}

// ── JSON ─────────────────────────────────────────────────────────────────────

func readJSONPreview(path string) PreviewResult {
	f, err := os.Open(path)
	if err != nil {
		return PreviewResult{Columns: []string{}, Rows: []map[string]string{}, Error: err.Error()}
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, maxPreviewBytes))
	if err != nil {
		return PreviewResult{Columns: []string{}, Rows: []map[string]string{}, Error: err.Error()}
	}

	// Try a JSON array first.
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err == nil {
		return jsonRecordsToPreview(arr)
	}

	// Fall back to newline-delimited JSON (NDJSON).
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	records := make([]map[string]any, 0, maxPreviewRows)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) == nil {
			records = append(records, rec)
			if len(records) >= maxPreviewRows {
				break
			}
		}
	}
	if len(records) > 0 {
		return jsonRecordsToPreview(records)
	}

	return PreviewResult{Columns: []string{}, Rows: []map[string]string{}, Error: "Unable to parse file as a JSON array or NDJSON"}
}

func jsonRecordsToPreview(records []map[string]any) PreviewResult {
	if len(records) == 0 {
		return PreviewResult{Columns: []string{}, Rows: []map[string]string{}}
	}

	// Collect column names in first-seen order.
	seen := make(map[string]struct{})
	cols := []string{}
	for _, rec := range records {
		for k := range rec {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				cols = append(cols, k)
			}
		}
	}

	rows := make([]map[string]string, 0, len(records))
	for i, rec := range records {
		if i >= maxPreviewRows {
			break
		}
		row := make(map[string]string, len(cols))
		for _, col := range cols {
			if v, ok := rec[col]; ok {
				row[col] = fmt.Sprintf("%v", v)
			}
		}
		rows = append(rows, row)
	}

	return PreviewResult{Columns: cols, Rows: rows}
}
