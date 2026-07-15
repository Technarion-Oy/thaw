// SPDX-License-Identifier: GPL-3.0-or-later

package authenticationpolicy

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestClientPolicyDrivers(t *testing.T) {
	drivers := ClientPolicyDrivers()
	if len(drivers) == 0 {
		t.Fatal("ClientPolicyDrivers returned no drivers")
	}

	seen := make(map[string]bool, len(drivers))
	for _, d := range drivers {
		if strings.TrimSpace(d) == "" {
			t.Error("blank driver token")
		}
		if seen[d] {
			t.Errorf("duplicate driver %q", d)
		}
		seen[d] = true
	}

	// A programmatic driver is selectable; CLI clients are filtered out as
	// inapplicable to CLIENT_POLICY's minimum-version enforcement.
	if !seen["JDBC_DRIVER"] {
		t.Error("JDBC_DRIVER should be selectable in CLIENT_POLICY")
	}
	for _, cli := range []string{"SNOWSQL", "SNOWFLAKE_CLI"} {
		if seen[cli] {
			t.Errorf("%s is a CLI client and must not be offered in CLIENT_POLICY", cli)
		}
	}
}

func TestBagOptions(t *testing.T) {
	o := BagOptions()
	for name, got := range map[string][]string{
		"MFAAllowedMethods":          o.MFAAllowedMethods,
		"MFAEnforceExternal":         o.MFAEnforceExternal,
		"PATNetworkPolicyEvaluation": o.PATNetworkPolicyEvaluation,
		"PATRequireRoleRestriction":  o.PATRequireRoleRestriction,
		"WorkloadAllowedProviders":   o.WorkloadAllowedProviders,
	} {
		if len(got) == 0 {
			t.Errorf("%s is empty", name)
		}
	}
	if o.MFAAllowedMethods[0] != "ALL" || o.WorkloadAllowedProviders[0] != "ALL" {
		t.Errorf("expected ALL first in the ALL-exclusive sets: %+v", o)
	}
}

func TestClientPolicyDriverVersions(t *testing.T) {
	info := []snowflake.ClientVersionInfo{
		{ClientID: "JDBC", ClientAppID: "JDBC", MinimumSupportedVersion: "3.13.0", RecommendedVersion: "3.25.0"},
		{ClientID: "SnowSQL", ClientAppID: "SnowSQL", MinimumSupportedVersion: "1.2.0", RecommendedVersion: "1.3.0"},
	}
	hints := ClientPolicyDriverVersions(info)

	var jdbc *DriverVersionHint
	for i := range hints {
		if hints[i].Driver == "JDBC_DRIVER" {
			jdbc = &hints[i]
		}
		if hints[i].Driver == "SNOWSQL" {
			t.Error("SnowSQL is not a CLIENT_POLICY driver and must be excluded from hints")
		}
	}
	if jdbc == nil {
		t.Fatal("expected a JDBC_DRIVER hint")
	}
	if jdbc.MinimumSupported != "3.13.0" || jdbc.Recommended != "3.25.0" {
		t.Errorf("JDBC hint = %+v", *jdbc)
	}
}
