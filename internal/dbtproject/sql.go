// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package dbtproject

import (
	"fmt"
	"strings"
	"thaw/internal/snowflake"
)

// CreateConfig holds the parameters for creating a Snowflake DBT PROJECT object.
type CreateConfig struct {
	Name                       string   `json:"name"`
	CaseSensitive              bool     `json:"caseSensitive"`
	OrReplace                  bool     `json:"orReplace"`
	IfNotExists                bool     `json:"ifNotExists"`
	SourceLocation             string   `json:"sourceLocation"`
	Comment                    string   `json:"comment"`
	DbtVersion                 string   `json:"dbtVersion"`
	DefaultTarget              string   `json:"defaultTarget"`
	ExternalAccessIntegrations []string `json:"externalAccessIntegrations"`
}

// AlterSetConfig holds the parameters for ALTER DBT PROJECT ... SET/UNSET.
type AlterSetConfig struct {
	DbtVersion                 string   `json:"dbtVersion"`
	DefaultTarget              string   `json:"defaultTarget"`
	ExternalAccessIntegrations []string `json:"externalAccessIntegrations"`
	Comment                    string   `json:"comment"`
}

// ExecuteConfig holds the parameters for EXECUTE DBT PROJECT.
type ExecuteConfig struct {
	Args          string `json:"args"`
	DbtVersion    string `json:"dbtVersion"`
	FromWorkspace string `json:"fromWorkspace"`
	ProjectRoot   string `json:"projectRoot"`
}

// DbtVersionInfo holds a single entry from SYSTEM$SUPPORTED_DBT_VERSIONS().
type DbtVersionInfo struct {
	DbtVersion string `json:"dbt_version"`
	Type       string `json:"type"`
}

// BuildDescribeSql returns a DESCRIBE DBT PROJECT statement.
func BuildDescribeSql(db, schema, name string) string {
	return fmt.Sprintf("DESCRIBE DBT PROJECT %s.%s.%s;",
		snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
}

// BuildCreateDbtProjectSql constructs a CREATE DBT PROJECT SQL statement.
func BuildCreateDbtProjectSql(db, schema string, cfg CreateConfig) (string, error) {
	if cfg.SourceLocation == "" {
		return "", fmt.Errorf("sourceLocation is required")
	}

	var sb strings.Builder

	createClause := snowflake.CreateClause("DBT PROJECT", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		// Placeholder for live SQL preview before the user types a name.
		// The frontend gates submission with canSubmit (name must be non-empty).
		name = "project_name"
	}

	fmt.Fprintf(&sb, "%s %s\n", createClause, snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))
	fmt.Fprintf(&sb, "  FROM '%s'", snowflake.EscapeStringLit(cfg.SourceLocation))

	if cfg.DbtVersion != "" {
		fmt.Fprintf(&sb, "\n  DBT_VERSION = '%s'", snowflake.EscapeStringLit(cfg.DbtVersion))
	}

	if cfg.DefaultTarget != "" {
		fmt.Fprintf(&sb, "\n  DEFAULT_TARGET = '%s'", snowflake.EscapeStringLit(cfg.DefaultTarget))
	}

	if len(cfg.ExternalAccessIntegrations) > 0 {
		quoted := make([]string, len(cfg.ExternalAccessIntegrations))
		for i, n := range cfg.ExternalAccessIntegrations {
			quoted[i] = snowflake.QuoteIdent(n)
		}
		fmt.Fprintf(&sb, "\n  EXTERNAL_ACCESS_INTEGRATIONS = (%s)", strings.Join(quoted, ", "))
	}

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}

