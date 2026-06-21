// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package objects

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ColumnComment holds a column name and its optional comment.
type ColumnComment struct {
	Column  string `json:"column"`
	Comment string `json:"comment"`
}

// BuildObjectPropertiesQuery returns the SHOW/DESCRIBE query that fetches the
// metadata for a single Snowflake object. kind is one of: DATABASE, SCHEMA,
// TABLE, VIEW, DYNAMIC TABLE, EXTERNAL TABLE, ICEBERG TABLE, HYBRID TABLE, EVENT TABLE, MATERIALIZED VIEW, ALERT, TAG,
// MASKING POLICY, ROW ACCESS POLICY, JOIN POLICY, PRIVACY POLICY, STORAGE LIFECYCLE POLICY, PASSWORD POLICY, SESSION POLICY, AGGREGATION POLICY, PROJECTION POLICY, AUTHENTICATION POLICY, PACKAGES POLICY, NETWORK RULE, IMAGE REPOSITORY, SERVICE, STREAMLIT, FUNCTION, EXTERNAL FUNCTION, DATA METRIC FUNCTION, PROCEDURE, SEQUENCE, STAGE, STREAM,
// TASK, FILE FORMAT, PIPE, SECRET, GIT REPOSITORY, DBT PROJECT, MODEL, WAREHOUSE, ROLE,
// USER.
func BuildObjectPropertiesQuery(database, schema, kind, name string) (string, error) {
	like := strings.ReplaceAll(name, `\`, `\\`)
	like = snowflake.EscapeStringLit(like)

	switch strings.ToUpper(kind) {
	case "DATABASE":
		return fmt.Sprintf("SHOW DATABASES LIKE '%s'", like), nil
	case "DYNAMIC TABLE":
		return fmt.Sprintf("SHOW DYNAMIC TABLES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "EXTERNAL TABLE":
		return fmt.Sprintf("SHOW EXTERNAL TABLES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "ICEBERG TABLE":
		return fmt.Sprintf("SHOW ICEBERG TABLES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "HYBRID TABLE":
		return fmt.Sprintf("SHOW HYBRID TABLES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "EVENT TABLE":
		return fmt.Sprintf("SHOW EVENT TABLES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "MATERIALIZED VIEW":
		return fmt.Sprintf("SHOW MATERIALIZED VIEWS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "ALERT":
		return fmt.Sprintf("SHOW ALERTS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "TAG":
		return fmt.Sprintf("SHOW TAGS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "MASKING POLICY":
		return fmt.Sprintf("SHOW MASKING POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "ROW ACCESS POLICY":
		return fmt.Sprintf("SHOW ROW ACCESS POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "JOIN POLICY":
		return fmt.Sprintf("SHOW JOIN POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "PRIVACY POLICY":
		return fmt.Sprintf("SHOW PRIVACY POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "STORAGE LIFECYCLE POLICY":
		return fmt.Sprintf("SHOW STORAGE LIFECYCLE POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "PASSWORD POLICY":
		return fmt.Sprintf("SHOW PASSWORD POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "SESSION POLICY":
		return fmt.Sprintf("SHOW SESSION POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "AGGREGATION POLICY":
		return fmt.Sprintf("SHOW AGGREGATION POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "PROJECTION POLICY":
		return fmt.Sprintf("SHOW PROJECTION POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "AUTHENTICATION POLICY":
		return fmt.Sprintf("SHOW AUTHENTICATION POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "PACKAGES POLICY":
		return fmt.Sprintf("SHOW PACKAGES POLICIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "NETWORK RULE":
		return fmt.Sprintf("SHOW NETWORK RULES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "IMAGE REPOSITORY":
		return fmt.Sprintf("SHOW IMAGE REPOSITORIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "SERVICE":
		return fmt.Sprintf("SHOW SERVICES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "STREAMLIT":
		return fmt.Sprintf("SHOW STREAMLITS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "SCHEMA":
		return fmt.Sprintf("SHOW SCHEMAS LIKE '%s' IN DATABASE %s", like, snowflake.QuoteIdent(database)), nil
	case "TABLE":
		return fmt.Sprintf("SHOW TABLES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "VIEW":
		return fmt.Sprintf("SHOW VIEWS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "FUNCTION":
		return fmt.Sprintf("SHOW FUNCTIONS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "EXTERNAL FUNCTION":
		return fmt.Sprintf("SHOW EXTERNAL FUNCTIONS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "DATA METRIC FUNCTION":
		return fmt.Sprintf("SHOW DATA METRIC FUNCTIONS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "PROCEDURE":
		return fmt.Sprintf("SHOW PROCEDURES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "SEQUENCE":
		return fmt.Sprintf("SHOW SEQUENCES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "STAGE":
		return fmt.Sprintf("SHOW STAGES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "STREAM":
		return fmt.Sprintf("SHOW STREAMS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "TASK":
		return fmt.Sprintf("SHOW TASKS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "FILE FORMAT":
		return fmt.Sprintf("SHOW FILE FORMATS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "PIPE":
		return fmt.Sprintf("SHOW PIPES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "SECRET":
		return fmt.Sprintf("SHOW SECRETS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "GIT REPOSITORY":
		return fmt.Sprintf("SHOW GIT REPOSITORIES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "DBT PROJECT":
		return fmt.Sprintf("SHOW DBT PROJECTS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "MODEL":
		return fmt.Sprintf("SHOW MODELS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "CORTEX SEARCH SERVICE":
		return fmt.Sprintf("SHOW CORTEX SEARCH SERVICES LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "AGENT":
		return fmt.Sprintf("SHOW AGENTS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "EXTERNAL AGENT":
		return fmt.Sprintf("SHOW EXTERNAL AGENTS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "MCP SERVER":
		return fmt.Sprintf("SHOW MCP SERVERS LIKE '%s' IN SCHEMA %s", like, snowflake.Qualify(database, schema)), nil
	case "WAREHOUSE":
		return fmt.Sprintf("SHOW WAREHOUSES LIKE '%s'", like), nil
	case "ROLE":
		return fmt.Sprintf("SHOW ROLES LIKE '%s'", like), nil
	case "USER":
		return fmt.Sprintf("SHOW USERS LIKE '%s'", like), nil
	default:
		return "", fmt.Errorf("unsupported object kind: %s", kind)
	}
}

// BuildDescribeStageQuery returns the DESCRIBE STAGE query used to enrich the
// SHOW STAGES result with stage-specific properties.
func BuildDescribeStageQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE STAGE %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribeMaskingPolicyQuery returns the DESCRIBE MASKING POLICY query used
// to enrich the SHOW MASKING POLICIES result with the policy's signature, return
// type, and body — none of which SHOW MASKING POLICIES reports.
func BuildDescribeMaskingPolicyQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE MASKING POLICY %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribeRowAccessPolicyQuery returns the DESCRIBE ROW ACCESS POLICY query
// used to enrich the SHOW ROW ACCESS POLICIES result with the policy's
// signature, return type, and body — none of which SHOW ROW ACCESS POLICIES
// reports.
func BuildDescribeRowAccessPolicyQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE ROW ACCESS POLICY %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribeJoinPolicyQuery returns the DESCRIBE JOIN POLICY query used to
// enrich the SHOW JOIN POLICIES result with the policy's signature, return type,
// and body — none of which SHOW JOIN POLICIES reports.
func BuildDescribeJoinPolicyQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE JOIN POLICY %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribePrivacyPolicyQuery returns the DESCRIBE PRIVACY POLICY query used
// to enrich the SHOW PRIVACY POLICIES result with the policy's signature, return
// type, and body — none of which SHOW PRIVACY POLICIES reports.
func BuildDescribePrivacyPolicyQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE PRIVACY POLICY %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribeStorageLifecyclePolicyQuery returns the DESCRIBE STORAGE LIFECYCLE
// POLICY query used to enrich the SHOW STORAGE LIFECYCLE POLICIES result with the
// policy's signature, return type, body, and archive settings — none of which
// SHOW STORAGE LIFECYCLE POLICIES reports.
func BuildDescribeStorageLifecyclePolicyQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE STORAGE LIFECYCLE POLICY %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribeAggregationPolicyQuery returns the DESCRIBE AGGREGATION POLICY
// query used to enrich the SHOW AGGREGATION POLICIES result with the policy's
// signature, return type, and body — none of which SHOW AGGREGATION POLICIES
// reports.
func BuildDescribeAggregationPolicyQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE AGGREGATION POLICY %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribeProjectionPolicyQuery returns the DESCRIBE PROJECTION POLICY
// query used to enrich the SHOW PROJECTION POLICIES result with the policy's
// signature, return type, and body — none of which SHOW PROJECTION POLICIES
// reports.
func BuildDescribeProjectionPolicyQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE PROJECTION POLICY %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribePackagesPolicyQuery returns the DESCRIBE PACKAGES POLICY query
// used to enrich the SHOW PACKAGES POLICIES result with the policy's language,
// allowlist, blocklist, and additional_creation_blocklist — none of which SHOW
// PACKAGES POLICIES reports (it returns only metadata).
func BuildDescribePackagesPolicyQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE PACKAGES POLICY %s", snowflake.Qualify(database, schema, name))
}

// appendPackagesPolicyDesc appends the language and allow/block-list properties
// from a DESCRIBE PACKAGES POLICY result to pairs. SHOW PACKAGES POLICIES reports
// only metadata, so DESCRIBE is the only source for these. It handles both shapes
// DESCRIBE may use, defensively (the exact shape isn't pinned by an integration
// test): a row-per-property table (columns "property"/"value", like DESCRIBE
// PASSWORD POLICY) or a single row whose columns are the property names (like
// DESCRIBE MASKING POLICY). Only the four configuration properties are appended
// (keys lowercased to match how the modal looks them up); everything else is left
// to the generic SHOW pairs. A SQL NULL cell becomes an empty string rather than
// the literal "<nil>". A nil/empty result appends nothing.
func appendPackagesPolicyDesc(pairs []snowflake.PropertyPair, descRes *snowflake.QueryResult) []snowflake.PropertyPair {
	if descRes == nil || len(descRes.Rows) == 0 {
		return pairs
	}
	wanted := map[string]bool{
		"language":                      true,
		"allowlist":                     true,
		"blocklist":                     true,
		"additional_creation_blocklist": true,
	}
	cell := func(v any) string {
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
	propIdx, valIdx := -1, -1
	for ci, col := range descRes.Columns {
		switch strings.ToLower(col) {
		case "property":
			propIdx = ci
		case "value":
			valIdx = ci
		}
	}
	if propIdx >= 0 && valIdx >= 0 {
		// Row-per-property shape.
		for _, row := range descRes.Rows {
			if propIdx >= len(row) || valIdx >= len(row) {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(cell(row[propIdx])))
			if wanted[key] {
				pairs = append(pairs, snowflake.PropertyPair{Key: key, Value: cell(row[valIdx])})
			}
		}
		return pairs
	}
	// Single-row-with-columns shape.
	row := descRes.Rows[0]
	for ci, col := range descRes.Columns {
		if ci >= len(row) {
			break
		}
		if wanted[strings.ToLower(col)] {
			pairs = append(pairs, snowflake.PropertyPair{Key: strings.ToLower(col), Value: cell(row[ci])})
		}
	}
	return pairs
}

// BuildDescribeNetworkRuleQuery returns the DESCRIBE NETWORK RULE query used to
// enrich the SHOW NETWORK RULES result with the rule's value_list — which SHOW
// NETWORK RULES reports only as a count (entries_in_valuelist), not the actual
// identifiers.
func BuildDescribeNetworkRuleQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE NETWORK RULE %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribeCortexSearchServiceQuery returns the DESCRIBE CORTEX SEARCH
// SERVICE query used to enrich the SHOW CORTEX SEARCH SERVICES result with the
// rich properties SHOW omits (search column, attributes, embedding model,
// definition, target lag, warehouse, and serving/indexing state).
func BuildDescribeCortexSearchServiceQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE CORTEX SEARCH SERVICE %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribeServiceQuery returns the DESCRIBE SERVICE query used to enrich the
// SHOW SERVICES result with the service's YAML spec — which SHOW SERVICES does
// not report.
func BuildDescribeServiceQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE SERVICE %s", snowflake.Qualify(database, schema, name))
}

// BuildDescribeStreamlitQuery returns the DESCRIBE STREAMLIT query used to enrich
// the SHOW STREAMLITS result with the app's root_location and main_file — which
// SHOW STREAMLITS does not report.
func BuildDescribeStreamlitQuery(database, schema, name string) string {
	return fmt.Sprintf("DESCRIBE STREAMLIT %s", snowflake.Qualify(database, schema, name))
}

// GetObjectProperties returns structured metadata for any Snowflake object by
// running the appropriate SHOW or DESCRIBE command and returning the result as
// key/value pairs. For STAGE objects it also appends DESCRIBE STAGE properties;
// for MASKING POLICY, ROW ACCESS POLICY, JOIN POLICY, PRIVACY POLICY,
// STORAGE LIFECYCLE POLICY, AGGREGATION POLICY, and PROJECTION POLICY objects it
// appends the DESCRIBE signature, return type, and body (plus the archive
// settings for STORAGE LIFECYCLE POLICY); for PACKAGES POLICY
// objects it appends the DESCRIBE language and allow/block lists; for NETWORK RULE objects it appends the
// DESCRIBE NETWORK RULE value_list; for SERVICE objects the DESCRIBE SERVICE spec
// and dns_name; for STREAMLIT objects the DESCRIBE STREAMLIT root_location and
// main_file.
func GetObjectProperties(ctx context.Context, client *snowflake.Client, database, schema, kind, name string) ([]snowflake.PropertyPair, error) {
	query, err := BuildObjectPropertiesQuery(database, schema, kind, name)
	if err != nil {
		return nil, err
	}

	res, err := client.Execute(ctx, query)
	if err != nil {
		return nil, err
	}
	pairs := snowflake.ResultToPairs(res)

	if strings.ToUpper(kind) == "MASKING POLICY" {
		descRes, err := client.Execute(ctx, BuildDescribeMaskingPolicyQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE MASKING POLICY returns one row whose columns include
			// signature, return_type, and body. Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the values.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "signature", "return_type", "body":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead, matching how the references
					// table renders nulls.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "ROW ACCESS POLICY" {
		descRes, err := client.Execute(ctx, BuildDescribeRowAccessPolicyQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE ROW ACCESS POLICY returns one row whose columns include
			// signature, return_type, and body. Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the values.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "signature", "return_type", "body":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead, matching how the references
					// table renders nulls.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "JOIN POLICY" {
		descRes, err := client.Execute(ctx, BuildDescribeJoinPolicyQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE JOIN POLICY returns one row whose columns include
			// signature, return_type, and body. Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the values.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "signature", "return_type", "body":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead, matching how the references
					// table renders nulls.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "PRIVACY POLICY" {
		descRes, err := client.Execute(ctx, BuildDescribePrivacyPolicyQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE PRIVACY POLICY returns one row whose columns include
			// signature, return_type, and body. Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the values.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "signature", "return_type", "body":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead, matching how the references
					// table renders nulls.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "STORAGE LIFECYCLE POLICY" {
		descRes, err := client.Execute(ctx, BuildDescribeStorageLifecyclePolicyQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE STORAGE LIFECYCLE POLICY returns one row whose columns
			// include signature, return_type, body, and the archive settings. Map
			// by column name so a column reordering on Snowflake's side doesn't
			// mislabel the values; the archive columns are surfaced when present.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "signature", "return_type", "body", "archive_tier", "archive_for_days":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead, matching how the references
					// table renders nulls.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "AGGREGATION POLICY" {
		descRes, err := client.Execute(ctx, BuildDescribeAggregationPolicyQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE AGGREGATION POLICY returns one row whose columns include
			// signature, return_type, and body. Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the values.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "signature", "return_type", "body":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead, matching how the references
					// table renders nulls.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "PROJECTION POLICY" {
		descRes, err := client.Execute(ctx, BuildDescribeProjectionPolicyQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE PROJECTION POLICY returns one row whose columns include
			// signature, return_type, and body. Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the values.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "signature", "return_type", "body":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead, matching how the references
					// table renders nulls.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "PACKAGES POLICY" {
		descRes, err := client.Execute(ctx, BuildDescribePackagesPolicyQuery(database, schema, name))
		if err == nil {
			pairs = appendPackagesPolicyDesc(pairs, descRes)
		}
	}

	if strings.ToUpper(kind) == "NETWORK RULE" {
		descRes, err := client.Execute(ctx, BuildDescribeNetworkRuleQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE NETWORK RULE returns one row whose columns include
			// value_list (the actual identifiers). Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the value.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				if strings.ToLower(col) == "value_list" {
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "SERVICE" {
		descRes, err := client.Execute(ctx, BuildDescribeServiceQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE SERVICE returns one row whose columns include spec (the
			// YAML specification) and dns_name. Map by column name so a column
			// reordering on Snowflake's side doesn't mislabel the values.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "spec", "dns_name":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "CORTEX SEARCH SERVICE" {
		descRes, err := client.Execute(ctx, BuildDescribeCortexSearchServiceQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE CORTEX SEARCH SERVICE returns one row with the rich columns
			// SHOW omits. Map by column name so a column reordering on Snowflake's
			// side doesn't mislabel the values.
			//
			// Some columns whitelisted below (e.g. target_lag / warehouse / comment)
			// may also be returned by SHOW CORTEX SEARCH SERVICES depending on the
			// edition; skip any key already present so the enrichment never produces a
			// duplicate PropertyPair regardless of SHOW's exact column set.
			seen := make(map[string]struct{}, len(pairs))
			for _, p := range pairs {
				seen[strings.ToLower(p.Key)] = struct{}{}
			}
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				lc := strings.ToLower(col)
				switch lc {
				case "search_column", "attribute_columns", "columns", "definition",
					"target_lag", "warehouse", "embedding_model", "service_query_url",
					"source_data_num_rows", "indexing_state", "indexing_error",
					"serving_state", "data_timestamp",
					// Mutable properties surfaced so the properties modal can show the
					// current value next to the ALTER … SET editor for each.
					"primary_key", "auto_suspend", "request_logging",
					"full_index_build_interval_days", "comment":
					if _, dup := seen[lc]; dup {
						continue
					}
					seen[lc] = struct{}{}
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "STREAMLIT" {
		descRes, err := client.Execute(ctx, BuildDescribeStreamlitQuery(database, schema, name))
		if err == nil && len(descRes.Rows) > 0 {
			// DESCRIBE STREAMLIT returns one row whose columns include
			// root_location and main_file (the app's source), which SHOW
			// STREAMLITS omits. Map by column name so a column reordering on
			// Snowflake's side doesn't mislabel the values.
			row := descRes.Rows[0]
			for ci, col := range descRes.Columns {
				if ci >= len(row) {
					break
				}
				switch strings.ToLower(col) {
				case "root_location", "main_file":
					// Guard against a SQL NULL rendering as the literal "<nil>";
					// emit an empty string instead.
					val := ""
					if row[ci] != nil {
						val = fmt.Sprintf("%v", row[ci])
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: col, Value: val})
				}
			}
		}
	}

	if strings.ToUpper(kind) == "STAGE" {
		descRes, err := client.Execute(ctx, BuildDescribeStageQuery(database, schema, name))
		if err == nil {
			for _, row := range descRes.Rows {
				if len(row) >= 4 {
					parent := fmt.Sprintf("%v", row[0]) // parent_property
					prop := fmt.Sprintf("%v", row[1])   // property
					val := fmt.Sprintf("%v", row[3])    // property_value
					key := prop
					if parent != "" && parent != "STAGE_PROPERTIES" && parent != "null" {
						key = parent + "." + prop
					}
					pairs = append(pairs, snowflake.PropertyPair{Key: key, Value: val})
				}
			}
		}
	}

	return pairs, nil
}

// BuildGetColumnCommentsQuery returns the INFORMATION_SCHEMA query that selects
// each column name and its comment, ordered by ordinal position.
func BuildGetColumnCommentsQuery(database, schema, table string) string {
	return fmt.Sprintf(
		`SELECT COLUMN_NAME, COALESCE(COMMENT, '') AS COMMENT`+
			` FROM %s.INFORMATION_SCHEMA.COLUMNS`+
			` WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'`+
			` ORDER BY ORDINAL_POSITION`,
		snowflake.QuoteIdent(database), snowflake.EscapeStringLit(strings.ToUpper(schema)), snowflake.EscapeStringLit(strings.ToUpper(table)),
	)
}

