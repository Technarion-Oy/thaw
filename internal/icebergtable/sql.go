// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package icebergtable

import (
	"fmt"
	"strings"

	"thaw/internal/snowflake"
)

// TableType selects which CREATE ICEBERG TABLE variant the builder emits. Each
// Snowflake variant has its own required attributes (see the per-variant docs):
//
//   - TableTypeSnowflake: Snowflake-managed (CATALOG = 'SNOWFLAKE'). Requires a
//     column list and BASE_LOCATION.
//   - TableTypeExternalCatalog: an external Iceberg catalog — Iceberg REST or
//     AWS Glue (identical table DDL; the difference is the catalog integration
//     type). Requires CATALOG_TABLE_NAME.
//   - TableTypeDelta: Delta Lake files in object storage. Requires BASE_LOCATION;
//     the catalog integration must use CATALOG_SOURCE = OBJECT_STORE and
//     TABLE_FORMAT = DELTA.
//   - TableTypeIcebergFiles: Iceberg metadata/data files in object storage
//     (unmanaged). Requires METADATA_FILE_PATH.
//
// An empty TableType defaults to Snowflake-managed.
const (
	TableTypeSnowflake       = "snowflake"
	TableTypeExternalCatalog = "external_catalog"
	TableTypeDelta           = "delta"
	TableTypeIcebergFiles    = "iceberg_files"
)

// IcebergColumn is one column of a Snowflake-managed Iceberg table:
// <name> <type>. Externally-managed tables infer their columns from the
// existing Iceberg metadata, so column definitions apply only to the
// Snowflake-managed path.
type IcebergColumn struct {
	Name string `json:"name"` // column identifier
	Type string `json:"type"` // Snowflake data type, emitted verbatim
}

// IcebergTableConfig holds the parameters for creating a Snowflake ICEBERG TABLE
// object. TableType selects the variant (see TableType constants); the required
// attributes differ per variant, so the builder only emits the fields relevant
// to the chosen type. EXTERNAL_VOLUME and CATALOG are optional for every variant
// (a schema/database/account default is used when omitted).
//
// Per-variant optional extras of the Iceberg REST form (PATH_LAYOUT,
// TARGET_FILE_SIZE, STORAGE_SERIALIZATION_POLICY, ICEBERG_MERGE_ON_READ_BEHAVIOR),
// policy attachments, START AT, COPY GRANTS, and the CREATE … AS SELECT form are
// intentionally out of scope for the visual builder and are left to raw SQL.
type IcebergTableConfig struct {
	Name                     string              `json:"name"`
	CaseSensitive            bool                `json:"caseSensitive"`
	OrReplace                bool                `json:"orReplace"`
	IfNotExists              bool                `json:"ifNotExists"`
	TableType                string              `json:"tableType"`                // see TableType constants (default "snowflake")
	Columns                  []IcebergColumn     `json:"columns"`                  // Snowflake-managed column list
	ExternalVolume           string              `json:"externalVolume"`           // EXTERNAL_VOLUME = '<name>' (optional, all variants)
	Catalog                  string              `json:"catalog"`                  // CATALOG = '<catalog integration>' (optional, external variants)
	BaseLocation             string              `json:"baseLocation"`             // BASE_LOCATION = '<dir>' (Snowflake-managed + Delta)
	CatalogTableName         string              `json:"catalogTableName"`         // CATALOG_TABLE_NAME = '<table>' (external catalog: REST / AWS Glue)
	CatalogNamespace         string              `json:"catalogNamespace"`         // CATALOG_NAMESPACE = '<namespace>' (external catalog)
	MetadataFilePath         string              `json:"metadataFilePath"`         // METADATA_FILE_PATH = '<path>' (Iceberg files)
	ReplaceInvalidCharacters string              `json:"replaceInvalidCharacters"` // TRUE | FALSE (or "") — external variants
	AutoRefresh              string              `json:"autoRefresh"`              // TRUE | FALSE (or "") — external catalog + Delta
	ClusterBy                string              `json:"clusterBy"`                // comma-separated clustering expressions or "" (Snowflake-managed)
	Comment                  string              `json:"comment"`
	Tags                     []snowflake.TagPair `json:"tags"` // table-level TAG (name = 'value', ...)
}

