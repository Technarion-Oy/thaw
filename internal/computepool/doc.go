// SPDX-License-Identifier: GPL-3.0-or-later

// Package computepool builds SQL for Snowpark Container Services COMPUTE POOL
// objects — the account-level pools of VM nodes that SERVICE and EXECUTE JOB
// SERVICE workloads run on. It covers CREATE COMPUTE POOL, the per-property
// ALTER … SET / UNSET statements behind the Properties modal, and STOP ALL.
// Listing (SHOW COMPUTE POOLS / SHOW NODES / SHOW COMPUTE POOL INSTANCE
// FAMILIES), DESCRIBE, SUSPEND / RESUME and DROP are plain one-liners issued
// from internal/app/computepool.go.
//
// thaw:domain: Snowpark & Developer Workflows
package computepool
