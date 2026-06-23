package sqlgrammar

// SHOW commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseShowObjs validates the Snowflake `SHOW <objects>` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show
func (v *Validator) ParseShowObjs() bool {
	return true
}

// ParseShowAccounts validates the Snowflake `SHOW ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-accounts
func (v *Validator) ParseShowAccounts() bool {
	return true
}

// ParseShowAgents validates the Snowflake `SHOW AGENTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-agents
func (v *Validator) ParseShowAgents() bool {
	return true
}

// ParseShowAggregationPolicies validates the Snowflake `SHOW AGGREGATION POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-aggregation-policies
func (v *Validator) ParseShowAggregationPolicies() bool {
	return true
}

// ParseShowAlerts validates the Snowflake `SHOW ALERTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-alerts
func (v *Validator) ParseShowAlerts() bool {
	return true
}

// ParseShowApplicationPackages validates the Snowflake `SHOW APPLICATION PACKAGES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-application-packages
func (v *Validator) ParseShowApplicationPackages() bool {
	return true
}

// ParseShowApplicationRoles validates the Snowflake `SHOW APPLICATION ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-application-roles
func (v *Validator) ParseShowApplicationRoles() bool {
	return true
}

// ParseShowApplications validates the Snowflake `SHOW APPLICATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-applications
func (v *Validator) ParseShowApplications() bool {
	return true
}

// ParseShowAuthenticationPolicies validates the Snowflake `SHOW AUTHENTICATION POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-authentication-policies
func (v *Validator) ParseShowAuthenticationPolicies() bool {
	return true
}

// ParseShowAvailableListings validates the Snowflake `SHOW AVAILABLE LISTINGS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-available-listings
func (v *Validator) ParseShowAvailableListings() bool {
	return true
}

// ParseShowAvailableOffers validates the Snowflake `SHOW AVAILABLE OFFERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-available-offers
func (v *Validator) ParseShowAvailableOffers() bool {
	return true
}

// ParseShowAvailableOrganizationProfiles validates the Snowflake `SHOW AVAILABLE ORGANIZATION PROFILES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-available-organization-profiles
func (v *Validator) ParseShowAvailableOrganizationProfiles() bool {
	return true
}

// ParseShowBackupPolicies validates the Snowflake `SHOW BACKUP POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-backup-policies
func (v *Validator) ParseShowBackupPolicies() bool {
	return true
}

// ParseShowBackupSets validates the Snowflake `SHOW BACKUP SETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-backup-sets
func (v *Validator) ParseShowBackupSets() bool {
	return true
}

// ParseShowBackupsInBackupSet validates the Snowflake `SHOW BACKUPS IN BACKUP SET` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-backups-in-backup-set
func (v *Validator) ParseShowBackupsInBackupSet() bool {
	return true
}

// ParseShowCallerGrants validates the Snowflake `SHOW CALLER GRANTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-caller-grants
func (v *Validator) ParseShowCallerGrants() bool {
	return true
}

// ParseShowCatalogIntegrations validates the Snowflake `SHOW CATALOG INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-catalog-integrations
func (v *Validator) ParseShowCatalogIntegrations() bool {
	return true
}

// ParseShowChannels validates the Snowflake `SHOW CHANNELS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-channels
func (v *Validator) ParseShowChannels() bool {
	return true
}

// ParseShowClasses validates the Snowflake `SHOW CLASSES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-classes
func (v *Validator) ParseShowClasses() bool {
	return true
}

// ParseShowColumns validates the Snowflake `SHOW COLUMNS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-columns
func (v *Validator) ParseShowColumns() bool {
	return true
}

// ParseShowComputePoolInstanceFamilies validates the Snowflake `SHOW COMPUTE POOL INSTANCE FAMILIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-compute-pool-instance-families
func (v *Validator) ParseShowComputePoolInstanceFamilies() bool {
	return true
}

// ParseShowComputePools validates the Snowflake `SHOW COMPUTE POOLS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-compute-pools
func (v *Validator) ParseShowComputePools() bool {
	return true
}

