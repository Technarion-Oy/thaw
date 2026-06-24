package sqlgrammar

import "testing"

// Tests for the SHOW-family grammar rules implemented in batch 4 (the final
// ~40 ParseShow* functions). Each uses the shared parseRule/assertValid/
// assertInvalid helpers from grammar_test.go.

func TestParseShowArtifactRepositories(t *testing.T) {
	assertValid(t, (*Validator).ParseShowArtifactRepositories,
		`SHOW ARTIFACT REPOSITORIES`,
		`SHOW ARTIFACT REPOSITORIES LIKE 'r%'`,
		`SHOW ARTIFACT REPOSITORIES IN SCHEMA my_schema LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowArtifactRepositories,
		``,
		`SHOW REPOSITORIES`,
		`ARTIFACT REPOSITORIES`,
	)
}

func TestParseShowCortexBaseModels(t *testing.T) {
	assertValid(t, (*Validator).ParseShowCortexBaseModels,
		`SHOW CORTEX BASE MODELS IN SNOWFLAKE.MODELS`,
		`SHOW CORTEX BASE MODELS LIKE 'gpt%' IN SCHEMA SNOWFLAKE.MODELS`,
		`SHOW CORTEX BASE MODELS IN SCHEMA SNOWFLAKE.MODELS`,
	)
	assertInvalid(t, (*Validator).ParseShowCortexBaseModels,
		``,
		`SHOW CORTEX BASE MODELS`,
		`SHOW CORTEX MODELS IN SNOWFLAKE.MODELS`,
	)
}

func TestParseShowEventRoutingTableOnOrganization(t *testing.T) {
	assertValid(t, (*Validator).ParseShowEventRoutingTableOnOrganization,
		`SHOW EVENT ROUTING TABLE ON ORGANIZATION FOR ALL APPLICATION LISTINGS`,
		`show event routing table on organization for all application listings`,
	)
	assertInvalid(t, (*Validator).ParseShowEventRoutingTableOnOrganization,
		``,
		`SHOW EVENT ROUTING TABLE ON ORGANIZATION`,
		`SHOW EVENT ROUTING TABLE FOR ALL APPLICATION LISTINGS`,
	)
}

func TestParseShowEventRoutingTables(t *testing.T) {
	assertValid(t, (*Validator).ParseShowEventRoutingTables,
		`SHOW EVENT ROUTING TABLES`,
		`SHOW EVENT ROUTING TABLES LIKE 't%'`,
		`SHOW EVENT ROUTING TABLES IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowEventRoutingTables,
		``,
		`SHOW ROUTING TABLES`,
	)
}

