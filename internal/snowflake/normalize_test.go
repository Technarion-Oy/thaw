// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

func TestNormalizePutGet(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "PUT with newlines gets collapsed and file path quoted",
			input: "PUT file:///tmp/data.csv\n@my_stage",
			want:  "PUT 'file:///tmp/data.csv' @my_stage",
		},
		{
			name:  "GET with newlines gets collapsed",
			input: "GET @my_stage\nfile:///tmp/out",
			want:  "GET @my_stage file:///tmp/out",
		},
		{
			name:  "non-PUT/GET statement is unchanged",
			input: "SELECT 1",
			want:  "SELECT 1",
		},
		{
			name:  "PUT with already-quoted path is unchanged",
			input: "PUT 'file:///tmp/data.csv' @my_stage",
			want:  "PUT 'file:///tmp/data.csv' @my_stage",
		},
		{
			name:  "PUT with CRLF newlines gets collapsed",
			input: "PUT file:///tmp/data.csv\r\n@my_stage",
			want:  "PUT 'file:///tmp/data.csv' @my_stage",
		},
		{
			name:  "GET with tab separator is recognized",
			input: "GET\t@my_stage file:///tmp/out",
			want:  "GET\t@my_stage file:///tmp/out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePutGet(tt.input)
			if got != tt.want {
				t.Errorf("normalizePutGet(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestQuotePutFilePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "unquoted file path gets quoted",
			input: "PUT file:///tmp/data.csv @my_stage",
			want:  "PUT 'file:///tmp/data.csv' @my_stage",
		},
		{
			name:  "already quoted path is unchanged",
			input: "PUT 'file:///tmp/data.csv' @my_stage",
			want:  "PUT 'file:///tmp/data.csv' @my_stage",
		},
		{
			name:  "no file:// in statement",
			input: "PUT @my_stage",
			want:  "PUT @my_stage",
		},
		{
			name:  "path with single quote gets escaped",
			input: "PUT file:///tmp/it's.csv @my_stage",
			want:  `PUT 'file:///tmp/it\'s.csv' @my_stage`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quotePutFilePath(tt.input)
			if got != tt.want {
				t.Errorf("quotePutFilePath(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}
