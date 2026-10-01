// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/computepool"
	"thaw/internal/service"
	"thaw/internal/snapshot"
)

var spcsToolNames = []string{
	"list_compute_pools",
	"describe_compute_pool",
	"list_compute_pool_nodes",
	"list_compute_pool_instance_families",
	"describe_service",
	"list_service_endpoints",
	"list_service_containers",
	"list_service_instances",
	"list_service_volumes",
	"list_service_roles",
	"get_service_logs",
	"list_images_in_repository",
	"list_snapshots",
	"describe_snapshot",
	"describe_gateway",
	"build_create_compute_pool_sql",
	"build_create_service_sql",
	"build_execute_job_service_sql",
	"build_create_snapshot_sql",
	"build_create_image_repository_sql",
	"build_create_gateway_sql",
}

// TestSPCSToolsRegistered verifies every SPCS tool is registered in all modes.
func TestSPCSToolsRegistered(t *testing.T) {
	for _, mode := range []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			names := toolNames(t, buildServer(nil, mode, SessionConfig{}, nil, nil, nil, nil))
			for _, tool := range spcsToolNames {
				if !hasToolName(names, tool) {
					t.Errorf("mode %q: expected tool %q to be registered", mode, tool)
				}
			}
		})
	}
}

// callSPCS calls a tool and returns its text plus whether it was an error.
func callSPCS(t *testing.T, cs *mcpsdk.ClientSession, name string, args any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	return extractText(t, res), res.IsError
}

func TestSPCSToolsValidationAndNilClient(t *testing.T) {
	cs := newTestSession(t)
	cases := []struct {
		tool, want string
		args       any
	}{
		{"list_compute_pools", "no Snowflake connection", emptyInput{}},
		{"describe_compute_pool", "name is required", nameInput{}},
		{"describe_compute_pool", "no Snowflake connection", nameInput{Name: "POOL"}},
		{"describe_service", "database is required", spcsObjectInput{Schema: "S", Name: "N"}},
		{"list_service_roles", "schema is required", spcsObjectInput{Database: "D", Name: "N"}},
		{"describe_gateway", "name is required", spcsObjectInput{Database: "D", Schema: "S"}},
		{"describe_snapshot", "no Snowflake connection", spcsObjectInput{Database: "D", Schema: "S", Name: "N"}},
		{"get_service_logs", "container is required", serviceLogsInput{Database: "D", Schema: "S", Name: "N"}},
		{"get_service_logs", "must be >= 0", serviceLogsInput{Database: "D", Schema: "S", Name: "N", Container: "c", Lines: -1}},
		{"get_service_logs", "no Snowflake connection", serviceLogsInput{Database: "D", Schema: "S", Name: "N", Container: "c"}},
		{"build_create_service_sql", "database is required", buildCreateServiceInput{Schema: "S"}},
		{"build_create_gateway_sql", "schema is required", buildCreateGatewayInput{Database: "D"}},
	}
	for _, c := range cases {
		text, isErr := callSPCS(t, cs, c.tool, c.args)
		if !isErr || !strings.Contains(text, c.want) {
			t.Errorf("%s(%+v): want error containing %q, got isError=%v %q", c.tool, c.args, c.want, isErr, text)
		}
	}
}

// TestSPCSBuildersMatchDomainBuilders verifies the builder tools render exactly
// what the dialogs' previews render (the domain builders).
func TestSPCSBuildersMatchDomainBuilders(t *testing.T) {
	cs := newTestSession(t)

	pool := computepool.ComputePoolConfig{Name: "POOL", MinNodes: 1, MaxNodes: 2, InstanceFamily: "CPU_X64_XS"}
	svc := service.ServiceConfig{Name: "SVC", ComputePool: "POOL", SpecSource: service.SpecSourceInline, SpecInline: "spec: {}"}
	job := service.JobServiceConfig{Name: "JOB", ComputePool: "POOL", SpecSource: service.SpecSourceInline, SpecInline: "spec: {}"}
	snap := snapshot.SnapshotConfig{Name: "SNAP", ServiceDatabase: "D", ServiceSchema: "S", ServiceName: "SVC", Volume: "data", Instance: "0"}

	wantPool, _ := computepool.BuildCreateComputePoolSql(pool)
	wantSvc, _ := service.BuildCreateServiceSql("D", "S", svc)
	wantJob, _ := service.BuildExecuteJobServiceSql(job)
	wantSnap, _ := snapshot.BuildCreateSnapshotSql("D", "S", snap)

	for _, c := range []struct {
		tool, want string
		args       any
	}{
		{"build_create_compute_pool_sql", wantPool, pool},
		{"build_create_service_sql", wantSvc, buildCreateServiceInput{Database: "D", Schema: "S", Config: svc}},
		{"build_execute_job_service_sql", wantJob, job},
		{"build_create_snapshot_sql", wantSnap, buildCreateSnapshotInput{Database: "D", Schema: "S", Config: snap}},
	} {
		text, isErr := callSPCS(t, cs, c.tool, c.args)
		if isErr || text != c.want {
			t.Errorf("%s: got isError=%v %q, want %q", c.tool, isErr, text, c.want)
		}
	}
}