// ParseShowConfigurations validates the Snowflake `SHOW CONFIGURATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-configurations
func (v *Validator) ParseShowConfigurations() bool {
	return true
}

// ParseShowConnections validates the Snowflake `SHOW CONNECTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-connections
func (v *Validator) ParseShowConnections() bool {
	return true
}

// ParseShowContacts validates the Snowflake `SHOW CONTACTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-contacts
func (v *Validator) ParseShowContacts() bool {
	return true
}

// ParseShowCortexSearchServices validates the Snowflake `SHOW CORTEX SEARCH SERVICES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-cortex-search
func (v *Validator) ParseShowCortexSearchServices() bool {
	return true
}

// ParseShowDataMetricFunctions validates the Snowflake `SHOW DATA METRIC FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-data-metric-functions
func (v *Validator) ParseShowDataMetricFunctions() bool {
	return true
}

// ParseShowDatabaseRoles validates the Snowflake `SHOW DATABASE ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-database-roles
func (v *Validator) ParseShowDatabaseRoles() bool {
	return true
}

// ParseShowDatabases validates the Snowflake `SHOW DATABASES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-databases
func (v *Validator) ParseShowDatabases() bool {
	return true
}

// ParseShowDatabasesInFailoverGroup validates the Snowflake `SHOW DATABASES IN FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-databases-in-failover-group
func (v *Validator) ParseShowDatabasesInFailoverGroup() bool {
	return true
}

// ParseShowDatabasesInReplicationGroup validates the Snowflake `SHOW DATABASES IN REPLICATION GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-databases-in-replication-group
func (v *Validator) ParseShowDatabasesInReplicationGroup() bool {
	return true
}

// ParseShowDatasets validates the Snowflake `SHOW DATASETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-datasets
func (v *Validator) ParseShowDatasets() bool {
	return true
}

// ParseShowDbtProjects validates the Snowflake `SHOW DBT PROJECTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-dbt-projects
func (v *Validator) ParseShowDbtProjects() bool {
	return true
}

// ParseShowDcmProjects validates the Snowflake `SHOW DCM PROJECTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-dcm-projects
func (v *Validator) ParseShowDcmProjects() bool {
	return true
}

// ParseShowDelegatedAuthorizations validates the Snowflake `SHOW DELEGATED AUTHORIZATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-delegated-authorizations
func (v *Validator) ParseShowDelegatedAuthorizations() bool {
	return true
}

// ParseShowDeploymentsInDcmProject validates the Snowflake `SHOW DEPLOYMENTS IN DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-deployments-in-dcm-project
func (v *Validator) ParseShowDeploymentsInDcmProject() bool {
	return true
}

// ParseShowDynamicTables validates the Snowflake `SHOW DYNAMIC TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-dynamic-tables
func (v *Validator) ParseShowDynamicTables() bool {
	return true
}

// ParseShowEndpoints validates the Snowflake `SHOW ENDPOINTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-endpoints
func (v *Validator) ParseShowEndpoints() bool {
	return true
}

// ParseShowEntitiesInDcmProject validates the Snowflake `SHOW ENTITIES IN DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-entities-in-dcm-project
func (v *Validator) ParseShowEntitiesInDcmProject() bool {
	return true
}

// ParseShowEventTables validates the Snowflake `SHOW EVENT TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-event-tables
func (v *Validator) ParseShowEventTables() bool {
	return true
}

// ParseShowExperiments validates the Snowflake `SHOW EXPERIMENTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-experiments
func (v *Validator) ParseShowExperiments() bool {
	return true
}

// ParseShowExternalAgents validates the Snowflake `SHOW EXTERNAL AGENTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-agents
func (v *Validator) ParseShowExternalAgents() bool {
	return true
}

// ParseShowExternalFunctions validates the Snowflake `SHOW EXTERNAL FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-functions
func (v *Validator) ParseShowExternalFunctions() bool {
	return true
}

// ParseShowExternalTables validates the Snowflake `SHOW EXTERNAL TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-tables
func (v *Validator) ParseShowExternalTables() bool {
	return true
}

// ParseShowExternalVolumes validates the Snowflake `SHOW EXTERNAL VOLUMES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-external-volumes
func (v *Validator) ParseShowExternalVolumes() bool {
	return true
}

