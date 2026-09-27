// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"strings"
	"testing"
)

func TestBuildCreateServiceSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ServiceConfig
		contains []string
		absent   []string
	}{
		{
			name: "full inline spec with all properties",
			cfg: ServiceConfig{
				Name:                       "ECHO_SVC",
				IfNotExists:                true,
				ComputePool:                "MY_POOL",
				SpecSource:                 SpecSourceInline,
				SpecInline:                 "spec:\n  containers:\n  - name: echo\n    image: /db/sc/repo/echo:latest",
				ExternalAccessIntegrations: "EAI_ONE, EAI_TWO",
				AutoResume:                 "true",
				MinInstances:               "1",
				MaxInstances:               "3",
				QueryWarehouse:             "MY_WH",
				Comment:                    "echo service",
			},
			contains: []string{
				"CREATE SERVICE IF NOT EXISTS \"DB\".\"SC\".ECHO_SVC",
				"IN COMPUTE POOL \"MY_POOL\"",
				"FROM SPECIFICATION $$",
				"image: /db/sc/repo/echo:latest",
				"$$",
				"EXTERNAL_ACCESS_INTEGRATIONS = (\"EAI_ONE\", \"EAI_TWO\")",
				"AUTO_RESUME = TRUE",
				"MIN_INSTANCES = 1",
				"MAX_INSTANCES = 3",
				"QUERY_WAREHOUSE = \"MY_WH\"",
				"COMMENT = 'echo service'",
			},
		},
		{
			name: "staged spec file",
			cfg: ServiceConfig{
				Name:        "WEB",
				ComputePool: "POOL",
				SpecSource:  SpecSourceStage,
				SpecStage:   "@specs",
				SpecFile:    "web/service.yaml",
			},
			contains: []string{
				"CREATE SERVICE \"DB\".\"SC\".WEB",
				"FROM @specs",
				"SPECIFICATION_FILE = 'web/service.yaml'",
			},
			absent: []string{"IF NOT EXISTS", "FROM SPECIFICATION $$", "@@"},
		},
		{
			name: "inline specification template with USING variables",
			cfg: ServiceConfig{
				Name:        "TPL_SVC",
				ComputePool: "POOL",
				SpecSource:  SpecSourceInline,
				Template:    true,
				SpecInline:  "spec:\n  containers:\n  - name: c\n    image: {{ image }}",
				TemplateVars: []TemplateVar{
					{Key: "image", Value: "/db/sc/repo/app:latest"},
					{Key: "replicas", Value: "3"},
					{Key: "debug", Value: "true"},
					{Key: "blank", Value: ""},   // valid key, empty value → ''
					{Key: "", Value: "ignored"}, // blank key → skipped
				},
			},
			contains: []string{
				"FROM SPECIFICATION_TEMPLATE $$",
				"image: {{ image }}",
				"USING (image => '/db/sc/repo/app:latest', replicas => 3, debug => TRUE, blank => '')",
			},
			absent: []string{"SPECIFICATION $$", "ignored"},
		},
		{
			name: "staged specification template file with USING",
			cfg: ServiceConfig{
				Name:         "TPL_FILE",
				ComputePool:  "POOL",
				SpecSource:   SpecSourceStage,
				Template:     true,
				SpecStage:    "@specs",
				SpecFile:     "tpl.yaml",
				TemplateVars: []TemplateVar{{Key: "tag", Value: "v2"}},
			},
			contains: []string{
				"FROM @specs",
				"SPECIFICATION_TEMPLATE_FILE = 'tpl.yaml'",
				"USING (tag => 'v2')",
			},
			absent: []string{"SPECIFICATION_FILE =", "SPECIFICATION_TEMPLATE $$"},
		},
		{
			name: "template with no usable variables omits USING",
			cfg: ServiceConfig{
				Name:         "NOVARS",
				ComputePool:  "POOL",
				SpecSource:   SpecSourceInline,
				Template:     true,
				SpecInline:   "spec: {}",
				TemplateVars: []TemplateVar{{Key: "", Value: "x"}},
			},
			contains: []string{"FROM SPECIFICATION_TEMPLATE $$"},
			absent:   []string{"USING"},
		},
		{
			name: "non-template ignores template vars and emits plain SPECIFICATION",
			cfg: ServiceConfig{
				Name:         "PLAIN",
				ComputePool:  "POOL",
				SpecSource:   SpecSourceInline,
				Template:     false,
				SpecInline:   "spec: {}",
				TemplateVars: []TemplateVar{{Key: "x", Value: "1"}},
			},
			contains: []string{"FROM SPECIFICATION $$"},
			absent:   []string{"USING", "SPECIFICATION_TEMPLATE"},
		},
		{
			name: "blank name and pool render placeholders",
			cfg:  ServiceConfig{},
			contains: []string{
				"CREATE SERVICE \"DB\".\"SC\".service_name",
				"IN COMPUTE POOL <compute_pool>",
				"FROM SPECIFICATION $$",
			},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: ServiceConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
				ComputePool:   "P",
			},
			contains: []string{"\"DB\".\"SC\".\"MixedCase\""},
		},
		{
			name: "comment with single quote is escaped",
			cfg: ServiceConfig{
				Name:        "S",
				ComputePool: "P",
				Comment:     "it's mine",
			},
			contains: []string{"COMMENT = 'it''s mine'"},
		},
		{
			name: "no optional properties emitted when unset",
			cfg: ServiceConfig{
				Name:        "BARE",
				ComputePool: "P",
				SpecSource:  SpecSourceInline,
				SpecInline:  "spec: {}",
			},
			absent: []string{
				"EXTERNAL_ACCESS_INTEGRATIONS",
				"AUTO_RESUME",
				"MIN_INSTANCES",
				"MAX_INSTANCES",
				"QUERY_WAREHOUSE",
				"COMMENT",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateServiceSql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(sql, ";") {
				t.Errorf("expected trailing semicolon, got:\n%s", sql)
			}
			// CREATE SERVICE has no OR REPLACE in Snowflake.
			if strings.Contains(sql, "OR REPLACE") {
				t.Errorf("CREATE SERVICE must never contain OR REPLACE, got:\n%s", sql)
			}
			for _, want := range tt.contains {
				if !strings.Contains(sql, want) {
					t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
				}
			}
			for _, bad := range tt.absent {
				if strings.Contains(sql, bad) {
					t.Errorf("expected SQL to NOT contain %q, got:\n%s", bad, sql)
				}
			}
		})
	}
}

