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
	if cfg.MinNodes < 0 || cfg.MaxNodes < 0 {
		return "", fmt.Errorf("node counts must be non-negative")
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

// BuildAlterComputePoolPropertySql builds ALTER COMPUTE POOL <name> SET (or
// UNSET, for a blank value on an unsettable property) for one property:
// minNodes, maxNodes, autoResume, autoSuspendSecs, placementGroup,
// instanceFamily, backupInstanceFamilies (comma-separated), comment.
func BuildAlterComputePoolPropertySql(name, property, value string) (string, error) {
	prefix := "ALTER COMPUTE POOL " + snowflake.QuoteIdent(name) + " "
	what := fmt.Sprintf("compute pool property %q", property)
	blank := strings.TrimSpace(value) == ""
	unset := func(kw string) (string, error) { return prefix + "UNSET " + kw, nil }
	set := func(kw, v string, err error) (string, error) {
		if err != nil {
			return "", err
		}
		return prefix + "SET " + kw + " = " + v, nil
	}
	switch property {
	case "minNodes":
		v, err := snowflake.ValidateNonNegativeInt(what, value)
		return set("MIN_NODES", v, err)
	case "maxNodes":
		v, err := snowflake.ValidateNonNegativeInt(what, value)
		return set("MAX_NODES", v, err)
	case "autoResume":
		if blank {
			return unset("AUTO_RESUME")
		}
		v, err := snowflake.ValidateEnumValue(what, value, "TRUE", "FALSE")
		return set("AUTO_RESUME", v, err)
	case "autoSuspendSecs":
		if blank {
			return unset("AUTO_SUSPEND_SECS")
		}
		v, err := snowflake.ValidateNonNegativeInt(what, value)
		return set("AUTO_SUSPEND_SECS", v, err)
	case "placementGroup":
		if blank {
			return unset("PLACEMENT_GROUP")
		}
		return set("PLACEMENT_GROUP", snowflake.QuoteTextLit(strings.TrimSpace(value)), nil)
	case "instanceFamily":
		v, err := word("instance family", value)
		return set("INSTANCE_FAMILY", v, err)
	case "backupInstanceFamilies":
		if blank {
			return unset("BACKUP_INSTANCE_FAMILIES")
		}
		v, err := familyList(strings.Split(value, ","))
		return set("BACKUP_INSTANCE_FAMILIES", v, err)
	case "comment":
		if blank {
			return unset("COMMENT")
		}
		return set("COMMENT", snowflake.QuoteTextLit(value), nil)
	default:
		return "", fmt.Errorf("unknown compute pool property: %s", property)
	}
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