// ParseShowFailoverGroups validates the Snowflake `SHOW FAILOVER GROUPS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-failover-groups
func (v *Validator) ParseShowFailoverGroups() bool {
	return true
}

// ParseShowFeaturePolicies validates the Snowflake `SHOW FEATURE POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-feature-policies
func (v *Validator) ParseShowFeaturePolicies() bool {
	return true
}

// ParseShowFileFormats validates the Snowflake `SHOW FILE FORMATS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-file-formats
func (v *Validator) ParseShowFileFormats() bool {
	return true
}

// ParseShowFunctions validates the Snowflake `SHOW FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-functions
func (v *Validator) ParseShowFunctions() bool {
	return true
}

// ParseShowFunctionsInModel validates the Snowflake `SHOW FUNCTIONS IN MODEL` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-functions-in-model
func (v *Validator) ParseShowFunctionsInModel() bool {
	return true
}

// ParseShowGateways validates the Snowflake `SHOW GATEWAYS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-gateways
func (v *Validator) ParseShowGateways() bool {
	return true
}

// ParseShowGitBranches validates the Snowflake `SHOW GIT BRANCHES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-git-branches
func (v *Validator) ParseShowGitBranches() bool {
	return true
}

// ParseShowGitRepositories validates the Snowflake `SHOW GIT REPOSITORIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-git-repositories
func (v *Validator) ParseShowGitRepositories() bool {
	return true
}

// ParseShowGitTags validates the Snowflake `SHOW GIT TAGS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-git-tags
func (v *Validator) ParseShowGitTags() bool {
	return true
}

// ParseShowGlobalAccounts validates the Snowflake `SHOW GLOBAL ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-global-accounts
func (v *Validator) ParseShowGlobalAccounts() bool {
	return true
}

// ParseShowGrants validates the Snowflake `SHOW GRANTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-grants
func (v *Validator) ParseShowGrants() bool {
	return true
}

// ParseShowGrantsInDcmProject validates the Snowflake `SHOW GRANTS IN DCM PROJECT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-grants-in-dcm-project
func (v *Validator) ParseShowGrantsInDcmProject() bool {
	return true
}

// ParseShowHybridTables validates the Snowflake `SHOW HYBRID TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-hybrid-tables
func (v *Validator) ParseShowHybridTables() bool {
	return true
}

// ParseShowIcebergTables validates the Snowflake `SHOW ICEBERG TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-iceberg-tables
func (v *Validator) ParseShowIcebergTables() bool {
	return true
}

// ParseShowImageRepositories validates the Snowflake `SHOW IMAGE REPOSITORIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-image-repositories
func (v *Validator) ParseShowImageRepositories() bool {
	return true
}

// ParseShowImagesInImageRepository validates the Snowflake `SHOW IMAGES IN IMAGE REPOSITORY` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-images-in-image-repository
func (v *Validator) ParseShowImagesInImageRepository() bool {
	return true
}

// ParseShowIndexes validates the Snowflake `SHOW INDEXES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-indexes
func (v *Validator) ParseShowIndexes() bool {
	return true
}

// ParseShowIntegrations validates the Snowflake `SHOW INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-integrations
func (v *Validator) ParseShowIntegrations() bool {
	return true
}

// ParseShowJoinPolicies validates the Snowflake `SHOW JOIN POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-join-policies
func (v *Validator) ParseShowJoinPolicies() bool {
	return true
}

// ParseShowListings validates the Snowflake `SHOW LISTINGS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-listings
func (v *Validator) ParseShowListings() bool {
	return true
}

// ParseShowListingsInFailoverGroup validates the Snowflake `SHOW LISTINGS IN FAILOVER GROUP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-listings-in-failover-group
func (v *Validator) ParseShowListingsInFailoverGroup() bool {
	return true
}

// ParseShowLocks validates the Snowflake `SHOW LOCKS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-locks
func (v *Validator) ParseShowLocks() bool {
	return true
}

// ParseShowMaintenancePolicies validates the Snowflake `SHOW MAINTENANCE POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-maintenance-policies
func (v *Validator) ParseShowMaintenancePolicies() bool {
	return true
}

