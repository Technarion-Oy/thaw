// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package stage

import "testing"

func TestValidateStageRef(t *testing.T) {
	valid := []string{
		`@DB.SCHEMA.STAGE`,
		`@"db"."schema"."stage"/data/2026`,
		`@stage/sub-dir_1/file.csv`,
	}
	for _, s := range valid {
		if err := validateStageRef(s); err != nil {
			t.Errorf("validateStageRef(%q) = %v, want nil", s, err)
		}
	}

	// The reported injection vector and its building blocks must be rejected.
	injections := []string{
		`@db.schema.stage/x; DROP TABLE foo; --`,
		`@db.schema.stage/x' OR '1'='1`,
		"@db.schema.stage/x\nSELECT 1",
	}
	for _, s := range injections {
		if err := validateStageRef(s); err == nil {
			t.Errorf("validateStageRef(%q) = nil, want error", s)
		}
	}
}
