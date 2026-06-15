// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"fmt"
	"strings"

	"thaw/internal/apperrors"
	"thaw/internal/snowflake"
)

// AlterService runs an ALTER SERVICE statement for the given service. clause is
// everything that follows the service name, e.g. "SUSPEND", "RESUME",
// "SET MIN_INSTANCES = 2", or "UNSET COMMENT". Services cannot be renamed, so no
// RENAME clause is ever issued. The caller is responsible for correct SQL
// quoting inside the clause; this method only double-quotes the service
// identifier.
func (a *App) AlterService(database, schema, name, clause string) error {
	if a.client == nil {
		return apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("ALTER SERVICE %s.%s.%s %s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name), clause)
	_, err := a.client.Execute(a.ctx, sql)
	return err
}

// ListServiceEndpoints returns the ingress endpoints exposed by the given
// service via SHOW ENDPOINTS IN SERVICE. The raw QueryResult is returned so the
// properties panel can render every column the Snowflake edition reports
// (typically name, port, protocol, ingress_enabled, ingress_url) without the
// backend pinning a fixed shape.
func (a *App) ListServiceEndpoints(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW ENDPOINTS IN SERVICE %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	return a.client.Execute(a.ctx, sql)
}

// GetServiceContainers returns the per-instance container status for the given
// service via SHOW SERVICE CONTAINERS IN SERVICE (the supported replacement for
// the deprecated SYSTEM$GET_SERVICE_STATUS). The raw QueryResult is returned so
// the properties panel can render every column the Snowflake edition reports
// (typically instance_id, container_name, status, message, image_name).
func (a *App) GetServiceContainers(database, schema, name string) (*snowflake.QueryResult, error) {
	if a.client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("SHOW SERVICE CONTAINERS IN SERVICE %s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	return a.client.Execute(a.ctx, sql)
}

// GetServiceLogs returns the container logs for a single service instance via
// SYSTEM$GET_SERVICE_LOGS('<fqn>', <instance_id>, '<container>'[, <num_lines>]).
// instanceID is the 0-based service instance index and containerName is the
// container name from the service spec; numLines, when > 0, caps the number of
// trailing log lines returned. The function returns the log text as a single
// string (Snowflake returns the logs in one cell).
func (a *App) GetServiceLogs(database, schema, name, containerName string, instanceID, numLines int) (string, error) {
	if a.client == nil {
		return "", apperrors.ErrNotConnected
	}
	fqn := fmt.Sprintf("%s.%s.%s",
		snowflake.QuoteIdent(database), snowflake.QuoteIdent(schema), snowflake.QuoteIdent(name))
	// fqn becomes a string-literal argument, so single-quote-escape it.
	fqnLit := strings.ReplaceAll(fqn, "'", "''")
	containerLit := snowflake.EscapeStringLit(containerName)

	var sql string
	if numLines > 0 {
		sql = fmt.Sprintf("SELECT SYSTEM$GET_SERVICE_LOGS('%s', %d, '%s', %d)",
			fqnLit, instanceID, containerLit, numLines)
	} else {
		sql = fmt.Sprintf("SELECT SYSTEM$GET_SERVICE_LOGS('%s', %d, '%s')",
			fqnLit, instanceID, containerLit)
	}

	res, err := a.client.Execute(a.ctx, sql)
	if err != nil {
		return "", err
	}
	if len(res.Rows) == 0 || len(res.Rows[0]) == 0 || res.Rows[0][0] == nil {
		return "", nil
	}
	return fmt.Sprintf("%v", res.Rows[0][0]), nil
}
