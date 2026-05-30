// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// thaw:file-domain: Core IPC & App Lifecycle
package main

import (
	"fmt"
	"strconv"
	"strings"
	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
	"time"
)

// GetRoleDDL returns the DDL definition of a Snowflake role.
func (a *App) GetRoleDDL(name string) (string, error) {
	if a.client == nil {
		return "", apperrors.ErrNotConnected
	}
	return a.client.GetRoleDDL(a.ctx, name)
}

// GetWarehouseDDL returns the DDL definition of a Snowflake warehouse.
func (a *App) GetWarehouseDDL(name string) (string, error) {
	if a.client == nil {
		return "", apperrors.ErrNotConnected
	}
	return a.client.GetWarehouseDDL(a.ctx, name)
}

// AlterWarehouseProperty applies a single SET property to a warehouse.
// property must be one of: size, warehouseType, autoSuspend, autoResume, comment,
// maxClusterCount, minClusterCount, scalingPolicy, resourceMonitor,
// enableQueryAcceleration, queryAccelerationMaxScaleFactor,
// maxConcurrencyLevel, statementQueuedTimeout, statementTimeout.
func (a *App) AlterWarehouseProperty(name, property, value string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	wh := snowflake.QuoteIdent(name)

	// allowlist checks for enum-typed values that are interpolated unquoted into SQL.
	checkEnum := func(v string, allowed ...string) (string, error) {
		u := strings.ToUpper(strings.TrimSpace(v))
		for _, a := range allowed {
			if u == a {
				return u, nil
			}
		}
		return "", fmt.Errorf("invalid value %q for warehouse property %q", v, property)
	}
	// validateInt parses v as a non-negative integer and returns it as a string safe for SQL interpolation.
	validateInt := func(v string) (string, error) {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 0 {
			return "", fmt.Errorf("invalid integer value %q for warehouse property %q", v, property)
		}
		return strconv.Itoa(n), nil
	}

	var query string
	switch property {
	case "size":
		v, err := checkEnum(value,
			"X-SMALL", "SMALL", "MEDIUM", "LARGE", "X-LARGE",
			"2X-LARGE", "3X-LARGE", "4X-LARGE", "5X-LARGE", "6X-LARGE")
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET WAREHOUSE_SIZE = %s`, wh, v)
	case "warehouseType":
		v, err := checkEnum(value, "STANDARD", "SNOWPARK-OPTIMIZED")
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET WAREHOUSE_TYPE = %s`, wh, v)
	case "autoSuspend":
		if value == "0" || value == "" {
			query = fmt.Sprintf(`ALTER WAREHOUSE %s SET AUTO_SUSPEND = NULL`, wh)
		} else {
			v, err := validateInt(value)
			if err != nil {
				return err
			}
			query = fmt.Sprintf(`ALTER WAREHOUSE %s SET AUTO_SUSPEND = %s`, wh, v)
		}
	case "autoResume":
		v, err := checkEnum(value, "TRUE", "FALSE")
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET AUTO_RESUME = %s`, wh, v)
	case "comment":
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET COMMENT = '%s'`, wh, snowflake.EscapeStringLit(value))
	case "maxClusterCount":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET MAX_CLUSTER_COUNT = %s`, wh, v)
	case "minClusterCount":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET MIN_CLUSTER_COUNT = %s`, wh, v)
	case "scalingPolicy":
		v, err := checkEnum(value, "STANDARD", "ECONOMY")
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET SCALING_POLICY = %s`, wh, v)
	case "resourceMonitor":
		if strings.TrimSpace(value) == "" {
			query = fmt.Sprintf(`ALTER WAREHOUSE %s SET RESOURCE_MONITOR = NULL`, wh)
		} else {
			query = fmt.Sprintf("ALTER WAREHOUSE %s SET RESOURCE_MONITOR = %s", wh, snowflake.QuoteIdent(value))
		}
	case "enableQueryAcceleration":
		v, err := checkEnum(value, "TRUE", "FALSE")
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET ENABLE_QUERY_ACCELERATION = %s`, wh, v)
	case "queryAccelerationMaxScaleFactor":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET QUERY_ACCELERATION_MAX_SCALE_FACTOR = %s`, wh, v)
	case "maxConcurrencyLevel":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET MAX_CONCURRENCY_LEVEL = %s`, wh, v)
	case "statementQueuedTimeout":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET STATEMENT_QUEUED_TIMEOUT_IN_SECONDS = %s`, wh, v)
	case "statementTimeout":
		v, err := validateInt(value)
		if err != nil {
			return err
		}
		query = fmt.Sprintf(`ALTER WAREHOUSE %s SET STATEMENT_TIMEOUT_IN_SECONDS = %s`, wh, v)
	default:
		return fmt.Errorf("unknown warehouse property: %s", property)
	}
	_, err := a.client.Execute(a.ctx, query)
	return err
}

// AlterWarehouseSuspend suspends the named warehouse.
func (a *App) AlterWarehouseSuspend(name string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("ALTER WAREHOUSE %s SUSPEND", snowflake.QuoteIdent(name)))
	return err
}

// AlterWarehouseResume resumes the named warehouse if it is suspended.
func (a *App) AlterWarehouseResume(name string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("ALTER WAREHOUSE %s RESUME IF SUSPENDED", snowflake.QuoteIdent(name)))
	return err
}

// AlterWarehouseAbortAllQueries issues ABORT ALL QUERIES on the named warehouse.
func (a *App) AlterWarehouseAbortAllQueries(name string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("ALTER WAREHOUSE %s ABORT ALL QUERIES", snowflake.QuoteIdent(name)))
	return err
}

// AlterWarehouseRename renames a warehouse and returns the new name.
func (a *App) AlterWarehouseRename(name, newName string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	_, err := a.client.Execute(a.ctx, fmt.Sprintf("ALTER WAREHOUSE %s RENAME TO %s", snowflake.QuoteIdent(name), snowflake.QuoteIdent(newName)))
	return err
}

// GetWarehouseParameters returns per-warehouse parameter overrides (MAX_CONCURRENCY_LEVEL,
// STATEMENT_QUEUED_TIMEOUT_IN_SECONDS, STATEMENT_TIMEOUT_IN_SECONDS) sourced from
// SHOW PARAMETERS IN WAREHOUSE. The returned map key is the parameter name.
func (a *App) GetWarehouseParameters(name string) ([]PropertyPair, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	qr, err := a.client.Execute(a.ctx, fmt.Sprintf("SHOW PARAMETERS IN WAREHOUSE %s", snowflake.QuoteIdent(name)))
	if err != nil {
		return nil, err
	}
	want := map[string]bool{
		"MAX_CONCURRENCY_LEVEL":               true,
		"STATEMENT_QUEUED_TIMEOUT_IN_SECONDS": true,
		"STATEMENT_TIMEOUT_IN_SECONDS":        true,
	}
	// Find column indices for "key" and "value".
	keyIdx, valIdx := -1, -1
	for i, c := range qr.Columns {
		switch strings.ToLower(c) {
		case "key":
			keyIdx = i
		case "value":
			valIdx = i
		}
	}
	var result []PropertyPair
	for _, row := range qr.Rows {
		if keyIdx < 0 || keyIdx >= len(row) {
			continue
		}
		key := fmt.Sprint(row[keyIdx])
		val := ""
		if valIdx >= 0 && valIdx < len(row) && row[valIdx] != nil {
			val = fmt.Sprint(row[valIdx])
		}
		if want[strings.ToUpper(key)] {
			result = append(result, PropertyPair{Key: key, Value: val})
		}
	}
	return result, nil
}

// WarehouseMeteringRow holds one row from ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY.
type WarehouseMeteringRow struct {
	StartTime                string  `json:"startTime"`
	EndTime                  string  `json:"endTime"`
	WarehouseName            string  `json:"warehouseName"`
	CreditsUsed              float64 `json:"creditsUsed"`
	CreditsUsedCompute       float64 `json:"creditsUsedCompute"`
	CreditsUsedCloudServices float64 `json:"creditsUsedCloudServices"`
}

// GetQueryHistory queries Snowflake's INFORMATION_SCHEMA.QUERY_HISTORY* table
// functions and returns a slice of QueryHistoryRow ordered by start time desc.
//
// filterType:          "session" | "user" | "warehouse" | "all"
// sessionID:           non-empty → SESSION_ID => <id>  (used when filterType="session")
// userName:            non-empty → USER_NAME => '<name>'
// warehouseName:       non-empty → WAREHOUSE_NAME => '<name>'
// endTimeStart/End:    RFC3339 strings or "" for no filter
// resultLimit:         max rows returned (1–10 000)
// includeClientGenerated: include client-generated statements
// CanViewWarehouseMeteringHistory returns true when the current role has SELECT
// access to SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY.  It runs a
// zero-row probe query so it is fast and never touches real data.
func (a *App) CanViewWarehouseMeteringHistory() (bool, error) {
	if a.client == nil {
		return false, apperrors.ErrNotConnected
	}
	_, err := a.client.QuerySingle(a.ctx,
		"SELECT 1 FROM SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY LIMIT 0")
	if err != nil {
		return false, nil //nolint:nilerr // permission denied is not a caller error
	}
	return true, nil
}

// GetWarehouseMeteringHistory returns hourly credit usage records from
// SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY. Rows are ordered by
// START_TIME ascending. warehouse, startDate, and endDate are all optional
// filters; dates must be RFC3339 strings when provided.
func (a *App) GetWarehouseMeteringHistory(warehouse, startDate, endDate string) ([]WarehouseMeteringRow, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	var conds []string
	if warehouse != "" {
		conds = append(conds, fmt.Sprintf("WAREHOUSE_NAME = '%s'", strings.ReplaceAll(warehouse, "'", "''")))
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
	query := fmt.Sprintf(`
