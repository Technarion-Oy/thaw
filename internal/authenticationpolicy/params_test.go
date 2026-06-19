// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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
