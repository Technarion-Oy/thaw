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
