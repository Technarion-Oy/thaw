// SPDX-License-Identifier: GPL-3.0-or-later

package snapshot

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildCreateSnapshotSql(t *testing.T) {
	full := SnapshotConfig{
		Name: "SNAP", OrReplace: true, IfNotExists: true,
		ServiceDatabase: "SDB", ServiceSchema: "SSC", ServiceName: "svc",
		Volume: "data", Instance: "1", Comment: "it's", Tags: []snowflake.TagPair{{Name: "T", Value: "v"}},
	}
	got, err := BuildCreateSnapshotSql("DB", "SC", full)
	if err != nil {
		t.Fatal(err)
	}
	want := "CREATE OR REPLACE SNAPSHOT \"DB\".\"SC\".SNAP\n  FROM SERVICE \"SDB\".\"SSC\".\"svc\"\n  VOLUME \"data\"\n  INSTANCE 1\n  COMMENT = 'it''s'\n  TAG (\"T\" = 'v');"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	blank, err := BuildCreateSnapshotSql("DB", "SC", SnapshotConfig{IfNotExists: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"IF NOT EXISTS", "snapshot_name", "<service>", `"<volume>"`, "<instance>"} {
		if !strings.Contains(blank, s) {
			t.Errorf("placeholder SQL missing %q:\n%s", s, blank)
		}
	}

	for _, bad := range []string{"-1", "1; DROP", "x"} {
		if _, err := BuildCreateSnapshotSql("DB", "SC", SnapshotConfig{Instance: bad}); err == nil {
			t.Errorf("instance %q: want error", bad)
		}
	}
}
