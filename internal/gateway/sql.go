// SPDX-License-Identifier: GPL-3.0-or-later

package gateway

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// defaultSpec is the placeholder traffic-split specification used when the
// caller supplies a blank spec, so the live preview reads as a completable
// template rather than invalid SQL.
const defaultSpec = `spec:
  type: traffic_split
  split_type: custom
  targets:
  - type: endpoint
    value: <db>.<schema>.<service>!<endpoint>
    weight: 100`

// GatewayConfig holds the parameters for creating a Snowflake GATEWAY object.
// The only payload is the traffic-split Specification (YAML); gateways have no
// COMMENT clause in CREATE. The Specification is emitted inside a $THAW$-tagged
// block so multi-line YAML needs no escaping.
type GatewayConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Specification string `json:"specification"` // YAML traffic-split spec for FROM SPECIFICATION
}

// wrapSpec renders the FROM SPECIFICATION clause, wrapping the spec in a tagged
// $THAW$ … $THAW$ dollar-quote so the multi-line YAML needs no escaping and a
// literal `$$` inside the spec can't prematurely close the block. A blank spec
// falls back to a minimal placeholder so the preview stays a completable
// template.
func wrapSpec(specification string) string {
	spec := strings.TrimRight(strings.TrimSpace(specification), "\n")
	if spec == "" {
		spec = defaultSpec
	}
	return fmt.Sprintf("FROM SPECIFICATION\n  $THAW$\n%s\n  $THAW$", spec)
}

// BuildCreateGatewaySql constructs a CREATE GATEWAY statement from the given
// config. When required parts are blank the builder substitutes placeholders so
// the live preview reads as a completable template rather than invalid SQL. OR
// REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake; the create
// modal prevents selecting both, and if both are set here OR REPLACE wins (IF
// NOT EXISTS is dropped by CreateClause).
//
//	CREATE [OR REPLACE] GATEWAY [IF NOT EXISTS] <fqn>
//	  FROM SPECIFICATION
//	  $THAW$
//	  <spec>
//	  $THAW$;
func BuildCreateGatewaySql(db, schema string, cfg GatewayConfig) (string, error) {
	createClause := snowflake.CreateClause("GATEWAY", cfg.OrReplace, cfg.IfNotExists)

	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "gateway_name"
	}

	return fmt.Sprintf("%s %s\n  %s;", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive), wrapSpec(cfg.Specification)), nil
}

// BuildAlterGatewaySpecSql constructs an ALTER GATEWAY … FROM SPECIFICATION
// statement that updates the traffic-split specification of an existing gateway.
// Updating the specification is the entire ALTER GATEWAY surface — there is no
// RENAME, SET COMMENT, or SET TAG — so this is the only mutation a gateway
// supports. The identifier is fully qualified and double-quoted (the object
// already exists with a fixed name); the spec is dollar-quoted via wrapSpec.
//
//	ALTER GATEWAY <fqn>
//	  FROM SPECIFICATION
//	  $THAW$
//	  <spec>
//	  $THAW$;
func BuildAlterGatewaySpecSql(db, schema, name, specification string) string {
	return fmt.Sprintf("ALTER GATEWAY %s\n  %s;",
		snowflake.Qualify(db, schema, name), wrapSpec(specification))
}
