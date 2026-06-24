package sqlgrammar

import "testing"

func TestParseAlterDcmProject(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterDcmProject,
		`ALTER DCM PROJECT my_proj SET LOG_LEVEL = 'INFO'`,
		`ALTER DCM PROJECT IF EXISTS my_proj UNSET COMMENT`,
		`ALTER DCM PROJECT db.sch.my_proj SET COMMENT = 'note'`,
	)
	assertInvalid(t, (*Validator).ParseAlterDcmProject,
		``,
		`ALTER DCM PROJECT my_proj`,
		`SELECT 1`,
	)
}

func TestParseAlterDynamicTable(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterDynamicTable,
		`ALTER DYNAMIC TABLE my_dt SUSPEND`,
		`ALTER DYNAMIC TABLE IF EXISTS my_dt RENAME TO new_dt`,
		`ALTER DYNAMIC TABLE my_dt REFRESH COPY SESSION`,
		`ALTER DYNAMIC TABLE my_dt SET TARGET_LAG = '1 minute'`,
		`ALTER DYNAMIC TABLE my_dt SWAP WITH other_dt`,
		`ALTER DYNAMIC TABLE my_dt UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterDynamicTable,
		``,
		`ALTER DYNAMIC TABLE my_dt`,
		`ALTER TABLE my_dt SUSPEND`,
		`ALTER DYNAMIC TABLE my_dt FOOBAR x`,
	)
}

func TestParseAlterExperiment(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterExperiment,
		`ALTER EXPERIMENT my_exp ADD RUN run1`,
		`ALTER EXPERIMENT my_exp COMMIT RUN run1`,
		`ALTER EXPERIMENT my_exp DROP RUN run1`,
	)
	assertInvalid(t, (*Validator).ParseAlterExperiment,
		``,
		`ALTER EXPERIMENT my_exp ADD run1`,
		`ALTER EXPERIMENT my_exp`,
	)
}

func TestParseAlterExternalAgent(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterExternalAgent,
		`ALTER EXTERNAL AGENT my_agent SET COMMENT = 'note'`,
		`ALTER EXTERNAL AGENT IF EXISTS my_agent ADD VERSION v1`,
		`ALTER EXTERNAL AGENT my_agent SET COMMENT = 'x'`,
	)
	assertInvalid(t, (*Validator).ParseAlterExternalAgent,
		``,
		`ALTER EXTERNAL AGENT my_agent`,
		`ALTER AGENT my_agent SET COMMENT = 'x'`,
	)
}

func TestParseAlterExternalAccessIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterExternalAccessIntegration,
		`ALTER EXTERNAL ACCESS INTEGRATION my_eai SET ENABLED = TRUE`,
		`ALTER EXTERNAL ACCESS INTEGRATION IF EXISTS my_eai UNSET COMMENT`,
		`ALTER EXTERNAL ACCESS INTEGRATION my_eai SET COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseAlterExternalAccessIntegration,
		``,
		`ALTER EXTERNAL ACCESS INTEGRATION my_eai`,
		`ALTER ACCESS INTEGRATION my_eai SET ENABLED = TRUE`,
	)
}

func TestParseAlterExternalTable(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterExternalTable,
		`ALTER EXTERNAL TABLE my_et REFRESH`,
		`ALTER EXTERNAL TABLE IF EXISTS my_et REFRESH '/path'`,
		`ALTER EXTERNAL TABLE my_et ADD FILES ('p/f1', 'p/f2')`,
		`ALTER EXTERNAL TABLE my_et DROP PARTITION LOCATION '/path'`,
		`ALTER EXTERNAL TABLE my_et SET AUTO_REFRESH = TRUE`,
	)
	assertInvalid(t, (*Validator).ParseAlterExternalTable,
		``,
		`ALTER EXTERNAL TABLE my_et`,
		`ALTER EXTERNAL TABLE my_et DROP PARTITION LOCATION`,
	)
}

