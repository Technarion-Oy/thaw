// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

func TestValidateStageRef(t *testing.T) {
	valid := []string{
		`@DB.SCHEMA.STAGE`,
		`@"db"."schema"."stage"/data/2026`,
		`@stage/sub-dir_1/file.csv`,       // single dashes are fine
		`@"my;stage"/a`,                   // ';' inside a quoted identifier is legal
		`@"we''ird"/a`,                    // escaped quote inside a quoted identifier
		`@"da--sh"/a`,                     // '--' inside a quoted identifier is legal
		`@~/branch/models/foo.sql`,        // user stage
		`@"a/../b"."schema"."stage"/path`, // '..' inside a quoted identifier is not traversal
	}
	for _, s := range valid {
		if err := ValidateStageRef(s); err != nil {
			t.Errorf("ValidateStageRef(%q) = %v, want nil", s, err)
		}
	}

	// The reported injection vectors and their building blocks must be rejected
	// when they appear in the unquoted path segment.
	injections := []string{
		`@db.schema.stage/x; DROP TABLE foo; --`,
		`@db.schema.stage/x' OR '1'='1`,
		"@db.schema.stage/x\nSELECT 1",
		`@db.schema.stage/data--`, // '--' would comment out trailing option clauses
		`@"unbalanced/x`,          // dangling quote
		// A quote in the path segment must not grant amnesty to the payload it wraps.
		`@"db"."schema"."stage"/data/x"; DROP TABLE t; --"y`,
		// A bare space lets a crafted name graft a second query in the SELECT sink.
		`@DB.SCHEMA.REPO/readme.sql UNION SELECT password FROM users`,
		"@DB.SCHEMA.REPO/foo\tbar.sql",
		// A '.' in the path segment must not open a spurious quoted identifier that
		// swallows the payload — quotes are only identifier delimiters in the prefix.
		`@db.schema.stage/x."; DROP TABLE foo; --"y`,
		`@"db"."schema"."stage"/data/x."; DROP TABLE t; --"y`,
		// '..' traversal segments.
		`@db.schema.stage/../../etc/passwd`,
		`@db.schema.stage/foo/../../secrets.txt`,
	}
	for _, s := range injections {
		if err := ValidateStageRef(s); err == nil {
			t.Errorf("ValidateStageRef(%q) = nil, want error", s)
		}
	}
}

// TestNormalizeStageFileRef covers the PUT/GET relaxation: whitespace in the path
// segment is legal there because the whole reference can be quoted, but nothing
// else about the rules changes — and a reference without whitespace must still
// come back byte-identical, so existing file-transfer SQL is unaffected.
func TestNormalizeStageFileRef(t *testing.T) {
	cases := []struct {
		in   string
		want string // "" means an error is expected
	}{
		// Unchanged when no whitespace is involved.
		{`@DB.SCHEMA.STAGE`, `@DB.SCHEMA.STAGE`},
		{`@DB.SCHEMA.STAGE/pages`, `@DB.SCHEMA.STAGE/pages`},
		{`DB.SCHEMA.STAGE/pages`, `@DB.SCHEMA.STAGE/pages`}, // '@' is added
		// Spaces in the path segment are quoted rather than rejected — the case
		// that used to fail an entire deploy at the first PUT into the folder.
		{`@DB.SCHEMA.STAGE/static files`, `'@DB.SCHEMA.STAGE/static files'`},
		{`@DB.SCHEMA.STAGE/my app/sub dir`, `'@DB.SCHEMA.STAGE/my app/sub dir'`},
		{"@DB.SCHEMA.STAGE/tab\tdir", "'@DB.SCHEMA.STAGE/tab\tdir'"},
		// Whitespace in the identifier prefix is still illegal unquoted, and still
		// legal inside a quoted identifier.
		{`@DB.SCH EMA.STAGE/x`, ``},
		{`@"my stage"/data`, `@"my stage"/data`},
		// Every other rule is untouched. A single-quote in particular must stay
		// rejected — it is what makes wrapping the reference in quotes safe.
		{`@DB.SCHEMA.STAGE/x' OR '1'='1`, ``},
		{`@DB.SCHEMA.STAGE/dir with space'; DROP TABLE t; --`, ``},
		{`@DB.SCHEMA.STAGE/a b; DROP TABLE t`, ``},
		{`@DB.SCHEMA.STAGE/a b--c`, ``},
		{`@DB.SCHEMA.STAGE/a b/../../secrets.txt`, ``},
		{"@DB.SCHEMA.STAGE/a b\nSELECT 1", ``},
		{`@DB.SCHEMA.STAGE/a b/x"; DROP TABLE t; --"y`, ``},
	}
	for _, tc := range cases {
		got, err := NormalizeStageFileRef(tc.in)
		if tc.want == "" {
			if err == nil {
				t.Errorf("NormalizeStageFileRef(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeStageFileRef(%q) = %v, want %q", tc.in, err, tc.want)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizeStageFileRef(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
