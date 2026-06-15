// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Package service builds SQL for Snowflake Snowpark Container Services (SPCS)
// SERVICE objects — CREATE SERVICE statements and the structured config behind
// them. A service is a long-running, containerized application that runs in a
// compute pool; it is defined by a YAML specification (inline or staged,
// optionally a Jinja SPECIFICATION_TEMPLATE bound by a USING ( key => value, … )
// clause) and exposes endpoints, logs, and a container status.
//
// Unlike most schema objects, services have no OR REPLACE and cannot be renamed
// (ALTER SERVICE has no RENAME TO clause). The mutable knobs — MIN_INSTANCES,
// MAX_INSTANCES, AUTO_RESUME, QUERY_WAREHOUSE, EXTERNAL_ACCESS_INTEGRATIONS,
// COMMENT — plus the SUSPEND/RESUME lifecycle are issued as free-form ALTER
// SERVICE statements from internal/app/service.go (App.AlterService). Container
// logs (SYSTEM$GET_SERVICE_LOGS), status (SHOW SERVICE CONTAINERS) and ingress
// endpoints (SHOW ENDPOINTS IN SERVICE) are surfaced lazily by the properties
// panel via dedicated App methods. GET_DDL does not support services, so there
// is no DDL-export path for this type.
//
// thaw:domain: Object Browser & Administration
package service