func TestParseAlterExternalVolume(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterExternalVolume,
		`ALTER EXTERNAL VOLUME my_vol SET ALLOW_WRITES = TRUE`,
		`ALTER EXTERNAL VOLUME IF EXISTS my_vol REMOVE STORAGE_LOCATION 'loc1'`,
		`ALTER EXTERNAL VOLUME my_vol SET COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseAlterExternalVolume,
		``,
		`ALTER EXTERNAL VOLUME my_vol`,
		`ALTER VOLUME my_vol SET ALLOW_WRITES = TRUE`,
	)
}

func TestParseAlterFailoverGroup(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterFailoverGroup,
		`ALTER FAILOVER GROUP my_fg REFRESH`,
		`ALTER FAILOVER GROUP IF EXISTS my_fg RENAME TO new_fg`,
		`ALTER FAILOVER GROUP my_fg SUSPEND IMMEDIATE`,
		`ALTER FAILOVER GROUP my_fg SET OBJECT_TYPES = DATABASES`,
		`ALTER FAILOVER GROUP my_fg ADD db1 TO ALLOWED_DATABASES`,
	)
	assertInvalid(t, (*Validator).ParseAlterFailoverGroup,
		``,
		`ALTER FAILOVER GROUP my_fg`,
		`ALTER GROUP my_fg REFRESH`,
		`ALTER FAILOVER GROUP my_fg FOOBAR x`,
	)
}

func TestParseAlterFeaturePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterFeaturePolicy,
		`ALTER FEATURE POLICY my_fp SET COMMENT = 'c'`,
		`ALTER FEATURE POLICY IF EXISTS my_fp RENAME TO new_fp`,
		`ALTER FEATURE POLICY my_fp UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterFeaturePolicy,
		``,
		`ALTER FEATURE POLICY my_fp`,
		`ALTER POLICY my_fp SET COMMENT = 'c'`,
	)
}

func TestParseAlterFileFormat(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterFileFormat,
		`ALTER FILE FORMAT my_ff RENAME TO new_ff`,
		`ALTER FILE FORMAT IF EXISTS my_ff SET COMMENT = 'c'`,
		`ALTER FILE FORMAT my_ff SET COMPRESSION = 'GZIP'`,
	)
	assertInvalid(t, (*Validator).ParseAlterFileFormat,
		``,
		`ALTER FILE FORMAT my_ff`,
		`ALTER FORMAT my_ff RENAME TO new_ff`,
	)
}

func TestParseAlterFunction(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterFunction,
		`ALTER FUNCTION my_fn(NUMBER) RENAME TO new_fn`,
		`ALTER FUNCTION IF EXISTS my_fn() SET SECURE`,
		`ALTER FUNCTION my_fn(VARCHAR, NUMBER) UNSET SECURE`,
	)
	assertInvalid(t, (*Validator).ParseAlterFunction,
		``,
		`ALTER FUNCTION my_fn RENAME TO new_fn`,
		`ALTER FUNCTION my_fn(NUMBER)`,
	)
}

func TestParseAlterFunctionDmf(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterFunctionDmf,
		`ALTER FUNCTION my_dmf(TABLE(NUMBER)) RENAME TO new_dmf`,
		`ALTER FUNCTION IF EXISTS my_dmf(TABLE(NUMBER)) SET SECURE`,
		`ALTER FUNCTION my_dmf(TABLE(NUMBER)) UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterFunctionDmf,
		``,
		`ALTER FUNCTION my_dmf RENAME TO new_dmf`,
		`ALTER FUNCTION my_dmf(TABLE(NUMBER))`,
	)
}

func TestParseAlterFunctionSnowparkContainerServices(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterFunctionSnowparkContainerServices,
		`ALTER FUNCTION my_fn(NUMBER) SET MAX_BATCH_ROWS = 10`,
		`ALTER FUNCTION IF EXISTS my_fn() RENAME TO new_fn`,
		`ALTER FUNCTION my_fn(VARCHAR) UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterFunctionSnowparkContainerServices,
		``,
		`ALTER FUNCTION my_fn SET MAX_BATCH_ROWS = 10`,
		`ALTER FUNCTION my_fn(NUMBER)`,
	)
}

