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
	"context"
	"fmt"
	"strconv"
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
	DirectoryEnabled                 bool   `json:"directoryEnabled"`
	DirectoryAutoRefresh             bool   `json:"directoryAutoRefresh"`
	DirectoryRefreshOnCreate         bool   `json:"directoryRefreshOnCreate"`
	DirectoryNotificationIntegration string `json:"directoryNotificationIntegration"`

	// File Format
	FileFormatName string                      `json:"fileFormatName"`
	FileFormat     fileformat.FileFormatConfig `json:"fileFormat"`

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
			fmt.Fprintf(&sb, "\n  URL = '%s'", snowflake.EscapeStringLit(cfg.Url))
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
		fmt.Fprintf(&sb, "\n  ENCRYPTION = (TYPE = '%s'", snowflake.EscapeStringLit(cfg.EncryptionType))
		if strings.TrimSpace(cfg.KmsKeyId) != "" {
			fmt.Fprintf(&sb, " KMS_KEY_ID = '%s'", snowflake.EscapeStringLit(cfg.KmsKeyId))
		}
		sb.WriteString(")")
	}

	// Comment
	if strings.TrimSpace(cfg.Comment) != "" {
		fmt.Fprintf(&sb, "\n  COMMENT = '%s'", snowflake.EscapeStringLit(cfg.Comment))
	}

	sb.WriteString(";")
	return sb.String()
}

// AlterStageConfig defines parameters that can be changed on an existing stage.
type AlterStageConfig struct {
	Name     string `json:"name"`
	Database string `json:"database"`
	Schema   string `json:"schema"`

	Action string `json:"action"` // RENAME, SET, UNSET, REFRESH

	NewName       string `json:"newName"`
	CaseSensitive bool   `json:"caseSensitive"`

	// Set/Unset parameters
	Comment            *string `json:"comment"`
	Url                *string `json:"url"`
	StorageIntegration *string `json:"storageIntegration"`
	DirectoryEnabled   *bool   `json:"directoryEnabled"`
}

// BuildAlterStageSql generates an ALTER STAGE statement.
func BuildAlterStageSql(cfg AlterStageConfig) string {
	target := snowflake.Qualify(cfg.Database, cfg.Schema, cfg.Name)

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
			if !first {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, " COMMENT = '%s'", snowflake.EscapeStringLit(*cfg.Comment))
			first = false
		}

		if cfg.Url != nil {
			if !first {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, " URL = '%s'", snowflake.EscapeStringLit(*cfg.Url))
			first = false
		}

		if cfg.StorageIntegration != nil {
			if !first {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, " STORAGE_INTEGRATION = %s", snowflake.QuoteIdent(*cfg.StorageIntegration))
			first = false
		}

		if cfg.DirectoryEnabled != nil {
			if !first {
				sb.WriteString(",")
			}
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
			if !first {
				sb.WriteString(",")
			}
			sb.WriteString(" COMMENT")
		}
	}

	sb.WriteString(";")
	return sb.String()
}

// StageFile represents a file stored on a Snowflake stage.
type StageFile struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	MD5          string `json:"md5"`
	LastModified string `json:"lastModified"`
}

// ListStageFiles returns the list of files on a Snowflake stage (internal or external).
// stageName can be a fully qualified name (e.g. "@DB.SCHEMA.STAGE") or a relative
// path (e.g. "@STAGE/path/"). pattern is an optional regex to filter results.
func ListStageFiles(ctx context.Context, client *snowflake.Client, stageName string, pattern string) ([]StageFile, error) {
	// Ensure stageName starts with @
	if !strings.HasPrefix(stageName, "@") {
		stageName = "@" + stageName
	}

	sql := fmt.Sprintf(`LIST %s`, stageName)
	if pattern != "" {
		sql += fmt.Sprintf(" PATTERN = '%s'", snowflake.EscapeStringLit(pattern))
	}

	res, err := client.Execute(ctx, sql)
	if err != nil {
		return nil, err
	}

	nameIdx := -1
	sizeIdx := -1
	md5Idx := -1
	lastModIdx := -1

	for i, col := range res.Columns {
		switch strings.ToUpper(col) {
		case "NAME":
			nameIdx = i
		case "SIZE":
			sizeIdx = i
		case "MD5":
			md5Idx = i
		case "LAST_MODIFIED":
			lastModIdx = i
		}
	}

	var files []StageFile
	for _, row := range res.Rows {
		f := StageFile{}
		if nameIdx != -1 {
			f.Name = strVal(row, nameIdx)
		}
		if sizeIdx != -1 {
			if v, err2 := strconv.ParseInt(strVal(row, sizeIdx), 10, 64); err2 == nil {
				f.Size = v
			}
		}
		if md5Idx != -1 {
			f.MD5 = strVal(row, md5Idx)
		}
		if lastModIdx != -1 {
			f.LastModified = strVal(row, lastModIdx)
		}
		files = append(files, f)
	}

	return files, nil
}

