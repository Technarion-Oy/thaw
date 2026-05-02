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
			cfg:   FileFormatConfig{ParseHeader: true, NullIf: []string{"N", "null"}}, // \N is unescaped to N by the default escape_unenclosed_field = \
			expected: PreviewResult{
				Columns: []string{"col1", "col2"},
				Rows: []map[string]string{
					{"col1": "val1", "col2": ""},
					{"col1": "", "col2": "val4"},
				},
			},
		},
		{
			name:  "Space delimiter with comma CSV",
			input: "a,b,c\n1,2,3\n4,5,6",
			cfg:   FileFormatConfig{FieldDelimiter: " "},
			expected: PreviewResult{
				Columns: []string{"COLUMN_1"},
				Rows: []map[string]string{
					{"COLUMN_1": "a,b,c"},
					{"COLUMN_1": "1,2,3"},
					{"COLUMN_1": "4,5,6"},
				},
			},
		},
		{
			name:  "Parse header with space delimiter, header only file",
			input: "col1 col2 col3",
			cfg:   FileFormatConfig{FieldDelimiter: " ", ParseHeader: true},
			expected: PreviewResult{
				Columns: []string{"col1", "col2", "col3"},
				Rows:    []map[string]string{},
			},
		},
		{
			name:  "Parse header with space delimiter and data rows",
			input: "name age\nAlice 30\nBob 25",
			cfg:   FileFormatConfig{FieldDelimiter: " ", ParseHeader: true},
			expected: PreviewResult{
				Columns: []string{"name", "age"},
				Rows: []map[string]string{
					{"name": "Alice", "age": "30"},
					{"name": "Bob", "age": "25"},
				},
			},
		},
		{
			name:  "Optionally enclosed by DOUBLE QUOTES",
			input: "col1,col2\nval1,\"val,2\"\nval3,\"val4\"",
			cfg:   FileFormatConfig{ParseHeader: true, FieldOptionallyEnclosedBy: "\""},
			expected: PreviewResult{
				Columns: []string{"col1", "col2"},
				Rows: []map[string]string{
					{"col1": "val1", "col2": "val,2"},
					{"col1": "val3", "col2": "val4"},
				},
			},
		},
		{
			name:  "Optionally enclosed by NONE ignores quotes",
			input: "col1,col2\nval1,\"val,2\"\nval3,\"val4\"",
			cfg:   FileFormatConfig{ParseHeader: true, FieldOptionallyEnclosedBy: "NONE"},
			// With NONE, quotes are treated as standard characters
			expected: PreviewResult{
				Columns: []string{"col1", "col2", "COLUMN_3"},
				Rows: []map[string]string{
					{"col1": "val1", "col2": "\"val", "COLUMN_3": "2\""},
					{"col1": "val3", "col2": "\"val4\""},
				},
			},
		},
		{
			name:  "User test preview without OptionallyEnclosedBy (default Snowflake)",
			input: "user_id,notes\n1,VIP customer\n5,\"Loves apples, oranges, and pears\"",
			cfg:   FileFormatConfig{ParseHeader: true},
			// Snowflake default optionally enclosed by is NONE, so the quotes are literal and 
			// the comma inside the quotes creates new columns.
			expected: PreviewResult{
				Columns: []string{"user_id", "notes", "COLUMN_3", "COLUMN_4"},
				Rows: []map[string]string{
					{"user_id": "1", "notes": "VIP customer"},
					{"user_id": "5", "notes": "\"Loves apples", "COLUMN_3": " oranges", "COLUMN_4": " and pears\""},
				},
			},
		},
		{
			name:  "User test preview WITH OptionallyEnclosedBy",
			input: "user_id,notes\n1,VIP customer\n5,\"Loves apples, oranges, and pears\"",
			cfg:   FileFormatConfig{ParseHeader: true, FieldOptionallyEnclosedBy: "\""},
			expected: PreviewResult{
				Columns: []string{"user_id", "notes"},
				Rows: []map[string]string{
					{"user_id": "1", "notes": "VIP customer"},
					{"user_id": "5", "notes": "Loves apples, oranges, and pears"},
				},
			},
		},
		{
			name:  "Custom Escape inside enclosed string",
			input: "col1,col2\nval1,\"he says \\\"hello\\\"\"",
			cfg:   FileFormatConfig{ParseHeader: true, FieldOptionallyEnclosedBy: "\"", Escape: "\\\\"},
			expected: PreviewResult{
				Columns: []string{"col1", "col2"},
				Rows: []map[string]string{
					{"col1": "val1", "col2": "he says \"hello\""},
				},
			},
		},
		{
			name:  "Custom Escape unenclosed field",
			input: "col1,col2\nval1,this is a \\, comma",
			cfg:   FileFormatConfig{ParseHeader: true, EscapeUnenclosedField: "\\\\"},
			expected: PreviewResult{
				Columns: []string{"col1", "col2"},
				Rows: []map[string]string{
					{"col1": "val1", "col2": "this is a , comma"},
				},
			},
		},
		{
			name:  "Custom Escape unenclosed field NONE",
			input: "col1,col2\nval1,this is a \\, comma",
			cfg:   FileFormatConfig{ParseHeader: true, EscapeUnenclosedField: "NONE"},
			expected: PreviewResult{
				Columns: []string{"col1", "col2", "COLUMN_3"},
				Rows: []map[string]string{
					{"col1": "val1", "col2": "this is a \\", "COLUMN_3": " comma"},
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