func TestParseAlterGateway(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterGateway,
		`ALTER GATEWAY my_gw FROM SPECIFICATION 'spec'`,
		`ALTER GATEWAY IF EXISTS my_gw FROM SPECIFICATION $$ yaml $$`,
		`ALTER GATEWAY db.sch.my_gw FROM SPECIFICATION 'text'`,
	)
	assertInvalid(t, (*Validator).ParseAlterGateway,
		``,
		`ALTER GATEWAY my_gw FROM SPECIFICATION`,
		`ALTER GATEWAY my_gw`,
	)
}

func TestParseAlterGitRepository(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterGitRepository,
		`ALTER GIT REPOSITORY my_repo FETCH`,
		`ALTER GIT REPOSITORY IF EXISTS my_repo SET COMMENT = 'c'`,
		`ALTER GIT REPOSITORY my_repo UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterGitRepository,
		``,
		`ALTER GIT REPOSITORY my_repo`,
		`ALTER REPOSITORY my_repo FETCH`,
	)
}

func TestParseAlterIcebergTable(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterIcebergTable,
		`ALTER ICEBERG TABLE my_it CLUSTER BY (c1, c2)`,
		`ALTER ICEBERG TABLE IF EXISTS my_it DROP CLUSTERING KEY`,
		`ALTER ICEBERG TABLE my_it SET AUTO_REFRESH = TRUE`,
		`ALTER ICEBERG TABLE my_it SUSPEND RECLUSTER`,
		`ALTER ICEBERG TABLE my_it ADD COLUMN c2 INT`,
	)
	assertInvalid(t, (*Validator).ParseAlterIcebergTable,
		``,
		`ALTER ICEBERG TABLE my_it`,
		`ALTER TABLE my_it CLUSTER BY (c1)`,
		`ALTER ICEBERG TABLE my_it FOOBAR x`,
	)
}

func TestParseAlterIcebergTableAlterColumnSetDataType(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterIcebergTableAlterColumnSetDataType,
		`ALTER ICEBERG TABLE my_it ALTER COLUMN c1 SET DATA TYPE NUMBER`,
		`ALTER ICEBERG TABLE IF EXISTS my_it ALTER COLUMN c1 SET DATA TYPE VARCHAR RENAME FIELDS`,
		`ALTER ICEBERG TABLE my_it ALTER COLUMN col SET DATA TYPE OBJECT(a NUMBER)`,
	)
	assertInvalid(t, (*Validator).ParseAlterIcebergTableAlterColumnSetDataType,
		``,
		`ALTER ICEBERG TABLE my_it ALTER COLUMN c1 SET DATA TYPE`,
		`ALTER ICEBERG TABLE my_it SET DATA TYPE NUMBER`,
	)
}

func TestParseAlterIcebergTableConvertToManaged(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterIcebergTableConvertToManaged,
		`ALTER ICEBERG TABLE my_it CONVERT TO MANAGED`,
		`ALTER ICEBERG TABLE IF EXISTS my_it CONVERT TO MANAGED BASE_LOCATION = 'dir'`,
		`ALTER ICEBERG TABLE my_it CONVERT TO MANAGED STORAGE_SERIALIZATION_POLICY = OPTIMIZED`,
	)
	assertInvalid(t, (*Validator).ParseAlterIcebergTableConvertToManaged,
		``,
		`ALTER ICEBERG TABLE my_it CONVERT TO`,
		`ALTER TABLE my_it CONVERT TO MANAGED`,
	)
}

func TestParseAlterIcebergTableRefresh(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterIcebergTableRefresh,
		`ALTER ICEBERG TABLE my_it REFRESH`,
		`ALTER ICEBERG TABLE IF EXISTS my_it REFRESH 'meta.json'`,
		`ALTER ICEBERG TABLE db.sch.my_it REFRESH`,
	)
	assertInvalid(t, (*Validator).ParseAlterIcebergTableRefresh,
		``,
		`ALTER ICEBERG TABLE my_it`,
		`ALTER TABLE my_it REFRESH`,
	)
}

func TestParseAlterIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterIntegration,
		`ALTER API INTEGRATION my_int SET ENABLED = TRUE`,
		`ALTER INTEGRATION my_int SET COMMENT = 'c'`,
		`ALTER STORAGE INTEGRATION IF EXISTS my_int UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterIntegration,
		``,
		`ALTER INTEGRATION my_int`,
		`SELECT 1`,
	)
}

func TestParseAlterJoinPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterJoinPolicy,
		`ALTER JOIN POLICY my_jp RENAME TO new_jp`,
		`ALTER JOIN POLICY my_jp SET BODY -> TRUE`,
		`ALTER JOIN POLICY IF EXISTS my_jp SET COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseAlterJoinPolicy,
		``,
		`ALTER JOIN POLICY my_jp`,
		`ALTER POLICY my_jp RENAME TO new_jp`,
	)
}

func TestParseAlterListing(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterListing,
		`ALTER LISTING my_listing PUBLISH`,
		`ALTER LISTING IF EXISTS my_listing RENAME TO new_listing`,
		`ALTER LISTING my_listing AS 'manifest' PUBLISH = TRUE`,
		`ALTER LISTING my_listing`,
	)
	assertInvalid(t, (*Validator).ParseAlterListing,
		``,
		`ALTER LISTING`,
		`SELECT 1`,
	)
}

func TestParseAlterMaintenancePolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterMaintenancePolicy,
		`ALTER MAINTENANCE POLICY my_mp SET SCHEDULE = '1 day'`,
		`ALTER MAINTENANCE POLICY IF EXISTS my_mp UNSET COMMENT`,
		`ALTER MAINTENANCE POLICY my_mp RENAME TO new_mp`,
	)
	assertInvalid(t, (*Validator).ParseAlterMaintenancePolicy,
		``,
		`ALTER MAINTENANCE POLICY my_mp`,
		`ALTER POLICY my_mp SET SCHEDULE = '1 day'`,
	)
}

func TestParseAlterMaskingPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterMaskingPolicy,
		`ALTER MASKING POLICY my_mp RENAME TO new_mp`,
		`ALTER MASKING POLICY my_mp SET BODY -> CASE WHEN 1=1 THEN val ELSE '***' END`,
		`ALTER MASKING POLICY IF EXISTS my_mp SET COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseAlterMaskingPolicy,
		``,
		`ALTER MASKING POLICY my_mp`,
		`ALTER POLICY my_mp RENAME TO new_mp`,
	)
}

