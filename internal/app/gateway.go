// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"fmt"

	"thaw/internal/apperrors"
	"thaw/internal/gateway"
	"thaw/internal/snowflake"
)

// DescribeGateway runs DESCRIBE GATEWAY and returns the raw QueryResult.
// SHOW GATEWAYS omits both the traffic-split specification and the ingress URL,
// so this is the source for the properties panel: the single result row carries
// columns name / ingress_url / privatelink_ingress_url / database_name /
// schema_name / owner / owner_role_type / spec / created_on / updated_on /
// comment. The spec column is only returned to roles holding USAGE, MODIFY, or
// OWNERSHIP on the gateway.
func (a *App) DescribeGateway(database, schema, name string) (*snowflake.QueryResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	sql := fmt.Sprintf("DESCRIBE GATEWAY %s",
		snowflake.Qualify(database, schema, name))
	return client.Execute(a.fctx(FeatureObjectEditor), sql)
}

// AlterGateway updates the traffic-split specification of an existing gateway
// via ALTER GATEWAY … FROM SPECIFICATION. Updating the specification is the
// entire ALTER GATEWAY surface — there is no RENAME, SET COMMENT, or SET TAG —
// so this is the only mutation a gateway supports. The specification is a YAML
// string; it is dollar-quoted inside the statement so multi-line YAML needs no
// escaping.
func (a *App) AlterGateway(database, schema, name, specification string) error {
	client := a.currentClient()
	if client == nil {
		return apperrors.ErrNotConnected
	}
	sql := gateway.BuildAlterGatewaySpecSql(database, schema, name, specification)
	_, err := client.Execute(a.fctx(FeatureObjectEditor), sql)
	return err
}
