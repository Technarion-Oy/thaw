// SPDX-License-Identifier: GPL-3.0-or-later

package snapshot

import (
	"fmt"
	"strconv"
	"strings"

	"thaw/internal/snowflake"
)

// SnapshotConfig holds the parameters for CREATE SNAPSHOT. The source service
// may live in any schema, so it carries its own database and schema.
type SnapshotConfig struct {
	Name            string              `json:"name"`
	CaseSensitive   bool                `json:"caseSensitive"`
	OrReplace       bool                `json:"orReplace"`
	IfNotExists     bool                `json:"ifNotExists"`
	ServiceDatabase string              `json:"serviceDatabase"`
	ServiceSchema   string              `json:"serviceSchema"`
	ServiceName     string              `json:"serviceName"`
	Volume          string              `json:"volume"`
	Instance        string              `json:"instance"` // instance id, a non-negative integer
	Comment         string              `json:"comment"`
	Tags            []snowflake.TagPair `json:"tags"`
}

// BuildCreateSnapshotSql constructs a CREATE SNAPSHOT statement. Blank name,
// service, volume or instance become placeholders so the live preview stays a
// completable template. OR REPLACE wins over IF NOT EXISTS when both are set.
// The volume name is always double-quoted: volume names come from the service
// spec and are case-sensitive.
//
//	CREATE [OR REPLACE] SNAPSHOT [IF NOT EXISTS] <fqn>
//	  FROM SERVICE <service> VOLUME "<volume>" INSTANCE <id>
//	  [COMMENT = '…'] [TAG (…)];
func BuildCreateSnapshotSql(db, schema string, cfg SnapshotConfig) (string, error) {
	var sb strings.Builder
	name := cfg.Name
	if strings.TrimSpace(name) == "" {
		name = "snapshot_name"
	}
	sb.WriteString(snowflake.CreateClause("SNAPSHOT", cfg.OrReplace, cfg.IfNotExists) + " " +
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	svc := "<service>"
	if strings.TrimSpace(cfg.ServiceName) != "" {
		svc = snowflake.Qualify(cfg.ServiceDatabase, cfg.ServiceSchema, cfg.ServiceName)
	}
	vol := `"<volume>"`
	if cfg.Volume != "" {
		vol = snowflake.QuoteIdent(cfg.Volume)
	}
	inst := "<instance>"
	if s := strings.TrimSpace(cfg.Instance); s != "" {
		if n, err := strconv.Atoi(s); err != nil || n < 0 {
			return "", fmt.Errorf("invalid instance id %q", s)
		}
		inst = s
	}
	fmt.Fprintf(&sb, "\n  FROM SERVICE %s\n  VOLUME %s\n  INSTANCE %s", svc, vol, inst)

	sb.WriteString(snowflake.CommentClause(cfg.Comment))
	if t := snowflake.TagClause(cfg.Tags); t != "" {
		sb.WriteString("\n  " + t)
	}
	return sb.String() + ";", nil
}
