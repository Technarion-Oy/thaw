package fileformat

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
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

	switch t {
	case "CSV":
		return previewCSV(path, cfg)
	case "JSON":
		return previewJSON(path, cfg)
	default:
		return PreviewResult{Error: fmt.Sprintf("Local preview is only supported for CSV and JSON file formats. Selected type: %s", t)}
	}
}

func previewCSV(path string, cfg FileFormatConfig) PreviewResult {
	f, err := os.Open(path)
	if err != nil {
		return PreviewResult{Error: err.Error()}
	}
	defer f.Close()

	// Handle custom delimiters
	delim := ','
	if cfg.FieldDelimiter != "" {
		if cfg.FieldDelimiter == "\\t" {
			delim = '\t'
		} else {
			// Snowflake allows multi-character delimiters, but standard encoding/csv doesn't.
			// We take the first rune for best-effort local preview.
			for _, r := range cfg.FieldDelimiter {
				delim = r
				break
			}
		}
	} else if cfg.RecordDelimiter != "\\n" && cfg.RecordDelimiter != "" {
		// Just for safety if people mix things up, though Snowflake defaults field to , and record to \n
	}

	// encoding/csv requires a single rune.
	reader := csv.NewReader(f)
	reader.Comma = delim
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = cfg.TrimSpace
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	if cfg.FieldOptionallyEnclosedBy != "" && cfg.FieldOptionallyEnclosedBy != "NONE" {
		if cfg.FieldOptionallyEnclosedBy == "'" || cfg.FieldOptionallyEnclosedBy == "\"" {
			// Go's csv package assumes double quotes. We cannot easily switch it to single quotes
			// without writing a custom parser. We will rely on default behavior for now, which
			// handles double quotes.
		}
	}

	// Skip header rows
	skip := cfg.SkipHeader
	for i := 0; i < skip; i++ {
		_, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return PreviewResult{Columns: []string{}, Rows: []map[string]string{}}
			}
			return PreviewResult{Error: err.Error()}
		}
	}

	// Read data
	var rows []map[string]string
	var columns []string
	limit := 50
	if cfg.ParseHeader {
		limit = 51 // First row becomes header
	}

	for i := 0; i < limit; i++ {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			// Best effort: if we get a parsing error, we can stop or continue. Let's stop.
			return PreviewResult{Error: fmt.Sprintf("CSV parse error at line %d: %v", skip+i+1, err)}
		}

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
				// We represent nulls implicitly by omitting them or leaving empty depending on frontend.
				// The previous stage preview returned string map. If we omit the key or set to empty string?
				// Stage preview typically omits or returns literal "null" or empty. Let's omit or set empty.
				// We will omit the key if it's null, mimicking SQL results where missing keys are null in JSON,
				// or we set it to empty string. Let's just set it to empty string for CSV columns.
				row[columns[j]] = ""
			} else {
				row[columns[j]] = val
			}
		}
		rows = append(rows, row)
	}

	if cfg.ParseHeader && len(rows) > 0 {
		headerRow := rows[0]
		newCols := make([]string, len(columns))
		for j, col := range columns {
			newCols[j] = headerRow[col]
		}
		columns = newCols
		rows = rows[1:]

		// Rebuild rows to use new column names
		var newRows []map[string]string
		for _, oldRow := range rows {
			newRow := make(map[string]string)
			for j := range columns {
				// The data from oldRow was keyed by COLUMN_X. 
				// The old columns slice is now overwritten. We need the old keys.
				oldKey := fmt.Sprintf("COLUMN_%d", j+1)
				newRow[columns[j]] = oldRow[oldKey]
			}
			newRows = append(newRows, newRow)
		}
		rows = newRows
	}

	return PreviewResult{Columns: columns, Rows: rows}
}

func previewJSON(path string, cfg FileFormatConfig) PreviewResult {
	// Read first 1MB
	f, err := os.Open(path)
	if err != nil {
		return PreviewResult{Error: err.Error()}
	}
	defer f.Close()

	buf := make([]byte, 1024*1024)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return PreviewResult{Error: err.Error()}
	}
	buf = buf[:n]

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
