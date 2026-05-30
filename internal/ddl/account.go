// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package ddl

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"thaw/internal/filesystem"
	"thaw/internal/snowflake"
)

// AccountExportResult reports the outcome of exporting account-level objects.
type AccountExportResult struct {
	Roles      int      `json:"roles"`
	Warehouses int      `json:"warehouses"`
	Errors     []string `json:"errors,omitempty"`
}

// ExportAccountObjects exports all accessible roles and warehouses as SQL files
// under <outputDir>/_account/roles/ and <outputDir>/_account/warehouses/.
func ExportAccountObjects(ctx context.Context, client *snowflake.Client, outputDir string) (AccountExportResult, error) {
	var result AccountExportResult

	// ── Roles ────────────────────────────────────────────────────────────────
	roles, err := client.ListRoles(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list roles: %v", err))
	} else {
		for _, role := range roles {
			src, err := client.GetRoleDDL(ctx, role)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("role %s: %v", role, err))
				continue
			}
			path := filepath.Join(outputDir, "_account", "roles", filesystem.SanitizeFilename(role)+".sql")
			if writeErr := filesystem.WriteFile(path, strings.TrimRight(src, "\n")+"\n"); writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("write role %s: %v", role, writeErr))
				continue
			}
			result.Roles++
		}
	}

	// ── Warehouses ───────────────────────────────────────────────────────────
	warehouses, err := client.ListWarehouses(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list warehouses: %v", err))
	} else {
		for _, wh := range warehouses {
			src, err := client.GetWarehouseDDL(ctx, wh)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("warehouse %s: %v", wh, err))
				continue
			}
			path := filepath.Join(outputDir, "_account", "warehouses", filesystem.SanitizeFilename(wh)+".sql")
			if writeErr := filesystem.WriteFile(path, strings.TrimRight(src, "\n")+"\n"); writeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("write warehouse %s: %v", wh, writeErr))
				continue
			}
			result.Warehouses++
		}
	}

	return result, nil
}
