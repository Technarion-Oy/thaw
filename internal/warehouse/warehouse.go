// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package warehouse

import (
	"context"
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// WarehouseMeteringRow holds one row from ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY.
type WarehouseMeteringRow struct {
	StartTime                string  `json:"startTime"`
	EndTime                  string  `json:"endTime"`
	WarehouseName            string  `json:"warehouseName"`
	CreditsUsed              float64 `json:"creditsUsed"`
	CreditsUsedCompute       float64 `json:"creditsUsedCompute"`
	CreditsUsedCloudServices float64 `json:"creditsUsedCloudServices"`
}

// BuildAlterWarehousePropertySQL builds an ALTER WAREHOUSE ... SET statement for
// a single property. property must be one of: size, warehouseType, autoSuspend,
// autoResume, comment, maxClusterCount, minClusterCount, scalingPolicy,
// resourceMonitor, enableQueryAcceleration, queryAccelerationMaxScaleFactor,
// maxConcurrencyLevel, statementQueuedTimeout, statementTimeout. Enum and integer
// values are validated before being interpolated into the SQL string.
func BuildAlterWarehousePropertySQL(name, property, value string) (string, error) {
	wh := snowflake.QuoteIdent(name)

	// Shared validators for enum/integer values that are interpolated unquoted
	// into SQL (also used by the internal/users property builder).
	what := fmt.Sprintf("warehouse property %q", property)
	checkEnum := func(v string, allowed ...string) (string, error) {
		return snowflake.ValidateEnumValue(what, v, allowed...)
	}
	validateInt := func(v string) (string, error) {
		return snowflake.ValidateNonNegativeInt(what, v)
	}

	switch property {
	case "size":
		v, err := checkEnum(value,
			"X-SMALL", "SMALL", "MEDIUM", "LARGE", "X-LARGE",
			"2X-LARGE", "3X-LARGE", "4X-LARGE", "5X-LARGE", "6X-LARGE")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET WAREHOUSE_SIZE = %s`, wh, v), nil
	case "warehouseType":
		v, err := checkEnum(value, "STANDARD", "SNOWPARK-OPTIMIZED")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET WAREHOUSE_TYPE = %s`, wh, v), nil
	case "autoSuspend":
		if value == "0" || value == "" {
			return fmt.Sprintf(`ALTER WAREHOUSE %s SET AUTO_SUSPEND = NULL`, wh), nil
		}
		v, err := validateInt(value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET AUTO_SUSPEND = %s`, wh, v), nil
	case "autoResume":
		v, err := checkEnum(value, "TRUE", "FALSE")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET AUTO_RESUME = %s`, wh, v), nil
	case "comment":
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET COMMENT = '%s'`, wh, snowflake.EscapeStringLit(value)), nil
	case "maxClusterCount":
		v, err := validateInt(value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET MAX_CLUSTER_COUNT = %s`, wh, v), nil
	case "minClusterCount":
		v, err := validateInt(value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET MIN_CLUSTER_COUNT = %s`, wh, v), nil
	case "scalingPolicy":
		v, err := checkEnum(value, "STANDARD", "ECONOMY")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET SCALING_POLICY = %s`, wh, v), nil
	case "resourceMonitor":
		if strings.TrimSpace(value) == "" {
			return fmt.Sprintf(`ALTER WAREHOUSE %s SET RESOURCE_MONITOR = NULL`, wh), nil
		}
		return fmt.Sprintf("ALTER WAREHOUSE %s SET RESOURCE_MONITOR = %s", wh, snowflake.QuoteIdent(value)), nil
	case "enableQueryAcceleration":
		v, err := checkEnum(value, "TRUE", "FALSE")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET ENABLE_QUERY_ACCELERATION = %s`, wh, v), nil
	case "queryAccelerationMaxScaleFactor":
		v, err := validateInt(value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET QUERY_ACCELERATION_MAX_SCALE_FACTOR = %s`, wh, v), nil
	case "maxConcurrencyLevel":
		v, err := validateInt(value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET MAX_CONCURRENCY_LEVEL = %s`, wh, v), nil
	case "statementQueuedTimeout":
		v, err := validateInt(value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET STATEMENT_QUEUED_TIMEOUT_IN_SECONDS = %s`, wh, v), nil
	case "statementTimeout":
		v, err := validateInt(value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`ALTER WAREHOUSE %s SET STATEMENT_TIMEOUT_IN_SECONDS = %s`, wh, v), nil
	default:
		return "", fmt.Errorf("unknown warehouse property: %s", property)
	}
}

// AlterProperty applies a single SET property to a warehouse.
func AlterProperty(ctx context.Context, client *snowflake.Client, name, property, value string) error {
	query, err := BuildAlterWarehousePropertySQL(name, property, value)
	if err != nil {
		return err
	}
	_, err = client.Execute(ctx, query)
	return err
}

// Suspend suspends the named warehouse.
func Suspend(ctx context.Context, client *snowflake.Client, name string) error {
	_, err := client.Execute(ctx, fmt.Sprintf("ALTER WAREHOUSE %s SUSPEND", snowflake.QuoteIdent(name)))
	return err
}

// Resume resumes the named warehouse if it is suspended.
func Resume(ctx context.Context, client *snowflake.Client, name string) error {
	_, err := client.Execute(ctx, fmt.Sprintf("ALTER WAREHOUSE %s RESUME IF SUSPENDED", snowflake.QuoteIdent(name)))
	return err
}

