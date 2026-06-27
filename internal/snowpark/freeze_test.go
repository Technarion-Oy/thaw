// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package snowpark

import "testing"

func TestIsFreezeLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		// Genuine pip freeze output.
		{"numpy==1.26.4", true},
		{"snowflake-snowpark-python==1.20.0", true},
		{"pandas>=2.0", true},
		{"requests===2.31.0", true},
		{"mypkg @ file:///tmp/mypkg", true},
		{"-e /path/to/editable", true},
		{"-e git+https://example.com/repo.git#egg=pkg", true},
		{"# Editable install with no version control (mypkg==1.0.0)", true},

		// conda / activation noise that must not reach requirements.txt.
		{"", false},
		{"   ", false},
		{"==> WARNING: A newer version of conda exists. <==", false},
		{"Retrieving notices: ...working... done", false},
		{"WARNING: something happened", false},
		{"Installing collected packages: numpy", false},
		{"bare-package-name", false}, // pip freeze always pins, so a bare name is noise
	}
	for _, c := range cases {
		if got := isFreezeLine(c.line); got != c.want {
			t.Errorf("isFreezeLine(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}
