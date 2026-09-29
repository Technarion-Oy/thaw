// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"fmt"
	"strconv"
	"strings"

	"thaw/internal/snowflake"
)

// Spec source modes for ServiceConfig.SpecSource.
const (
	SpecSourceInline = "inline" // FROM SPECIFICATION[_TEMPLATE] $$ … $$
	SpecSourceStage  = "stage"  // FROM @<stage> SPECIFICATION[_TEMPLATE]_FILE = '…'
)

// TemplateVar is a single name => value binding for the USING clause of a
// templated service specification (SPECIFICATION_TEMPLATE). Values are rendered
// as SQL literals: numbers and TRUE/FALSE/NULL keywords are emitted bare, all
// other values are single-quoted string literals.
type TemplateVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ServiceConfig holds the parameters for creating a Snowflake SERVICE (Snowpark
// Container Services). Snowflake requires the compute pool and the service
// specification first, then the remaining properties in documented order. The
// specification can be supplied inline (a YAML string) or referenced from a
// stage file; SpecSource selects which. When Template is set, the specification
// is treated as a Jinja template (SPECIFICATION_TEMPLATE / TEMPLATE_FILE) and
// the TemplateVars are emitted as a USING ( key => value, … ) clause. SERVICE
// has no OR REPLACE, so only IF NOT EXISTS is offered.
type ServiceConfig struct {
	Name                       string        `json:"name"`
	CaseSensitive              bool          `json:"caseSensitive"`
	IfNotExists                bool          `json:"ifNotExists"`
	ComputePool                string        `json:"computePool"`
	SpecSource                 string        `json:"specSource"`                 // "inline" | "stage"
	Template                   bool          `json:"template"`                   // use SPECIFICATION_TEMPLATE[_FILE] + USING
	SpecInline                 string        `json:"specInline"`                 // YAML text (SpecSource = inline)
	SpecStage                  string        `json:"specStage"`                  // stage name, with or without leading @ (SpecSource = stage)
	SpecFile                   string        `json:"specFile"`                   // path to the YAML file within the stage (SpecSource = stage)
	TemplateVars               []TemplateVar `json:"templateVars"`               // USING ( key => value, … ) bindings (Template only)
	ExternalAccessIntegrations string        `json:"externalAccessIntegrations"` // comma-separated EAI names
	AutoResume                 string        `json:"autoResume"`                 // TRUE | FALSE (or "" for default)
	MinInstances               string        `json:"minInstances"`               // integer string or ""
	MaxInstances               string        `json:"maxInstances"`               // integer string or ""
	QueryWarehouse             string        `json:"queryWarehouse"`             // warehouse for service functions / queries
	Comment                    string        `json:"comment"`
}

// specClause renders the FROM … specification clause (and, for templates, the
// trailing USING binding clause). Inline specs are wrapped in dollar-quoting
// ($$ … $$) so multi-line YAML needs no escaping; staged specs reference a stage
// and a file path. The TEMPLATE variants are emitted when cfg.Template is set.
// When the chosen source is empty the builder emits an obvious placeholder so
// the preview stays a completable template. An inline spec containing $$ would
// end the dollar-quoted block early, so it is rejected.
func specClause(cfg ServiceConfig) (string, error) {
	var sb strings.Builder

	switch cfg.SpecSource {
	case SpecSourceStage:
		stage := strings.TrimSpace(cfg.SpecStage)
		stage = strings.TrimPrefix(stage, "@")
		if stage == "" {
			stage = "<stage>"
		}
		file := strings.TrimSpace(cfg.SpecFile)
		if file == "" {
			file = "spec.yaml"
		}
		keyword := "SPECIFICATION_FILE"
		if cfg.Template {
			keyword = "SPECIFICATION_TEMPLATE_FILE"
		}
		fmt.Fprintf(&sb, "FROM @%s\n  %s = '%s'", stage, keyword, snowflake.EscapeStringLit(file))
	default: // inline
		if err := checkDollarSpec(cfg.SpecInline); err != nil {
			return "", err
		}
		spec := strings.TrimRight(strings.TrimSpace(cfg.SpecInline), "\n")
		if spec == "" {
			spec = "spec:\n  containers:\n  - name: main\n    image: /db/schema/repo/image:latest"
		}
		keyword := "SPECIFICATION"
		if cfg.Template {
			keyword = "SPECIFICATION_TEMPLATE"
		}
		fmt.Fprintf(&sb, "FROM %s $$\n%s\n$$", keyword, spec)
	}

	// USING ( key => value, … ) applies only to templated specifications.
	if cfg.Template {
		if u := usingClause(cfg.TemplateVars); u != "" {
			fmt.Fprintf(&sb, "\n  %s", u)
		}
	}

	return sb.String(), nil
}

// checkDollarSpec rejects an inline spec containing $$, which would end the
// $$ … $$ block it is wrapped in.
func checkDollarSpec(spec string) error {
	if strings.Contains(spec, "$$") {
		return fmt.Errorf("inline specification cannot contain $$")
	}
	return nil
}

// usingClause renders the USING ( key => value, … ) binding list for a templated
// specification, skipping entries with a blank key. Values are rendered as SQL
// literals via renderUsingValue. Returns "" when no usable bindings exist.
func usingClause(vars []TemplateVar) string {
	var parts []string
	for _, v := range vars {
		key := strings.TrimSpace(v.Key)
		if key == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s => %s", key, renderUsingValue(v.Value)))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("USING (%s)", strings.Join(parts, ", "))
}

// renderUsingValue renders a template-variable value as a SQL literal: integers
// and floats are emitted bare, the keywords TRUE/FALSE/NULL are emitted bare
// (case-normalized), and every other value is a single-quoted string literal.
func renderUsingValue(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return "''"
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return s
	}
	switch strings.ToUpper(s) {
	case "TRUE", "FALSE", "NULL":
		return strings.ToUpper(s)
	}
	return fmt.Sprintf("'%s'", snowflake.EscapeStringLit(s))
}

// BuildCreateServiceSql constructs a CREATE SERVICE statement from the given
// config. The compute pool and a specification are required by Snowflake; when
// they are empty the builder substitutes placeholders so the live preview reads
// as a completable template rather than invalid SQL. Optional clauses are
// emitted only when set, in the order Snowflake documents them.
//
//	CREATE SERVICE [IF NOT EXISTS] <fqn>
//	  IN COMPUTE POOL <pool>
//	  { FROM SPECIFICATION $$ … $$
//	    | FROM @<stage> SPECIFICATION_FILE = '…'
//	    | FROM SPECIFICATION_TEMPLATE $$ … $$ [USING ( k => v, … )]
//	    | FROM @<stage> SPECIFICATION_TEMPLATE_FILE = '…' [USING ( k => v, … )] }
//	  [EXTERNAL_ACCESS_INTEGRATIONS = ( … )]
//	  [AUTO_RESUME = { TRUE | FALSE }]
//	  [MIN_INSTANCES = <num>]
//	  [MAX_INSTANCES = <num>]
//	  [QUERY_WAREHOUSE = <warehouse>]
//	  [COMMENT = '…'];
func BuildCreateServiceSql(db, schema string, cfg ServiceConfig) (string, error) {
	var sb strings.Builder

	createClause := snowflake.CreateClause("SERVICE", false, cfg.IfNotExists)

	name := cfg.Name
	if name == "" {
		name = "service_name"
	}

	fmt.Fprintf(&sb, "%s %s", createClause,
		snowflake.QualifyOrBare(db, schema, name, cfg.CaseSensitive))

	spec, err := specClause(cfg)
	if err != nil {
		return "", err
	}
	sb.WriteString(poolLine(cfg.ComputePool) + "\n  " + spec + eaiClause(cfg.ExternalAccessIntegrations))
	if ar := strings.TrimSpace(cfg.AutoResume); ar != "" {
		fmt.Fprintf(&sb, "\n  AUTO_RESUME = %s", strings.ToUpper(ar))
	}
	if mi := strings.TrimSpace(cfg.MinInstances); mi != "" {
		fmt.Fprintf(&sb, "\n  MIN_INSTANCES = %s", mi)
	}
	if ma := strings.TrimSpace(cfg.MaxInstances); ma != "" {
		fmt.Fprintf(&sb, "\n  MAX_INSTANCES = %s", ma)
	}
	sb.WriteString(warehouseClause(cfg.QueryWarehouse) + snowflake.CommentClause(cfg.Comment))

	return sb.String() + ";", nil
}

// BuildAlterServiceSpecClause renders the clause that redeploys a service from a
// new specification — everything after `ALTER SERVICE <fqn>`, ready for
// App.AlterService. It reuses the spec-source branch of BuildCreateServiceSql
// (inline $$ … $$, staged file, and the TEMPLATE/USING variants) but, unlike the
// CREATE preview, rejects an empty source instead of emitting a placeholder:
//
//	FROM SPECIFICATION $$ … $$
//	FROM @<stage> SPECIFICATION_FILE = '…'
//	FROM SPECIFICATION_TEMPLATE $$ … $$ USING ( k => v, … )
func BuildAlterServiceSpecClause(cfg ServiceConfig) (string, error) {
	if cfg.SpecSource == SpecSourceStage {
		if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cfg.SpecStage), "@")) == "" ||
			strings.TrimSpace(cfg.SpecFile) == "" {
			return "", fmt.Errorf("stage and specification file are required")
		}
	} else {
		if strings.TrimSpace(cfg.SpecInline) == "" {
			return "", fmt.Errorf("specification cannot be empty")
		}
	}
	return specClause(cfg)
}

