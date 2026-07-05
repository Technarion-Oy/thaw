// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package fileformat

import (
	"context"
	"fmt"
	"time"

	"thaw/internal/snowflake"
)

// PreviewStageFile queries a Snowflake stage file with an inline FILE_FORMAT
// derived from cfg and returns up to 50 rows. stagePath must be a fully
// qualified stage reference, e.g. "@DB.SCHEMA.STAGE/path/to/file.csv".
func PreviewStageFile(ctx context.Context, client *snowflake.Client, stagePath string, cfg FileFormatConfig) (PreviewResult, error) {
	// stagePath is spliced unquoted into SELECT ... FROM <stagePath>; guard it
	// against injection (see snowflake.ValidateStageRef).
	if err := snowflake.ValidateStageRef(stagePath); err != nil {
		return PreviewResult{}, err
	}

	// Snowflake SELECT queries ignore PARSE_HEADER=TRUE for naming columns (it skips the row but leaves columns as $1, $2).
	// To provide a useful preview that looks like the target table, if ParseHeader is true,
	// we turn it off for the query, fetch one extra row, and use the first returned row as our column headers.
	parseHeader := cfg.ParseHeader
	queryCfg := cfg
	if parseHeader {
		queryCfg.ParseHeader = false
	}

	inline := BuildInlineFileFormat(queryCfg)
	limit := 50
	if parseHeader {
		limit = 51
	}

	// Use a temporary file format to avoid "Table function argument is required to be a constant" errors.
	formatName := fmt.Sprintf("THAW_PREVIEW_%d", time.Now().UnixNano())
	createSql := BuildCreateTemporaryFileFormatSql(formatName, queryCfg)
	selectSql := fmt.Sprintf("SELECT * FROM %s (FILE_FORMAT => '%s') LIMIT %d;", stagePath, formatName, limit)

	// Execute both statements. Execute returns the last result set.
	result, err := client.Execute(ctx, createSql+"\n"+selectSql)

	// Clean up the temporary format (best effort)
	defer func() {
		_, _ = client.Execute(ctx, fmt.Sprintf("DROP FILE_FORMAT IF EXISTS %s;", formatName))
	}()

	if err != nil {
		// Fallback: if the temporary format failed (e.g. no active database), try inline.
		// Some Snowflake accounts/configurations might still support inline formats
		// and this provides a last-resort recovery.
		fallbackQuery := fmt.Sprintf("SELECT * FROM %s (FILE_FORMAT => (%s)) LIMIT %d;", stagePath, inline, limit)
		var fallbackErr error
		result, fallbackErr = client.QuerySingle(ctx, fallbackQuery)
		if fallbackErr != nil {
			return PreviewResult{Error: err.Error()}, nil // return the original error
		}
	}

	if result == nil || len(result.Rows) == 0 {
		return PreviewResult{Columns: []string{}, Rows: []map[string]string{}}, nil
	}

	cols := result.Columns
	dataRows := result.Rows

	if parseHeader {
		headerRow := result.Rows[0]
		extractedCols := make([]string, len(headerRow))
		for i, val := range headerRow {
			if val != nil {
				extractedCols[i] = fmt.Sprintf("%v", val)
			} else {
				extractedCols[i] = fmt.Sprintf("COLUMN_%d", i+1)
			}
		}
		cols = extractedCols
		dataRows = result.Rows[1:]
	}

	rows := make([]map[string]string, 0, len(dataRows))
	for _, raw := range dataRows {
		row := make(map[string]string, len(cols))
		for i, col := range cols {
			if i < len(raw) && raw[i] != nil {
				row[col] = fmt.Sprintf("%v", raw[i])
			}
		}
		rows = append(rows, row)
	}
	return PreviewResult{Columns: cols, Rows: rows}, nil
}
