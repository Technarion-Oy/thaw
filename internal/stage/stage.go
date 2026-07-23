// SPDX-License-Identifier: GPL-3.0-or-later
// thaw:domain: Object Browser & Administration

package stage

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	stageName, err := snowflake.NormalizeStageRef(stageName)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`LIST %s`, stageName)
	if pattern != "" {
		sql += fmt.Sprintf(" PATTERN = '%s'", snowflake.EscapeStringLit(pattern))
	}

	res, err := client.Execute(ctx, sql)
	if err != nil {
		return nil, err
	}

	// StrVal returns "" for an absent (-1) column, so the cells are read
	// unconditionally.
	idxs := snowflake.ColumnIndexes(res, "name", "size", "md5", "last_modified")

	var files []StageFile
	for _, row := range res.Rows {
		f := StageFile{
			Name:         snowflake.StrVal(row, idxs["name"]),
			MD5:          snowflake.StrVal(row, idxs["md5"]),
			LastModified: snowflake.StrVal(row, idxs["last_modified"]),
		}
		if v, err2 := strconv.ParseInt(snowflake.StrVal(row, idxs["size"]), 10, 64); err2 == nil {
			f.Size = v
		}
		files = append(files, f)
	}

	return files, nil
}

// UploadFileToStage executes a PUT command to upload a local file to an internal stage.
func UploadFileToStage(ctx context.Context, client *snowflake.Client, localPath string, stageName string, parallel int, autoCompress bool, sourceCompression string, overwrite bool) error {
	stageName, err := snowflake.NormalizeStageRef(stageName)
	if err != nil {
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

	_, err = client.Execute(ctx, sql)
	return err
}

// ── Recursive directory upload ──────────────────────────────────────────────

// dirUpload is one planned file upload: an absolute local file path and the
// '/'-separated stage subdirectory (relative to the stage root) it belongs in
// ("" for the folder root).
type dirUpload struct {
	Path   string
	RelDir string
}

// isJunkDir reports whether a directory should be skipped when uploading a local
// folder: VCS metadata, Python bytecode caches, and any hidden (dot) directory.
func isJunkDir(name string) bool {
	return name == ".git" || name == "__pycache__" || strings.HasPrefix(name, ".")
}

// isJunkFile reports whether a file should be skipped: OS junk and any hidden
// (dot) file (".DS_Store" is covered by the dot rule but named for clarity).
func isJunkFile(name string) bool {
	return name == ".DS_Store" || strings.HasPrefix(name, ".")
}

// planDirUploads walks the folder at root and returns an ordered, deterministic
// set of file uploads that reproduce its non-junk tree, preserving relative
// paths. It performs no I/O beyond the walk, so it is unit-testable without a
// live connection. Skips .git/, __pycache__/, other hidden directories, hidden
// files, and .DS_Store (see isJunkDir / isJunkFile).
func planDirUploads(root string) ([]dirUpload, error) {
	var ups []dirUpload
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != root && isJunkDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isJunkFile(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relDir := filepath.ToSlash(filepath.Dir(rel))
		if relDir == "." {
			relDir = ""
		}
		ups = append(ups, dirUpload{Path: path, RelDir: relDir})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(ups, func(i, j int) bool { return ups[i].Path < ups[j].Path })
	return ups, nil
}

// UploadDirToStage recursively uploads every non-junk file under localDir to
// stageName, preserving each file's path relative to localDir — one PUT per file
// via UploadFileToStage (AUTO_COMPRESS=FALSE). stageName is the stage root (e.g.
// "@DB.SCHEMA.STAGE"); files land under "<stageName>/<relative subdir>". VCS
// metadata, hidden files, __pycache__, and .DS_Store are skipped.
func UploadDirToStage(ctx context.Context, client *snowflake.Client, localDir, stageName string, overwrite bool) error {
	root, err := filepath.Abs(localDir)
	if err != nil {
		return fmt.Errorf("resolve app dir: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("stat app dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("app path is not a directory: %s", root)
	}

	ups, err := planDirUploads(root)
	if err != nil {
		return err
	}
	if len(ups) == 0 {
		return fmt.Errorf("no files to upload in %s", root)
	}

	base := strings.TrimRight(stageName, "/")
	for _, u := range ups {
		target := base
		if u.RelDir != "" {
			target = base + "/" + u.RelDir
		}
		if err := UploadFileToStage(ctx, client, u.Path, target, 0, false, "", overwrite); err != nil {
			return fmt.Errorf("upload %s: %w", u.Path, err)
		}
	}
	return nil
}

// DownloadFileFromStage executes a GET command to download files from an internal stage to a local directory.
func DownloadFileFromStage(ctx context.Context, client *snowflake.Client, stageName string, localDirPath string, parallel int, pattern string) error {
	stageName, err := snowflake.NormalizeStageRef(stageName)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf("GET %s 'file://%s'", stageName, strings.ReplaceAll(localDirPath, "'", "\\'"))
	if parallel > 0 {
		sql += fmt.Sprintf(" PARALLEL = %d", parallel)
	}
	if pattern != "" {
		sql += fmt.Sprintf(" PATTERN = '%s'", snowflake.EscapeStringLit(pattern))
	}

	_, err = client.Execute(ctx, sql)
	return err
}

// RemoveStageFiles deletes files from a stage using the REMOVE command.
func RemoveStageFiles(ctx context.Context, client *snowflake.Client, stageName string, pattern string) error {
	stageName, err := snowflake.NormalizeStageRef(stageName)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf("REMOVE %s", stageName)
	if pattern != "" {
		sql += fmt.Sprintf(" PATTERN = '%s'", snowflake.EscapeStringLit(pattern))
	}

	_, err = client.Execute(ctx, sql)
	return err
}