// Grantee kinds a service role can be granted to.
const (
	GranteeRole            = "ROLE"
	GranteeDatabaseRole    = "DATABASE ROLE"
	GranteeApplicationRole = "APPLICATION ROLE"
)

// ServiceRoleGrant describes a GRANT / REVOKE SERVICE ROLE target: the service
// role (declared in the service spec) and the grantee. Parent is the database
// (for a DATABASE ROLE grantee) or application (for an APPLICATION ROLE
// grantee) and is required for those kinds; it is ignored for account roles.
type ServiceRoleGrant struct {
	Role        string `json:"role"`        // service role name
	GranteeKind string `json:"granteeKind"` // ROLE | DATABASE ROLE | APPLICATION ROLE
	Parent      string `json:"parent"`      // database / application of the grantee
	Grantee     string `json:"grantee"`     // grantee role name
}

// serviceRoleGrantParts validates g and returns the quoted `<svc>!<role>`
// reference, the canonical grantee kind, and the quoted grantee.
func serviceRoleGrantParts(db, schema, svc string, g ServiceRoleGrant) (ref, kind, grantee string, err error) {
	if strings.TrimSpace(g.Role) == "" || strings.TrimSpace(g.Grantee) == "" {
		return "", "", "", fmt.Errorf("service role and grantee are required")
	}
	kind, err = snowflake.ValidateEnumValue("grantee kind", g.GranteeKind,
		GranteeRole, GranteeDatabaseRole, GranteeApplicationRole)
	if err != nil {
		return "", "", "", err
	}
	ref = snowflake.Qualify(db, schema, svc) + "!" + snowflake.QuoteIdent(g.Role)
	switch {
	case kind == GranteeRole:
		grantee = snowflake.QuoteIdent(g.Grantee)
	case strings.TrimSpace(g.Parent) == "":
		return "", "", "", fmt.Errorf("%s grantee requires its database / application name", strings.ToLower(kind))
	default:
		grantee = snowflake.Qualify(g.Parent, g.Grantee)
	}
	return ref, kind, grantee, nil
}

