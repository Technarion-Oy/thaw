// SPDX-License-Identifier: GPL-3.0-or-later

package integrations

import (
	"strings"
	"testing"
)

func TestBuildAlterIntegrationPropertySQL(t *testing.T) {
	cases := []struct {
		name                         string
		kind, integ, property, value string
		want                         string // "" → expect an error
	}{
		// bool
		{"enabled", "STORAGE", "MY_INT", "ENABLED", "true", `ALTER STORAGE INTEGRATION "MY_INT" SET ENABLED = TRUE`},
		{"enabled on every kind", "external access", "eai", "enabled", "FALSE", `ALTER EXTERNAL ACCESS INTEGRATION "eai" SET ENABLED = FALSE`},
		{"bool rejects junk", "STORAGE", "I", "ENABLED", "TRUE; DROP INTEGRATION I", ""},
		{"bool cannot be emptied", "STORAGE", "I", "ENABLED", "", ""},

		// text
		{"comment", "API", "I", "COMMENT", "it's", `ALTER API INTEGRATION "I" SET COMMENT = 'it''s'`},
		{"comment unset", "API", "I", "COMMENT", "  ", `ALTER API INTEGRATION "I" UNSET COMMENT`},
		{"comment without UNSET in grammar", "SECURITY", "I", "COMMENT", "", `ALTER SECURITY INTEGRATION "I" SET COMMENT = ''`},
		{"text injection", "STORAGE", "I", "STORAGE_AWS_ROLE_ARN", `x'; DROP INTEGRATION I; --`,
			`ALTER STORAGE INTEGRATION "I" SET STORAGE_AWS_ROLE_ARN = 'x''; DROP INTEGRATION I; --'`},
		{"text backslash-quote injection", "STORAGE", "I", "COMMENT", `\'; DROP`,
			`ALTER STORAGE INTEGRATION "I" SET COMMENT = '\\''; DROP'`},
		{"name is quoted", "STORAGE", `a"b`, "ENABLED", "TRUE", `ALTER STORAGE INTEGRATION "a""b" SET ENABLED = TRUE`},

		// string list
		{"list", "STORAGE", "I", "STORAGE_ALLOWED_LOCATIONS", "s3://a/, s3://b/",
			`ALTER STORAGE INTEGRATION "I" SET STORAGE_ALLOWED_LOCATIONS = ('s3://a/', 's3://b/')`},
		{"list in DESCRIBE bracket form", "SECURITY", "I", "EXTERNAL_OAUTH_AUDIENCE_LIST", `['a', 'b']`,
			`ALTER SECURITY INTEGRATION "I" SET EXTERNAL_OAUTH_AUDIENCE_LIST = ('a', 'b')`},
		{"list injection", "STORAGE", "I", "STORAGE_ALLOWED_LOCATIONS", `s3://a/') ENABLED = FALSE --`,
			`ALTER STORAGE INTEGRATION "I" SET STORAGE_ALLOWED_LOCATIONS = ('s3://a/'') ENABLED = FALSE --')`},
		{"list unset", "STORAGE", "I", "STORAGE_BLOCKED_LOCATIONS", "", `ALTER STORAGE INTEGRATION "I" UNSET STORAGE_BLOCKED_LOCATIONS`},
		{"list that cannot be unset", "STORAGE", "I", "STORAGE_ALLOWED_LOCATIONS", "", ""},

		// identifier list
		{"ident list", "EXTERNAL ACCESS", "I", "ALLOWED_NETWORK_RULES", "[DB.S.R1, DB.S.R2]",
			`ALTER EXTERNAL ACCESS INTEGRATION "I" SET ALLOWED_NETWORK_RULES = (DB.S.R1, DB.S.R2)`},
		{"ident list keyword", "EXTERNAL ACCESS", "I", "ALLOWED_AUTHENTICATION_SECRETS", "all",
			`ALTER EXTERNAL ACCESS INTEGRATION "I" SET ALLOWED_AUTHENTICATION_SECRETS = ALL`},
		{"ident list unset", "EXTERNAL ACCESS", "I", "ALLOWED_NETWORK_RULES", "", `ALTER EXTERNAL ACCESS INTEGRATION "I" UNSET ALLOWED_NETWORK_RULES`},
		{"ident list injection", "EXTERNAL ACCESS", "I", "ALLOWED_NETWORK_RULES", "R1) ENABLED = FALSE --", ""},

		// identifier
		{"ident", "NOTIFICATION", "I", "WEBHOOK_SECRET", "DB.S.SEC", `ALTER NOTIFICATION INTEGRATION "I" SET WEBHOOK_SECRET = DB.S.SEC`},
		{"ident injection", "NOTIFICATION", "I", "WEBHOOK_SECRET", "S; DROP", ""},

		// number
		{"number", "CATALOG", "I", "REFRESH_INTERVAL_SECONDS", "060", `ALTER CATALOG INTEGRATION "I" SET REFRESH_INTERVAL_SECONDS = 60`},
		{"number rejects junk", "CATALOG", "I", "REFRESH_INTERVAL_SECONDS", "60 COMMENT = 'x'", ""},

		// enum
		{"enum", "SECURITY", "I", "EXTERNAL_OAUTH_ANY_ROLE_MODE", "enable", `ALTER SECURITY INTEGRATION "I" SET EXTERNAL_OAUTH_ANY_ROLE_MODE = ENABLE`},
		{"enum rejected", "SECURITY", "I", "EXTERNAL_OAUTH_ANY_ROLE_MODE", "MAYBE", ""},

		// secrets
		{"secret", "API", "I", "API_KEY", "k'1", `ALTER API INTEGRATION "I" SET API_KEY = 'k''1'`},
		{"secret unset", "API", "I", "API_KEY", "", `ALTER API INTEGRATION "I" UNSET API_KEY`},
		{"nested secret", "CATALOG", "I", "BEARER_TOKEN", "tok",
			`ALTER CATALOG INTEGRATION "I" SET REST_AUTHENTICATION = (BEARER_TOKEN = 'tok')`},
		{"secret cannot be blanked", "CATALOG", "I", "BEARER_TOKEN", "", ""},

		// allow-list
		{"read-only property", "STORAGE", "I", "STORAGE_AWS_EXTERNAL_ID", "x", ""},
		{"property of another kind", "STORAGE", "I", "API_KEY", "x", ""},
		{"property as injection", "STORAGE", "I", "ENABLED = TRUE COMMENT", "x", ""},
		{"unknown kind", "STORAGE INTEGRATION X; DROP", "I", "ENABLED", "TRUE", ""},
		{"empty name", "STORAGE", " ", "ENABLED", "TRUE", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildAlterIntegrationPropertySQL(tc.kind, tc.integ, tc.property, tc.value)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("expected an error, got SQL: %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

// Every allow-listed property must build with a representative value, so a
// typo'd type in the table can't ship.
func TestAlterableTableBuilds(t *testing.T) {
	sample := map[string]string{
		tBool: "TRUE", tInt: "1", tText: "x", tList: "a, b", tIdent: "A.B", tIdentList: "A.B, C",
	}
	for kind, props := range alterable {
		for name, spec := range props {
			v := sample[spec.typ]
			if spec.typ == tEnum {
				v = spec.options[0]
			}
			sql, err := BuildAlterIntegrationPropertySQL(kind, "I", name, v)
			if err != nil {
				t.Errorf("%s.%s: %v", kind, name, err)
			} else if !strings.HasPrefix(sql, "ALTER "+kind+` INTEGRATION "I" SET `) {
				t.Errorf("%s.%s: unexpected SQL %s", kind, name, sql)
			}
		}
	}
}

func TestAnnotateProperties(t *testing.T) {
	byName := func(rows []Property) map[string]Property {
		m := map[string]Property{}
		for _, r := range rows {
			m[r.Name] = r
		}
		return m
	}

	rows := byName(AnnotateProperties("API", []Property{
		{Name: "ENABLED", Value: "true"},
		{Name: "API_PROVIDER", Value: "AWS_API_GATEWAY"},
		{Name: "API_AWS_ROLE_ARN", Value: "arn"},
		{Name: "API_BLOCKED_PREFIXES"},
	}))
	if p := rows["ENABLED"]; !p.Editable || p.Editor != tBool {
		t.Errorf("ENABLED: %+v", p)
	}
	if p := rows["API_PROVIDER"]; p.Editable || p.Options == nil {
		t.Errorf("API_PROVIDER must be read-only with non-nil options: %+v", p)
	}
	if p := rows["API_BLOCKED_PREFIXES"]; !p.Editable || !p.Unsettable || p.Editor != tList {
		t.Errorf("API_BLOCKED_PREFIXES: %+v", p)
	}
	if p, ok := rows["API_KEY"]; !ok || !p.Secret || !p.Editable {
		t.Errorf("API_KEY set-only row missing: %+v", p)
	}

	// A git_https_api integration has neither role ARN nor AD app id → no API_KEY row.
	if _, ok := byName(AnnotateProperties("API", []Property{{Name: "API_PROVIDER"}}))["API_KEY"]; ok {
		t.Error("API_KEY must not be offered when its subtype marker is absent")
	}

	// Snowflake OAuth reports a generated OAUTH_CLIENT_ID but no OAUTH_GRANT → read-only.
	sec := byName(AnnotateProperties("SECURITY", []Property{{Name: "OAUTH_CLIENT_ID"}, {Name: "OAUTH_REDIRECT_URI"}}))
	if sec["OAUTH_CLIENT_ID"].Editable || !sec["OAUTH_REDIRECT_URI"].Editable {
		t.Errorf("subtype gating wrong: %+v", sec)
	}
	if _, ok := sec["OAUTH_CLIENT_SECRET"]; ok {
		t.Error("OAUTH_CLIENT_SECRET must not be offered without OAUTH_GRANT")
	}

	// Unknown kind: everything read-only, nothing appended.
	if got := AnnotateProperties("BOGUS", []Property{{Name: "ENABLED"}}); len(got) != 1 || got[0].Editable {
		t.Errorf("unknown kind: %+v", got)
	}
}