// ParseShowManagedAccounts validates the Snowflake `SHOW MANAGED ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-managed-accounts
func (v *Validator) ParseShowManagedAccounts() bool {
	return true
}

// ParseShowMaskingPolicies validates the Snowflake `SHOW MASKING POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-masking-policies
func (v *Validator) ParseShowMaskingPolicies() bool {
	return true
}

// ParseShowMaterializedViews validates the Snowflake `SHOW MATERIALIZED VIEWS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-materialized-views
func (v *Validator) ParseShowMaterializedViews() bool {
	return true
}

// ParseShowMcpServers validates the Snowflake `SHOW MCP SERVERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-mcp-servers
func (v *Validator) ParseShowMcpServers() bool {
	return true
}

// ParseShowMfaMethods validates the Snowflake `SHOW MFA METHODS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-mfa-methods
func (v *Validator) ParseShowMfaMethods() bool {
	return true
}

// ParseShowModelMonitors validates the Snowflake `SHOW MODEL MONITORS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-model-monitors
func (v *Validator) ParseShowModelMonitors() bool {
	return true
}

// ParseShowModels validates the Snowflake `SHOW MODELS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-models
func (v *Validator) ParseShowModels() bool {
	return true
}

// ParseShowNetworkPolicies validates the Snowflake `SHOW NETWORK POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-network-policies
func (v *Validator) ParseShowNetworkPolicies() bool {
	return true
}

// ParseShowNetworkRules validates the Snowflake `SHOW NETWORK RULES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-network-rules
func (v *Validator) ParseShowNetworkRules() bool {
	return true
}

// ParseShowNotebookProjects validates the Snowflake `SHOW NOTEBOOK PROJECTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-notebook-projects
func (v *Validator) ParseShowNotebookProjects() bool {
	return true
}

// ParseShowNotebooks validates the Snowflake `SHOW NOTEBOOKS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-notebooks
func (v *Validator) ParseShowNotebooks() bool {
	return true
}

// ParseShowNotificationIntegrations validates the Snowflake `SHOW NOTIFICATION INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-notification-integrations
func (v *Validator) ParseShowNotificationIntegrations() bool {
	return true
}

// ParseShowObjects validates the Snowflake `SHOW OBJECTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-objects
func (v *Validator) ParseShowObjects() bool {
	return true
}

// ParseShowOnlineFeatureTables validates the Snowflake `SHOW ONLINE FEATURE TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-online-feature-tables
func (v *Validator) ParseShowOnlineFeatureTables() bool {
	return true
}

// ParseShowOpenListingProviders validates the Snowflake `SHOW OPEN LISTING PROVIDERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-open-listing-providers
func (v *Validator) ParseShowOpenListingProviders() bool {
	return true
}

// ParseShowOrganizationAccounts validates the Snowflake `SHOW ORGANIZATION ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-accounts
func (v *Validator) ParseShowOrganizationAccounts() bool {
	return true
}

// ParseShowOrganizationProfiles validates the Snowflake `SHOW ORGANIZATION PROFILES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-profiles
func (v *Validator) ParseShowOrganizationProfiles() bool {
	return true
}

// ParseShowOrganizationUsers validates the Snowflake `SHOW ORGANIZATION USERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-users
func (v *Validator) ParseShowOrganizationUsers() bool {
	return true
}

// ParseShowOrganizationUserGroups validates the Snowflake `SHOW ORGANIZATION USER GROUPS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organization-user-groups
func (v *Validator) ParseShowOrganizationUserGroups() bool {
	return true
}

// ParseShowOrganizations validates the Snowflake `SHOW ORGANIZATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-organizations
func (v *Validator) ParseShowOrganizations() bool {
	return true
}

// ParseShowPackagesPolicies validates the Snowflake `SHOW PACKAGES POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-packages-policies
func (v *Validator) ParseShowPackagesPolicies() bool {
	return true
}

// ParseShowPasswordPolicies validates the Snowflake `SHOW PASSWORD POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-password-policies
func (v *Validator) ParseShowPasswordPolicies() bool {
	return true
}