func TestParseShowInteractiveTables(t *testing.T) {
	assertValid(t, (*Validator).ParseShowInteractiveTables,
		`SHOW INTERACTIVE TABLES`,
		`SHOW INTERACTIVE TABLES LIKE 'x%' IN DATABASE db1`,
		`SHOW INTERACTIVE TABLES STARTS WITH 'abc' LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowInteractiveTables,
		``,
		`SHOW TABLES`,
	)
}

func TestParseShowObjectsOwnedByApplication(t *testing.T) {
	assertValid(t, (*Validator).ParseShowObjectsOwnedByApplication,
		`SHOW OBJECTS OWNED BY APPLICATION my_app`,
		`SHOW OBJECTS OWNED BY APPLICATION db.app`,
		`show objects owned by application "My App"`,
	)
	assertInvalid(t, (*Validator).ParseShowObjectsOwnedByApplication,
		``,
		`SHOW OBJECTS OWNED BY APPLICATION`,
		`SHOW OBJECTS OWNED BY my_app`,
	)
}

func TestParseShowOffers(t *testing.T) {
	assertValid(t, (*Validator).ParseShowOffers,
		`SHOW OFFERS IN LISTING my_listing`,
		`SHOW OFFERS LIKE 'o%' IN LISTING my_listing`,
		`show offers in listing l1`,
	)
	assertInvalid(t, (*Validator).ParseShowOffers,
		``,
		`SHOW OFFERS`,
		`SHOW OFFERS IN LISTING`,
	)
}

func TestParseShowOpenflowDataPlaneIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseShowOpenflowDataPlaneIntegration,
		`SHOW OPENFLOW DATA PLANE INTEGRATIONS`,
		`SHOW OPENFLOW DATA PLANE INTEGRATIONS LIKE 'i%'`,
		`SHOW OPENFLOW DATA PLANE INTEGRATIONS STARTS WITH 'abc' LIMIT 5 FROM 'x'`,
	)
	assertInvalid(t, (*Validator).ParseShowOpenflowDataPlaneIntegration,
		``,
		`SHOW DATA PLANE INTEGRATIONS`,
	)
}

func TestParseShowPricingPlans(t *testing.T) {
	assertValid(t, (*Validator).ParseShowPricingPlans,
		`SHOW PRICING PLANS IN LISTING my_listing`,
		`SHOW PRICING PLANS LIKE 'p%' IN LISTING l1`,
		`show pricing plans in listing l2`,
	)
	assertInvalid(t, (*Validator).ParseShowPricingPlans,
		``,
		`SHOW PRICING PLANS`,
		`SHOW PLANS IN LISTING l1`,
	)
}

func TestParseShowPrivacyPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowPrivacyPolicies,
		`SHOW PRIVACY POLICIES`,
		`SHOW PRIVACY POLICIES LIKE 'p%'`,
		`SHOW PRIVACY POLICIES IN DATABASE db1`,
	)
	assertInvalid(t, (*Validator).ParseShowPrivacyPolicies,
		``,
		`SHOW POLICIES`,
	)
}

func TestParseShowReferences(t *testing.T) {
	assertValid(t, (*Validator).ParseShowReferences,
		`SHOW REFERENCES IN APPLICATION my_app`,
		`SHOW REFERENCES IN APPLICATION db.app`,
		`show references in application a1`,
	)
	assertInvalid(t, (*Validator).ParseShowReferences,
		``,
		`SHOW REFERENCES`,
		`SHOW REFERENCES IN APPLICATION`,
	)
}

func TestParseShowReleaseChannels(t *testing.T) {
	assertValid(t, (*Validator).ParseShowReleaseChannels,
		`SHOW RELEASE CHANNELS IN APPLICATION PACKAGE my_pkg`,
		`SHOW RELEASE CHANNELS IN LISTING my_listing`,
		`show release channels in application package p1`,
	)
	assertInvalid(t, (*Validator).ParseShowReleaseChannels,
		``,
		`SHOW RELEASE CHANNELS`,
		`SHOW RELEASE CHANNELS IN APPLICATION PACKAGE`,
	)
}

func TestParseShowReleaseDirectives(t *testing.T) {
	assertValid(t, (*Validator).ParseShowReleaseDirectives,
		`SHOW RELEASE DIRECTIVES IN APPLICATION PACKAGE my_pkg`,
		`SHOW RELEASE DIRECTIVES LIKE 'd%' IN APPLICATION PACKAGE p1`,
		`SHOW RELEASE DIRECTIVES IN APPLICATION PACKAGE p1 FOR RELEASE CHANNEL ch1`,
	)
	assertInvalid(t, (*Validator).ParseShowReleaseDirectives,
		``,
		`SHOW RELEASE DIRECTIVES`,
		`SHOW RELEASE DIRECTIVES IN APPLICATION PACKAGE`,
	)
}

func TestParseShowRolesInService(t *testing.T) {
	assertValid(t, (*Validator).ParseShowRolesInService,
		`SHOW ROLES IN SERVICE my_service`,
		`SHOW ROLES IN SERVICE db.schema.svc`,
		`show roles in service s1`,
	)
	assertInvalid(t, (*Validator).ParseShowRolesInService,
		``,
		`SHOW ROLES IN SERVICE`,
		`SHOW ROLES my_service`,
	)
}

func TestParseShowRulesInEventRoutingTable(t *testing.T) {
	assertValid(t, (*Validator).ParseShowRulesInEventRoutingTable,
		`SHOW RULES IN EVENT ROUTING TABLE (my_table)`,
		`SHOW RULES IN EVENT ROUTING TABLE (db.schema.t1)`,
		`show rules in event routing table (t1)`,
	)
	assertInvalid(t, (*Validator).ParseShowRulesInEventRoutingTable,
		``,
		`SHOW RULES IN EVENT ROUTING TABLE my_table`,
		`SHOW RULES IN EVENT ROUTING (my_table)`,
	)
}

func TestParseShowRunInExperiment(t *testing.T) {
	assertValid(t, (*Validator).ParseShowRunInExperiment,
		`SHOW RUN METRICS IN EXPERIMENT exp1`,
		`SHOW RUN PARAMETERS LIKE 'p%' IN EXPERIMENT exp1 RUN run1`,
		`SHOW RUN METRICS IN EXPERIMENT exp1 RUN run1 LIMIT 5 FROM 'x'`,
	)
	assertInvalid(t, (*Validator).ParseShowRunInExperiment,
		``,
		`SHOW RUN METRICS`,
		`SHOW RUN METRICS IN EXPERIMENT`,
	)
}

func TestParseShowRunsInExperiment(t *testing.T) {
	assertValid(t, (*Validator).ParseShowRunsInExperiment,
		`SHOW RUNS IN EXPERIMENT exp1`,
		`SHOW RUNS LIKE 'r%' IN EXPERIMENT exp1`,
		`show runs in experiment e1`,
	)
	assertInvalid(t, (*Validator).ParseShowRunsInExperiment,
		``,
		`SHOW RUNS`,
		`SHOW RUNS IN EXPERIMENT`,
	)
}

func TestParseShowSemanticDimensions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSemanticDimensions,
		`SHOW SEMANTIC DIMENSIONS`,
		`SHOW SEMANTIC DIMENSIONS LIKE 'd%' IN ACCOUNT`,
		`SHOW SEMANTIC DIMENSIONS IN my_view STARTS WITH 'a' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowSemanticDimensions,
		``,
		`SHOW DIMENSIONS`,
	)
}

