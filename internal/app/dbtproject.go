// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"encoding/json"
	"fmt"
	"thaw/internal/apperrors"
	"thaw/internal/dbt"
	"thaw/internal/dbtproject"
	"thaw/internal/snowflake"
)

// DescribeDbtProject runs DESCRIBE DBT PROJECT and returns key/value pairs.
func (a *App) DescribeDbtProject(database, schema, name string) ([]snowflake.PropertyPair, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	res, err := client.Execute(a.ctx, dbtproject.BuildDescribeSql(database, schema, name))
	if err != nil {
		return nil, err
	}
	return snowflake.ResultToPairs(res), nil
}

// ListSupportedDbtVersions returns the dbt versions supported by the account.
func (a *App) ListSupportedDbtVersions() ([]dbtproject.DbtVersionInfo, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	res, err := client.Execute(a.ctx, "SELECT SYSTEM$SUPPORTED_DBT_VERSIONS()")
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
		return nil, nil
	}
	raw, ok := res.Rows[0][0].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected type for dbt versions: %T", res.Rows[0][0])
	}
	var versions []dbtproject.DbtVersionInfo
	if err := json.Unmarshal([]byte(raw), &versions); err != nil {
		return nil, fmt.Errorf("failed to parse dbt versions: %w", err)
	}
	return versions, nil
}

// ListDbtProjectVersions returns all versions of a DBT PROJECT.
func (a *App) ListDbtProjectVersions(database, schema, name string) ([]snowflake.DbtProjectVersion, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListDbtProjectVersions(a.ctx, database, schema, name)
}

// ListDbtProjectEntries returns directory-aware entries within a DBT PROJECT version directory.
func (a *App) ListDbtProjectEntries(database, schema, name, dirPath string) ([]snowflake.GitRepoEntry, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return client.ListStageEntries(a.ctx, database, schema, name, dirPath) // SQL pattern is identical: LIST @db.schema.name/path
}

// CreateDbtProject scaffolds a new dbt project pre-wired to the active
// Snowflake connection.
//
// req describes the project name, output directory and optional profile name.
// schemasMap maps database names to the list of schemas to include as sources.
func (a *App) CreateDbtProject(req dbt.CreateRequest, schemasMap map[string][]string) (*dbt.CreateResult, error) {
	client := a.currentClient()
	if client == nil {
		return nil, apperrors.ErrNotConnected
	}
	return dbt.CreateProject(a.ctx, client, req, schemasMap)
}
