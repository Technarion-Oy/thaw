// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package dataset

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// DatasetConfig holds the parameters for creating a Snowflake DATASET object.
//
// CREATE DATASET is intentionally minimal — the statement only names the dataset
// and carries the OR REPLACE / IF NOT EXISTS flags. There is no COMMENT or other
// property on CREATE; data is loaded afterwards one version at a time via
// ALTER DATASET … ADD VERSION (or the Snowpark ML Python API).
type DatasetConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
}

// BuildCreateDatasetSql constructs a CREATE DATASET statement from the given
// config. When the name is blank the builder substitutes a placeholder so the
// live preview reads as a completable template rather than invalid SQL. OR
// REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake; the create modal
// prevents selecting both, and if both are set here OR REPLACE wins (IF NOT
// EXISTS is dropped).
//
//	CREATE [OR REPLACE] DATASET [IF NOT EXISTS] <fqn>;
func BuildCreateDatasetSql(db, schema string, cfg DatasetConfig) (string, error) {
	createClause := snowflake.CreateClause("DATASET", cfg.OrReplace, cfg.IfNotExists)

	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "dataset_name"
	}

	return fmt.Sprintf("%s %s;", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive)), nil
}
