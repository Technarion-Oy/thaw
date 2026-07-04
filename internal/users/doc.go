// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package users implements Snowflake user administration: the ALTER USER
// per-property SQL builder (with enum/integer validation and empty-value →
// UNSET semantics), layered over the Snowflake client. Mirrors the
// internal/warehouse property-builder pattern.
//
// thaw:domain: Object Browser & Administration
package users