func TestBuildAlterServiceSpecClause(t *testing.T) {
	got, err := BuildAlterServiceSpecClause(ServiceConfig{SpecSource: SpecSourceInline, SpecInline: "spec:\n  containers: []\n"})
	if err != nil || got != "FROM SPECIFICATION $$\nspec:\n  containers: []\n$$" {
		t.Errorf("inline: got %q, %v", got, err)
	}
	got, err = BuildAlterServiceSpecClause(ServiceConfig{SpecSource: SpecSourceStage, SpecStage: `"DB"."SC"."STG"`, SpecFile: "a/spec.yaml"})
	if err != nil || got != `FROM @"DB"."SC"."STG"`+"\n  SPECIFICATION_FILE = 'a/spec.yaml'" {
		t.Errorf("stage: got %q, %v", got, err)
	}
	for _, bad := range []ServiceConfig{
		{SpecSource: SpecSourceInline, SpecInline: "  "},
		{SpecSource: SpecSourceInline, SpecInline: "x: $$"},
		{SpecSource: SpecSourceStage, SpecStage: "@", SpecFile: "spec.yaml"},
		{SpecSource: SpecSourceStage, SpecStage: "STG"},
	} {
		if _, err := BuildAlterServiceSpecClause(bad); err == nil {
			t.Errorf("expected error for %+v", bad)
		}
	}
}

func TestBuildServiceRoleGrantSql(t *testing.T) {
	tests := []struct {
		g      ServiceRoleGrant
		grant  string
		revoke string
	}{
		{
			ServiceRoleGrant{Role: "all_endpoints_usage", GranteeKind: "role", Grantee: "ANALYST"},
			`GRANT SERVICE ROLE "DB"."SC"."SVC"!"all_endpoints_usage" TO ROLE "ANALYST";`,
			`REVOKE SERVICE ROLE "DB"."SC"."SVC"!"all_endpoints_usage" FROM ROLE "ANALYST";`,
		},
		{
			ServiceRoleGrant{Role: "r!x", GranteeKind: "DATABASE ROLE", Parent: "DB", Grantee: `D"R`},
			`GRANT SERVICE ROLE "DB"."SC"."SVC"!"r!x" TO DATABASE ROLE "DB"."D""R";`,
			`REVOKE SERVICE ROLE "DB"."SC"."SVC"!"r!x" FROM DATABASE ROLE "DB"."D""R";`,
		},
		{
			ServiceRoleGrant{Role: "R", GranteeKind: "application role", Parent: "APP", Grantee: "AR"},
			`GRANT SERVICE ROLE "DB"."SC"."SVC"!"R" TO APPLICATION ROLE "APP"."AR";`,
			`REVOKE SERVICE ROLE "DB"."SC"."SVC"!"R" FROM APPLICATION ROLE "APP"."AR";`,
		},
	}
	for _, tt := range tests {
		if got, err := BuildGrantServiceRoleSql("DB", "SC", "SVC", tt.g); err != nil || got != tt.grant {
			t.Errorf("grant: got %q, %v; want %q", got, err, tt.grant)
		}
		if got, err := BuildRevokeServiceRoleSql("DB", "SC", "SVC", tt.g); err != nil || got != tt.revoke {
			t.Errorf("revoke: got %q, %v; want %q", got, err, tt.revoke)
		}
	}
	for _, bad := range []ServiceRoleGrant{
		{Role: "R", GranteeKind: "USER", Grantee: "U"},
		{GranteeKind: "ROLE", Grantee: "U"},
		{Role: "R", GranteeKind: "ROLE"},
	} {
		if _, err := BuildGrantServiceRoleSql("DB", "SC", "SVC", bad); err == nil {
			t.Errorf("expected error for %+v", bad)
		}
	}
}
