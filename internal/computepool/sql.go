// SPDX-License-Identifier: GPL-3.0-or-later

package computepool

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ComputePoolConfig holds the parameters for CREATE COMPUTE POOL. MinNodes,
// MaxNodes and InstanceFamily are required by Snowflake; every string field is
// optional and omitted when blank.
type ComputePoolConfig struct {
	Name                   string              `json:"name"`
	IfNotExists            bool                `json:"ifNotExists"`
	ForApplication         string              `json:"forApplication"`
	MinNodes               int                 `json:"minNodes"`
	MaxNodes               int                 `json:"maxNodes"`
	InstanceFamily         string              `json:"instanceFamily"`
	AutoResume             string              `json:"autoResume"`         // TRUE | FALSE | ""
	InitiallySuspended     string              `json:"initiallySuspended"` // TRUE | FALSE | ""
	AutoSuspendSecs        string              `json:"autoSuspendSecs"`    // integer string or ""
	Tags                   []snowflake.TagPair `json:"tags"`
	Comment                string              `json:"comment"`
	PlacementGroup         string              `json:"placementGroup"`
	BackupInstanceFamilies []string            `json:"backupInstanceFamilies"`
}

// word validates an instance family or workload type name (CPU_X64_XS,
// GPU_NV_S, …). Both are emitted unquoted, so anything that isn't a bare
// identifier is rejected rather than interpolated.
func word(what, v string) (string, error) {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" || snowflake.NeedsQuoting(v) {
		return "", fmt.Errorf("invalid %s %q", what, v)
	}
	return v, nil
}

// familyList renders ( 'A', 'B' ) for BACKUP_INSTANCE_FAMILIES.
func familyList(fams []string) (string, error) {
	parts := make([]string, 0, len(fams))
	for _, f := range snowflake.CleanList(fams) {
		w, err := word("instance family", f)
		if err != nil {
			return "", err
		}
		parts = append(parts, "'"+w+"'")
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("no instance families given")
	}
	return "(" + strings.Join(parts, ", ") + ")", nil
}

// BuildCreateComputePoolSql constructs a CREATE COMPUTE POOL statement. A blank
// name or instance family becomes a placeholder so the live preview stays a
// completable template.
//
//	CREATE COMPUTE POOL [IF NOT EXISTS] <name> [FOR APPLICATION <app>]
//	  MIN_NODES = <n> MAX_NODES = <n> INSTANCE_FAMILY = <fam>
//	  [AUTO_RESUME = …] [INITIALLY_SUSPENDED = …] [AUTO_SUSPEND_SECS = <n>]
//	  [TAG (…)] [COMMENT = '…'] [PLACEMENT_GROUP = '…']
//	  [BACKUP_INSTANCE_FAMILIES = ( '<fam>', … )];
func BuildCreateComputePoolSql(cfg ComputePoolConfig) (string, error) {
	var sb strings.Builder
	name := "pool_name"
	if n := strings.TrimSpace(cfg.Name); n != "" {
		name = snowflake.QuoteOrBare(n, false)
	}
	sb.WriteString(snowflake.CreateClause("COMPUTE POOL", false, cfg.IfNotExists) + " " + name)
	if app := strings.TrimSpace(cfg.ForApplication); app != "" {
		sb.WriteString("\n  FOR APPLICATION " + snowflake.QuoteOrBare(app, false))
	}
	if cfg.MinNodes < 1 || cfg.MaxNodes < 1 {
		return "", fmt.Errorf("node counts must be at least 1")
	}
	if cfg.MaxNodes < cfg.MinNodes {
		return "", fmt.Errorf("MAX_NODES (%d) must be >= MIN_NODES (%d)", cfg.MaxNodes, cfg.MinNodes)
	}
	fmt.Fprintf(&sb, "\n  MIN_NODES = %d\n  MAX_NODES = %d", cfg.MinNodes, cfg.MaxNodes)
	fam := "<instance_family>"
	if strings.TrimSpace(cfg.InstanceFamily) != "" {
		w, err := word("instance family", cfg.InstanceFamily)
		if err != nil {
			return "", err
		}
		fam = w
	}
	sb.WriteString("\n  INSTANCE_FAMILY = " + fam)
	for _, b := range []struct{ kw, v string }{
		{"AUTO_RESUME", cfg.AutoResume},
		{"INITIALLY_SUSPENDED", cfg.InitiallySuspended},
	} {
		if strings.TrimSpace(b.v) == "" {
			continue
		}
		v, err := snowflake.ValidateEnumValue(b.kw, b.v, "TRUE", "FALSE")
		if err != nil {
			return "", err
		}
		sb.WriteString("\n  " + b.kw + " = " + v)
	}
	if strings.TrimSpace(cfg.AutoSuspendSecs) != "" {
		v, err := snowflake.ValidateNonNegativeInt("AUTO_SUSPEND_SECS", cfg.AutoSuspendSecs)
		if err != nil {
			return "", err
		}
		sb.WriteString("\n  AUTO_SUSPEND_SECS = " + v)
	}
	if t := snowflake.TagClause(cfg.Tags); t != "" {
		sb.WriteString("\n  " + t)
	}
	sb.WriteString(snowflake.CommentClause(cfg.Comment))
	if pg := strings.TrimSpace(cfg.PlacementGroup); pg != "" {
		sb.WriteString("\n  PLACEMENT_GROUP = " + snowflake.QuoteTextLit(pg))
	}
	if len(snowflake.CleanList(cfg.BackupInstanceFamilies)) > 0 {
		l, err := familyList(cfg.BackupInstanceFamilies)
		if err != nil {
			return "", err
		}
		sb.WriteString("\n  BACKUP_INSTANCE_FAMILIES = " + l)
	}
	return sb.String() + ";", nil
}