SELECT START_TIME, END_TIME, WAREHOUSE_NAME,
       CREDITS_USED, CREDITS_USED_COMPUTE, CREDITS_USED_CLOUD_SERVICES
FROM SNOWFLAKE.ACCOUNT_USAGE.WAREHOUSE_METERING_HISTORY
%s
ORDER BY START_TIME ASC`, where)

	res, err := a.client.QuerySingle(a.ctx, query)
	if err != nil {
		return nil, err
	}

	startIdx := colIdx(res.Columns, "start_time")
	endIdx := colIdx(res.Columns, "end_time")
	nameIdx := colIdx(res.Columns, "warehouse_name")
	usedIdx := colIdx(res.Columns, "credits_used")
	compIdx := colIdx(res.Columns, "credits_used_compute")
	cloudIdx := colIdx(res.Columns, "credits_used_cloud_services")

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch t := v.(type) {
		case time.Time:
			return t.Format(time.RFC3339)
		default:
			return fmt.Sprint(v)
		}
	}
	toFloat := func(v interface{}) float64 {
		if v == nil {
			return 0
		}
		switch t := v.(type) {
		case float64:
			return t
		case float32:
			return float64(t)
		case []byte:
			f, _ := strconv.ParseFloat(string(t), 64)
			return f
		case string:
			f, _ := strconv.ParseFloat(t, 64)
			return f
		}
		return 0
	}

	rows := make([]WarehouseMeteringRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		rows = append(rows, WarehouseMeteringRow{
			StartTime:                toString(row[startIdx]),
			EndTime:                  toString(row[endIdx]),
			WarehouseName:            toString(row[nameIdx]),
			CreditsUsed:              toFloat(row[usedIdx]),
			CreditsUsedCompute:       toFloat(row[compIdx]),
			CreditsUsedCloudServices: toFloat(row[cloudIdx]),
		})
	}
	return rows, nil
}
