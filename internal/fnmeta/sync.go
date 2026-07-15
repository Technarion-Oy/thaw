// SPDX-License-Identifier: GPL-3.0-or-later

package fnmeta

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// fetchBuiltins is retained for reference but not called by SyncFromSnowflake —
// see the comment there for why. It may be useful for diagnostics.
var _ = fetchBuiltins

// SyncFromSnowflake syncs user-defined functions from the live Snowflake
// connection into the local SQLite cache.
//
// Built-in functions are intentionally NOT synced here. Snowflake's
// SHOW FUNCTIONS returns type-only signatures (e.g. "ABS(NUMBER) RETURN NUMBER")
// with no parameter names, which would create duplicate low-quality entries
// alongside the embedded fallback catalog that has proper named signatures
// (e.g. "ABS(expr FLOAT) RETURN FLOAT"). The fallback covers all standard
// built-ins; UDFs are synced live because Snowflake does include parameter
// names for user-defined functions.
func SyncFromSnowflake(ctx context.Context, client *snowflake.Client, store *Store) error {
	// UDFs are best-effort — failure is silently ignored.
	if udfs, err := fetchUDFs(ctx, client); err == nil {
		_ = store.Upsert(udfs) //nolint:errcheck
	}
	return nil
}

// fetchBuiltins executes SHOW FUNCTIONS and returns only the built-in entries.
// The gosnowflake driver returns SHOW rows directly (no RESULT_SCAN needed),
// so we filter is_builtin = 'Y' in Go to avoid session-affinity issues with
// LAST_QUERY_ID() across pooled connections.
func fetchBuiltins(ctx context.Context, client *snowflake.Client) ([]FunctionMeta, error) {
	result, err := client.Execute(ctx, "SHOW FUNCTIONS")
	if err != nil {
		return nil, err
	}
	return toFunctionMeta(result, "BUILTIN", "is_builtin", "Y"), nil
}

// fetchUDFs executes SHOW USER FUNCTIONS and returns all entries as UDFs.
func fetchUDFs(ctx context.Context, client *snowflake.Client) ([]FunctionMeta, error) {
	result, err := client.Execute(ctx, "SHOW USER FUNCTIONS")
	if err != nil {
		return nil, err
	}
	return toFunctionMeta(result, "UDF", "", ""), nil
}

// toFunctionMeta converts a raw QueryResult from a SHOW command into a
// []FunctionMeta slice. Columns are located by name (case-insensitive) so
// column order does not matter. When filterCol/filterVal are non-empty, only
// rows where that column equals filterVal are included.
func toFunctionMeta(result *snowflake.QueryResult, fnType, filterCol, filterVal string) []FunctionMeta {
	if result == nil || len(result.Rows) == 0 {
		return nil
	}
	nameIdx, sigIdx, descIdx, filterIdx := -1, -1, -1, -1
	for i, col := range result.Columns {
		switch strings.ToLower(col) {
		case "name":
			nameIdx = i
		case "arguments":
			sigIdx = i
		case "description":
			descIdx = i
		default:
			if filterCol != "" && strings.EqualFold(col, filterCol) {
				filterIdx = i
			}
		}
	}
	if nameIdx < 0 || sigIdx < 0 {
		return nil
	}
	metas := make([]FunctionMeta, 0, len(result.Rows))
	for _, row := range result.Rows {
		if nameIdx >= len(row) || sigIdx >= len(row) {
			continue
		}
		// Apply row filter if requested.
		if filterIdx >= 0 && filterIdx < len(row) {
			if fmt.Sprint(row[filterIdx]) != filterVal {
				continue
			}
		}
		m := FunctionMeta{
			FunctionName:      strings.ToUpper(fmt.Sprint(row[nameIdx])),
			FunctionSignature: fmt.Sprint(row[sigIdx]),
			FunctionType:      fnType,
		}
		if descIdx >= 0 && descIdx < len(row) && row[descIdx] != nil {
			m.Description = fmt.Sprint(row[descIdx])
		}
		metas = append(metas, m)
	}
	return metas
}