// settable lists the properties ALTER COMPUTE POOL … SET accepts, keyed by
// their Snowflake keyword (the frontend's ComputePoolPropertiesModal uses the
// same keys). unsettable ones UNSET on a blank value; render validates a
// non-blank value into its SQL form.
var settable = map[string]struct {
	unsettable bool
	render     func(what, v string) (string, error)
}{
	"MIN_NODES":                {false, nodeCount},
	"MAX_NODES":                {false, nodeCount},
	"INSTANCE_FAMILY":          {false, func(_, v string) (string, error) { return word("instance family", v) }},
	"AUTO_RESUME":              {true, func(what, v string) (string, error) { return snowflake.ValidateEnumValue(what, v, "TRUE", "FALSE") }},
	"AUTO_SUSPEND_SECS":        {true, snowflake.ValidateNonNegativeInt},
	"PLACEMENT_GROUP":          {true, func(_, v string) (string, error) { return snowflake.QuoteTextLit(strings.TrimSpace(v)), nil }},
	"BACKUP_INSTANCE_FAMILIES": {true, func(_, v string) (string, error) { return familyList(strings.Split(v, ",")) }},
	"COMMENT":                  {true, func(_, v string) (string, error) { return snowflake.QuoteTextLit(v), nil }},
}

// nodeCount validates a MIN_NODES / MAX_NODES value: Snowflake requires >= 1.
func nodeCount(what, v string) (string, error) {
	n, err := snowflake.ValidateNonNegativeInt(what, v)
	if err != nil || n == "0" {
		return "", fmt.Errorf("invalid node count %q for %s (must be >= 1)", v, what)
	}
	return n, nil
}

// BuildAlterComputePoolPropertySql builds ALTER COMPUTE POOL <name> SET
// <property> = <value>, or UNSET <property> for a blank value where Snowflake
// allows it. property is the Snowflake keyword (MIN_NODES, AUTO_RESUME, …; see
// settable). It sees one property at a time, so MIN_NODES <= MAX_NODES is left
// to the caller (which knows the pool's other value) and to Snowflake.
func BuildAlterComputePoolPropertySql(name, property, value string) (string, error) {
	p, ok := settable[property]
	if !ok {
		return "", fmt.Errorf("unknown compute pool property: %s", property)
	}
	if property == "BACKUP_INSTANCE_FAMILIES" {
		// A list of only commas/blanks ("," from a half-cleared field) means UNSET.
		value = strings.Join(snowflake.CleanList(strings.Split(value, ",")), ",")
	}
	prefix := "ALTER COMPUTE POOL " + snowflake.QuoteIdent(name) + " "
	if p.unsettable && strings.TrimSpace(value) == "" {
		return prefix + "UNSET " + property, nil
	}
	v, err := p.render(fmt.Sprintf("compute pool property %q", property), value)
	if err != nil {
		return "", err
	}
	return prefix + "SET " + property + " = " + v, nil
}

// BuildStopAllSql builds ALTER COMPUTE POOL <name> STOP ALL [OF TYPE t, …],
// which terminates every service and job running on the pool (optionally only
// those of the given workload types).
func BuildStopAllSql(name string, workloadTypes []string) (string, error) {
	sql := "ALTER COMPUTE POOL " + snowflake.QuoteIdent(name) + " STOP ALL"
	types := snowflake.CleanList(workloadTypes)
	for i, t := range types {
		w, err := word("workload type", t)
		if err != nil {
			return "", err
		}
		types[i] = w
	}
	if len(types) > 0 {
		sql += " OF TYPE " + strings.Join(types, ", ")
	}
	return sql, nil
}
