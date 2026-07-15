// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

func TestDedupeMaterializedViews(t *testing.T) {
	basic := []SnowflakeObject{
		{Name: "ORDERS", Kind: "TABLE", Schema: "PUBLIC"},
		{Name: "DAILY_TOTALS", Kind: "VIEW", Schema: "PUBLIC"},    // surfaced as a materialized view too
		{Name: "daily_totals", Kind: "VIEW", Schema: "analytics"}, // different schema, case-insensitive match
		{Name: "CUSTOMERS", Kind: "VIEW", Schema: "PUBLIC"},
	}
	extended := []SnowflakeObject{
		{Name: "DAILY_TOTALS", Kind: "MATERIALIZED VIEW", Schema: "PUBLIC"},
		{Name: "DAILY_TOTALS", Kind: "MATERIALIZED VIEW", Schema: "ANALYTICS"},
		{Name: "MY_PIPE", Kind: "PIPE", Schema: "PUBLIC"},
	}

	got := dedupeMaterializedViews(basic, extended)

	want := map[string]bool{
		"PUBLIC.ORDERS:TABLE":   true,
		"PUBLIC.CUSTOMERS:VIEW": true,
	}
	if len(got) != len(want) {
		var names []string
		for _, o := range got {
			names = append(names, o.Schema+"."+o.Name+":"+o.Kind)
		}
		t.Fatalf("expected %d objects, got %d: %v", len(want), len(got), names)
	}
	for _, o := range got {
		key := o.Schema + "." + o.Name + ":" + o.Kind
		if !want[key] {
			t.Errorf("unexpected object survived dedup: %s", key)
		}
	}
}

func TestDedupeMaterializedViewsNoMaterializedViews(t *testing.T) {
	basic := []SnowflakeObject{
		{Name: "ORDERS", Kind: "TABLE", Schema: "PUBLIC"},
	}
	extended := []SnowflakeObject{
		{Name: "MY_PIPE", Kind: "PIPE", Schema: "PUBLIC"},
	}
	got := dedupeMaterializedViews(basic, extended)
	if len(got) != 1 || got[0].Name != "ORDERS" {
		t.Fatalf("expected basic returned unchanged, got %v", got)
	}
}
