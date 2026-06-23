package sqlgrammar

import "testing"

func TestParseShowFailoverGroups(t *testing.T) {
	assertValid(t, (*Validator).ParseShowFailoverGroups,
		`SHOW FAILOVER GROUPS`,
		`SHOW FAILOVER GROUPS IN ACCOUNT`,
		`SHOW FAILOVER GROUPS IN ACCOUNT myacct`,
	)
	assertInvalid(t, (*Validator).ParseShowFailoverGroups,
		``,
		`SHOW GROUPS`,
		`FAILOVER GROUPS`,
	)
}

func TestParseShowFeaturePolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowFeaturePolicies,
		`SHOW FEATURE POLICIES`,
		`SHOW FEATURE POLICIES IN ACCOUNT`,
		`SHOW FEATURE POLICIES ON APPLICATION my_app`,
		`SHOW FEATURE POLICIES IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowFeaturePolicies,
		``,
		`SHOW POLICIES`,
		`SHOW FEATURE`,
	)
}

func TestParseShowFileFormats(t *testing.T) {
	assertValid(t, (*Validator).ParseShowFileFormats,
		`SHOW FILE FORMATS`,
		`SHOW FILE FORMATS LIKE 'csv%'`,
		`SHOW FILE FORMATS IN SCHEMA my_db.my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowFileFormats,
		``,
		`SHOW FORMATS`,
	)
}

func TestParseShowFunctions(t *testing.T) {
	assertValid(t, (*Validator).ParseShowFunctions,
		`SHOW FUNCTIONS`,
		`SHOW FUNCTIONS LIKE 'my%'`,
		`SHOW FUNCTIONS IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowFunctions,
		``,
		`SHOW FUNCTION`,
	)
}

func TestParseShowFunctionsInModel(t *testing.T) {
	assertValid(t, (*Validator).ParseShowFunctionsInModel,
		`SHOW FUNCTIONS IN MODEL my_model`,
		`SHOW FUNCTIONS LIKE 'f%' IN MODEL db.sch.my_model`,
		`SHOW FUNCTIONS IN MODEL my_model VERSION v1`,
	)
	assertInvalid(t, (*Validator).ParseShowFunctionsInModel,
		``,
		`SHOW FUNCTIONS IN MODEL`,
		`SHOW FUNCTIONS`,
	)
}

func TestParseShowGateways(t *testing.T) {
	assertValid(t, (*Validator).ParseShowGateways,
		`SHOW GATEWAYS`,
		`SHOW GATEWAYS LIKE 'g%'`,
		`SHOW GATEWAYS IN DATABASE my_db LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowGateways,
		``,
		`SHOW GATEWAY`,
	)
}

func TestParseShowGitBranches(t *testing.T) {
	assertValid(t, (*Validator).ParseShowGitBranches,
		`SHOW GIT BRANCHES IN my_repo`,
		`SHOW GIT BRANCHES IN GIT REPOSITORY my_repo`,
		`SHOW GIT BRANCHES LIKE 'main%' IN my_repo`,
	)
	assertInvalid(t, (*Validator).ParseShowGitBranches,
		``,
		`SHOW GIT BRANCHES`,
		`SHOW BRANCHES IN my_repo`,
	)
}

