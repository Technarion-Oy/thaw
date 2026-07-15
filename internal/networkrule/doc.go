// SPDX-License-Identifier: GPL-3.0-or-later

// Package networkrule builds SQL for Snowflake network rule objects — CREATE
// NETWORK RULE statements and the structured config behind them. A network rule
// groups a set of network identifiers (IP addresses or CIDR ranges, VPC/private
// endpoint IDs, or host:port destinations) selected by its TYPE, and a MODE that
// declares how the rule is used (ingress to Snowflake, egress to external
// destinations, or internal-stage access). Network rules are referenced by
// network policies and external-access integrations.
//
// TYPE and MODE are fixed at creation; only VALUE_LIST and COMMENT can be
// altered (and network rules cannot be renamed), so the edit clauses (SET/UNSET
// VALUE_LIST, SET/UNSET COMMENT) are issued as free-form ALTER NETWORK RULE
// statements from internal/app/networkrule.go (App.AlterNetworkRule).
//
// thaw:domain: Object Browser & Administration
package networkrule