// AbortAllQueries issues ABORT ALL QUERIES on the named warehouse.
func AbortAllQueries(ctx context.Context, client *snowflake.Client, name string) error {
	_, err := client.Execute(ctx, fmt.Sprintf("ALTER WAREHOUSE %s ABORT ALL QUERIES", snowflake.QuoteIdent(name)))
	return err
}

// Rename renames a warehouse.
func Rename(ctx context.Context, client *snowflake.Client, name, newName string) error {
	_, err := client.Execute(ctx, fmt.Sprintf("ALTER WAREHOUSE %s RENAME TO %s", snowflake.QuoteIdent(name), snowflake.QuoteIdent(newName)))
	return err
}

// GetParameters returns per-warehouse parameter overrides (MAX_CONCURRENCY_LEVEL,
// STATEMENT_QUEUED_TIMEOUT_IN_SECONDS, STATEMENT_TIMEOUT_IN_SECONDS) sourced from
// SHOW PARAMETERS IN WAREHOUSE.
func GetParameters(ctx context.Context, client *snowflake.Client, name string) ([]snowflake.PropertyPair, error) {
	qr, err := client.Execute(ctx, fmt.Sprintf("SHOW PARAMETERS IN WAREHOUSE %s", snowflake.QuoteIdent(name)))
	if err != nil {
		return nil, err
	}
	want := map[string]bool{
		"MAX_CONCURRENCY_LEVEL":               true,
		"STATEMENT_QUEUED_TIMEOUT_IN_SECONDS": true,
		"STATEMENT_TIMEOUT_IN_SECONDS":        true,
	}
	keyIdx := snowflake.ColIdx(qr.Columns, "key")
	valIdx := snowflake.ColIdx(qr.Columns, "value")
	var result []snowflake.PropertyPair
	for _, row := range qr.Rows {
		if keyIdx < 0 || keyIdx >= len(row) {
			continue
		}
		key := snowflake.CellString(row[keyIdx])
		val := ""
		if valIdx >= 0 && valIdx < len(row) {
			val = snowflake.CellString(row[valIdx])
		}
		if want[strings.ToUpper(key)] {
			result = append(result, snowflake.PropertyPair{Key: key, Value: val})
		}
	}
	return result, nil
}

// BuildMeteringHistoryQuery builds the WAREHOUSE_METERING_HISTORY query with
// optional warehouse-name and date-range filters. Dates must be RFC3339 strings.
func BuildMeteringHistoryQuery(warehouse, startDate, endDate string) string {
	var conds []string
	if warehouse != "" {
		conds = append(conds, fmt.Sprintf("WAREHOUSE_NAME = %s", snowflake.QuoteStringLit(warehouse)))
	}
	if startDate != "" {
		conds = append(conds, fmt.Sprintf("START_TIME >= '%s'::TIMESTAMP_LTZ", startDate))
	}
	if endDate != "" {
		conds = append(conds, fmt.Sprintf("START_TIME < '%s'::TIMESTAMP_LTZ", endDate))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	return fmt.Sprintf(`
SELECT START_TIME, END_TIME, WAREHOUSE_NAME,
       CREDITS_USED, CREDITS_USED_COMPUTE, CREDITS_USED_CLOUD_SERVICES
FROM SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY
%s
ORDER BY START_TIME ASC`, where)
}

// ParseMeteringHistory projects a metering-history query result into WarehouseMeteringRow
// values.
func ParseMeteringHistory(res *snowflake.QueryResult) []WarehouseMeteringRow {
	if res == nil {
		return []WarehouseMeteringRow{}
	}
	startIdx := snowflake.ColIdx(res.Columns, "start_time")
	endIdx := snowflake.ColIdx(res.Columns, "end_time")
	nameIdx := snowflake.ColIdx(res.Columns, "warehouse_name")
	usedIdx := snowflake.ColIdx(res.Columns, "credits_used")
	compIdx := snowflake.ColIdx(res.Columns, "credits_used_compute")
	cloudIdx := snowflake.ColIdx(res.Columns, "credits_used_cloud_services")

	cell := func(row []interface{}, idx int) interface{} {
		if idx < 0 || idx >= len(row) {
			return nil
		}
		return row[idx]
	}

	rows := make([]WarehouseMeteringRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		rows = append(rows, WarehouseMeteringRow{
			StartTime:                snowflake.CellString(cell(row, startIdx)),
			EndTime:                  snowflake.CellString(cell(row, endIdx)),
			WarehouseName:            snowflake.CellString(cell(row, nameIdx)),
			CreditsUsed:              snowflake.CellFloat(cell(row, usedIdx)),
			CreditsUsedCompute:       snowflake.CellFloat(cell(row, compIdx)),
			CreditsUsedCloudServices: snowflake.CellFloat(cell(row, cloudIdx)),
		})
	}
	return rows
}

// GetMeteringHistory returns hourly credit usage records from
// SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY, ordered by START_TIME
// ascending. warehouse, startDate, and endDate are optional filters.
func GetMeteringHistory(ctx context.Context, client *snowflake.Client, warehouse, startDate, endDate string) ([]WarehouseMeteringRow, error) {
	res, err := client.QuerySingle(ctx, BuildMeteringHistoryQuery(warehouse, startDate, endDate))
	if err != nil {
		return nil, err
	}
	return ParseMeteringHistory(res), nil
}
