// SPDX-License-Identifier: GPL-3.0-or-later

// Package users implements Snowflake user administration: the ALTER USER
// per-property SQL builder (with enum/integer validation and empty-value →
// UNSET semantics), layered over the Snowflake client. Mirrors the
// internal/warehouse property-builder pattern.
//
// thaw:domain: Object Browser & Administration
package users
