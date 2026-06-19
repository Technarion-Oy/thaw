// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowflake

import "testing"

func TestClientDrivers(t *testing.T) {
	drivers := ClientDrivers()
	if len(drivers) == 0 {
		t.Fatal("ClientDrivers returned empty catalog")
	}

	byToken := make(map[string]ClientDriver, len(drivers))
	for _, d := range drivers {
		if d.Token == "" {
			t.Error("catalog has an empty token")
		}
		if _, dup := byToken[d.Token]; dup {
			t.Errorf("duplicate token %q in catalog", d.Token)
		}
		byToken[d.Token] = d
	}

	// A representative programmatic driver is version-governed; a CLI client is not.
	if d, ok := byToken["JDBC_DRIVER"]; !ok || !d.VersionGoverned {
		t.Errorf("JDBC_DRIVER should be present and version-governed, got %+v (present=%v)", d, ok)
	}
	if d, ok := byToken["SNOWSQL"]; !ok || d.VersionGoverned {
		t.Errorf("SNOWSQL should be present and NOT version-governed, got %+v (present=%v)", d, ok)
	}
}

func TestMatchClientVersions(t *testing.T) {
	// clientId / clientAppId shapes mirror SYSTEM$CLIENT_VERSION_INFO() — different
	// separators/casing from the policy tokens — to exercise the normalized match.
	info := []ClientVersionInfo{
		{ClientID: "DOTNETDriver", ClientAppID: ".NET", MinimumSupportedVersion: "2.0.9", RecommendedVersion: "2.1.5"},
		{ClientID: "GoDriver", ClientAppID: "Go", MinimumSupportedVersion: "1.7.0", RecommendedVersion: "1.14.1"},
		{ClientID: "JDBC", ClientAppID: "JDBC", MinimumSupportedVersion: "3.13.0", RecommendedVersion: "3.25.0"},
		{ClientID: "PythonConnector", ClientAppID: "Python Connector", MinimumSupportedVersion: "3.0.0", RecommendedVersion: "3.12.0"},
		{ClientID: "Mystery", ClientAppID: "Mystery", RecommendedVersion: "9.9.9"}, // no catalog alias → ignored
	}
	got := MatchClientVersions(info)

	for token, wantRec := range map[string]string{
		"DOTNET_DRIVER": "2.1.5",
		"GO_DRIVER":     "1.14.1",
		"JDBC_DRIVER":   "3.25.0",
		"PYTHON_DRIVER": "3.12.0",
	} {
		if e, ok := got[token]; !ok || e.RecommendedVersion != wantRec {
			t.Errorf("%s: got %+v (present=%v), want recommended %q", token, e, ok, wantRec)
		}
	}
	// An entry with no matching alias must not invent a token.
	if _, ok := got["MYSTERY"]; ok {
		t.Error("unmatched client should not appear in the result")
	}
}