// ParseServiceRoleGrants converts a SHOW GRANTS OF SERVICE ROLE result into
// ServiceRoleGrant rows for role. granted_to (ROLE / DATABASE_ROLE /
// APPLICATION_ROLE) becomes the grantee kind; for database/application roles
// grantee_name ("PARENT.ROLE", parts quoted when needed) is split with the
// quote-aware snowflake.SplitQualifiedName, so a parent like "MY.DB" survives.
// Rows whose grantee name does not parse fall back to the raw name, unqualified.
func ParseServiceRoleGrants(res *snowflake.QueryResult, role string) []ServiceRoleGrant {
	if res == nil {
		return nil
	}
	kindIdx := snowflake.ColIdx(res.Columns, "granted_to")
	nameIdx := snowflake.ColIdx(res.Columns, "grantee_name")
	out := make([]ServiceRoleGrant, 0, len(res.Rows))
	for _, row := range res.Rows {
		g := ServiceRoleGrant{
			Role:        role,
			GranteeKind: strings.ToUpper(strings.ReplaceAll(snowflake.Cell(row, kindIdx), "_", " ")),
			Grantee:     snowflake.Cell(row, nameIdx),
		}
		if g.GranteeKind != GranteeRole {
			if parts, err := snowflake.SplitQualifiedName(g.Grantee, 2); err == nil && len(parts) == 2 {
				g.Parent, g.Grantee = parts[0].Text, parts[1].Text
			}
		}
		out = append(out, g)
	}
	return out
}

