// SPDX-License-Identifier: GPL-3.0-or-later

package modelmonitor

import (
	"strings"
	"testing"
)

func TestBuildCreateModelMonitorSql(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ModelMonitorConfig
		contains []string
		absent   []string
	}{
		{
			name: "minimal required fields with one prediction column",
			cfg: ModelMonitorConfig{
				Name:                   "MY_MONITOR",
				Model:                  "MY_MODEL",
				Version:                "V1",
				Function:               "predict",
				Source:                 "INFERENCE_TBL",
				Warehouse:              "MY_WH",
				RefreshInterval:        "1 hour",
				AggregationWindow:      "1 day",
				TimestampColumn:        "TS",
				PredictionScoreColumns: []string{"PRED_SCORE"},
			},
			contains: []string{
				"CREATE MODEL MONITOR \"DB\".\"SC\".MY_MONITOR WITH",
				"MODEL = \"DB\".\"SC\".\"MY_MODEL\"",
				"VERSION = 'V1'",
				"FUNCTION = 'predict'",
				"SOURCE = \"DB\".\"SC\".\"INFERENCE_TBL\"",
				"WAREHOUSE = \"MY_WH\"",
				"REFRESH_INTERVAL = '1 hour'",
				"AGGREGATION_WINDOW = '1 day'",
				"TIMESTAMP_COLUMN = TS",
				"PREDICTION_SCORE_COLUMNS = ('PRED_SCORE')",
			},
			absent: []string{
				"OR REPLACE", "IF NOT EXISTS", "BASELINE", "ID_COLUMNS",
				"PREDICTION_CLASS_COLUMNS", "ACTUAL_CLASS_COLUMNS",
				"SEGMENT_COLUMNS", "CUSTOM_METRIC_COLUMNS",
			},
		},
		{
			name: "all optional parameters",
			cfg: ModelMonitorConfig{
				Name:                   "EVERYTHING",
				OrReplace:              true,
				Model:                  "M",
				Version:                "V2",
				Function:               "predict_proba",
				Source:                 "SRC",
				Warehouse:              "WH",
				RefreshInterval:        "30 minutes",
				AggregationWindow:      "7 days",
				TimestampColumn:        "EVENT_TS",
				Baseline:               "BASE_TBL",
				IDColumns:              []string{"ID", "REGION"},
				PredictionClassColumns: []string{"PRED_CLASS"},
				PredictionScoreColumns: []string{"PRED_SCORE"},
				ActualClassColumns:     []string{"ACTUAL_CLASS"},
				ActualScoreColumns:     []string{"ACTUAL_SCORE"},
				SegmentColumns:         []string{"SEG_A", "SEG_B"},
				CustomMetricColumns:    []string{"CUSTOM_1"},
			},
			contains: []string{
				"CREATE OR REPLACE MODEL MONITOR \"DB\".\"SC\".EVERYTHING WITH",
				"MODEL = \"DB\".\"SC\".\"M\"",
				"SOURCE = \"DB\".\"SC\".\"SRC\"",
				"BASELINE = \"DB\".\"SC\".\"BASE_TBL\"",
				"ID_COLUMNS = ('ID', 'REGION')",
				"PREDICTION_CLASS_COLUMNS = ('PRED_CLASS')",
				"PREDICTION_SCORE_COLUMNS = ('PRED_SCORE')",
				"ACTUAL_CLASS_COLUMNS = ('ACTUAL_CLASS')",
				"ACTUAL_SCORE_COLUMNS = ('ACTUAL_SCORE')",
				"SEGMENT_COLUMNS = ('SEG_A', 'SEG_B')",
				"CUSTOM_METRIC_COLUMNS = ('CUSTOM_1')",
			},
			absent: []string{"IF NOT EXISTS"},
		},
		{
			name: "or replace wins over if not exists",
			cfg: ModelMonitorConfig{
				Name:        "BOTH",
				OrReplace:   true,
				IfNotExists: true,
			},
			contains: []string{"CREATE OR REPLACE MODEL MONITOR \"DB\".\"SC\".BOTH WITH"},
			absent:   []string{"IF NOT EXISTS"},
		},
		{
			name: "if not exists alone",
			cfg: ModelMonitorConfig{
				Name:        "INE",
				IfNotExists: true,
			},
			contains: []string{"CREATE MODEL MONITOR IF NOT EXISTS \"DB\".\"SC\".INE WITH"},
			absent:   []string{"OR REPLACE"},
		},
		{
			name: "case-sensitive name is quoted",
			cfg: ModelMonitorConfig{
				Name:          "MixedCase",
				CaseSensitive: true,
			},
			contains: []string{"\"DB\".\"SC\".\"MixedCase\""},
		},
		{
			name: "blank required fields render placeholders",
			cfg:  ModelMonitorConfig{},
			contains: []string{
				"CREATE MODEL MONITOR \"DB\".\"SC\".model_monitor_name WITH",
				"MODEL = model_name",
				"VERSION = 'version_name'",
				"FUNCTION = 'function_name'",
				"SOURCE = source_table",
				"WAREHOUSE = warehouse_name",
				"REFRESH_INTERVAL = '1 day'",
				"AGGREGATION_WINDOW = '1 day'",
				"TIMESTAMP_COLUMN = timestamp_column",
			},
		},
		{
			name: "blank tokens in arrays are dropped, all-blank array omitted",
			cfg: ModelMonitorConfig{
				Name:                   "T",
				IDColumns:              []string{"", "  ", "ONLY_ONE"},
				SegmentColumns:         []string{"", "   "},
				PredictionScoreColumns: []string{"PS"},
			},
			contains: []string{"ID_COLUMNS = ('ONLY_ONE')"},
			absent:   []string{"SEGMENT_COLUMNS"},
		},
		{
			name: "string-literal fields escape single quotes",
			cfg: ModelMonitorConfig{
				Name:    "Q",
				Version: "v'1",
			},
			contains: []string{"VERSION = 'v''1'"},
		},
		{
			name: "column-array elements escape single quotes",
			cfg: ModelMonitorConfig{
				Name:                   "E",
				PredictionScoreColumns: []string{"od'd", "plain"},
			},
			contains: []string{"PREDICTION_SCORE_COLUMNS = ('od''d', 'plain')"},
		},
		{
			name: "timestamp column stays bare for a plain identifier",
			cfg: ModelMonitorConfig{
				Name:            "B",
				TimestampColumn: "event_ts",
			},
			contains: []string{"TIMESTAMP_COLUMN = event_ts"},
		},
		{
			name: "timestamp column is quoted only when it needs quoting",
			cfg: ModelMonitorConfig{
				Name:            "R",
				TimestampColumn: "select",
			},
			contains: []string{"TIMESTAMP_COLUMN = \"select\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, err := BuildCreateModelMonitorSql("DB", "SC", tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(sql, ";") {
				t.Errorf("expected trailing semicolon, got:\n%s", sql)
			}
			for _, want := range tt.contains {
				if !strings.Contains(sql, want) {
					t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
				}
			}
			for _, bad := range tt.absent {
				if strings.Contains(sql, bad) {
					t.Errorf("expected SQL to NOT contain %q, got:\n%s", bad, sql)
				}
			}
		})
	}
}
