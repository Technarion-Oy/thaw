// SPDX-License-Identifier: GPL-3.0-or-later

package fileformat

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/valyala/fastjson"
)

// PreviewLocalFile reads the first 50 rows of a local file based on the given
// FileFormatConfig. It mimics Snowflake's basic parsing behavior for CSV and JSON.
// It returns an error if the format is unsupported or if parsing fails.
func PreviewLocalFile(path string, cfg FileFormatConfig) PreviewResult {
	t := strings.ToUpper(strings.TrimSpace(cfg.Type))
	if t == "" {
		t = "CSV"
	}

	// Thaw is a native desktop application. This file preview feature reads files from
	// the user's local filesystem on their own machine, usually selected via a native
	// file picker. It is not a server-side LFI vulnerability.
	cleanPath := filepath.Clean(path)

	switch t {
	case "CSV":
		return previewCSV(cleanPath, cfg)
	case "JSON":
		return previewJSON(cleanPath, cfg)
	default:
		return PreviewResult{Error: fmt.Sprintf("Local preview is only supported for CSV and JSON file formats. Selected type: %s", t)}
	}
}

func previewCSV(path string, cfg FileFormatConfig) PreviewResult {
	f, err := os.Open(path)
	if err != nil {
		return PreviewResult{Error: err.Error()}
	}
	defer func() { _ = f.Close() }()
	return previewCSVReader(f, cfg)
}

func previewCSVReader(r io.Reader, cfg FileFormatConfig) PreviewResult {
	// Custom CSV Parser to handle Snowflake's FIELD_OPTIONALLY_ENCLOSED_BY and ESCAPE rules.

	delim := ","
	if cfg.FieldDelimiter != "" {
		if cfg.FieldDelimiter == "\\t" {
			delim = "\t"
		} else {
			delim = cfg.FieldDelimiter
		}
	}
	
	recordDelim := "\n"
	if cfg.RecordDelimiter != "" && cfg.RecordDelimiter != "NONE" {
		switch cfg.RecordDelimiter {
		case "\\n":
			recordDelim = "\n"
		case "\\r\\n":
			recordDelim = "\r\n"
		default:
			recordDelim = cfg.RecordDelimiter
		}
	}

	enclosedBy := ""
	if cfg.FieldOptionallyEnclosedBy == "'" || cfg.FieldOptionallyEnclosedBy == "\"" {
		enclosedBy = cfg.FieldOptionallyEnclosedBy
	}

	escape := ""
	if cfg.Escape != "NONE" && cfg.Escape != "" {
		if cfg.Escape == "\\\\" {
			escape = "\\"
		} else {
			escape = cfg.Escape
		}
	}

	escapeUnenclosed := "\\"
	if cfg.EscapeUnenclosedField == "NONE" {
		escapeUnenclosed = ""
	} else if cfg.EscapeUnenclosedField != "" {
		if cfg.EscapeUnenclosedField == "\\\\" {
			escapeUnenclosed = "\\"
		} else {
			escapeUnenclosed = cfg.EscapeUnenclosedField
		}
	}

	// Read at most 1MB to prevent OOM (Denial of Service) on massive files
	lr := io.LimitReader(r, 1024*1024)
	b, err := io.ReadAll(lr)
	if err != nil && err != io.EOF {
		return PreviewResult{Error: err.Error()}
	}
	content := string(b)

	var records [][]string
	var currentRecord []string
	var currentField strings.Builder

	inQuotes := false
	inEscape := false
	i := 0
	
	// Helper to check prefix
	hasPrefix := func(idx int, prefix string) bool {
		if prefix == "" {
			return false
		}
		if idx+len(prefix) > len(content) {
			return false
		}
		return content[idx:idx+len(prefix)] == prefix
	}

	for i < len(content) {
		char := string(content[i])
		
		if inEscape {
			currentField.WriteString(char)
			inEscape = false
			i++
			continue
		}

		if inQuotes {
			if escape != "" && hasPrefix(i, escape) && !hasPrefix(i, enclosedBy+enclosedBy) {
				inEscape = true
				i += len(escape)
				continue
			}

			if hasPrefix(i, enclosedBy) {
				// Check for doubled quote (escaped quote)
				if hasPrefix(i+len(enclosedBy), enclosedBy) {
					currentField.WriteString(enclosedBy)
					i += len(enclosedBy) * 2
					continue
				}
				// End of quotes
				inQuotes = false
				i += len(enclosedBy)
				continue
			}

			currentField.WriteString(char)
			i++
			continue
		}

		// Not in quotes
		if escapeUnenclosed != "" && hasPrefix(i, escapeUnenclosed) {
			inEscape = true
			i += len(escapeUnenclosed)
			continue
		}

		if enclosedBy != "" && hasPrefix(i, enclosedBy) && currentField.Len() == 0 {
			inQuotes = true
			i += len(enclosedBy)
			continue
		}

		if hasPrefix(i, recordDelim) {
			currentRecord = append(currentRecord, currentField.String())
			currentField.Reset()
			records = append(records, currentRecord)
			currentRecord = nil
			i += len(recordDelim)
			continue
		}

		if hasPrefix(i, delim) {
			currentRecord = append(currentRecord, currentField.String())
			currentField.Reset()
			i += len(delim)
			continue
		}

		currentField.WriteString(char)
		i++
	}

	// Add the last field/record if there's no trailing record delimiter
	if currentField.Len() > 0 || len(currentRecord) > 0 {
		currentRecord = append(currentRecord, currentField.String())
		records = append(records, currentRecord)
	}

	// Now process the records
	skip := cfg.SkipHeader
	if skip > len(records) {
		skip = len(records)
	}
	records = records[skip:]

	var rows []map[string]string
	var columns []string
	limit := 50

	if cfg.ParseHeader && len(records) > 0 {
		record := records[0]
		records = records[1:]
		
		for j, val := range record {
			if cfg.TrimSpace {
				val = strings.TrimSpace(val)
			}
			if val == "" {
				columns = append(columns, fmt.Sprintf("COLUMN_%d", j+1))
			} else {
				columns = append(columns, val)
			}
		}
	}

	if limit > len(records) {
		limit = len(records)
	}

	for i := 0; i < limit; i++ {
		record := records[i]

		if len(columns) < len(record) {
			for j := len(columns); j < len(record); j++ {
				columns = append(columns, fmt.Sprintf("COLUMN_%d", j+1))
			}
		}

		row := make(map[string]string)
		for j, val := range record {
			if cfg.TrimSpace {
				val = strings.TrimSpace(val)
			}
			
			// Null handling
			isNull := false
			if cfg.EmptyFieldAsNull && val == "" {
				isNull = true
			}
			for _, n := range cfg.NullIf {
				if val == n || (n == "\\N" && val == "\\N") {
					isNull = true
					break
				}
			}

			if isNull {
				row[columns[j]] = ""
			} else {
				row[columns[j]] = val
			}
		}
		rows = append(rows, row)
	}

	if columns == nil {
		columns = []string{}
	}
	if rows == nil {
		rows = []map[string]string{}
	}
	return PreviewResult{Columns: columns, Rows: rows}
}

