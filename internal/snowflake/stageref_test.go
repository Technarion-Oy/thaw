// Copyright (c) 2026 Technarion Oy. All rights reserved.

package snowflake

import "testing"

func TestValidateStageRef(t *testing.T) {
	valid := []string{
		`@DB.SCHEMA.STAGE`,
		`@"db"."schema"."stage"/data/2026`,
		`@stage/sub-dir_1/file.csv`, // single dashes are fine
		`@"my;stage"/a`,             // ';' inside a quoted identifier is legal
		`@"we''ird"/a`,              // escaped quote inside a quoted identifier
		`@"da--sh"/a`,               // '--' inside a quoted identifier is legal
		`@~/branch/models/foo.sql`,  // user stage
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
	}
	for _, s := range injections {
		if err := ValidateStageRef(s); err == nil {
			t.Errorf("ValidateStageRef(%q) = nil, want error", s)
		}
	}
}