// strVal handles type assertions for interface{} row values to strings.
func strVal(row []interface{}, idx int) string {
	if idx < 0 || idx >= len(row) || row[idx] == nil {
		return ""
	}
	switch v := row[idx].(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// validateStageRef guards a stage reference that is spliced unquoted into a PUT/GET
// statement. Its path segment can be free-typed by the user (see UploadToStageModal),
// so this is the choke point that stops injection such as ".../x; DROP TABLE y; --"
// from being split into a second statement by client.Execute. A legitimate reference
// is @[db.schema.]stage[/path] with optionally double-quoted identifiers, so it never
// contains a statement terminator, a string-literal quote, or a newline.
func validateStageRef(stageName string) error {
	if strings.ContainsAny(stageName, ";'\n\r\x00") {
		return fmt.Errorf("invalid stage reference %q: contains illegal characters", stageName)
	}
	return nil
}

// UploadFileToStage executes a PUT command to upload a local file to an internal stage.
func UploadFileToStage(ctx context.Context, client *snowflake.Client, localPath string, stageName string, parallel int, autoCompress bool, sourceCompression string, overwrite bool) error {
	// Ensure stageName starts with @
	if !strings.HasPrefix(stageName, "@") {
		stageName = "@" + stageName
	}

	if err := validateStageRef(stageName); err != nil {
		return err
	}

	sql := fmt.Sprintf("PUT 'file://%s' %s", strings.ReplaceAll(localPath, "'", "\\'"), stageName)
	if parallel > 0 {
		sql += fmt.Sprintf(" PARALLEL = %d", parallel)
	}
	if autoCompress {
		sql += " AUTO_COMPRESS = TRUE"
	} else {
		sql += " AUTO_COMPRESS = FALSE"
	}
	if sourceCompression != "" && sourceCompression != "AUTO_DETECT" {
		sql += fmt.Sprintf(" SOURCE_COMPRESSION = %s", sourceCompression)
	}
	if overwrite {
		sql += " OVERWRITE = TRUE"
	} else {
		sql += " OVERWRITE = FALSE"
	}

	_, err := client.Execute(ctx, sql)
	return err
}

// DownloadFileFromStage executes a GET command to download files from an internal stage to a local directory.
func DownloadFileFromStage(ctx context.Context, client *snowflake.Client, stageName string, localDirPath string, parallel int, pattern string) error {
	// Ensure stageName starts with @
	if !strings.HasPrefix(stageName, "@") {
		stageName = "@" + stageName
	}

	sql := fmt.Sprintf("GET %s 'file://%s'", stageName, strings.ReplaceAll(localDirPath, "'", "\\'"))
	if parallel > 0 {
		sql += fmt.Sprintf(" PARALLEL = %d", parallel)
	}
	if pattern != "" {
		sql += fmt.Sprintf(" PATTERN = '%s'", snowflake.EscapeStringLit(pattern))
	}

	_, err := client.Execute(ctx, sql)
	return err
}

// RemoveStageFiles deletes files from a stage using the REMOVE command.
func RemoveStageFiles(ctx context.Context, client *snowflake.Client, stageName string, pattern string) error {
	// Ensure stageName starts with @
	if !strings.HasPrefix(stageName, "@") {
		stageName = "@" + stageName
	}

	sql := fmt.Sprintf("REMOVE %s", stageName)
	if pattern != "" {
		sql += fmt.Sprintf(" PATTERN = '%s'", snowflake.EscapeStringLit(pattern))
	}

	_, err := client.Execute(ctx, sql)
	return err
}
