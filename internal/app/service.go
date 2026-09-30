// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"
	"strings"

	"thaw/internal/apperrors"
	"thaw/internal/service"
	"thaw/internal/snowflake"
)

// AlterService runs an ALTER SERVICE statement for the given service. clause is
// everything that follows the service name, e.g. "SUSPEND", "RESUME",
// "SET MIN_INSTANCES = 2", or "UNSET COMMENT". Services cannot be renamed, so no
// RENAME clause is ever issued. The caller is responsible for correct SQL
// quoting inside the clause; this method only double-quotes the service
// identifier.
func (a *App) AlterService(database, schema, name, clause string) error {
	return a.alterObject("SERVICE", database, schema, name, clause)
}

// ListServiceEndpoints returns the ingress endpoints exposed by the given
// service via SHOW ENDPOINTS IN SERVICE. The raw QueryResult is returned so the
// properties panel can render every column the Snowflake edition reports
// (typically name, port, protocol, ingress_enabled, ingress_url) without the
// backend pinning a fixed shape.
func (a *App) ListServiceEndpoints(database, schema, name string) (*snowflake.QueryResult, error) {
	return a.showInService("SHOW ENDPOINTS IN SERVICE", database, schema, name)
}

// showInService runs `<show> <fqn>` for a service-scoped SHOW command and
// returns the raw QueryResult, so the properties panel renders every column
// the Snowflake edition reports without the backend pinning a fixed shape.
func (a *App) showInService(show, database, schema, name string) (*snowflake.QueryResult, error) {
	return a.execObjectSQL(show + " " + snowflake.Qualify(database, schema, name))
}

// execObjectSQL runs one statement on the current connection under the
// object-editor feature context.
func (a *App) execObjectSQL(sql string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}

// GetServiceContainers returns the per-instance container status for the given
// service via SHOW SERVICE CONTAINERS IN SERVICE (the supported replacement for
// the deprecated SYSTEM$GET_SERVICE_STATUS). The raw QueryResult is returned so
// the properties panel can render every column the Snowflake edition reports
// (typically instance_id, container_name, status, message, image_name).
func (a *App) GetServiceContainers(database, schema, name string) (*snowflake.QueryResult, error) {
	return a.showInService("SHOW SERVICE CONTAINERS IN SERVICE", database, schema, name)
}

// ListServiceInstances returns the per-instance status of the given service via
// SHOW SERVICE INSTANCES IN SERVICE (typically instance_id, status,
// spec_digest, creation_time, start_time).
func (a *App) ListServiceInstances(database, schema, name string) (*snowflake.QueryResult, error) {
	return a.showInService("SHOW SERVICE INSTANCES IN SERVICE", database, schema, name)
}

// ListServiceVolumes returns the volumes mounted by the given service's
// containers via SHOW SERVICE VOLUMES IN SERVICE (typically volume_name,
// instance_id, container_name, volume_type, size, iops, throughput).
func (a *App) ListServiceVolumes(database, schema, name string) (*snowflake.QueryResult, error) {
	return a.showInService("SHOW SERVICE VOLUMES IN SERVICE", database, schema, name)
}

// ListServiceRoles returns the service roles declared in the given service's
// specification via SHOW ROLES IN SERVICE.
func (a *App) ListServiceRoles(database, schema, name string) (*snowflake.QueryResult, error) {
	return a.showInService("SHOW ROLES IN SERVICE", database, schema, name)
}

// ListServiceRoleGrants returns the grantees of one service role via
// SHOW GRANTS OF SERVICE ROLE <svc>!<role>, parsed into ServiceRoleGrant rows
// (parent/grantee already split, ready to pass back to RevokeServiceRole).
// Accounts without this SHOW variant return an error, which the panel surfaces
// as "grantees unavailable" while keeping the grant form usable.
func (a *App) ListServiceRoleGrants(database, schema, name, role string) ([]service.ServiceRoleGrant, error) {
	res, err := a.execObjectSQL("SHOW GRANTS OF SERVICE ROLE " +
		snowflake.Qualify(database, schema, name) + "!" + snowflake.QuoteIdent(role))
	if err != nil {
		return nil, err
	}
	return service.ParseServiceRoleGrants(res, role), nil
}