func TestParseAlterMaterializedView(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterMaterializedView,
		`ALTER MATERIALIZED VIEW my_mv RENAME TO new_mv`,
		`ALTER MATERIALIZED VIEW my_mv CLUSTER BY (c1, c2)`,
		`ALTER MATERIALIZED VIEW my_mv SUSPEND RECLUSTER`,
		`ALTER MATERIALIZED VIEW my_mv SET COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseAlterMaterializedView,
		``,
		`ALTER MATERIALIZED VIEW my_mv`,
		`ALTER VIEW my_mv RENAME TO new_mv`,
	)
}

func TestParseAlterModel(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterModel,
		`ALTER MODEL my_model RENAME TO new_model`,
		`ALTER MODEL IF EXISTS my_model SET COMMENT = 'c'`,
		`ALTER MODEL my_model VERSION v1 SET ALIAS = champion`,
	)
	assertInvalid(t, (*Validator).ParseAlterModel,
		``,
		`ALTER MODEL my_model`,
		`ALTER my_model RENAME TO new_model`,
	)
}

func TestParseAlterModelAddVersion(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterModelAddVersion,
		`ALTER MODEL my_model ADD VERSION v1 FROM MODEL src_model`,
		`ALTER MODEL IF EXISTS my_model ADD VERSION v1 FROM MODEL src VERSION sv1`,
		`ALTER MODEL my_model ADD VERSION v2 FROM @my_stage/path`,
	)
	assertInvalid(t, (*Validator).ParseAlterModelAddVersion,
		``,
		`ALTER MODEL my_model ADD VERSION v1 FROM`,
		`ALTER MODEL my_model ADD VERSION v1`,
	)
}

func TestParseAlterModelDropVersion(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterModelDropVersion,
		`ALTER MODEL my_model DROP VERSION v1`,
		`ALTER MODEL IF EXISTS my_model DROP VERSION v2`,
		`ALTER MODEL db.sch.my_model DROP VERSION v3`,
	)
	assertInvalid(t, (*Validator).ParseAlterModelDropVersion,
		``,
		`ALTER MODEL my_model DROP VERSION`,
		`ALTER MODEL my_model DROP v1`,
	)
}

func TestParseAlterModelModifyVersion(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterModelModifyVersion,
		`ALTER MODEL my_model MODIFY VERSION v1 SET COMMENT = 'c'`,
		`ALTER MODEL IF EXISTS my_model MODIFY VERSION v1 SET METADATA = '{}'`,
		`ALTER MODEL my_model MODIFY VERSION v2 SET`,
	)
	assertInvalid(t, (*Validator).ParseAlterModelModifyVersion,
		``,
		`ALTER MODEL my_model MODIFY VERSION v1`,
		`ALTER MODEL my_model SET COMMENT = 'c'`,
	)
}

func TestParseAlterModelMonitor(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterModelMonitor,
		`ALTER MODEL MONITOR my_mon SUSPEND`,
		`ALTER MODEL MONITOR IF EXISTS my_mon SET WAREHOUSE = wh1`,
		`ALTER MODEL MONITOR my_mon ADD SEGMENT_COLUMN = 'region'`,
	)
	assertInvalid(t, (*Validator).ParseAlterModelMonitor,
		``,
		`ALTER MODEL MONITOR my_mon`,
		`ALTER MONITOR my_mon SUSPEND`,
	)
}

func TestParseAlterNetworkPolicy(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNetworkPolicy,
		`ALTER NETWORK POLICY my_np RENAME TO new_np`,
		`ALTER NETWORK POLICY IF EXISTS my_np SET COMMENT = 'c'`,
		`ALTER NETWORK POLICY my_np UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterNetworkPolicy,
		``,
		`ALTER NETWORK POLICY my_np`,
		`ALTER POLICY my_np RENAME TO new_np`,
	)
}

func TestParseAlterNetworkRule(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNetworkRule,
		`ALTER NETWORK RULE my_nr SET VALUE_LIST = ('a', 'b')`,
		`ALTER NETWORK RULE IF EXISTS my_nr UNSET COMMENT`,
		`ALTER NETWORK RULE my_nr SET COMMENT = 'c'`,
	)
	assertInvalid(t, (*Validator).ParseAlterNetworkRule,
		``,
		`ALTER NETWORK RULE my_nr`,
		`ALTER RULE my_nr SET VALUE_LIST = ('a')`,
	)
}

func TestParseAlterNotebook(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotebook,
		`ALTER NOTEBOOK my_nb RENAME TO new_nb`,
		`ALTER NOTEBOOK IF EXISTS my_nb SET COMMENT = 'c'`,
		`ALTER NOTEBOOK my_nb SET QUERY_WAREHOUSE = wh1`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotebook,
		``,
		`ALTER NOTEBOOK my_nb`,
		`ALTER my_nb RENAME TO new_nb`,
	)
}

func TestParseAlterNotificationIntegration(t *testing.T) {
	assertValid(t, (*Validator).ParseAlterNotificationIntegration,
		`ALTER NOTIFICATION INTEGRATION my_ni SET ENABLED = TRUE`,
		`ALTER INTEGRATION my_ni SET COMMENT = 'c'`,
		`ALTER NOTIFICATION INTEGRATION IF EXISTS my_ni UNSET COMMENT`,
	)
	assertInvalid(t, (*Validator).ParseAlterNotificationIntegration,
		``,
		`ALTER NOTIFICATION INTEGRATION my_ni`,
		`SELECT 1`,
	)
}