func TestParseShowGitRepositories(t *testing.T) {
	assertValid(t, (*Validator).ParseShowGitRepositories,
		`SHOW GIT REPOSITORIES`,
		`SHOW GIT REPOSITORIES LIKE 'r%'`,
		`SHOW GIT REPOSITORIES IN SCHEMA my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowGitRepositories,
		``,
		`SHOW REPOSITORIES`,
	)
}

func TestParseShowGitTags(t *testing.T) {
	assertValid(t, (*Validator).ParseShowGitTags,
		`SHOW GIT TAGS IN my_repo`,
		`SHOW GIT TAGS IN GIT REPOSITORY my_repo`,
		`SHOW GIT TAGS LIKE 'v%' IN my_repo`,
	)
	assertInvalid(t, (*Validator).ParseShowGitTags,
		``,
		`SHOW GIT TAGS`,
		`SHOW TAGS IN my_repo`,
	)
}

func TestParseShowGlobalAccounts(t *testing.T) {
	assertValid(t, (*Validator).ParseShowGlobalAccounts,
		`SHOW GLOBAL ACCOUNTS`,
		`SHOW GLOBAL ACCOUNTS LIKE 'acc%'`,
		`SHOW GLOBAL ACCOUNTS LIKE 'x'`,
	)
	assertInvalid(t, (*Validator).ParseShowGlobalAccounts,
		``,
		`SHOW ACCOUNTS`,
	)
}

func TestParseShowGrants(t *testing.T) {
	assertValid(t, (*Validator).ParseShowGrants,
		`SHOW GRANTS`,
		`SHOW GRANTS ON ACCOUNT`,
		`SHOW GRANTS TO ROLE my_role`,
		`SHOW FUTURE GRANTS IN SCHEMA my_schema`,
		`SHOW GRANTS LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowGrants,
		``,
		`GRANTS ON ACCOUNT`,
	)
}

func TestParseShowGrantsInDcmProject(t *testing.T) {
	assertValid(t, (*Validator).ParseShowGrantsInDcmProject,
		`SHOW GRANTS IN DCM PROJECT my_proj`,
		`SHOW FUTURE GRANTS IN DCM PROJECT my_proj`,
		`SHOW GRANTS IN DCM PROJECT my_proj LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowGrantsInDcmProject,
		``,
		`SHOW GRANTS IN DCM PROJECT`,
		`SHOW GRANTS IN PROJECT my_proj`,
	)
}

func TestParseShowHybridTables(t *testing.T) {
	assertValid(t, (*Validator).ParseShowHybridTables,
		`SHOW HYBRID TABLES`,
		`SHOW TERSE HYBRID TABLES LIKE 't%'`,
		`SHOW HYBRID TABLES IN SCHEMA my_schema STARTS WITH 'a'`,
	)
	assertInvalid(t, (*Validator).ParseShowHybridTables,
		``,
		`SHOW HYBRID`,
	)
}

func TestParseShowIcebergTables(t *testing.T) {
	assertValid(t, (*Validator).ParseShowIcebergTables,
		`SHOW ICEBERG TABLES`,
		`SHOW TERSE ICEBERG TABLES LIKE 't%'`,
		`SHOW ICEBERG TABLES IN DATABASE my_db LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowIcebergTables,
		``,
		`SHOW ICEBERG`,
	)
}

func TestParseShowImageRepositories(t *testing.T) {
	assertValid(t, (*Validator).ParseShowImageRepositories,
		`SHOW IMAGE REPOSITORIES`,
		`SHOW IMAGE REPOSITORIES LIKE 'r%'`,
		`SHOW IMAGE REPOSITORIES IN SCHEMA my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowImageRepositories,
		``,
		`SHOW REPOSITORIES`,
	)
}

func TestParseShowImagesInImageRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseShowImagesInImageRepository,
		`SHOW IMAGES IN IMAGE REPOSITORY my_repo`,
		`SHOW IMAGES IN IMAGE REPOSITORY db.sch.my_repo`,
		`SHOW IMAGES IN IMAGE REPOSITORY "MyRepo"`,
	)
	assertInvalid(t, (*Validator).ParseShowImagesInImageRepository,
		``,
		`SHOW IMAGES IN IMAGE REPOSITORY`,
		`SHOW IMAGES`,
	)
}

func TestParseShowIndexes(t *testing.T) {
	assertValid(t, (*Validator).ParseShowIndexes,
		`SHOW INDEXES`,
		`SHOW TERSE INDEXES LIKE 'i%'`,
		`SHOW INDEXES IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowIndexes,
		``,
		`SHOW INDEX`,
	)
}

