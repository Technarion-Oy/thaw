// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// thaw:domain: Object Browser & Administration

package stage

import (
	"fmt"
	"strings"

	"thaw/internal/fileformat"
	"thaw/internal/snowflake"
)

// StageConfig holds parameters for creating a Snowflake STAGE object.
type StageConfig struct {
	Name          string `json:"name"`
	Database      string `json:"database"`
	Schema        string `json:"schema"`
	CaseSensitive bool   `json:"caseSensitive"`
	OrReplace     bool   `json:"orReplace"`
	IfNotExists   bool   `json:"ifNotExists"`
	Type          string `json:"type"` // INTERNAL or EXTERNAL

	// External Params
	Url                    string `json:"url"`
	StorageIntegration     string `json:"storageIntegration"`
	UsePrivatelinkEndpoint bool   `json:"usePrivatelinkEndpoint"`
	// Note: Credentials like AWS_KEY_ID are intentionally omitted from UI config payload 
	// for now unless explicitly requested due to security, but structure can be expanded.

	// Encryption
	EncryptionType string `json:"encryptionType"`
	KmsKeyId       string `json:"kmsKeyId"`

	// Directory Table
	DirectoryEnabled             bool   `json:"directoryEnabled"`
	DirectoryAutoRefresh         bool   `json:"directoryAutoRefresh"`
	DirectoryRefreshOnCreate     bool   `json:"directoryRefreshOnCreate"`
	DirectoryNotificationIntegration string `json:"directoryNotificationIntegration"`

	// File Format
	FileFormatName string                        `json:"fileFormatName"`
	FileFormat     fileformat.FileFormatConfig   `json:"fileFormat"`

	Comment string `json:"comment"`
	Tags    string `json:"tags"` // JSON string map, or parsed later
}

// BuildCreateStageSql generates a CREATE STAGE SQL statement.
func BuildCreateStageSql(cfg StageConfig) string {
	var sb strings.Builder

	clause := "CREATE"
	if cfg.OrReplace {
		clause += " OR REPLACE"
	}
	clause += " STAGE"
	if cfg.IfNotExists && !cfg.OrReplace {
		clause += " IF NOT EXISTS"
	}

	nameToken := snowflake.QuoteOrBare(cfg.Name, cfg.CaseSensitive)
	if strings.TrimSpace(cfg.Name) == "" {
		nameToken = "stage_name"
	}

	fmt.Fprintf(&sb, "%s %s.%s.%s", clause,
		snowflake.QuoteIdent(cfg.Database), snowflake.QuoteIdent(cfg.Schema), nameToken)

	if cfg.Type == "EXTERNAL" {
		if strings.TrimSpace(cfg.Url) != "" {
			fmt.Fprintf(&sb, "\n  URL = '%s'", strings.ReplaceAll(cfg.Url, "'", "''"))
		}
		if strings.TrimSpace(cfg.StorageIntegration) != "" {
			fmt.Fprintf(&sb, "\n  STORAGE_INTEGRATION = %s", snowflake.QuoteIdent(cfg.StorageIntegration))
		}
		// CREDENTIALS could go here
	}

	// Directory Table
	if cfg.DirectoryEnabled {
		sb.WriteString("\n  DIRECTORY = (ENABLE = TRUE")
		if cfg.Type == "EXTERNAL" {
			if cfg.DirectoryAutoRefresh {
				sb.WriteString(" AUTO_REFRESH = TRUE")
			}
			if strings.TrimSpace(cfg.DirectoryNotificationIntegration) != "" {
				fmt.Fprintf(&sb, " NOTIFICATION_INTEGRATION = %s", snowflake.QuoteIdent(cfg.DirectoryNotificationIntegration))
			}
		} else {
			if cfg.DirectoryRefreshOnCreate {
				sb.WriteString(" REFRESH_ON_CREATE = TRUE")
			}
		}
		sb.WriteString(")")
	}

	// File format
	if strings.TrimSpace(cfg.FileFormatName) != "" {
		fmt.Fprintf(&sb, "\n  FILE_FORMAT = (FORMAT_NAME = %s)", cfg.FileFormatName)
	} else if strings.TrimSpace(cfg.FileFormat.Type) != "" {
		inline := fileformat.BuildInlineFileFormat(cfg.FileFormat)
		if inline != "" {
			fmt.Fprintf(&sb, "\n  FILE_FORMAT = (%s)", inline)
		}
	}

	// Encryption
	if cfg.EncryptionType != "" && cfg.EncryptionType != "NONE" {
		fmt.Fprintf(&sb, "\n  ENCRYPTION = (TYPE = '%s'", strings.ReplaceAll(cfg.EncryptionType, "'", "''"))
		if strings.TrimSpace(cfg.KmsKeyId) != "" {
			fmt.Fprintf(&sb, " KMS_KEY_ID = '%s'", strings.ReplaceAll(cfg.KmsKeyId, "'", "''"))
		}
		sb.WriteString(")")
	}

	// Comment
	if strings.TrimSpace(cfg.Comment) != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", strings.ReplaceAll(cfg.Comment, "'", "''"))
	}

	sb.WriteString(";")
	return sb.String()
}