// GrantServiceRole runs GRANT SERVICE ROLE <svc>!<role> TO <grantee>.
func (a *App) GrantServiceRole(database, schema, name string, g service.ServiceRoleGrant) error {
	sql, err := service.BuildGrantServiceRoleSql(database, schema, name, g)
	if err != nil {
		return err
	}
	_, err = a.execObjectSQL(sql)
	return err
}

// RevokeServiceRole runs REVOKE SERVICE ROLE <svc>!<role> FROM <grantee>.
func (a *App) RevokeServiceRole(database, schema, name string, g service.ServiceRoleGrant) error {
	sql, err := service.BuildRevokeServiceRoleSql(database, schema, name, g)
	if err != nil {
		return err
	}
	_, err = a.execObjectSQL(sql)
	return err
}

// RedeployService runs ALTER SERVICE <fqn> FROM SPECIFICATION … (inline or
// staged, per cfg.SpecSource); Snowflake restarts the service instances with
// the new spec. Only the spec-source fields of cfg are used.
func (a *App) RedeployService(database, schema, name string, cfg service.ServiceConfig) error {
	clause, err := service.BuildAlterServiceSpecClause(cfg)
	if err != nil {
		return err
	}
	return a.AlterService(database, schema, name, clause)
}

// ListDatabaseRoles returns the names of the database roles in the given
// database (SHOW DATABASE ROLES IN DATABASE), for grantee pickers.
func (a *App) ListDatabaseRoles(database string) ([]string, error) {
	res, err := a.execObjectSQL("SHOW DATABASE ROLES IN DATABASE " + snowflake.QuoteIdent(database))
	if err != nil {
		return nil, err
	}
	idx := snowflake.ColIdx(res.Columns, "name")
	names := make([]string, 0, len(res.Rows))
	for _, row := range res.Rows {
		if n := snowflake.Cell(row, idx); n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}

// GetServiceLogs returns the container logs for a single service instance via
// SYSTEM$GET_SERVICE_LOGS('<fqn>', <instance_id>, '<container>'[, <num_lines>]).
// instanceID is the 0-based service instance index and containerName is the
// container name from the service spec; numLines, when > 0, caps the number of
// trailing log lines returned. The function returns the log text as a single
// string (Snowflake returns the logs in one cell).
func (a *App) GetServiceLogs(database, schema, name, containerName string, instanceID, numLines int) (string, error) {
	client := a.currentClient()
	if client == nil {
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

	res, err := client.Execute(a.fctx(FeatureObjectEditor), sql)
	if err != nil {
		return "", err
	}
	if len(res.Rows) == 0 || len(res.Rows[0]) == 0 || res.Rows[0][0] == nil {
		return "", nil
	}
	return fmt.Sprintf("%v", res.Rows[0][0]), nil
}

// ListJobServices runs SHOW JOB SERVICES IN ACCOUNT for the Container Services
// dialog's Jobs tab (running, completed and async job services; is_job = true).
func (a *App) ListJobServices() (*snowflake.QueryResult, error) {
	return a.execPoolSQL("SHOW JOB SERVICES IN ACCOUNT")
}

// DropJobService runs DROP SERVICE <fqn> for a job service.
func (a *App) DropJobService(database, schema, name string) error {
	_, err := a.execPoolSQL("DROP SERVICE " + snowflake.Qualify(database, schema, name))
	return err
}

// ListServicesInAccount runs SHOW SERVICES EXCLUDE JOBS IN ACCOUNT for the
// Container Services dialog's Services tab — job services are listed
// separately by ListJobServices.
func (a *App) ListServicesInAccount() (*snowflake.QueryResult, error) {
	return a.execPoolSQL("SHOW SERVICES EXCLUDE JOBS IN ACCOUNT")
}

// DropService runs DROP SERVICE <fqn> for a non-job service.
func (a *App) DropService(database, schema, name string) error {
	_, err := a.execPoolSQL("DROP SERVICE " + snowflake.Qualify(database, schema, name))
	return err
}