// ParseShowParameters validates the Snowflake `SHOW PARAMETERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-parameters
func (v *Validator) ParseShowParameters() bool {
	return true
}

// ParseShowPipes validates the Snowflake `SHOW PIPES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-pipes
func (v *Validator) ParseShowPipes() bool {
	return true
}

// ParseShowPostgresInstances validates the Snowflake `SHOW POSTGRES INSTANCES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-postgres-instances
func (v *Validator) ParseShowPostgresInstances() bool {
	return true
}

// ParseShowPrimaryKeys validates the Snowflake `SHOW PRIMARY KEYS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-primary-keys
func (v *Validator) ParseShowPrimaryKeys() bool {
	return true
}

// ParseShowPrivileges validates the Snowflake `SHOW PRIVILEGES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-privileges
func (v *Validator) ParseShowPrivileges() bool {
	return true
}

// ParseShowProcedures validates the Snowflake `SHOW PROCEDURES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-procedures
func (v *Validator) ParseShowProcedures() bool {
	return true
}

// ParseShowProvisionedThroughput validates the Snowflake `SHOW PROVISIONED THROUGHPUT` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-provisioned-throughput
func (v *Validator) ParseShowProvisionedThroughput() bool {
	return true
}

// ParseShowProjectionPolicies validates the Snowflake `SHOW PROJECTION POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-projection-policies
func (v *Validator) ParseShowProjectionPolicies() bool {
	return true
}

// ParseShowQueries validates the Snowflake `SHOW QUERIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-queries
func (v *Validator) ParseShowQueries() bool {
	return true
}

// ParseShowRegions validates the Snowflake `SHOW REGIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-regions
func (v *Validator) ParseShowRegions() bool {
	return true
}

// ParseShowReplicatedDatabases validates the Snowflake `SHOW REPLICATED DATABASES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replicated-databases
func (v *Validator) ParseShowReplicatedDatabases() bool {
	return true
}

// ParseShowReplicationAccounts validates the Snowflake `SHOW REPLICATION ACCOUNTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replication-accounts
func (v *Validator) ParseShowReplicationAccounts() bool {
	return true
}

// ParseShowReplicationDatabases validates the Snowflake `SHOW REPLICATION DATABASES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replication-databases
func (v *Validator) ParseShowReplicationDatabases() bool {
	return true
}

// ParseShowReplicationGroups validates the Snowflake `SHOW REPLICATION GROUPS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-replication-groups
func (v *Validator) ParseShowReplicationGroups() bool {
	return true
}

// ParseShowResourceMonitors validates the Snowflake `SHOW RESOURCE MONITORS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-resource-monitors
func (v *Validator) ParseShowResourceMonitors() bool {
	return true
}

// ParseShowRoles validates the Snowflake `SHOW ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-roles
func (v *Validator) ParseShowRoles() bool {
	return true
}

// ParseShowRowAccessPolicies validates the Snowflake `SHOW ROW ACCESS POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-row-access-policies
func (v *Validator) ParseShowRowAccessPolicies() bool {
	return true
}

// ParseShowSchemas validates the Snowflake `SHOW SCHEMAS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-schemas
func (v *Validator) ParseShowSchemas() bool {
	return true
}

// ParseShowSearchIndexes validates the Snowflake `SHOW SEARCH INDEXES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-search-indexes
func (v *Validator) ParseShowSearchIndexes() bool {
	return true
}

// ParseShowSecrets validates the Snowflake `SHOW SECRETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-secrets
func (v *Validator) ParseShowSecrets() bool {
	return true
}

// ParseShowSecurityIntegrations validates the Snowflake `SHOW SECURITY INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-security-integrations
func (v *Validator) ParseShowSecurityIntegrations() bool {
	return true
}

// ParseShowSemanticViews validates the Snowflake `SHOW SEMANTIC VIEWS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-semantic-views
func (v *Validator) ParseShowSemanticViews() bool {
	return true
}

// ParseShowSequences validates the Snowflake `SHOW SEQUENCES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-sequences
func (v *Validator) ParseShowSequences() bool {
	return true
}

