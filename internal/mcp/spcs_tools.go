// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/computepool"
	"thaw/internal/gateway"
	"thaw/internal/imagerepository"
	"thaw/internal/service"
	"thaw/internal/snapshot"
	"thaw/internal/snowflake"
)

// Tool input types for Snowpark Container Services tools.

type spcsObjectInput struct {
	Database string `json:"database" jsonschema:"the database name"`
	Schema   string `json:"schema" jsonschema:"the schema name"`
	Name     string `json:"name" jsonschema:"the object name"`
}

type serviceLogsInput struct {
	Database   string `json:"database" jsonschema:"the database name"`
	Schema     string `json:"schema" jsonschema:"the schema name"`
	Name       string `json:"name" jsonschema:"the service name"`
	Container  string `json:"container" jsonschema:"the container name from the service specification (see list_service_containers)"`
	InstanceID int    `json:"instanceId,omitempty" jsonschema:"the 0-based service instance index (default 0)"`
	Lines      int    `json:"lines,omitempty" jsonschema:"cap on the number of trailing log lines; 0 or omitted returns all"`
}

type buildCreateServiceInput struct {
	Database string                `json:"database" jsonschema:"the database name"`
	Schema   string                `json:"schema" jsonschema:"the schema name"`
	Config   service.ServiceConfig `json:"config" jsonschema:"the service configuration"`
}

type buildCreateSnapshotInput struct {
	Database string                  `json:"database" jsonschema:"the database name"`
	Schema   string                  `json:"schema" jsonschema:"the schema name"`
	Config   snapshot.SnapshotConfig `json:"config" jsonschema:"the snapshot configuration"`
}

type buildCreateImageRepositoryInput struct {
	Database string                                `json:"database" jsonschema:"the database name"`
	Schema   string                                `json:"schema" jsonschema:"the schema name"`
	Config   imagerepository.ImageRepositoryConfig `json:"config" jsonschema:"the image repository configuration"`
}

type buildCreateGatewayInput struct {
	Database string                `json:"database" jsonschema:"the database name"`
	Schema   string                `json:"schema" jsonschema:"the schema name"`
	Config   gateway.GatewayConfig `json:"config" jsonschema:"the gateway configuration"`
}

