// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import (
	"strings"
	"testing"
)

func TestBuildUsageDependencyQuery(t *testing.T) {
	t.Run("depends_on filters on REFERENCING and selects REFERENCED", func(t *testing.T) {
		q, err := buildUsageDependencyQuery("DB", "SC", "OBJ", DependsOn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, want := range []string{
			"SELECT REFERENCED_DATABASE, REFERENCED_SCHEMA, REFERENCED_OBJECT_NAME, REFERENCED_OBJECT_DOMAIN",
			"WHERE REFERENCING_DATABASE = 'DB' AND REFERENCING_SCHEMA = 'SC' AND REFERENCING_OBJECT_NAME = 'OBJ'",
			"SNOWFLAKE.ACCOUNT_USAGE.OBJECT_DEPENDENCIES",
		} {
			if !strings.Contains(q, want) {
				t.Errorf("query missing %q\ngot: %s", want, q)
			}
		}
	})

	t.Run("referenced_by filters on REFERENCED and selects REFERENCING", func(t *testing.T) {
		q, err := buildUsageDependencyQuery("DB", "SC", "OBJ", ReferencedBy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(q, "SELECT REFERENCING_DATABASE") {
			t.Errorf("expected REFERENCING projection, got: %s", q)
		}
		if !strings.Contains(q, "WHERE REFERENCED_DATABASE = 'DB'") {
			t.Errorf("expected REFERENCED filter, got: %s", q)
		}
	})

	t.Run("single quotes in identifiers are escaped", func(t *testing.T) {
		q, err := buildUsageDependencyQuery("DB", "SC", "O'BJ", DependsOn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(q, "REFERENCING_OBJECT_NAME = 'O''BJ'") {
			t.Errorf("expected escaped quote, got: %s", q)
		}
	})

	t.Run("unknown direction errors", func(t *testing.T) {
		if _, err := buildUsageDependencyQuery("DB", "SC", "OBJ", UsageDependencyDirection("sideways")); err == nil {
			t.Fatal("expected error for unknown direction")
		}
	})
}