// BuildAlterDbtProjectSetSql constructs one or more ALTER DBT PROJECT SET/UNSET statements.
// origComment, origDbtVersion, origDefaultTarget, and origIntegrations are the current values
// used to detect changes and UNSET operations.
func BuildAlterDbtProjectSetSql(db, schema, name string, cfg AlterSetConfig, origComment, origDbtVersion, origDefaultTarget string, origIntegrations []string) ([]string, error) {
	ref := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	var statements []string
	var setClauses []string

	if cfg.DbtVersion != "" && cfg.DbtVersion != origDbtVersion {
		setClauses = append(setClauses, fmt.Sprintf("DBT_VERSION = '%s'", snowflake.EscapeStringLit(cfg.DbtVersion)))
	}

	if cfg.DefaultTarget != "" && cfg.DefaultTarget != origDefaultTarget {
		setClauses = append(setClauses, fmt.Sprintf("DEFAULT_TARGET = '%s'", snowflake.EscapeStringLit(cfg.DefaultTarget)))
	}

	if cfg.Comment != "" && cfg.Comment != origComment {
		setClauses = append(setClauses, "COMMENT = "+snowflake.QuoteTextLit(cfg.Comment))
	}

	// Check if integrations changed (case-insensitive: Snowflake identifiers
	// are uppercased by default, but DESCRIBE may return a different casing
	// than the Select component provides).
	if len(cfg.ExternalAccessIntegrations) > 0 {
		origSet := make(map[string]bool, len(origIntegrations))
		for _, i := range origIntegrations {
			origSet[strings.ToUpper(i)] = true
		}
		newSet := make(map[string]bool, len(cfg.ExternalAccessIntegrations))
		for _, i := range cfg.ExternalAccessIntegrations {
			newSet[strings.ToUpper(i)] = true
		}
		changed := len(origSet) != len(newSet)
		if !changed {
			for k := range newSet {
				if !origSet[k] {
					changed = true
					break
				}
			}
		}
		if changed {
			quoted := make([]string, len(cfg.ExternalAccessIntegrations))
			for i, n := range cfg.ExternalAccessIntegrations {
				quoted[i] = snowflake.QuoteIdent(n)
			}
			setClauses = append(setClauses, fmt.Sprintf("EXTERNAL_ACCESS_INTEGRATIONS = (%s)", strings.Join(quoted, ", ")))
		}
	}

	if len(setClauses) > 0 {
		statements = append(statements, fmt.Sprintf("ALTER DBT PROJECT %s SET\n  %s;", ref, strings.Join(setClauses, "\n  ")))
	}

	// UNSET operations
	var unsetClauses []string
	if origDbtVersion != "" && cfg.DbtVersion == "" {
		unsetClauses = append(unsetClauses, "DBT_VERSION")
	}
	if origDefaultTarget != "" && cfg.DefaultTarget == "" {
		unsetClauses = append(unsetClauses, "DEFAULT_TARGET")
	}
	if origComment != "" && cfg.Comment == "" {
		unsetClauses = append(unsetClauses, "COMMENT")
	}
	if len(origIntegrations) > 0 && len(cfg.ExternalAccessIntegrations) == 0 {
		unsetClauses = append(unsetClauses, "EXTERNAL_ACCESS_INTEGRATIONS")
	}
	if len(unsetClauses) > 0 {
		statements = append(statements, fmt.Sprintf("ALTER DBT PROJECT %s UNSET %s;", ref, strings.Join(unsetClauses, ", ")))
	}

	return statements, nil
}

// BuildExecuteDbtProjectSql constructs an EXECUTE DBT PROJECT SQL statement.
func BuildExecuteDbtProjectSql(db, schema, name string, cfg ExecuteConfig) (string, error) {
	var sb strings.Builder

	if cfg.FromWorkspace != "" {
		fmt.Fprintf(&sb, "EXECUTE DBT PROJECT\n  FROM WORKSPACE %s", snowflake.QuoteIdent(cfg.FromWorkspace))
		if cfg.ProjectRoot != "" {
			fmt.Fprintf(&sb, "\n  PROJECT_ROOT = '%s'", snowflake.EscapeStringLit(cfg.ProjectRoot))
		}
	} else {
		ref := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
		fmt.Fprintf(&sb, "EXECUTE DBT PROJECT %s", ref)
	}

	if cfg.Args != "" {
		fmt.Fprintf(&sb, "\n  ARGS = '%s'", snowflake.EscapeStringLit(cfg.Args))
	}

	if cfg.DbtVersion != "" {
		fmt.Fprintf(&sb, "\n  DBT_VERSION = '%s'", snowflake.EscapeStringLit(cfg.DbtVersion))
	}

	return sb.String() + ";", nil
}

// BuildAddVersionSql constructs an ALTER DBT PROJECT ... ADD VERSION statement.
func BuildAddVersionSql(db, schema, name, versionAlias, sourceLocation string) (string, error) {
	if sourceLocation == "" {
		return "", fmt.Errorf("sourceLocation is required")
	}
	ref := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	var sb strings.Builder
	fmt.Fprintf(&sb, "ALTER DBT PROJECT %s ADD VERSION", ref)
	if versionAlias != "" {
		fmt.Fprintf(&sb, " %s", snowflake.QuoteIdent(versionAlias))
	}
	fmt.Fprintf(&sb, "\n  FROM '%s'", snowflake.EscapeStringLit(sourceLocation))
	return sb.String() + ";", nil
}