// AlterStageConfig defines parameters that can be changed on an existing stage.
type AlterStageConfig struct {
	Name          string `json:"name"`
	Database      string `json:"database"`
	Schema        string `json:"schema"`
	
	Action        string `json:"action"` // RENAME, SET, UNSET, REFRESH
	
	NewName       string `json:"newName"`
	CaseSensitive bool   `json:"caseSensitive"`

	// Set/Unset parameters
	Comment       *string `json:"comment"`
	Url           *string `json:"url"`
	StorageIntegration *string `json:"storageIntegration"`
	DirectoryEnabled *bool `json:"directoryEnabled"`
}

// BuildAlterStageSql generates an ALTER STAGE statement.
func BuildAlterStageSql(cfg AlterStageConfig) string {
	target := fmt.Sprintf("%s.%s.%s", snowflake.QuoteIdent(cfg.Database), snowflake.QuoteIdent(cfg.Schema), snowflake.QuoteIdent(cfg.Name))
	
	var sb strings.Builder
	fmt.Fprintf(&sb, "ALTER STAGE %s", target)
	
	switch cfg.Action {
	case "RENAME":
		newName := snowflake.QuoteOrBare(cfg.NewName, cfg.CaseSensitive)
		fmt.Fprintf(&sb, " RENAME TO %s", newName)
	
	case "REFRESH":
		sb.WriteString(" REFRESH")
		
	case "SET":
		sb.WriteString(" SET")
		first := true
		
		if cfg.Comment != nil {
			if !first { sb.WriteString(",") }
			fmt.Fprintf(&sb, " COMMENT = '%s'", strings.ReplaceAll(*cfg.Comment, "'", "''"))
			first = false
		}
		
		if cfg.Url != nil {
			if !first { sb.WriteString(",") }
			fmt.Fprintf(&sb, " URL = '%s'", strings.ReplaceAll(*cfg.Url, "'", "''"))
			first = false
		}
		
		if cfg.StorageIntegration != nil {
			if !first { sb.WriteString(",") }
			fmt.Fprintf(&sb, " STORAGE_INTEGRATION = %s", snowflake.QuoteIdent(*cfg.StorageIntegration))
			first = false
		}
		
		if cfg.DirectoryEnabled != nil {
			if !first { sb.WriteString(",") }
			if *cfg.DirectoryEnabled {
				sb.WriteString(" DIRECTORY = (ENABLE = TRUE)")
			} else {
				sb.WriteString(" DIRECTORY = (ENABLE = FALSE)")
			}
		}
		
	case "UNSET":
		sb.WriteString(" UNSET")
		first := true
		
		if cfg.Comment != nil {
			if !first { sb.WriteString(",") }
			sb.WriteString(" COMMENT")
		}
	}
	
	sb.WriteString(";")
	return sb.String()
}
