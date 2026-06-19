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

// TestMatchClientVersionsLiveSample pins the mapping against the real
// SYSTEM$CLIENT_VERSION_INFO() clientId values observed from a live account, so a
// driver silently losing its version suggestion is caught.
func TestMatchClientVersionsLiveSample(t *testing.T) {
	// (clientId, clientAppId) pairs exactly as the function reports them.
	sample := []struct{ id, app string }{
		{"DOTNETDriver", ".NET"},
		{"GO", "Go"},
		{"JDBC", "JDBC"},
		{"JSDriver", "JavaScript"},
		{"ODBC", "ODBC"},
		{"PHP_PDO", "PDO"},
		{"PyCore", "PyCore"},
		{"PythonConnector", "PythonConnector"},
		{"PythonSnowpark", "PythonSnowpark"},
		{"SQLAPI", "SQLAPI"},
		{"SnowSQL", "SnowSQL"},
		{"SnowflakeSQLAlchemy", "SnowflakeSQLAlchemy"},
		{"Snowflake_CLI", "Snowflake_CLI"},
		{"Snowpark", "Snowpark"},
	}
	info := make([]ClientVersionInfo, len(sample))
	for i, s := range sample {
		info[i] = ClientVersionInfo{ClientID: s.id, ClientAppID: s.app, RecommendedVersion: "1.0.0"}
	}
	got := MatchClientVersions(info)

	// Every driver the function reports must resolve to its policy token.
	wantTokens := []string{
		"DOTNET_DRIVER", "GO_DRIVER", "JDBC_DRIVER", "JAVASCRIPT_DRIVER", "ODBC_DRIVER",
		"PHP_DRIVER", "PY_CORE", "PYTHON_DRIVER", "PYTHON_SNOWPARK", "SQL_API",
		"SNOWSQL", "SQL_ALCHEMY", "SNOWFLAKE_CLI", "SNOWPARK",
	}
	for _, token := range wantTokens {
		if _, ok := got[token]; !ok {
			t.Errorf("%s did not resolve from the live sample", token)
		}
	}
}
