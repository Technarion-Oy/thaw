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

// GetLocalFilePreview reads a local file and returns up to 50 rows.
// CSV and JSON (array or NDJSON) are supported. All other format types
// return a PreviewResult with an Error message directing the user to use
// Stage Preview instead.
func GetLocalFilePreview(path string, cfg FileFormatConfig) PreviewResult {
	t := strings.ToUpper(strings.TrimSpace(cfg.Type))
	switch t {
	case "CSV":
		return readCSVPreview(path, cfg)
	case "JSON":
		return readJSONPreview(path)
	default:
		return PreviewResult{
			Error: fmt.Sprintf(
				"Local preview is not supported for %s files. "+
					"Select a Snowflake Stage source to preview %s files via Snowflake's compute engine.",
				t, t,
			),
		}
	}
}

// ── CSV ──────────────────────────────────────────────────────────────────────

func readCSVPreview(path string, cfg FileFormatConfig) PreviewResult {
	f, err := os.Open(path)
	if err != nil {
		return PreviewResult{Error: err.Error()}
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(io.LimitReader(f, maxPreviewBytes))
	r.LazyQuotes = true
	r.TrimLeadingSpace = cfg.TrimSpace

	if cfg.FieldDelimiter != "" && cfg.FieldDelimiter != "," {
		runes := []rune(cfg.FieldDelimiter)
		if len(runes) > 0 {
			r.Comma = runes[0]
		}
	}

	// Skip header rows (SKIP_HEADER).
	for i := 0; i < cfg.SkipHeader; i++ {
		if _, err := r.Read(); err != nil {
			break
		}
	}

	// First non-skipped row is treated as column headers.
	headers, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return PreviewResult{Columns: []string{}, Rows: []map[string]string{}}
		}
		return PreviewResult{Error: err.Error()}
	}

	rows := make([]map[string]string, 0, maxPreviewRows)
	for len(rows) < maxPreviewRows {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed rows rather than aborting the preview.
			continue
		}
		row := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(record) {
				row[h] = record[i]
			}
		}
		rows = append(rows, row)
	}

	return PreviewResult{Columns: headers, Rows: rows}
}

// ── JSON ─────────────────────────────────────────────────────────────────────

func readJSONPreview(path string) PreviewResult {
	f, err := os.Open(path)
	if err != nil {
		return PreviewResult{Error: err.Error()}
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, maxPreviewBytes))
	if err != nil {
		return PreviewResult{Error: err.Error()}
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

	return PreviewResult{Error: "Unable to parse file as a JSON array or NDJSON"}
}

func jsonRecordsToPreview(records []map[string]any) PreviewResult {
	if len(records) == 0 {
		return PreviewResult{Columns: []string{}, Rows: []map[string]string{}}
	}

	// Collect column names in insertion order (first-seen wins).
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
