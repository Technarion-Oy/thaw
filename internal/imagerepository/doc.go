// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package imagerepository builds SQL for Snowflake image repository objects —
// CREATE IMAGE REPOSITORY statements and the structured config behind them. An
// image repository is an OCI-compliant registry that stores container images
// used by Snowpark Container Services (services and jobs). Each repository
// exposes a repository_url; images pushed to it are listed with SHOW IMAGES IN
// IMAGE REPOSITORY.
//
// Image repositories have very few knobs: only OR REPLACE / IF NOT EXISTS and an
// optional COMMENT on creation. They cannot be renamed, and the only mutable
// property is COMMENT, so the edit clauses (SET/UNSET COMMENT) are issued as
// free-form ALTER IMAGE REPOSITORY statements from
// internal/app/imagerepository.go (App.AlterImageRepository). GET_DDL does not
// support image repositories, so there is no DDL-export path for this type.
//
// thaw:domain: Object Browser & Administration
package imagerepository