// BuildGrantServiceRoleSql emits
// `GRANT SERVICE ROLE "db"."sc"."svc"!"role" TO { ROLE | DATABASE ROLE | APPLICATION ROLE } <grantee>;`.
// Every identifier is double-quoted, so names containing `!` or `.` stay intact.
func BuildGrantServiceRoleSql(db, schema, svc string, g ServiceRoleGrant) (string, error) {
	ref, kind, grantee, err := serviceRoleGrantParts(db, schema, svc, g)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("GRANT SERVICE ROLE %s TO %s %s;", ref, kind, grantee), nil
}

// BuildRevokeServiceRoleSql is the REVOKE … FROM counterpart of
// BuildGrantServiceRoleSql.
func BuildRevokeServiceRoleSql(db, schema, svc string, g ServiceRoleGrant) (string, error) {
	ref, kind, grantee, err := serviceRoleGrantParts(db, schema, svc, g)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("REVOKE SERVICE ROLE %s FROM %s %s;", ref, kind, grantee), nil
}

// poolLine, eaiClause and warehouseClause render clauses shared by CREATE
// SERVICE and EXECUTE [INFERENCE] JOB SERVICE (each emits them in its own
// documented order). A blank pool becomes a placeholder; blank options are "".
func poolLine(pool string) string {
	if p := strings.TrimSpace(pool); p != "" {
		return "\n  IN COMPUTE POOL " + snowflake.QuoteIdent(p)
	}
	return "\n  IN COMPUTE POOL <compute_pool>"
}

func eaiClause(eai string) string {
	if list := snowflake.SplitIdentList(eai, true); len(list) > 0 {
		return fmt.Sprintf("\n  EXTERNAL_ACCESS_INTEGRATIONS = (%s)", strings.Join(list, ", "))
	}
	return ""
}

func warehouseClause(wh string) string {
	if w := strings.TrimSpace(wh); w != "" {
		return "\n  QUERY_WAREHOUSE = " + snowflake.QuoteIdent(w)
	}
	return ""
}

// jobOptions renders the NAME / ASYNC / REPLICAS options shared by both
// EXECUTE … JOB SERVICE variants.
func jobOptions(name string, async bool, replicas string) (string, error) {
	var sb strings.Builder
	if strings.TrimSpace(name) != "" {
		ref, err := snowflake.RenderQualifiedName("job name", name, 3)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&sb, "\n  NAME = %s", ref)
	}
	if async {
		sb.WriteString("\n  ASYNC = TRUE")
	}
	if r := strings.TrimSpace(replicas); r != "" {
		if n, err := strconv.Atoi(r); err != nil || n < 1 {
			return "", fmt.Errorf("replicas must be a positive integer")
		}
		fmt.Fprintf(&sb, "\n  REPLICAS = %s", r)
	}
	return sb.String(), nil
}

// JobServiceConfig holds the parameters for EXECUTE JOB SERVICE. The spec
// fields mirror ServiceConfig's (same JSON names), so the frontend spec form is
// shared with CREATE SERVICE. Name is optional ([db.schema.]name; Snowflake
// generates JOB_<uuid> when blank).
type JobServiceConfig struct {
	Name                       string        `json:"name"`
	ComputePool                string        `json:"computePool"`
	SpecSource                 string        `json:"specSource"`
	Template                   bool          `json:"template"`
	SpecInline                 string        `json:"specInline"`
	SpecStage                  string        `json:"specStage"`
	SpecFile                   string        `json:"specFile"`
	TemplateVars               []TemplateVar `json:"templateVars"`
	Async                      bool          `json:"async"`
	Replicas                   string        `json:"replicas"` // integer string or ""
	QueryWarehouse             string        `json:"queryWarehouse"`
	ExternalAccessIntegrations string        `json:"externalAccessIntegrations"`
	Comment                    string        `json:"comment"`
}

// BuildExecuteJobServiceSql constructs an EXECUTE JOB SERVICE statement. Like
// BuildCreateServiceSql it emits placeholders for a missing pool / spec so the
// preview stays a completable template. The options come before the spec: the
// docs' usage notes ("compute pool, followed by other properties, and finally
// the service specification") and every example use that order, even though
// the syntax block lists the spec first.
//
//	EXECUTE JOB SERVICE
//	  IN COMPUTE POOL <pool>
//	  [NAME = [<db>.<schema>.]<name>] [ASYNC = TRUE] [REPLICAS = <n>]
//	  [QUERY_WAREHOUSE = <wh>] [COMMENT = '…'] [EXTERNAL_ACCESS_INTEGRATIONS = ( … )]
//	  <spec clause, as CREATE SERVICE>;
func BuildExecuteJobServiceSql(cfg JobServiceConfig) (string, error) {
	opts, err := jobOptions(cfg.Name, cfg.Async, cfg.Replicas)
	if err != nil {
		return "", err
	}
	spec, err := specClause(ServiceConfig{
		SpecSource: cfg.SpecSource, Template: cfg.Template, SpecInline: cfg.SpecInline,
		SpecStage: cfg.SpecStage, SpecFile: cfg.SpecFile, TemplateVars: cfg.TemplateVars,
	})
	if err != nil {
		return "", err
	}
	return "EXECUTE JOB SERVICE" + poolLine(cfg.ComputePool) + opts +
		warehouseClause(cfg.QueryWarehouse) + snowflake.CommentClause(cfg.Comment) +
		eaiClause(cfg.ExternalAccessIntegrations) + "\n  " + spec + ";", nil
}