func TestParseShowIntegrations(t *testing.T) {
	assertValid(t, (*Validator).ParseShowIntegrations,
		`SHOW INTEGRATIONS`,
		`SHOW API INTEGRATIONS LIKE 'i%'`,
		`SHOW EXTERNAL ACCESS INTEGRATIONS`,
		`SHOW STORAGE INTEGRATIONS`,
	)
	assertInvalid(t, (*Validator).ParseShowIntegrations,
		``,
		`SHOW INTEGRATION`,
	)
}

func TestParseShowJoinPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowJoinPolicies,
		`SHOW JOIN POLICIES`,
		`SHOW JOIN POLICIES LIKE 'p%'`,
		`SHOW JOIN POLICIES IN SCHEMA my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowJoinPolicies,
		``,
		`SHOW POLICIES`,
	)
}

func TestParseShowListings(t *testing.T) {
	assertValid(t, (*Validator).ParseShowListings,
		`SHOW LISTINGS`,
		`SHOW LISTINGS LIKE 'l%'`,
		`SHOW LISTINGS STARTS WITH 'a' LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowListings,
		``,
		`SHOW LISTING`,
	)
}

func TestParseShowListingsInFailoverGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseShowListingsInFailoverGroup,
		`SHOW LISTINGS IN FAILOVER GROUP my_group`,
		`SHOW LISTINGS IN FAILOVER GROUP db.my_group`,
		`SHOW LISTINGS IN FAILOVER GROUP "G"`,
	)
	assertInvalid(t, (*Validator).ParseShowListingsInFailoverGroup,
		``,
		`SHOW LISTINGS IN FAILOVER GROUP`,
		`SHOW LISTINGS`,
	)
}

func TestParseShowLocks(t *testing.T) {
	assertValid(t, (*Validator).ParseShowLocks,
		`SHOW LOCKS`,
		`SHOW LOCKS IN ACCOUNT`,
		`SHOW LOCKS  IN  ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowLocks,
		``,
		`SHOW LOCK`,
	)
}

func TestParseShowMaintenancePolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowMaintenancePolicies,
		`SHOW MAINTENANCE POLICIES ON ACCOUNT`,
		`SHOW MAINTENANCE POLICIES IN ACCOUNT`,
		`SHOW MAINTENANCE POLICIES ON APPLICATION my_app`,
		`SHOW MAINTENANCE POLICIES IN TABLE my_table`,
	)
	assertInvalid(t, (*Validator).ParseShowMaintenancePolicies,
		``,
		`SHOW MAINTENANCE POLICIES`,
	)
}

func TestParseShowManagedAccounts(t *testing.T) {
	assertValid(t, (*Validator).ParseShowManagedAccounts,
		`SHOW MANAGED ACCOUNTS`,
		`SHOW MANAGED ACCOUNTS LIKE 'a%'`,
		`SHOW MANAGED ACCOUNTS LIKE 'x'`,
	)
	assertInvalid(t, (*Validator).ParseShowManagedAccounts,
		``,
		`SHOW ACCOUNTS`,
	)
}

func TestParseShowMaskingPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowMaskingPolicies,
		`SHOW MASKING POLICIES`,
		`SHOW MASKING POLICIES LIKE 'm%'`,
		`SHOW MASKING POLICIES IN SCHEMA my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowMaskingPolicies,
		``,
		`SHOW MASKING`,
	)
}

func TestParseShowMaterializedViews(t *testing.T) {
	assertValid(t, (*Validator).ParseShowMaterializedViews,
		`SHOW MATERIALIZED VIEWS`,
		`SHOW MATERIALIZED VIEWS LIKE 'v%'`,
		`SHOW MATERIALIZED VIEWS IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowMaterializedViews,
		``,
		`SHOW MATERIALIZED`,
	)
}

func TestParseShowMcpServers(t *testing.T) {
	assertValid(t, (*Validator).ParseShowMcpServers,
		`SHOW MCP SERVERS`,
		`SHOW MCP SERVERS LIKE 's%'`,
		`SHOW MCP SERVERS IN SCHEMA my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowMcpServers,
		``,
		`SHOW SERVERS`,
	)
}

