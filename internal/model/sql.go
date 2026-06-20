// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package model

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// SourceType values for ModelConfig.SourceType — the two FROM variants of
// CREATE MODEL.
const (
	// SourceModel copies an existing model: FROM MODEL <src> [VERSION <v>].
	SourceModel = "model"
	// SourceStage loads serialized model artifacts from an internal stage:
	// FROM @<stage>[/<path>].
	SourceStage = "stage"
)

// ModelConfig holds the parameters for creating a Snowflake MODEL object.
//
// CREATE MODEL has two shapes, selected by SourceType:
//   - "model" — copy an existing model (FROM MODEL <SourceModel> [VERSION <SourceVersion>]).
//   - "stage" — load artifacts from an internal stage (FROM <StageLocation>).
//
// CREATE MODEL itself has no COMMENT or TAG clause (those are applied afterwards
// via ALTER MODEL), so the config is limited to the name, the OR REPLACE / IF NOT
// EXISTS flags, an optional WITH VERSION name for the new model, and the source.
type ModelConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`

	// VersionName sets WITH VERSION <version_name> — the name of the version
	// created in the new model. Optional; omitted when blank.
	VersionName string `json:"versionName"`

	// SourceType selects the FROM clause: SourceModel ("model") or
	// SourceStage ("stage"). Anything other than "stage" is treated as "model".
	SourceType string `json:"sourceType"`

	// SourceModel is the source model identifier for SourceType == "model"
	// (FROM MODEL <SourceModel>); it may be a fully qualified db.schema.name.
	SourceModel string `json:"sourceModel"`
	// SourceVersion optionally selects which source version to copy
	// (FROM MODEL <SourceModel> VERSION <SourceVersion>); when blank the source
	// model's default version is used.
	SourceVersion string `json:"sourceVersion"`

	// StageLocation is the internal-stage path for SourceType == "stage"
	// (e.g. @my_stage/model_path).
	StageLocation string `json:"stageLocation"`
}

// BuildCreateModelSql constructs a CREATE MODEL statement from the given config.
// When required fields are blank the builder substitutes placeholders so the live
// preview reads as a completable template rather than invalid SQL. OR REPLACE and
// IF NOT EXISTS are mutually exclusive in Snowflake; the create modal prevents
// selecting both, and if both are set here OR REPLACE wins (IF NOT EXISTS is
// dropped).
//
//	-- SourceType "model"
//	CREATE [OR REPLACE] MODEL [IF NOT EXISTS] <fqn> [WITH VERSION <v>]
//	  FROM MODEL <source_model> [VERSION <source_version>];
//
//	-- SourceType "stage"
//	CREATE [OR REPLACE] MODEL [IF NOT EXISTS] <fqn> [WITH VERSION <v>]
//	  FROM @<stage>[/<path>];
func BuildCreateModelSql(db, schema string, cfg ModelConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("MODEL", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "model_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	if v := strings.TrimSpace(cfg.VersionName); v != "" {
		fmt.Fprintf(&sb, " WITH VERSION %s", v)
	}

	switch strings.ToLower(strings.TrimSpace(cfg.SourceType)) {
	case SourceStage:
		loc := strings.TrimSpace(cfg.StageLocation)
		if loc == "" {
			loc = "@my_stage/model_path"
		}
		fmt.Fprintf(&sb, "\n  FROM %s", loc)
	default: // SourceModel
		src := strings.TrimSpace(cfg.SourceModel)
		if src == "" {
			src = "source_model_name"
		}
		fmt.Fprintf(&sb, "\n  FROM MODEL %s", src)
		if sv := strings.TrimSpace(cfg.SourceVersion); sv != "" {
			fmt.Fprintf(&sb, " VERSION %s", sv)
		}
	}

	return sb.String() + ";", nil
}
