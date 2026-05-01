package fileformat

import (
	"reflect"
	"strings"
	"testing"
)

func TestPreviewCSVReader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		cfg      FileFormatConfig
		expected PreviewResult
	}{
		{
			name:  "Basic CSV",
			input: "a,b,c\n1,2,3\n4,5,6",
			cfg:   FileFormatConfig{},
			expected: PreviewResult{
				Columns: []string{"COLUMN_1", "COLUMN_2", "COLUMN_3"},
				Rows: []map[string]string{
					{"COLUMN_1": "a", "COLUMN_2": "b", "COLUMN_3": "c"},
					{"COLUMN_1": "1", "COLUMN_2": "2", "COLUMN_3": "3"},
					{"COLUMN_1": "4", "COLUMN_2": "5", "COLUMN_3": "6"},
				},
			},
		},
		{
			name:  "Parse Header",
			input: "col1,col2\nval1,val2\nval3,val4",
			cfg:   FileFormatConfig{ParseHeader: true},
			expected: PreviewResult{
				Columns: []string{"col1", "col2"},
				Rows: []map[string]string{
					{"col1": "val1", "col2": "val2"},
					{"col1": "val3", "col2": "val4"},
				},
			},
		},
		{
			name:  "Custom Delimiter and Trim Space",
			input: "col1 | col2 \n val1 | val2 ",
			cfg:   FileFormatConfig{FieldDelimiter: "|", TrimSpace: true},
			expected: PreviewResult{
				Columns: []string{"COLUMN_1", "COLUMN_2"},
				Rows: []map[string]string{
					{"COLUMN_1": "col1", "COLUMN_2": "col2"},
					{"COLUMN_1": "val1", "COLUMN_2": "val2"},
				},
			},
		},
		{
			name:  "Skip Header",
			input: "ignore,me\ncol1,col2\nval1,val2",
			cfg:   FileFormatConfig{SkipHeader: 1, ParseHeader: true},
			expected: PreviewResult{
				Columns: []string{"col1", "col2"},
				Rows: []map[string]string{
					{"col1": "val1", "col2": "val2"},
				},
			},
		},
		{
			name:  "Null Handling",
			input: "col1,col2\nval1,\\N\nnull,val4",
			cfg:   FileFormatConfig{ParseHeader: true, NullIf: []string{"\\N", "null"}},
			expected: PreviewResult{
				Columns: []string{"col1", "col2"},
				Rows: []map[string]string{
					{"col1": "val1", "col2": ""},
					{"col1": "", "col2": "val4"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := previewCSVReader(strings.NewReader(tc.input), tc.cfg)
			if res.Error != "" {
				t.Fatalf("unexpected error: %s", res.Error)
			}
			if !reflect.DeepEqual(res.Columns, tc.expected.Columns) {
				t.Errorf("Columns mismatch.\nExpected: %v\nGot:      %v", tc.expected.Columns, res.Columns)
			}
			if !reflect.DeepEqual(res.Rows, tc.expected.Rows) {
				t.Errorf("Rows mismatch.\nExpected: %v\nGot:      %v", tc.expected.Rows, res.Rows)
			}
		})
	}
}

func TestPreviewJSONBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		cfg      FileFormatConfig
		expected PreviewResult
	}{
		{
			name:  "JSON Array of Objects",
			input: `[{"id": 1, "name": "alice"}, {"id": 2, "name": "bob"}]`,
			cfg:   FileFormatConfig{},
			expected: PreviewResult{
				Columns: []string{"id", "name"},
				Rows: []map[string]string{
					{"id": "1", "name": "alice"},
					{"id": "2", "name": "bob"},
				},
			},
		},
		{
			name:  "NDJSON",
			input: `{"id": 1, "name": "alice"}
{"id": 2, "name": "bob"}
{"id": 3, "name": "charlie"}`,
			cfg:   FileFormatConfig{},
			expected: PreviewResult{
				Columns: []string{"id", "name"},
				Rows: []map[string]string{
					{"id": "1", "name": "alice"},
					{"id": "2", "name": "bob"},
					{"id": "3", "name": "charlie"},
				},
			},
		},
		{
			name:  "JSON Single Object",
			input: `{"id": 1, "status": "active"}`,
			cfg:   FileFormatConfig{},
			expected: PreviewResult{
				Columns: []string{"id", "status"},
				Rows: []map[string]string{
					{"id": "1", "status": "active"},
				},
			},
		},
		{
			name:  "Strip Null Values (false)",
			input: `[{"id": 1, "val": null}]`,
			cfg:   FileFormatConfig{StripNullValues: false},
			expected: PreviewResult{
				Columns: []string{"id", "val"},
				Rows: []map[string]string{
					{"id": "1", "val": "null"},
				},
			},
		},
		{
			name:  "Strip Null Values (true)",
			input: `[{"id": 1, "val": null}]`,
			cfg:   FileFormatConfig{StripNullValues: true},
			expected: PreviewResult{
				Columns: []string{"id", "val"},
				Rows: []map[string]string{
					{"id": "1"},
				},
			},
		},
		{
			name:  "Nested Object Stringification",
			input: `[{"id": 1, "data": {"nested": "value"}}]`,
			cfg:   FileFormatConfig{},
			expected: PreviewResult{
				Columns: []string{"id", "data"},
				Rows: []map[string]string{
					{"id": "1", "data": `{"nested":"value"}`},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := previewJSONBytes([]byte(tc.input), tc.cfg)
			if res.Error != "" {
				t.Fatalf("unexpected error: %s", res.Error)
			}
			if !reflect.DeepEqual(res.Columns, tc.expected.Columns) {
				t.Errorf("Columns mismatch.\nExpected: %v\nGot:      %v", tc.expected.Columns, res.Columns)
			}
			if !reflect.DeepEqual(res.Rows, tc.expected.Rows) {
				t.Errorf("Rows mismatch.\nExpected: %v\nGot:      %v", tc.expected.Rows, res.Rows)
			}
		})
	}
}
