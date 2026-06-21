// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package modelmonitor

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ModelMonitorConfig holds the parameters for creating a Snowflake MODEL MONITOR
// object.
//
// The eight required parameters (Model, Version, Function, Source, Warehouse,
// RefreshInterval, AggregationWindow, TimestampColumn) map to the WITH-clause
// fields of the same name. The remaining fields are optional: Baseline and the
// column-array parameters. At least one prediction column (a score or a class
// column) is required by Snowflake; the create modal enforces this before the
// statement is run.
//
// Quoting differs per field and follows the published grammar exactly:
//   - Model, Warehouse, TimestampColumn are identifiers (emitted verbatim).
//   - Source and Baseline are table/view references; they are fully qualified
//     with the monitor's own database & schema (the create modal only offers
//     objects from db.schema) so creation works even when the session's current
//     schema differs from the monitor's target schema.
//   - Version, Function, RefreshInterval, AggregationWindow are string literals
//     (single-quoted).
//   - The column arrays are parenthesised, comma-separated identifier lists.
type ModelMonitorConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`

	// Required WITH-clause parameters.
	Model             string `json:"model"`             // MODEL = <ident>
	Version           string `json:"version"`           // VERSION = '<lit>'
	Function          string `json:"function"`          // FUNCTION = '<lit>'
	Source            string `json:"source"`            // SOURCE = <db.schema.ident>
	Warehouse         string `json:"warehouse"`         // WAREHOUSE = <ident>
	RefreshInterval   string `json:"refreshInterval"`   // REFRESH_INTERVAL = '<lit>'
	AggregationWindow string `json:"aggregationWindow"` // AGGREGATION_WINDOW = '<lit>'
	TimestampColumn   string `json:"timestampColumn"`   // TIMESTAMP_COLUMN = <ident>

	// Optional WITH-clause parameters.
	Baseline               string   `json:"baseline"`               // BASELINE = <db.schema.ident>
	IDColumns              []string `json:"idColumns"`              // ID_COLUMNS = (cols)
	PredictionClassColumns []string `json:"predictionClassColumns"` // PREDICTION_CLASS_COLUMNS = (cols)
	PredictionScoreColumns []string `json:"predictionScoreColumns"` // PREDICTION_SCORE_COLUMNS = (cols)
	ActualClassColumns     []string `json:"actualClassColumns"`     // ACTUAL_CLASS_COLUMNS = (cols)
	ActualScoreColumns     []string `json:"actualScoreColumns"`     // ACTUAL_SCORE_COLUMNS = (cols)
	SegmentColumns         []string `json:"segmentColumns"`         // SEGMENT_COLUMNS = (cols)
	CustomMetricColumns    []string `json:"customMetricColumns"`    // CUSTOM_METRIC_COLUMNS = (cols)
}

// columnArray renders a parenthesised, comma-separated identifier list for the
// column-array parameters, e.g. "(ID, REGION)". Blank tokens are skipped; an
// all-blank list yields "" so the caller omits the parameter entirely.
func columnArray(cols []string) string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		c = strings.TrimSpace(c)
		if c != "" {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return "(" + strings.Join(out, ", ") + ")"
}

// BuildCreateModelMonitorSql constructs a CREATE MODEL MONITOR statement from the
// given config. When required fields are blank the builder substitutes
// placeholders so the live preview reads as a completable template rather than
// invalid SQL. OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
// the create modal prevents selecting both, and if both are set here OR REPLACE
// wins (IF NOT EXISTS is dropped).
//
//	CREATE [OR REPLACE] MODEL MONITOR [IF NOT EXISTS] <fqn> WITH
//	  MODEL = <model>
//	  VERSION = '<version>'
//	  FUNCTION = '<function>'
//	  SOURCE = <db.schema.source>
//	  WAREHOUSE = <warehouse>
//	  REFRESH_INTERVAL = '<refresh_interval>'
//	  AGGREGATION_WINDOW = '<aggregation_window>'
//	  TIMESTAMP_COLUMN = <timestamp_column>
//	  [ BASELINE = <db.schema.baseline> ]
//	  [ ID_COLUMNS = (cols) ]
//	  [ PREDICTION_CLASS_COLUMNS = (cols) ]
//	  [ PREDICTION_SCORE_COLUMNS = (cols) ]
//	  [ ACTUAL_CLASS_COLUMNS = (cols) ]
//	  [ ACTUAL_SCORE_COLUMNS = (cols) ]
//	  [ SEGMENT_COLUMNS = (cols) ]
//	  [ CUSTOM_METRIC_COLUMNS = (cols) ];
func BuildCreateModelMonitorSql(db, schema string, cfg ModelMonitorConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("MODEL MONITOR", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "model_monitor_name"
	}

	fmt.Fprintf(&sb, "%s %s WITH", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	// orPlaceholder returns the trimmed value, or the placeholder when blank, so
	// the preview reads as a template before the field is filled.
	orPlaceholder := func(v, placeholder string) string {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
		return placeholder
	}

	// Required identifier parameters (emitted verbatim).
	fmt.Fprintf(&sb, "\n  MODEL = %s", orPlaceholder(cfg.Model, "model_name"))
	// Required string-literal parameters.
	fmt.Fprintf(&sb, "\n  VERSION = '%s'", snowflake.EscapeTextLit(orPlaceholder(cfg.Version, "version_name")))
	fmt.Fprintf(&sb, "\n  FUNCTION = '%s'", snowflake.EscapeTextLit(orPlaceholder(cfg.Function, "function_name")))
	// SOURCE is a table/view reference. It is fully qualified with the monitor's
	// own database & schema (the create modal's source picker only offers objects
	// from db.schema) so creation succeeds even when the session's current schema
	// differs from the monitor's target schema. A blank value emits the bare
	// placeholder so the live preview reads as a template.
	if src := strings.TrimSpace(cfg.Source); src != "" {
		fmt.Fprintf(&sb, "\n  SOURCE = %s", snowflake.Qualify(db, schema, src))
	} else {
		fmt.Fprint(&sb, "\n  SOURCE = source_table")
	}
	fmt.Fprintf(&sb, "\n  WAREHOUSE = %s", orPlaceholder(cfg.Warehouse, "warehouse_name"))
	fmt.Fprintf(&sb, "\n  REFRESH_INTERVAL = '%s'", snowflake.EscapeTextLit(orPlaceholder(cfg.RefreshInterval, "1 day")))
	fmt.Fprintf(&sb, "\n  AGGREGATION_WINDOW = '%s'", snowflake.EscapeTextLit(orPlaceholder(cfg.AggregationWindow, "1 day")))
	fmt.Fprintf(&sb, "\n  TIMESTAMP_COLUMN = %s", orPlaceholder(cfg.TimestampColumn, "timestamp_column"))

	// Optional parameters — emitted only when set. BASELINE is a table reference;
	// like SOURCE it is qualified with the monitor's database & schema.
	if b := strings.TrimSpace(cfg.Baseline); b != "" {
		fmt.Fprintf(&sb, "\n  BASELINE = %s", snowflake.Qualify(db, schema, b))
	}
	for _, p := range []struct {
		kw   string
		cols []string
	}{
		{"ID_COLUMNS", cfg.IDColumns},
		{"PREDICTION_CLASS_COLUMNS", cfg.PredictionClassColumns},
		{"PREDICTION_SCORE_COLUMNS", cfg.PredictionScoreColumns},
		{"ACTUAL_CLASS_COLUMNS", cfg.ActualClassColumns},
		{"ACTUAL_SCORE_COLUMNS", cfg.ActualScoreColumns},
		{"SEGMENT_COLUMNS", cfg.SegmentColumns},
		{"CUSTOM_METRIC_COLUMNS", cfg.CustomMetricColumns},
	} {
		if arr := columnArray(p.cols); arr != "" {
			fmt.Fprintf(&sb, "\n  %s = %s", p.kw, arr)
		}
	}

	return sb.String() + ";", nil
}
