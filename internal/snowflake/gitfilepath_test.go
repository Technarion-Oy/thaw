// Copyright (c) 2026 Technarion Oy. All rights reserved.

package snowflake

import "testing"

func TestValidateStageFilePath(t *testing.T) {
	valid := []string{"branches/main/models/foo.sql", "a_b-c.1/d.sql", "file with space.sql"}
	invalid := []string{"foo.sql; DROP TABLE t", "foo'.sql", "foo/*x*/.sql", "foo`.sql", ""}

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
