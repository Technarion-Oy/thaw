// Copyright (c) 2026 Technarion Oy. All rights reserved.

package snowflake

import "testing"

func TestGitFilePathRe(t *testing.T) {
	valid := []string{"branches/main/models/foo.sql", "a_b-c.1/d.sql", "file with space.sql"}
	invalid := []string{"foo.sql; DROP TABLE t", "foo'.sql", "foo/*x*/.sql", "foo`.sql", ""}

	for _, p := range valid {
		if !gitFilePathRe.MatchString(p) {
			t.Errorf("expected %q to be valid", p)
		}
	}
	for _, p := range invalid {
		if gitFilePathRe.MatchString(p) {
			t.Errorf("expected %q to be rejected", p)
		}
	}
}