func TestParseShowSemanticDimensionsForMetric(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSemanticDimensionsForMetric,
		`SHOW SEMANTIC DIMENSIONS IN my_view FOR METRIC m1`,
		`SHOW SEMANTIC DIMENSIONS LIKE 'd%' IN my_view FOR METRIC m1`,
		`SHOW SEMANTIC DIMENSIONS IN my_view FOR METRIC m1 STARTS WITH 'a' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowSemanticDimensionsForMetric,
		``,
		`SHOW SEMANTIC DIMENSIONS IN my_view`,
		`SHOW SEMANTIC DIMENSIONS FOR METRIC m1`,
	)
}

func TestParseShowSemanticFacts(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSemanticFacts,
		`SHOW SEMANTIC FACTS`,
		`SHOW SEMANTIC FACTS LIKE 'f%' IN DATABASE db1`,
		`SHOW SEMANTIC FACTS IN my_view STARTS WITH 'a' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowSemanticFacts,
		``,
		`SHOW FACTS`,
	)
}

func TestParseShowSemanticMetrics(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSemanticMetrics,
		`SHOW SEMANTIC METRICS`,
		`SHOW SEMANTIC METRICS LIKE 'm%' IN SCHEMA db1.s1`,
		`SHOW SEMANTIC METRICS IN my_view LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowSemanticMetrics,
		``,
		`SHOW METRICS`,
	)
}

func TestParseShowServiceContainersInService(t *testing.T) {
	assertValid(t, (*Validator).ParseShowServiceContainersInService,
		`SHOW SERVICE CONTAINERS IN SERVICE my_svc`,
		`SHOW SERVICE CONTAINERS IN SERVICE db.schema.svc`,
		`show service containers in service s1`,
	)
	assertInvalid(t, (*Validator).ParseShowServiceContainersInService,
		``,
		`SHOW SERVICE CONTAINERS IN SERVICE`,
		`SHOW SERVICE CONTAINERS my_svc`,
	)
}

func TestParseShowServiceInstancesInService(t *testing.T) {
	assertValid(t, (*Validator).ParseShowServiceInstancesInService,
		`SHOW SERVICE INSTANCES IN SERVICE my_svc`,
		`SHOW SERVICE INSTANCES IN SERVICE db.schema.svc`,
		`show service instances in service s1`,
	)
	assertInvalid(t, (*Validator).ParseShowServiceInstancesInService,
		``,
		`SHOW SERVICE INSTANCES IN SERVICE`,
		`SHOW SERVICE INSTANCES my_svc`,
	)
}

func TestParseShowServiceVolumesInService(t *testing.T) {
	assertValid(t, (*Validator).ParseShowServiceVolumesInService,
		`SHOW SERVICE VOLUMES IN SERVICE my_svc`,
		`SHOW SERVICE VOLUMES IN SERVICE db.schema.svc`,
		`show service volumes in service s1`,
	)
	assertInvalid(t, (*Validator).ParseShowServiceVolumesInService,
		``,
		`SHOW SERVICE VOLUMES IN SERVICE`,
		`SHOW SERVICE VOLUMES my_svc`,
	)
}

func TestParseShowSharedContent(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSharedContent,
		`SHOW SHARED CONTENT IN APPLICATION PACKAGE pkg1 FOR VERSION v1`,
		`SHOW SHARED CONTENT IN APPLICATION PACKAGE db.pkg FOR VERSION v2`,
		`show shared content in application package p1 for version v1`,
	)
	assertInvalid(t, (*Validator).ParseShowSharedContent,
		``,
		`SHOW SHARED CONTENT IN APPLICATION PACKAGE pkg1`,
		`SHOW SHARED CONTENT FOR VERSION v1`,
	)
}

func TestParseShowSharesInFailoverGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSharesInFailoverGroup,
		`SHOW SHARES IN FAILOVER GROUP fg1`,
		`SHOW SHARES IN FAILOVER GROUP my_db.fg`,
		`show shares in failover group g1`,
	)
	assertInvalid(t, (*Validator).ParseShowSharesInFailoverGroup,
		``,
		`SHOW SHARES IN FAILOVER GROUP`,
		`SHOW SHARES IN GROUP fg1`,
	)
}

func TestParseShowSharesInReplicationGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSharesInReplicationGroup,
		`SHOW SHARES IN REPLICATION GROUP rg1`,
		`SHOW SHARES IN REPLICATION GROUP my_db.rg`,
		`show shares in replication group g1`,
	)
	assertInvalid(t, (*Validator).ParseShowSharesInReplicationGroup,
		``,
		`SHOW SHARES IN REPLICATION GROUP`,
		`SHOW SHARES IN GROUP rg1`,
	)
}

func TestParseShowSnapshotsInSnapshotSet(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSnapshotsInSnapshotSet,
		`SHOW SNAPSHOTS IN SNAPSHOT SET ss1`,
		`SHOW SNAPSHOTS IN SNAPSHOT SET db.schema.ss`,
		`show snapshots in snapshot set s1`,
	)
	assertInvalid(t, (*Validator).ParseShowSnapshotsInSnapshotSet,
		``,
		`SHOW SNAPSHOTS IN SNAPSHOT SET`,
		`SHOW SNAPSHOTS IN SET ss1`,
	)
}

func TestParseShowSpecifications(t *testing.T) {
	assertValid(t, (*Validator).ParseShowSpecifications,
		`SHOW SPECIFICATIONS`,
		`SHOW APPROVED SPECIFICATIONS`,
		`SHOW PENDING SPECIFICATIONS IN APPLICATION app1`,
	)
	assertInvalid(t, (*Validator).ParseShowSpecifications,
		``,
		`SHOW SPECS`,
	)
}

func TestParseShowTelemetryEventDefinitions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowTelemetryEventDefinitions,
		`SHOW TELEMETRY EVENT DEFINITIONS IN APPLICATION app1`,
		`SHOW TELEMETRY EVENT DEFINITIONS IN APPLICATION db.app`,
		`show telemetry event definitions in application a1`,
	)
	assertInvalid(t, (*Validator).ParseShowTelemetryEventDefinitions,
		``,
		`SHOW TELEMETRY EVENT DEFINITIONS`,
		`SHOW TELEMETRY EVENT DEFINITIONS IN APPLICATION`,
	)
}

func TestParseShowUserProcedures(t *testing.T) {
	assertValid(t, (*Validator).ParseShowUserProcedures,
		`SHOW USER PROCEDURES`,
		`SHOW USER PROCEDURES LIKE 'p%'`,
		`SHOW USER PROCEDURES IN DATABASE db1`,
	)
	assertInvalid(t, (*Validator).ParseShowUserProcedures,
		``,
		`SHOW PROCEDURES`,
	)
}

func TestParseShowUserProgrammaticAccessTokens(t *testing.T) {
	assertValid(t, (*Validator).ParseShowUserProgrammaticAccessTokens,
		`SHOW USER PROGRAMMATIC ACCESS TOKENS`,
		`SHOW USER PATS FOR USER alice`,
		`SHOW USER PROGRAMMATIC ACCESS TOKENS FOR USER bob`,
	)
	assertInvalid(t, (*Validator).ParseShowUserProgrammaticAccessTokens,
		``,
		`SHOW USER TOKENS`,
		`SHOW PROGRAMMATIC ACCESS TOKENS`,
	)
}

func TestParseShowUserWorkloadIdentityAuthenticationMethods(t *testing.T) {
	assertValid(t, (*Validator).ParseShowUserWorkloadIdentityAuthenticationMethods,
		`SHOW USER WORKLOAD IDENTITY AUTHENTICATION METHODS`,
		`SHOW USER WORKLOAD IDENTITY AUTHENTICATION METHODS FOR USER alice`,
		`show user workload identity authentication methods for user bob`,
	)
	assertInvalid(t, (*Validator).ParseShowUserWorkloadIdentityAuthenticationMethods,
		``,
		`SHOW USER WORKLOAD IDENTITY METHODS`,
		`SHOW WORKLOAD IDENTITY AUTHENTICATION METHODS`,
	)
}

func TestParseShowVersions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowVersions,
		`SHOW VERSIONS IN APPLICATION PACKAGE pkg1`,
		`SHOW VERSIONS LIKE 'v%' IN APPLICATION PACKAGE pkg1`,
		`show versions in application package db.pkg`,
	)
	assertInvalid(t, (*Validator).ParseShowVersions,
		``,
		`SHOW VERSIONS`,
		`SHOW VERSIONS IN APPLICATION PACKAGE`,
	)
}

func TestParseShowVersionsInDataset(t *testing.T) {
	assertValid(t, (*Validator).ParseShowVersionsInDataset,
		`SHOW VERSIONS IN DATASET ds1`,
		`SHOW VERSIONS LIKE 'v%' IN DATASET ds1`,
		`SHOW VERSIONS IN DATASET db.schema.ds LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowVersionsInDataset,
		``,
		`SHOW VERSIONS IN DATASET`,
		`SHOW VERSIONS ds1`,
	)
}

