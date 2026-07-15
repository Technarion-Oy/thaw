// SPDX-License-Identifier: GPL-3.0-or-later

package imagerepository

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// ImageRepositoryConfig holds the parameters for creating a Snowflake IMAGE
// REPOSITORY object. Image repositories have a minimal grammar — beyond the
// name they only accept OR REPLACE / IF NOT EXISTS and an optional COMMENT — so
// the config is correspondingly small.
type ImageRepositoryConfig struct {
	Name          string `json:"name"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Comment       string `json:"comment"`
}

// BuildCreateImageRepositorySql constructs a CREATE IMAGE REPOSITORY statement
// from the given config. When the name is blank the builder substitutes a
// placeholder so the live preview reads as a completable template rather than
// invalid SQL. OR REPLACE and IF NOT EXISTS are mutually exclusive in
// Snowflake; the create modal prevents selecting both, and if both are set here
// OR REPLACE wins (IF NOT EXISTS is dropped).
//
//	CREATE [OR REPLACE] IMAGE REPOSITORY [IF NOT EXISTS] <fqn>
//	  [COMMENT = '…'];
func BuildCreateImageRepositorySql(db, schema string, cfg ImageRepositoryConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("IMAGE REPOSITORY", cfg.OrReplace, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "image_repository_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	sb.WriteString(snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}