func previewJSON(path string, cfg FileFormatConfig) PreviewResult {
	// Read first 1MB
	f, err := os.Open(path)
	if err != nil {
		return PreviewResult{Error: err.Error()}
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 1024*1024)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return PreviewResult{Error: err.Error()}
	}
	buf = buf[:n]

	return previewJSONBytes(buf, cfg)
}

func previewJSONBytes(buf []byte, cfg FileFormatConfig) PreviewResult {
	// Try to parse as array first
	var p fastjson.Parser
	v, err := p.ParseBytes(buf)
	
	var objects []*fastjson.Value

	if err == nil {
		if v.Type() == fastjson.TypeArray {
			arr, _ := v.Array()
			objects = append(objects, arr...)
		} else {
			objects = append(objects, v)
		}
	} else {
		// Might be NDJSON
		lines := strings.Split(string(buf), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var p fastjson.Parser
			v, err := p.Parse(line)
			if err != nil {
				continue // Skip bad lines in NDJSON preview
			}
			objects = append(objects, v)
			if len(objects) >= 50 {
				break
			}
		}
	}

	if len(objects) == 0 {
		if err != nil {
			return PreviewResult{Error: "Failed to parse JSON: " + err.Error()}
		}
		return PreviewResult{Columns: []string{}, Rows: []map[string]string{}}
	}

	if len(objects) > 50 {
		objects = objects[:50]
	}

	var rows []map[string]string
	columnsSet := make(map[string]bool)
	var columns []string

	for _, obj := range objects {
		row := make(map[string]string)
		
		if obj.Type() == fastjson.TypeObject {
			o, _ := obj.Object()
			o.Visit(func(key []byte, v *fastjson.Value) {
				k := string(key)
				if !columnsSet[k] {
					columnsSet[k] = true
					columns = append(columns, k)
				}
				
				if v.Type() == fastjson.TypeNull {
					if !cfg.StripNullValues {
						row[k] = "null"
					}
				} else if v.Type() == fastjson.TypeString {
					b, _ := v.StringBytes()
					row[k] = string(b)
				} else {
					row[k] = v.String()
				}
			})
		} else {
			// If it's a primitive or array at the top level
			if !columnsSet["VALUE"] {
				columnsSet["VALUE"] = true
				columns = append(columns, "VALUE")
			}
			row["VALUE"] = obj.String()
		}
		
		rows = append(rows, row)
	}

	return PreviewResult{Columns: columns, Rows: rows}
}