// ParseShowServiceRoles validates the Snowflake `SHOW SERVICE ROLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-service-roles
func (v *Validator) ParseShowServiceRoles() bool {
	return true
}

// ParseShowServices validates the Snowflake `SHOW SERVICES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-services
func (v *Validator) ParseShowServices() bool {
	return true
}

// ParseShowSessionPolicies validates the Snowflake `SHOW SESSION POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-session-policies
func (v *Validator) ParseShowSessionPolicies() bool {
	return true
}

// ParseShowSessions validates the Snowflake `SHOW SESSIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-sessions
func (v *Validator) ParseShowSessions() bool {
	return true
}

// ParseShowShares validates the Snowflake `SHOW SHARES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-shares
func (v *Validator) ParseShowShares() bool {
	return true
}

// ParseShowSnapshots validates the Snowflake `SHOW SNAPSHOTS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-snapshots
func (v *Validator) ParseShowSnapshots() bool {
	return true
}

// ParseShowSnapshotPolicies validates the Snowflake `SHOW SNAPSHOT POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-snapshot-policies
func (v *Validator) ParseShowSnapshotPolicies() bool {
	return true
}

// ParseShowSnapshotSets validates the Snowflake `SHOW SNAPSHOT SETS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-snapshot-sets
func (v *Validator) ParseShowSnapshotSets() bool {
	return true
}

// ParseShowStages validates the Snowflake `SHOW STAGES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-stages
func (v *Validator) ParseShowStages() bool {
	return true
}

// ParseShowStorageIntegrations validates the Snowflake `SHOW STORAGE INTEGRATIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-storage-integrations
func (v *Validator) ParseShowStorageIntegrations() bool {
	return true
}

// ParseShowStorageLifecyclePolicies validates the Snowflake `SHOW STORAGE LIFECYCLE POLICIES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-storage-lifecycle-policies
func (v *Validator) ParseShowStorageLifecyclePolicies() bool {
	return true
}

// ParseShowStreams validates the Snowflake `SHOW STREAMS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-streams
func (v *Validator) ParseShowStreams() bool {
	return true
}

// ParseShowStreamlits validates the Snowflake `SHOW STREAMLITS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-streamlits
func (v *Validator) ParseShowStreamlits() bool {
	return true
}

// ParseShowTableFunctions validates the Snowflake `SHOW TABLE FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-table-functions
func (v *Validator) ParseShowTableFunctions() bool {
	return true
}

// ParseShowTables validates the Snowflake `SHOW TABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-tables
func (v *Validator) ParseShowTables() bool {
	return true
}

// ParseShowTags validates the Snowflake `SHOW TAGS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-tags
func (v *Validator) ParseShowTags() bool {
	return true
}

// ParseShowTasks validates the Snowflake `SHOW TASKS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-tasks
func (v *Validator) ParseShowTasks() bool {
	return true
}

// ParseShowTransactions validates the Snowflake `SHOW TRANSACTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-transactions
func (v *Validator) ParseShowTransactions() bool {
	return true
}

// ParseShowTypes validates the Snowflake `SHOW TYPES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-types
func (v *Validator) ParseShowTypes() bool {
	return true
}

// ParseShowUniqueKeys validates the Snowflake `SHOW UNIQUE KEYS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-unique-keys
func (v *Validator) ParseShowUniqueKeys() bool {
	return true
}

// ParseShowUserFunctions validates the Snowflake `SHOW USER FUNCTIONS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-user-functions
func (v *Validator) ParseShowUserFunctions() bool {
	return true
}

// ParseShowUsers validates the Snowflake `SHOW USERS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-users
func (v *Validator) ParseShowUsers() bool {
	return true
}

// ParseShowVariables validates the Snowflake `SHOW VARIABLES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-variables
func (v *Validator) ParseShowVariables() bool {
	return true
}

// ParseShowViews validates the Snowflake `SHOW VIEWS` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-views
func (v *Validator) ParseShowViews() bool {
	return true
}

// ParseShowWarehouses validates the Snowflake `SHOW WAREHOUSES` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/show-warehouses
func (v *Validator) ParseShowWarehouses() bool {
	return true
}
