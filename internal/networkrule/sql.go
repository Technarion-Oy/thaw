// SPDX-License-Identifier: GPL-3.0-or-later

package networkrule

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// NetworkRuleConfig holds the parameters for creating a Snowflake NETWORK RULE
// object. The fields map to the CREATE NETWORK RULE options in the order
// Snowflake documents them: TYPE, VALUE_LIST, MODE, then COMMENT. NETWORK RULE
// has no IF NOT EXISTS form, so only OrReplace is modeled.
type NetworkRuleConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	// Type is the network-identifier kind, e.g. IPV4, IPV6, AWSVPCEID,
	// AZURELINKID, GCPPSCID, HOST_PORT, PRIVATE_HOST_PORT, COMPUTE_POOL.
	Type string `json:"type"`
	// Mode is how the rule is used: INGRESS, INTERNAL_STAGE,
	// SNOWFLAKE_MANAGED_STORAGE_VOLUME, or EGRESS.
	Mode string `json:"mode"`
	// ValueList holds the network identifiers; each entry is emitted as a quoted
	// string literal. Blank entries are dropped.
	ValueList []string `json:"valueList"`
	Comment   string   `json:"comment"`
}

// BuildCreateNetworkRuleSql constructs a CREATE NETWORK RULE statement from the
// given config. When required parts are blank the builder substitutes
// placeholders so the live preview reads as a completable template rather than
// invalid SQL.
//
//	CREATE [OR REPLACE] NETWORK RULE <fqn>
//	  TYPE = <type>
//	  VALUE_LIST = ('<value>' [, …])
//	  MODE = <mode>
//	  [COMMENT = '…'];
func BuildCreateNetworkRuleSql(db, schema string, cfg NetworkRuleConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("NETWORK RULE", cfg.OrReplace, false)

	name := cfg.Name
	if name == "" {
		name = "network_rule_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	ruleType := strings.TrimSpace(cfg.Type)
	if ruleType == "" {
		ruleType = "IPV4"
	}
	fmt.Fprintf(&sb, "\n  TYPE = %s", ruleType)

	// VALUE_LIST: emit each non-blank entry as a quoted, escaped literal.
	// VALUE_LIST is required by Snowflake on CREATE; an empty list renders as ()
	// only so the live preview stays well-formed while the user is still typing
	// (the create modal blocks submitting with zero values). An existing rule's
	// list can be emptied later via ALTER … UNSET VALUE_LIST.
	vals := make([]string, 0, len(cfg.ValueList))
	for _, v := range cfg.ValueList {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		vals = append(vals, fmt.Sprintf("'%s'", snowflake.EscapeStringLit(v)))
	}
	fmt.Fprintf(&sb, "\n  VALUE_LIST = (%s)", strings.Join(vals, ", "))

	mode := strings.TrimSpace(cfg.Mode)
	if mode == "" {
		mode = "INGRESS"
	}
	fmt.Fprintf(&sb, "\n  MODE = %s", mode)

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