func TestParseShowVersionsInDbtProject(t *testing.T) {
	assertValid(t, (*Validator).ParseShowVersionsInDbtProject,
		`SHOW VERSIONS IN DBT PROJECT proj1`,
		`SHOW VERSIONS IN DBT PROJECT db.schema.proj`,
		`SHOW VERSIONS IN DBT PROJECT proj1 LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowVersionsInDbtProject,
		``,
		`SHOW VERSIONS IN DBT PROJECT`,
		`SHOW VERSIONS IN PROJECT proj1`,
	)
}

func TestParseShowVersionsInListing(t *testing.T) {
	assertValid(t, (*Validator).ParseShowVersionsInListing,
		`SHOW VERSIONS IN LISTING l1`,
		`SHOW VERSIONS IN LISTING db.l1 LIMIT 5`,
		`show versions in listing l2`,
	)
	assertInvalid(t, (*Validator).ParseShowVersionsInListing,
		``,
		`SHOW VERSIONS IN LISTING`,
		`SHOW VERSIONS l1`,
	)
}

func TestParseShowVersionsInModel(t *testing.T) {
	assertValid(t, (*Validator).ParseShowVersionsInModel,
		`SHOW VERSIONS IN MODEL m1`,
		`SHOW VERSIONS LIKE 'v%' IN MODEL m1`,
		`SHOW VERSIONS IN MODEL db.schema.m1`,
	)
	assertInvalid(t, (*Validator).ParseShowVersionsInModel,
		``,
		`SHOW VERSIONS IN MODEL`,
		`SHOW VERSIONS m1`,
	)
}

func TestParseShowVersionsInOrganizationProfile(t *testing.T) {
	assertValid(t, (*Validator).ParseShowVersionsInOrganizationProfile,
		`SHOW VERSIONS IN ORGANIZATION PROFILE op1`,
		`SHOW VERSIONS IN ORGANIZATION PROFILE db.op`,
		`show versions in organization profile p1`,
	)
	assertInvalid(t, (*Validator).ParseShowVersionsInOrganizationProfile,
		``,
		`SHOW VERSIONS IN ORGANIZATION PROFILE`,
		`SHOW VERSIONS IN PROFILE op1`,
	)
}

func TestParseShowWorkspaces(t *testing.T) {
	assertValid(t, (*Validator).ParseShowWorkspaces,
		`SHOW WORKSPACES`,
		`SHOW WORKSPACES LIKE 'w%'`,
		`SHOW WORKSPACES IN DATABASE db1`,
	)
	assertInvalid(t, (*Validator).ParseShowWorkspaces,
		``,
		`SHOW SPACES`,
	)
}
