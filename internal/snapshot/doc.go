// SPDX-License-Identifier: GPL-3.0-or-later

// Package snapshot builds SQL for Snowpark Container Services volume snapshots —
// CREATE SNAPSHOT statements and the structured config behind them. A snapshot
// is a point-in-time copy of one service instance's block-storage volume,
// stored as a schema-level object; services can later be created with volumes
// restored from it.
//
// ALTER SNAPSHOT's entire surface is SET COMMENT (there is no RENAME and no
// UNSET), so the edit and the DESCRIBE / UNDROP / DROP / SHOW … IN ACCOUNT
// statements are issued directly from internal/app/snapshot.go. GET_DDL does
// not list SNAPSHOT as an object type, so there is no DDL-export path.
//
// thaw:domain: Object Browser & Administration
package snapshot
