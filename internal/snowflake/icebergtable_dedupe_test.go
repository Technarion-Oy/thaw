// SPDX-License-Identifier: GPL-3.0-or-later

package snowflake

import "testing"

func TestDedupeIcebergTables(t *testing.T) {
	basic := []SnowflakeObject{
		{Name: "ORDERS", Kind: "TABLE", Schema: "PUBLIC"},
		{Name: "LAKE_EVENTS", Kind: "TABLE", Schema: "PUBLIC"},    // surfaced as an iceberg table too
		{Name: "lake_events", Kind: "TABLE", Schema: "analytics"}, // different schema, case-insensitive match
		{Name: "CUSTOMERS", Kind: "VIEW", Schema: "PUBLIC"},
	}
	extended := []SnowflakeObject{
		{Name: "LAKE_EVENTS", Kind: "ICEBERG TABLE", Schema: "PUBLIC"},
		{Name: "LAKE_EVENTS", Kind: "ICEBERG TABLE", Schema: "ANALYTICS"},
		{Name: "MY_PIPE", Kind: "PIPE", Schema: "PUBLIC"},
	}

	got := dedupeIcebergTables(basic, extended)

	var names []string
	for _, o := range got {
		names = append(names, o.Schema+"."+o.Name+":"+o.Kind)
	}

	// Both LAKE_EVENTS basic rows (PUBLIC + analytics) must be dropped; ORDERS
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

func TestDedupeIcebergTablesNoIcebergTables(t *testing.T) {
	basic := []SnowflakeObject{
		{Name: "ORDERS", Kind: "TABLE", Schema: "PUBLIC"},
	}
	extended := []SnowflakeObject{
		{Name: "MY_PIPE", Kind: "PIPE", Schema: "PUBLIC"},
	}
	got := dedupeIcebergTables(basic, extended)
	// No iceberg tables → basic returned unchanged (same backing slice).
	if len(got) != 1 || got[0].Name != "ORDERS" {
		t.Fatalf("expected basic returned unchanged, got %v", got)
	}
}
