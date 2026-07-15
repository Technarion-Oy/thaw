// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

func TestDedupeDynamicTables(t *testing.T) {
	basic := []SnowflakeObject{
		{Name: "ORDERS", Kind: "TABLE", Schema: "PUBLIC"},
		{Name: "DAILY_SALES", Kind: "TABLE", Schema: "PUBLIC"},   // surfaced as a dynamic table too
		{Name: "daily_sales", Kind: "TABLE", Schema: "analytics"}, // different schema, case-insensitive match
		{Name: "CUSTOMERS", Kind: "VIEW", Schema: "PUBLIC"},
	}
	extended := []SnowflakeObject{
		{Name: "DAILY_SALES", Kind: "DYNAMIC TABLE", Schema: "PUBLIC"},
		{Name: "DAILY_SALES", Kind: "DYNAMIC TABLE", Schema: "ANALYTICS"},
		{Name: "MY_PIPE", Kind: "PIPE", Schema: "PUBLIC"},
	}

	got := dedupeDynamicTables(basic, extended)

	var names []string
	for _, o := range got {
		names = append(names, o.Schema+"."+o.Name+":"+o.Kind)
	}

	// Both DAILY_SALES basic rows (PUBLIC + analytics) must be dropped; ORDERS
	// and CUSTOMERS must remain.
	want := map[string]bool{
		"PUBLIC.ORDERS:TABLE":   true,
		"PUBLIC.CUSTOMERS:VIEW": true,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d objects, got %d: %v", len(want), len(got), names)
	}
	for _, o := range got {
		key := o.Schema + "." + o.Name + ":" + o.Kind
		if !want[key] {
			t.Errorf("unexpected object survived dedup: %s", key)
		}
	}
}

func TestDedupeDynamicTablesNoDynamicTables(t *testing.T) {
	basic := []SnowflakeObject{
		{Name: "ORDERS", Kind: "TABLE", Schema: "PUBLIC"},
	}
	extended := []SnowflakeObject{
		{Name: "MY_PIPE", Kind: "PIPE", Schema: "PUBLIC"},
	}
	got := dedupeDynamicTables(basic, extended)
	// No dynamic tables → basic returned unchanged (same backing slice).
	if len(got) != 1 || got[0].Name != "ORDERS" {
		t.Fatalf("expected basic returned unchanged, got %v", got)
	}
}
