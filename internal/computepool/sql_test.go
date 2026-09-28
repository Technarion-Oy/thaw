// SPDX-License-Identifier: GPL-3.0-or-later

package computepool

import (
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildCreateComputePoolSql(t *testing.T) {
	got, err := BuildCreateComputePoolSql(ComputePoolConfig{
		Name: "my_pool", IfNotExists: true, ForApplication: "APP",
		MinNodes: 1, MaxNodes: 3, InstanceFamily: "gpu_nv_s",
		AutoResume: "true", InitiallySuspended: "FALSE", AutoSuspendSecs: "600",
		Tags:    []snowflake.TagPair{{Name: "COST", Value: "ml"}},
		Comment: "it's", PlacementGroup: "pg1",
		BackupInstanceFamilies: []string{"GPU_NV_M", " ", "cpu_x64_s"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `CREATE COMPUTE POOL IF NOT EXISTS my_pool
  FOR APPLICATION APP
  MIN_NODES = 1
  MAX_NODES = 3
  INSTANCE_FAMILY = GPU_NV_S
  AUTO_RESUME = TRUE
  INITIALLY_SUSPENDED = FALSE
  AUTO_SUSPEND_SECS = 600
  TAG ("COST" = 'ml')
  COMMENT = 'it''s'
  PLACEMENT_GROUP = 'pg1'
  BACKUP_INSTANCE_FAMILIES = ('GPU_NV_M', 'CPU_X64_S');`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	got, _ = BuildCreateComputePoolSql(ComputePoolConfig{MinNodes: 1, MaxNodes: 1})
	if got != "CREATE COMPUTE POOL pool_name\n  MIN_NODES = 1\n  MAX_NODES = 1\n  INSTANCE_FAMILY = <instance_family>;" {
		t.Errorf("placeholder: %s", got)
	}
	if _, err := BuildCreateComputePoolSql(ComputePoolConfig{MinNodes: 3, MaxNodes: 1}); err == nil {
		t.Error("expected MAX_NODES < MIN_NODES to be rejected")
	}
	if _, err := BuildCreateComputePoolSql(ComputePoolConfig{InstanceFamily: "X; DROP"}); err == nil {
		t.Error("expected invalid instance family to be rejected")
	}
}

func TestBuildAlterComputePoolPropertySql(t *testing.T) {
	cases := []struct{ prop, val, want string }{
		{"MIN_NODES", "2", `ALTER COMPUTE POOL "P" SET MIN_NODES = 2`},
		{"AUTO_RESUME", "false", `ALTER COMPUTE POOL "P" SET AUTO_RESUME = FALSE`},
		{"AUTO_RESUME", "", `ALTER COMPUTE POOL "P" UNSET AUTO_RESUME`},
		{"AUTO_SUSPEND_SECS", "", `ALTER COMPUTE POOL "P" UNSET AUTO_SUSPEND_SECS`},
		{"PLACEMENT_GROUP", `g'1\`, `ALTER COMPUTE POOL "P" SET PLACEMENT_GROUP = 'g''1\\'`},
		{"INSTANCE_FAMILY", "cpu_x64_m", `ALTER COMPUTE POOL "P" SET INSTANCE_FAMILY = CPU_X64_M`},
		{"BACKUP_INSTANCE_FAMILIES", "A, B", `ALTER COMPUTE POOL "P" SET BACKUP_INSTANCE_FAMILIES = ('A', 'B')`},
		{"BACKUP_INSTANCE_FAMILIES", " ", `ALTER COMPUTE POOL "P" UNSET BACKUP_INSTANCE_FAMILIES`},
		{"BACKUP_INSTANCE_FAMILIES", " , ,", `ALTER COMPUTE POOL "P" UNSET BACKUP_INSTANCE_FAMILIES`},
		{"COMMENT", "", `ALTER COMPUTE POOL "P" UNSET COMMENT`},
	}
	for _, c := range cases {
		got, err := BuildAlterComputePoolPropertySql("P", c.prop, c.val)
		if err != nil || got != c.want {
			t.Errorf("%s=%q: got %q (%v), want %q", c.prop, c.val, got, err, c.want)
		}
	}
	for _, bad := range [][2]string{{"MIN_NODES", "-1"}, {"MAX_NODES", ""}, {"INSTANCE_FAMILY", "a b"}, {"nope", "x"}, {"MIN_NODES", ""}} {
		if _, err := BuildAlterComputePoolPropertySql("P", bad[0], bad[1]); err == nil {
			t.Errorf("%s=%q: expected error", bad[0], bad[1])
		}
	}
}

func TestBuildStopAllSql(t *testing.T) {
	if got, _ := BuildStopAllSql("P", nil); got != `ALTER COMPUTE POOL "P" STOP ALL` {
		t.Error(got)
	}
	if got, _ := BuildStopAllSql("P", []string{"user_service", ""}); got != `ALTER COMPUTE POOL "P" STOP ALL OF TYPE USER_SERVICE` {
		t.Error(got)
	}
	if _, err := BuildStopAllSql("P", []string{"x;y"}); err == nil {
		t.Error("expected error")
	}
}