// Input source modes for InferenceJobConfig.InputSource.
const (
	InputSourceQuery = "query" // FROM ( <subquery> )
	InputSourceStage = "stage" // FROM @stage[/path]
)

// InferenceJobConfig holds the parameters for EXECUTE INFERENCE JOB SERVICE.
// Model is [db.schema.]model (a quoted FQN from ListModels round-trips).
type InferenceJobConfig struct {
	Name        string `json:"name"`
	ComputePool string `json:"computePool"`
	Spec        string `json:"spec"`        // inline YAML, dollar-quoted
	InputSource string `json:"inputSource"` // "query" | "stage"
	Query       string `json:"query"`
	StagePath   string `json:"stagePath"` // @stage[/path], leading @ optional
	Model       string `json:"model"`
	Version     string `json:"version"`
	Function    string `json:"function"`
	Async       bool   `json:"async"`
	Replicas    string `json:"replicas"`
}

// BuildExecuteInferenceJobServiceSql constructs an EXECUTE INFERENCE JOB SERVICE
// statement. The subquery is wrapped in parentheses and the function name is a
// quoted string literal; missing required parts become placeholders.
//
//	EXECUTE INFERENCE JOB SERVICE
//	  IN COMPUTE POOL <pool>
//	  WITH SPECIFICATION $$ … $$
//	  FROM { ( <subquery> ) | @stage[/path] }
//	  MODEL = [<db>.<schema>.]<model> [VERSION = <v>] [FUNCTION = '<fn>']
//	  [NAME = …] [ASYNC = TRUE] [REPLICAS = <n>];
func BuildExecuteInferenceJobServiceSql(cfg InferenceJobConfig) (string, error) {
	var sb strings.Builder
	sb.WriteString("EXECUTE INFERENCE JOB SERVICE" + poolLine(cfg.ComputePool))

	spec := strings.TrimSpace(cfg.Spec)
	if err := checkDollarSpec(spec); err != nil {
		return "", err
	}
	if spec == "" {
		spec = "output:\n  stage_location: \"@db.schema.stage/results/\""
	}
	fmt.Fprintf(&sb, "\n  WITH SPECIFICATION $$\n%s\n$$", spec)

	if cfg.InputSource == InputSourceStage {
		p := strings.TrimPrefix(strings.TrimSpace(cfg.StagePath), "@")
		if p == "" {
			p = "<stage>/<path>"
		}
		fmt.Fprintf(&sb, "\n  FROM @%s", p)
	} else {
		q := strings.TrimRight(strings.TrimSpace(cfg.Query), "; \n\t")
		if q == "" {
			q = "SELECT * FROM <table>"
		}
		// ")" on its own line: a trailing "-- comment" in q would swallow it.
		fmt.Fprintf(&sb, "\n  FROM (\n%s\n  )", q)
	}

	if strings.TrimSpace(cfg.Model) == "" {
		sb.WriteString("\n  MODEL = <model>")
	} else {
		ref, err := snowflake.RenderQualifiedName("model", cfg.Model, 3)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&sb, "\n  MODEL = %s", ref)
	}
	if v := strings.TrimSpace(cfg.Version); v != "" {
		fmt.Fprintf(&sb, "\n  VERSION = %s", snowflake.QuoteOrBare(v, false))
	}
	if f := strings.TrimSpace(cfg.Function); f != "" {
		fmt.Fprintf(&sb, "\n  FUNCTION = %s", snowflake.QuoteTextLit(f))
	}

	opts, err := jobOptions(cfg.Name, cfg.Async, cfg.Replicas)
	if err != nil {
		return "", err
	}
	return sb.String() + opts + ";", nil
}
