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
		{
			name:  "PUT with newline separator is recognized",
			input: "PUT\nfile:///tmp/data.csv @my_stage",
			want:  "PUT 'file:///tmp/data.csv' @my_stage",
		},
		{
			name:  "line-comment-prefixed PUT is recognized and normalized",
			input: "-- upload data\nPUT file:///tmp/data.csv\n@my_stage",
			want:  "-- upload data\nPUT 'file:///tmp/data.csv' @my_stage",
		},
		{
			name:  "block-comment-prefixed GET is recognized",
			input: "/* fetch */ GET @my_stage\nfile:///tmp/out",
			want:  "/* fetch */ GET @my_stage file:///tmp/out",
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

func TestIsContextChangingQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "USE ROLE", input: "USE ROLE ANALYST", want: true},
		{name: "USE DATABASE", input: "USE DATABASE mydb", want: true},
		{name: "ALTER SESSION SET", input: "ALTER SESSION SET TIMEZONE = 'UTC'", want: true},
		{name: "comment-prefixed USE", input: "/* switch */ USE ROLE ANALYST", want: true},
		{name: "line-comment-prefixed ALTER SESSION", input: "-- tz\nALTER SESSION SET TIMEZONE = 'UTC'", want: true},
		{name: "lowercase use", input: "use warehouse wh", want: true},

		{name: "plain SELECT", input: "SELECT 1", want: false},
		{name: "empty", input: "", want: false},
		// False-positive guards: SESSION appearing in an identifier or literal
		// must not classify a non-session statement as context-changing.
		{name: "ALTER TABLE session_events", input: "ALTER TABLE session_events RENAME TO events", want: false},
		{name: "ALTER TASK comment mentioning session", input: "ALTER TASK t SET COMMENT = 'session cleanup'", want: false},
		// Word-boundary guard: identifiers starting with USE/ALTER must not match.
		{name: "SELECT from user_events (USE prefix)", input: "SELECT * FROM user_events", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isContextChangingQuery(tt.input); got != tt.want {
				t.Errorf("isContextChangingQuery(%q) = %v, want %v", tt.input, got, tt.want)
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