func TestParseShowMfaMethods(t *testing.T) {
	assertValid(t, (*Validator).ParseShowMfaMethods,
		`SHOW MFA METHODS`,
		`SHOW MFA METHODS FOR USER my_user`,
		`SHOW MFA METHODS FOR USER "Bob"`,
	)
	assertInvalid(t, (*Validator).ParseShowMfaMethods,
		``,
		`SHOW MFA`,
	)
}

func TestParseShowModelMonitors(t *testing.T) {
	assertValid(t, (*Validator).ParseShowModelMonitors,
		`SHOW MODEL MONITORS`,
		`SHOW MODEL MONITORS LIKE 'm%'`,
		`SHOW MODEL MONITORS IN SCHEMA my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowModelMonitors,
		``,
		`SHOW MONITORS`,
	)
}

func TestParseShowModels(t *testing.T) {
	assertValid(t, (*Validator).ParseShowModels,
		`SHOW MODELS`,
		`SHOW MODELS LIKE 'm%'`,
		`SHOW MODELS IN DATABASE my_db`,
	)
	assertInvalid(t, (*Validator).ParseShowModels,
		``,
		`SHOW MODEL`,
	)
}

func TestParseShowNetworkPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowNetworkPolicies,
		`SHOW NETWORK POLICIES`,
		`SHOW  NETWORK  POLICIES`,
		`show network policies`,
	)
	assertInvalid(t, (*Validator).ParseShowNetworkPolicies,
		``,
		`SHOW POLICIES`,
	)
}

func TestParseShowNetworkRules(t *testing.T) {
	assertValid(t, (*Validator).ParseShowNetworkRules,
		`SHOW NETWORK RULES`,
		`SHOW NETWORK RULES LIKE 'r%'`,
		`SHOW NETWORK RULES IN SCHEMA my_schema LIMIT 5`,
	)
	assertInvalid(t, (*Validator).ParseShowNetworkRules,
		``,
		`SHOW RULES`,
	)
}

func TestParseShowNotebookProjects(t *testing.T) {
	assertValid(t, (*Validator).ParseShowNotebookProjects,
		`SHOW NOTEBOOK PROJECTS`,
		`SHOW NOTEBOOK PROJECTS IN ACCOUNT`,
		`SHOW NOTEBOOK PROJECTS IN SCHEMA my_db.my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowNotebookProjects,
		``,
		`SHOW PROJECTS`,
	)
}

func TestParseShowNotebooks(t *testing.T) {
	assertValid(t, (*Validator).ParseShowNotebooks,
		`SHOW NOTEBOOKS`,
		`SHOW NOTEBOOKS LIKE 'n%'`,
		`SHOW NOTEBOOKS IN SCHEMA my_schema STARTS WITH 'a'`,
	)
	assertInvalid(t, (*Validator).ParseShowNotebooks,
		``,
		`SHOW NOTEBOOK`,
	)
}

func TestParseShowNotificationIntegrations(t *testing.T) {
	assertValid(t, (*Validator).ParseShowNotificationIntegrations,
		`SHOW NOTIFICATION INTEGRATIONS`,
		`SHOW NOTIFICATION INTEGRATIONS LIKE 'i%'`,
		`SHOW NOTIFICATION INTEGRATIONS LIKE 'x'`,
	)
	assertInvalid(t, (*Validator).ParseShowNotificationIntegrations,
		``,
		`SHOW INTEGRATIONS`,
	)
}

func TestParseShowObjects(t *testing.T) {
	assertValid(t, (*Validator).ParseShowObjects,
		`SHOW OBJECTS`,
		`SHOW TERSE OBJECTS LIKE 'o%'`,
		`SHOW OBJECTS IN SCHEMA my_schema LIMIT 10`,
	)
	assertInvalid(t, (*Validator).ParseShowObjects,
		``,
		`SHOW OBJECT`,
	)
}

func TestParseShowOnlineFeatureTables(t *testing.T) {
	assertValid(t, (*Validator).ParseShowOnlineFeatureTables,
		`SHOW ONLINE FEATURE TABLES`,
		`SHOW ONLINE FEATURE TABLES LIKE 't%'`,
		`SHOW ONLINE FEATURE TABLES IN SCHEMA my_schema`,
	)
	assertInvalid(t, (*Validator).ParseShowOnlineFeatureTables,
		``,
		`SHOW FEATURE TABLES`,
	)
}