// ParseColumnComments projects a column-comment query result into ColumnComment
// rows (column name + comment), preserving ordinal order.
func ParseColumnComments(res *snowflake.QueryResult) []ColumnComment {
	if res == nil {
		return []ColumnComment{}
	}
	out := make([]ColumnComment, 0, len(res.Rows))
	for _, row := range res.Rows {
		col, cmt := "", ""
		if len(row) > 0 && row[0] != nil {
			col = fmt.Sprint(row[0])
		}
		if len(row) > 1 && row[1] != nil {
			cmt = fmt.Sprint(row[1])
		}
		out = append(out, ColumnComment{Column: col, Comment: cmt})
	}
	return out
}

// GetColumnComments returns the comment for every column in a table, ordered
// by ordinal position.
func GetColumnComments(ctx context.Context, client *snowflake.Client, database, schema, table string) ([]ColumnComment, error) {
	res, err := client.Execute(ctx, BuildGetColumnCommentsQuery(database, schema, table))
	if err != nil {
		return nil, err
	}
	return ParseColumnComments(res), nil
}

// BuildSetColumnCommentSql returns the ALTER TABLE ... MODIFY COLUMN ... COMMENT
// statement that sets (or clears) the comment on a single table column.
func BuildSetColumnCommentSql(database, schema, table, column, comment string) string {
	return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s COMMENT '%s'",
		snowflake.Qualify(database, schema, table),
		snowflake.QuoteIdent(column), snowflake.EscapeStringLit(comment),
	)
}

// SetColumnComment sets (or clears) the COMMENT on a single table column.
func SetColumnComment(ctx context.Context, client *snowflake.Client, database, schema, table, column, comment string) error {
	_, err := client.Execute(ctx, BuildSetColumnCommentSql(database, schema, table, column, comment))
	return err
}
