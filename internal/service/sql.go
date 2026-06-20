// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package service

import (
	"fmt"
	"strconv"
	"strings"

	"thaw/internal/snowflake"
)

// Spec source modes for ServiceConfig.SpecSource.
const (
	SpecSourceInline = "inline" // FROM SPECIFICATION[_TEMPLATE] $$ … $$
	SpecSourceStage  = "stage"  // FROM @<stage> SPECIFICATION[_TEMPLATE]_FILE = '…'
)

// TemplateVar is a single name => value binding for the USING clause of a
// templated service specification (SPECIFICATION_TEMPLATE). Values are rendered
// as SQL literals: numbers and TRUE/FALSE/NULL keywords are emitted bare, all
// other values are single-quoted string literals.
type TemplateVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ServiceConfig holds the parameters for creating a Snowflake SERVICE (Snowpark
// Container Services). Snowflake requires the compute pool and the service
// specification first, then the remaining properties in documented order. The
// specification can be supplied inline (a YAML string) or referenced from a
// stage file; SpecSource selects which. When Template is set, the specification
// is treated as a Jinja template (SPECIFICATION_TEMPLATE / TEMPLATE_FILE) and
// the TemplateVars are emitted as a USING ( key => value, … ) clause. SERVICE
// has no OR REPLACE, so only IF NOT EXISTS is offered.
type ServiceConfig struct {
	Name                       string        `json:"name"`
	CaseSensitive              bool          `json:"caseSensitive"`
	IfNotExists                bool          `json:"ifNotExists"`
	ComputePool                string        `json:"computePool"`
	SpecSource                 string        `json:"specSource"`                 // "inline" | "stage"
	Template                   bool          `json:"template"`                   // use SPECIFICATION_TEMPLATE[_FILE] + USING
	SpecInline                 string        `json:"specInline"`                 // YAML text (SpecSource = inline)
	SpecStage                  string        `json:"specStage"`                  // stage name, with or without leading @ (SpecSource = stage)
	SpecFile                   string        `json:"specFile"`                   // path to the YAML file within the stage (SpecSource = stage)
	TemplateVars               []TemplateVar `json:"templateVars"`               // USING ( key => value, … ) bindings (Template only)
	ExternalAccessIntegrations string        `json:"externalAccessIntegrations"` // comma-separated EAI names
	AutoResume                 string        `json:"autoResume"`                 // TRUE | FALSE (or "" for default)
	MinInstances               string        `json:"minInstances"`               // integer string or ""
	MaxInstances               string        `json:"maxInstances"`               // integer string or ""
	QueryWarehouse             string        `json:"queryWarehouse"`             // warehouse for service functions / queries
	Comment                    string        `json:"comment"`
}

// specClause renders the FROM … specification clause (and, for templates, the
// trailing USING binding clause). Inline specs are wrapped in dollar-quoting
// ($$ … $$) so multi-line YAML needs no escaping; staged specs reference a stage
// and a file path. The TEMPLATE variants are emitted when cfg.Template is set.
// When the chosen source is empty the builder emits an obvious placeholder so
// the preview stays a completable template.
func specClause(cfg ServiceConfig) string {
	var sb strings.Builder

	switch cfg.SpecSource {
	case SpecSourceStage:
		stage := strings.TrimSpace(cfg.SpecStage)
		stage = strings.TrimPrefix(stage, "@")
		if stage == "" {
			stage = "<stage>"
		}
		file := strings.TrimSpace(cfg.SpecFile)
		if file == "" {
			file = "spec.yaml"
		}
		keyword := "SPECIFICATION_FILE"
		if cfg.Template {
			keyword = "SPECIFICATION_TEMPLATE_FILE"
		}
		fmt.Fprintf(&sb, "FROM @%s\n  %s = '%s'", stage, keyword, snowflake.EscapeStringLit(file))
	default: // inline
		spec := strings.TrimRight(strings.TrimSpace(cfg.SpecInline), "\n")
		if spec == "" {
			spec = "spec:\n  containers:\n  - name: main\n    image: /db/schema/repo/image:latest"
		}
		keyword := "SPECIFICATION"
		if cfg.Template {
			keyword = "SPECIFICATION_TEMPLATE"
		}
		fmt.Fprintf(&sb, "FROM %s $$\n%s\n$$", keyword, spec)
	}

	// USING ( key => value, … ) applies only to templated specifications.
	if cfg.Template {
		if u := usingClause(cfg.TemplateVars); u != "" {
			fmt.Fprintf(&sb, "\n  %s", u)
		}
	}

	return sb.String()
}

// usingClause renders the USING ( key => value, … ) binding list for a templated
// specification, skipping entries with a blank key. Values are rendered as SQL
// literals via renderUsingValue. Returns "" when no usable bindings exist.
func usingClause(vars []TemplateVar) string {
	var parts []string
	for _, v := range vars {
		key := strings.TrimSpace(v.Key)
		if key == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s => %s", key, renderUsingValue(v.Value)))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("USING (%s)", strings.Join(parts, ", "))
}

// renderUsingValue renders a template-variable value as a SQL literal: integers
// and floats are emitted bare, the keywords TRUE/FALSE/NULL are emitted bare
// (case-normalized), and every other value is a single-quoted string literal.
func renderUsingValue(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return "''"
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return s
	}
	switch strings.ToUpper(s) {
	case "TRUE", "FALSE", "NULL":
		return strings.ToUpper(s)
	}
	return fmt.Sprintf("'%s'", snowflake.EscapeStringLit(s))
}

// BuildCreateServiceSql constructs a CREATE SERVICE statement from the given
// config. The compute pool and a specification are required by Snowflake; when
// they are empty the builder substitutes placeholders so the live preview reads
// as a completable template rather than invalid SQL. Optional clauses are
// emitted only when set, in the order Snowflake documents them.
//
//	CREATE SERVICE [IF NOT EXISTS] <fqn>
//	  IN COMPUTE POOL <pool>
//	  { FROM SPECIFICATION $$ … $$
//	    | FROM @<stage> SPECIFICATION_FILE = '…'
//	    | FROM SPECIFICATION_TEMPLATE $$ … $$ [USING ( k => v, … )]
//	    | FROM @<stage> SPECIFICATION_TEMPLATE_FILE = '…' [USING ( k => v, … )] }
//	  [EXTERNAL_ACCESS_INTEGRATIONS = ( … )]
//	  [AUTO_RESUME = { TRUE | FALSE }]
//	  [MIN_INSTANCES = <num>]
//	  [MAX_INSTANCES = <num>]
//	  [QUERY_WAREHOUSE = <warehouse>]
//	  [COMMENT = '…'];
func BuildCreateServiceSql(db, schema string, cfg ServiceConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("SERVICE", false, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "service_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	pool := strings.TrimSpace(cfg.ComputePool)
	if pool == "" {
		fmt.Fprintf(&sb, "\n  IN COMPUTE POOL <compute_pool>")
	} else {
		fmt.Fprintf(&sb, "\n  IN COMPUTE POOL %s", snowflake.QuoteIdent(pool))
	}

	fmt.Fprintf(&sb, "\n  %s", specClause(cfg))

	if eai := snowflake.SplitIdentList(cfg.ExternalAccessIntegrations, true); len(eai) > 0 {
		fmt.Fprintf(&sb, "\n  EXTERNAL_ACCESS_INTEGRATIONS = (%s)", strings.Join(eai, ", "))
	}
	if ar := strings.TrimSpace(cfg.AutoResume); ar != "" {
		fmt.Fprintf(&sb, "\n  AUTO_RESUME = %s", strings.ToUpper(ar))
	}
	if mi := strings.TrimSpace(cfg.MinInstances); mi != "" {
		fmt.Fprintf(&sb, "\n  MIN_INSTANCES = %s", mi)
	}
	if ma := strings.TrimSpace(cfg.MaxInstances); ma != "" {
		fmt.Fprintf(&sb, "\n  MAX_INSTANCES = %s", ma)
	}
	if qw := strings.TrimSpace(cfg.QueryWarehouse); qw != "" {
		fmt.Fprintf(&sb, "\n  QUERY_WAREHOUSE = %s", snowflake.QuoteIdent(qw))
	}
	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