func TestParseShowOpenListingProviders(t *testing.T) {
	assertValid(t, (*Validator).ParseShowOpenListingProviders,
		`SHOW OPEN LISTING PROVIDERS`,
		`SHOW OPEN LISTING PROVIDERS LIKE 'p%'`,
		`SHOW OPEN LISTING PROVIDERS IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowOpenListingProviders,
		``,
		`SHOW LISTING PROVIDERS`,
	)
}

func TestParseShowOrganizationAccounts(t *testing.T) {
	assertValid(t, (*Validator).ParseShowOrganizationAccounts,
		`SHOW ORGANIZATION ACCOUNTS`,
		`SHOW ORGANIZATION ACCOUNTS LIKE 'a%'`,
		`SHOW ORGANIZATION ACCOUNTS LIKE 'x'`,
	)
	assertInvalid(t, (*Validator).ParseShowOrganizationAccounts,
		``,
		`SHOW ACCOUNTS`,
	)
}

func TestParseShowOrganizationProfiles(t *testing.T) {
	assertValid(t, (*Validator).ParseShowOrganizationProfiles,
		`SHOW ORGANIZATION PROFILES`,
		`SHOW  ORGANIZATION  PROFILES`,
		`show organization profiles`,
	)
	assertInvalid(t, (*Validator).ParseShowOrganizationProfiles,
		``,
		`SHOW PROFILES`,
	)
}

func TestParseShowOrganizationUsers(t *testing.T) {
	assertValid(t, (*Validator).ParseShowOrganizationUsers,
		`SHOW ORGANIZATION USERS`,
		`SHOW ORGANIZATION USERS IN ORGANIZATION USER GROUP my_group`,
		`SHOW ORGANIZATION USERS IN ORGANIZATION USER GROUP "G"`,
	)
	assertInvalid(t, (*Validator).ParseShowOrganizationUsers,
		``,
		`SHOW USERS`,
	)
}

func TestParseShowOrganizationUserGroups(t *testing.T) {
	assertValid(t, (*Validator).ParseShowOrganizationUserGroups,
		`SHOW ORGANIZATION USER GROUPS`,
		`SHOW  ORGANIZATION  USER  GROUPS`,
		`show organization user groups`,
	)
	assertInvalid(t, (*Validator).ParseShowOrganizationUserGroups,
		``,
		`SHOW USER GROUPS`,
	)
}

func TestParseShowOrganizations(t *testing.T) {
	assertValid(t, (*Validator).ParseShowOrganizations,
		`SHOW ORGANIZATIONS`,
		`SHOW ORGANIZATIONS LIKE 'o%'`,
		`SHOW ORGANIZATIONS IN ACCOUNT`,
	)
	assertInvalid(t, (*Validator).ParseShowOrganizations,
		``,
		`SHOW ORG`,
	)
}

func TestParseShowPackagesPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowPackagesPolicies,
		`SHOW PACKAGES POLICIES`,
		`SHOW PACKAGES POLICIES IN SCHEMA my_schema`,
		`SHOW PACKAGES POLICIES LIKE 'p%'`,
	)
	assertInvalid(t, (*Validator).ParseShowPackagesPolicies,
		``,
		`SHOW POLICIES`,
	)
}

func TestParseShowPasswordPolicies(t *testing.T) {
	assertValid(t, (*Validator).ParseShowPasswordPolicies,
		`SHOW PASSWORD POLICIES`,
		`SHOW PASSWORD POLICIES LIKE 'p%' IN SCHEMA my_schema`,
		`SHOW PASSWORD POLICIES ON ACCOUNT`,
		`SHOW PASSWORD POLICIES ON USER my_user`,
	)
	assertInvalid(t, (*Validator).ParseShowPasswordPolicies,
		``,
		`SHOW PASSWORD`,
	)
}