// BuildCreateIcebergTableSql constructs a CREATE ICEBERG TABLE statement from the
// given config. The variant is chosen by cfg.TableType; only the clauses that
// variant uses are emitted, and the variant's required attribute is emitted with
// a completable placeholder when empty so the live preview reads as a template
// rather than invalid SQL. Optional clauses are emitted only when set, in the
// order Snowflake documents them. EXTERNAL_VOLUME and CATALOG are optional for
// every variant. OR REPLACE and IF NOT EXISTS are mutually exclusive; if both are
// set OR REPLACE wins.
//
// Snowflake-managed (TableTypeSnowflake):
//
//	CREATE … ICEBERG TABLE … <fqn> ( <cols> )
//	  [EXTERNAL_VOLUME = '<vol>'] CATALOG = 'SNOWFLAKE' BASE_LOCATION = '<dir>'
//	  [CLUSTER BY ( … )] [COMMENT = '…'] [TAG ( … )];
//
// External Iceberg catalog — REST / AWS Glue (TableTypeExternalCatalog):
//
//	CREATE … ICEBERG TABLE … <fqn>
//	  [EXTERNAL_VOLUME = '<vol>'] [CATALOG = '<integration>']
//	  CATALOG_TABLE_NAME = '<table>' [CATALOG_NAMESPACE = '<ns>']
//	  [REPLACE_INVALID_CHARACTERS = …] [AUTO_REFRESH = …] [COMMENT = '…'] [TAG ( … )];
//
// Delta Lake files (TableTypeDelta):
//
//	CREATE … ICEBERG TABLE … <fqn>
//	  [EXTERNAL_VOLUME = '<vol>'] [CATALOG = '<integration>'] BASE_LOCATION = '<dir>'
//	  [REPLACE_INVALID_CHARACTERS = …] [AUTO_REFRESH = …] [COMMENT = '…'] [TAG ( … )];
//
// Iceberg files in object storage (TableTypeIcebergFiles):
//
//	CREATE … ICEBERG TABLE … <fqn>
//	  [EXTERNAL_VOLUME = '<vol>'] [CATALOG = '<integration>'] METADATA_FILE_PATH = '<path>'
//	  [REPLACE_INVALID_CHARACTERS = …] [COMMENT = '…'] [TAG ( … )];
func BuildCreateIcebergTableSql(db, schema string, cfg IcebergTableConfig) (string, error) {
	var sb strings.Builder

	createClause := "CREATE"
	if cfg.OrReplace {
		createClause += " OR REPLACE"
	}
	createClause += " ICEBERG TABLE"
	if cfg.IfNotExists && !cfg.OrReplace {
		createClause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if cfg.Name == "" {
		nameToken = "iceberg_table_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", createClause, snowflake.QuoteIdent(db), snowflake.QuoteIdent(schema), nameToken)

	tableType := strings.TrimSpace(cfg.TableType)
	if tableType == "" {
		tableType = TableTypeSnowflake
	}
	snowflakeManaged := strings.EqualFold(tableType, TableTypeSnowflake)

	// Only Snowflake-managed tables carry their own column definitions; every
	// other variant infers columns from the existing files/catalog metadata.
	if snowflakeManaged {
		cols := make([]IcebergColumn, 0, len(cfg.Columns))
		for _, c := range cfg.Columns {
			if strings.TrimSpace(c.Name) == "" {
				continue
			}
			cols = append(cols, c)
		}
		if len(cols) > 0 {
			lines := make([]string, 0, len(cols))
			for _, c := range cols {
				typ := strings.TrimSpace(c.Type)
				if typ == "" {
					typ = "VARCHAR"
				}
				lines = append(lines, fmt.Sprintf("  %s %s",
					snowflake.QuoteOrBare(c.Name, cfg.CaseSensitive), typ))
			}
			fmt.Fprintf(&sb, " (\n%s\n)", strings.Join(lines, ",\n"))
		} else {
			fmt.Fprintf(&sb, " (\n  <column> <type>\n)")
		}
	}

	// EXTERNAL_VOLUME is optional for every variant (a default external volume
	// may be set on the database, schema, or account); emit it only when set.
	if ev := strings.TrimSpace(cfg.ExternalVolume); ev != "" {
		fmt.Fprintf(&sb, "\n  EXTERNAL_VOLUME = '%s'", snowflake.EscapeStringLit(ev))
	}

	// CATALOG: Snowflake-managed always uses the literal 'SNOWFLAKE'; the
	// external variants take an optional catalog-integration name (a default may
	// be set on the schema/database/account), emitted only when supplied.
	if snowflakeManaged {
		fmt.Fprintf(&sb, "\n  CATALOG = 'SNOWFLAKE'")
	} else if catalog := strings.TrimSpace(cfg.Catalog); catalog != "" {
		fmt.Fprintf(&sb, "\n  CATALOG = '%s'", snowflake.EscapeStringLit(catalog))
	}

	// The variant's required locator clause.
	switch tableType {
	case TableTypeExternalCatalog:
		ctn := strings.TrimSpace(cfg.CatalogTableName)
		if ctn == "" {
			ctn = "<catalog_table_name>"
		}
		fmt.Fprintf(&sb, "\n  CATALOG_TABLE_NAME = '%s'", snowflake.EscapeStringLit(ctn))
		if ns := strings.TrimSpace(cfg.CatalogNamespace); ns != "" {
			fmt.Fprintf(&sb, "\n  CATALOG_NAMESPACE = '%s'", snowflake.EscapeStringLit(ns))
		}
	case TableTypeIcebergFiles:
		mfp := strings.TrimSpace(cfg.MetadataFilePath)
		if mfp == "" {
			mfp = "<metadata_file_path>"
		}
		fmt.Fprintf(&sb, "\n  METADATA_FILE_PATH = '%s'", snowflake.EscapeStringLit(mfp))
	default: // TableTypeSnowflake, TableTypeDelta — both use BASE_LOCATION
		base := strings.TrimSpace(cfg.BaseLocation)
		if base == "" {
			base = "<base_location>"
		}
		fmt.Fprintf(&sb, "\n  BASE_LOCATION = '%s'", snowflake.EscapeStringLit(base))
	}

	// REPLACE_INVALID_CHARACTERS applies only to the external (non-Snowflake)
	// variants.
	if !snowflakeManaged {
		if ric := strings.TrimSpace(cfg.ReplaceInvalidCharacters); ric != "" {
			fmt.Fprintf(&sb, "\n  REPLACE_INVALID_CHARACTERS = %s", strings.ToUpper(ric))
		}
	}
	// AUTO_REFRESH is supported by the external catalog and Delta variants, but
	// not by the Iceberg-files variant or Snowflake-managed tables.
	if tableType == TableTypeExternalCatalog || tableType == TableTypeDelta {
		if ar := strings.TrimSpace(cfg.AutoRefresh); ar != "" {
			fmt.Fprintf(&sb, "\n  AUTO_REFRESH = %s", strings.ToUpper(ar))
		}
	}
	// CLUSTER BY applies only to Snowflake-managed tables.
	if snowflakeManaged {
		if cb := strings.TrimSpace(cfg.ClusterBy); cb != "" {
			fmt.Fprintf(&sb, "\n  CLUSTER BY (%s)", cb)
		}
	}
	if cfg.Comment != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", snowflake.EscapeStringLit(cfg.Comment))
	}
	if tc := snowflake.TagClause(cfg.Tags); tc != "" {
		fmt.Fprintf(&sb, "\n  %s", tc)
	}

	return sb.String() + ";", nil
}
