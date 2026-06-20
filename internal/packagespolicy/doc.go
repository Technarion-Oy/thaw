// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package packagespolicy builds SQL for Snowflake packages policy objects —
// CREATE PACKAGES POLICY statements and the structured config behind them. A
// packages policy is a schema-level governance object that controls which
// third-party packages (currently Python) may be imported by UDFs and stored
// procedures: an ALLOWLIST of permitted package specifications, a BLOCKLIST of
// forbidden ones (the blocklist takes precedence), and an
// ADDITIONAL_CREATION_BLOCKLIST that blocks packages only at object-creation
// time. LANGUAGE PYTHON is required (the only language currently supported). A
// policy is attached to the account via ALTER ACCOUNT … SET PACKAGES POLICY.
//
// The list parameters are slices of bare package-spec tokens (e.g. "numpy",
// "numpy==1.26.4", "*") which the builder renders as single-quoted string
// literals. The ALTER clauses (SET/UNSET ALLOWLIST, BLOCKLIST,
// ADDITIONAL_CREATION_BLOCKLIST, COMMENT) are simple enough to be issued as
// free-form ALTER PACKAGES POLICY statements from
// internal/app/packagespolicy.go (App.AlterPackagesPolicy); the current list
// values are read back via the DESCRIBE enrichment in internal/objects.
// ALTER PACKAGES POLICY has no RENAME or TAG support.
//
// thaw:domain: Object Browser & Administration
package packagespolicy
