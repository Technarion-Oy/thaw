// Copyright (c) 2026 Technarion Oy. All rights reserved.

package snowflake

import "testing"

func TestValidateStageFilePath(t *testing.T) {
	valid := []string{
		"branches/main/models/foo.sql", "a_b-c.1/d.sql", "a/..b/c.sql",
	}
	// Space terminates an unquoted stage path, so anything with whitespace (or a
	// quote / separator / non-ASCII) must be rejected — a space alone is enough to
	// graft a trailing clause onto the statement.
	invalid := []string{
		"readme.sql UNION SELECT password FROM users--",
		"foo.sql; DROP TABLE t", "foo'.sql", `foo".sql`, "foo`.sql",
		`foo\bar.sql`, "foo\nbar.sql", "file with space.sql", "notes (draft).sql",
		"café.sql", "v1.0+hotfix.sql", "../../etc/x.sql", "a/../b.sql", "",
	}

	for _, p := range valid {
		if err := validateStageFilePath(p); err != nil {
			t.Errorf("expected %q to be valid, got %v", p, err)
		}
	}
	for _, p := range invalid {
		if err := validateStageFilePath(p); err == nil {
			t.Errorf("expected %q to be rejected", p)
		}
	}
}