// registerSPCSTools wires Snowpark Container Services tools onto srv: read-only
// SHOW / DESCRIBE / log tools (raw QueryResult so every column the account
// reports comes through) and pure CREATE builders. All are registered in every
// execution mode. There is deliberately no mutating tool — suspend / resume /
// EXECUTE JOB SERVICE terminate or start containers, so the AI proposes SQL via
// the builders and the user runs it through open_sql_tab.
func registerSPCSTools(srv *mcpsdk.Server, client *snowflake.Client) {

	// run executes one read-only statement and returns the raw QueryResult.
	run := func(ctx context.Context, sql string) (*mcpsdk.CallToolResult, any, error) {
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		res, err := client.Execute(ctx, sql)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(res), nil, nil
	}

	// ── Account-wide listings ────────────────────────────────────────────

	addAccount := func(name, desc, sql string) {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: name, Description: desc},
			func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
				return run(ctx, sql)
			})
	}
	addAccount("list_compute_pools",
		"List all compute pools (SHOW COMPUTE POOLS) with every column: state, instance family, min/max nodes, active/idle nodes, auto-suspend, owner, comment.",
		"SHOW COMPUTE POOLS")
	addAccount("list_compute_pool_instance_families",
		"List the compute pool instance families available in this account/region (SHOW COMPUTE POOL INSTANCE FAMILIES): vCPU, memory, GPU.",
		"SHOW COMPUTE POOL INSTANCE FAMILIES")
	addAccount("list_snapshots",
		"List all service block-volume snapshots in the account (SHOW SNAPSHOTS IN ACCOUNT).",
		"SHOW SNAPSHOTS IN ACCOUNT")

	// ── Compute pool (account-level object, name only) ───────────────────

	addPool := func(name, desc, prefix string) {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: name, Description: desc},
			func(ctx context.Context, _ *mcpsdk.CallToolRequest, in nameInput) (*mcpsdk.CallToolResult, any, error) {
				if in.Name == "" {
					return nil, nil, fmt.Errorf("name is required")
				}
				return run(ctx, prefix+" "+snowflake.QuoteIdent(in.Name))
			})
	}
	addPool("describe_compute_pool",
		"Describe a compute pool (DESCRIBE COMPUTE POOL): state, nodes, instance family, applications, error code/status message.",
		"DESCRIBE COMPUTE POOL")
	addPool("list_compute_pool_nodes",
		"List the nodes of a compute pool (SHOW NODES IN COMPUTE POOL).",
		"SHOW NODES IN COMPUTE POOL")

	// ── Schema-level objects (database.schema.name) ──────────────────────

	addObject := func(name, desc, prefix string) {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: name, Description: desc},
			func(ctx context.Context, _ *mcpsdk.CallToolRequest, in spcsObjectInput) (*mcpsdk.CallToolResult, any, error) {
				if in.Database == "" {
					return nil, nil, fmt.Errorf("database is required")
				}
				if in.Schema == "" {
					return nil, nil, fmt.Errorf("schema is required")
				}
				if in.Name == "" {
					return nil, nil, fmt.Errorf("name is required")
				}
				return run(ctx, prefix+" "+snowflake.Qualify(in.Database, in.Schema, in.Name))
			})
	}
	addObject("describe_service",
		"Describe a Snowpark Container Services service (DESCRIBE SERVICE): status, compute pool, instances, DNS name, and the service specification.",
		"DESCRIBE SERVICE")
	addObject("list_service_endpoints",
		"List the endpoints of a service (SHOW ENDPOINTS IN SERVICE): port, protocol, ingress enabled, ingress URL.",
		"SHOW ENDPOINTS IN SERVICE")
	addObject("list_service_containers",
		"List the containers of a service per instance (SHOW SERVICE CONTAINERS IN SERVICE): status, message, image, restart count.",
		"SHOW SERVICE CONTAINERS IN SERVICE")
	addObject("list_service_instances",
		"List the instances of a service (SHOW SERVICE INSTANCES IN SERVICE): status, spec digest, start time.",
		"SHOW SERVICE INSTANCES IN SERVICE")
	addObject("list_service_volumes",
		"List the volumes mounted by a service's containers (SHOW SERVICE VOLUMES IN SERVICE).",
		"SHOW SERVICE VOLUMES IN SERVICE")
	addObject("list_service_roles",
		"List the service roles declared in a service's specification (SHOW ROLES IN SERVICE).",
		"SHOW ROLES IN SERVICE")
	addObject("list_images_in_repository",
		"List the images stored in an image repository (SHOW IMAGES IN IMAGE REPOSITORY): image name, tags, digest, image path.",
		"SHOW IMAGES IN IMAGE REPOSITORY")
	addObject("describe_snapshot",
		"Describe a service block-volume snapshot (DESCRIBE SNAPSHOT).",
		"DESCRIBE SNAPSHOT")
	addObject("describe_gateway",
		"Describe a gateway (DESCRIBE GATEWAY): ingress URL and the traffic-split specification.",
		"DESCRIBE GATEWAY")

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_service_logs",
		Description: "Return the container logs of one service instance via SYSTEM$GET_SERVICE_LOGS. " +
			"Use list_service_containers to find container names and instance ids.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in serviceLogsInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema is required")
		}
		if in.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		if in.Container == "" {
			return nil, nil, fmt.Errorf("container is required")
		}
		if in.InstanceID < 0 || in.Lines < 0 {
			return nil, nil, fmt.Errorf("instanceId and lines must be >= 0")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		sql := service.BuildGetServiceLogsSql(in.Database, in.Schema, in.Name, in.Container, in.InstanceID, in.Lines)
		res, err := client.Execute(ctx, sql)
		if err != nil {
			return nil, nil, err
		}
		if res == nil || len(res.Rows) == 0 || len(res.Rows[0]) == 0 || res.Rows[0][0] == nil {
			return textResult(""), nil, nil
		}
		return textResult(fmt.Sprint(res.Rows[0][0])), nil, nil
	})

	// ── Builders (pure SQL generators, no client) ────────────────────────

	requireDBSchema := func(db, schema string) error {
		if db == "" {
			return fmt.Errorf("database is required")
		}
		if schema == "" {
			return fmt.Errorf("schema is required")
		}
		return nil
	}
	sqlResult := func(sql string, err error) (*mcpsdk.CallToolResult, any, error) {
		if err != nil {
			return nil, nil, err
		}
		return textResult(sql), nil, nil
	}

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_create_compute_pool_sql",
		Description: "Generate a CREATE COMPUTE POOL statement from a compute pool configuration. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in computepool.ComputePoolConfig) (*mcpsdk.CallToolResult, any, error) {
		return sqlResult(computepool.BuildCreateComputePoolSql(in))
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_create_service_sql",
		Description: "Generate a CREATE SERVICE statement (inline or staged specification) from a service configuration. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildCreateServiceInput) (*mcpsdk.CallToolResult, any, error) {
		if err := requireDBSchema(in.Database, in.Schema); err != nil {
			return nil, nil, err
		}
		return sqlResult(service.BuildCreateServiceSql(in.Database, in.Schema, in.Config))
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_execute_job_service_sql",
		Description: "Generate an EXECUTE JOB SERVICE statement from a job service configuration. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in service.JobServiceConfig) (*mcpsdk.CallToolResult, any, error) {
		return sqlResult(service.BuildExecuteJobServiceSql(in))
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_create_snapshot_sql",
		Description: "Generate a CREATE SNAPSHOT statement (snapshot of a service block volume) from a snapshot configuration. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildCreateSnapshotInput) (*mcpsdk.CallToolResult, any, error) {
		if err := requireDBSchema(in.Database, in.Schema); err != nil {
			return nil, nil, err
		}
		return sqlResult(snapshot.BuildCreateSnapshotSql(in.Database, in.Schema, in.Config))
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_create_image_repository_sql",
		Description: "Generate a CREATE IMAGE REPOSITORY statement. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildCreateImageRepositoryInput) (*mcpsdk.CallToolResult, any, error) {
		if err := requireDBSchema(in.Database, in.Schema); err != nil {
			return nil, nil, err
		}
		return sqlResult(imagerepository.BuildCreateImageRepositorySql(in.Database, in.Schema, in.Config))
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "build_create_gateway_sql",
		Description: "Generate a CREATE GATEWAY statement (traffic-split specification) from a gateway configuration. Returns the SQL string without executing it.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in buildCreateGatewayInput) (*mcpsdk.CallToolResult, any, error) {
		if err := requireDBSchema(in.Database, in.Schema); err != nil {
			return nil, nil, err
		}
		return sqlResult(gateway.BuildCreateGatewaySql(in.Database, in.Schema, in.Config))
	})
}
