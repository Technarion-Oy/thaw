package sqlgrammar

// GRANT / REVOKE commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseGrantApplicationRole validates the Snowflake `GRANT APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-application-role
//
// Syntax:
//
//	GRANT APPLICATION ROLE <name> TO  { ROLE <parent_role_name> | APPLICATION ROLE <application_role> | APPLICATION <application_name> | USER <user_name> }
func (v *Validator) ParseGrantApplicationRole() bool {
	return true
}

// ParseGrantCaller validates the Snowflake `GRANT CALLER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-caller
//
// Syntax:
//
//	GRANT CALLER <object_privilege> [ , <object_privilege> ... ]
//	  ON <object_type> <object_name>
//	  TO { ROLE | DATABASE ROLE | APPLICATION } <grantee_name>
//
//	GRANT ALL CALLER PRIVILEGES
//	  ON <object_type> <object_name>
//	  TO { ROLE | DATABASE ROLE | APPLICATION } <grantee_name>
//
//	GRANT INHERITED CALLER <object_privilege> [ , <object_privilege> ... ]
//	  ON ALL <object_type_plural>
//	  IN { ACCOUNT | DATABASE <db_name> | SCHEMA <schema_name> | APPLICATION <app_name> | APPLICATION PACKAGE <app_pkg_name> }
//	  TO { ROLE | DATABASE ROLE | APPLICATION } <grantee_name>
//
//	GRANT ALL INHERITED CALLER PRIVILEGES
//	  ON ALL <object_type_plural>
//	  IN { ACCOUNT | DATABASE <db_name> | SCHEMA <schema_name> | APPLICATION <app_name> | APPLICATION PACKAGE <app_pkg_name> }
//	  TO { ROLE | DATABASE ROLE | APPLICATION } <grantee_name>
func (v *Validator) ParseGrantCaller() bool {
	return true
}

// ParseGrantDatabaseRole validates the Snowflake `GRANT DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-database-role
//
// Syntax:
//
//	GRANT DATABASE ROLE <name> TO { DATABASE ROLE <parent_role_name> | ROLE <parent_role_name> | USER <user_name> }
//
//	GRANT DATABASE ROLE <name> TO APPLICATION <app_name>
func (v *Validator) ParseGrantDatabaseRole() bool {
	return true
}

// ParseGrantDatabaseRoleToShare validates the Snowflake `GRANT DATABASE ROLE TO SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-database-role-share
//
// Syntax:
//
//	GRANT DATABASE ROLE <name>
//	  TO SHARE <share_name>
func (v *Validator) ParseGrantDatabaseRoleToShare() bool {
	return true
}

// ParseGrantOwnership validates the Snowflake `GRANT OWNERSHIP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-ownership
//
// Syntax:
//
//	GRANT OWNERSHIP
//	  { ON {
//	            <object_type> <object_name>
//	          | ALL <object_type_plural> IN { DATABASE <database_name> | SCHEMA <schema_name> }
//	       }
//	    | ON FUTURE <object_type_plural> IN { DATABASE <database_name> | SCHEMA <schema_name> }
//	  }
//	  TO { ROLE <role_name> | DATABASE ROLE <database_role_name> }
//	  [ { REVOKE | COPY } CURRENT GRANTS ]
//
//	GRANT OWNERSHIP
//	  ON  <class_name> <instance_name>
//	  TO { ROLE <role_name> | DATABASE ROLE <database_role_name> }
//	  [ { REVOKE | COPY } CURRENT GRANTS ]
func (v *Validator) ParseGrantOwnership() bool {
	return true
}

// ParseGrantPrivsToRole validates the Snowflake `GRANT <privileges> TO ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-privilege
//
// Syntax:
//
//	-- Account roles:
//	GRANT {  { globalPrivileges         | ALL [ PRIVILEGES ] } ON ACCOUNT
//	       | { accountObjectPrivileges  | ALL [ PRIVILEGES ] } ON { USER | RESOURCE MONITOR | WAREHOUSE | COMPUTE POOL | DATABASE | INTEGRATION | CONNECTION | FAILOVER GROUP | REPLICATION GROUP | EXTERNAL VOLUME } <object_name>
//	       | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { SCHEMA <schema_name> | ALL SCHEMAS IN DATABASE <db_name> }
//	       | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { FUTURE SCHEMAS IN DATABASE <db_name> }
//	       | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON { <object_type> <object_name> | ALL <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> } }
//	       | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON FUTURE <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> }
//	      }
//	  TO [ ROLE ] <role_name> [ WITH GRANT OPTION ]
//
//	-- Database roles:
//	GRANT {  { CREATE SCHEMA | MODIFY | MONITOR | USAGE } [ , ... ] } ON DATABASE <object_name>
//	       | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { SCHEMA <schema_name> | ALL SCHEMAS IN DATABASE <db_name> }
//	       | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { FUTURE SCHEMAS IN DATABASE <db_name> }
//	       | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON { <object_type> <object_name> | ALL <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> } }
//	       | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON FUTURE <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> }
//	      }
//	  TO DATABASE ROLE <database_role_name> [ WITH GRANT OPTION ]
func (v *Validator) ParseGrantPrivsToRole() bool {
	return true
}

// ParseGrantPrivsToApplication validates the Snowflake `GRANT <privileges> TO APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-privilege-application
//
// Syntax:
//
//	GRANT {  { globalPrivileges } ON ACCOUNT
//	       | { accountObjectPrivileges  | ALL [ PRIVILEGES ] } ON { USER | RESOURCE MONITOR | WAREHOUSE | COMPUTE POOL | DATABASE | INTEGRATION | CONNECTION | FAILOVER GROUP | REPLICATION GROUP | EXTERNAL VOLUME } <object_name>
//	       | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { SCHEMA <schema_name> | ALL SCHEMAS IN DATABASE <db_name> }
//	       | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON { <object_type> <object_name> | ALL <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> }
//	      }
//	    TO APPLICATION <name>
//
//	Where:
//
//	globalPrivileges ::=
//	  {
//	      CREATE {
//	       COMPUTE POOL | DATABASE | WAREHOUSE
//	      }
//	      | BIND SERVICE ENDPOINT
//	      | EXECUTE MANAGED TASK
//	      | MANAGE WAREHOUSES
//	      | READ SESSION
//	  }
//	  [ , ... ]
//
//	accountObjectPrivileges ::=
//	  -- per object type: { APPLYBUDGET | FAILOVER | IMPORTED PRIVILEGES | MODIFY | MONITOR | OPERATE | REPLICATE | USAGE | USE_ANY_ROLE | ... } [ , ... ]
//
//	schemaPrivileges ::=
//	  ADD SEARCH OPTIMIZATION
//	  | CREATE { ALERT | EXTERNAL TABLE | FILE FORMAT | FUNCTION | IMAGE REPOSITORY | MATERIALIZED VIEW | PIPE | PROCEDURE
//	      | { AGGREGATION | MASKING | PASSWORD | PROJECTION | ROW ACCESS | SESSION } POLICY
//	      | SECRET | SEMANTIC VIEW | SEQUENCE | SERVICE | SNAPSHOT | STAGE | STREAM | TAG | TABLE | TASK | VIEW }
//	  | MODIFY | MONITOR | USAGE
//	  [ , ... ]
//
//	schemaObjectPrivileges ::=
//	  -- per object type, e.g. SELECT / INSERT / USAGE / REFERENCES / APPLY / READ / WRITE / OPERATE / MONITOR / etc.
func (v *Validator) ParseGrantPrivsToApplication() bool {
	return true
}

// ParseGrantPrivsToApplicationRole validates the Snowflake `GRANT <privileges> TO APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-privilege-application-role
//
// Syntax:
//
//	GRANT {
//	        { schemaPrivileges         | ALL [ PRIVILEGES ] } ON SCHEMA <schema_name>
//	        | { schemaObjectPrivileges | ALL [ PRIVILEGES ] } ON { <object_type> <object_name> | ALL <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> }
//	        | { schemaObjectPrivileges | ALL [ PRIVILEGES ] } ON FUTURE <object_type_plural> IN SCHEMA <schema_name>
//	      }
//	    TO APPLICATION ROLE <name> [ WITH GRANT OPTION ]
//
//	Where:
//
//	schemaPrivileges ::=
//	  {
//	    ADD SEARCH OPTIMIZATION
//	    | CREATE { ALERT | EXTERNAL TABLE | FILE FORMAT | FUNCTION | IMAGE REPOSITORY | MATERIALIZED VIEW | PIPE | PROCEDURE
//	        | { AGGREGATION | MASKING | PASSWORD | PROJECTION | ROW ACCESS | SESSION } POLICY
//	        | SECRET | SEMANTIC VIEW | SEQUENCE | SERVICE | SNAPSHOT | STAGE | STREAM | TAG | TABLE | TASK | VIEW }
//	    | MODIFY | MONITOR | USAGE
//	  }
//	  [ , ... ]
//
//	schemaObjectPrivileges ::=
//	  -- per object type, e.g. SELECT / INSERT / USAGE / REFERENCES / APPLY / READ / WRITE / OPERATE / MONITOR / etc.
func (v *Validator) ParseGrantPrivsToApplicationRole() bool {
	return true
}

// ParseGrantPrivToShare validates the Snowflake `GRANT <privilege> TO SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-privilege-share
//
// Syntax:
//
//	GRANT objectPrivilege ON
//	     {  DATABASE <name>
//	      | SCHEMA <name>
//	      | FUNCTION <name>
//	      | SEMANTIC VIEW <name>
//	      | { TABLE <name> | ALL TABLES IN SCHEMA <schema_name> }
//	      | { EXTERNAL TABLE <name> | ALL EXTERNAL TABLES IN SCHEMA <schema_name> }
//	      | { ICEBERG TABLE <name> | ALL ICEBERG TABLES IN SCHEMA <schema_name> }
//	      | { DYNAMIC TABLE <name> | ALL DYNAMIC TABLES IN SCHEMA <schema_name> }
//	      | TAG <name>
//	      | VIEW <name>  }
//	  TO SHARE <share_name>
//
//	Where:
//
//	objectPrivilege ::=
//	-- For DATABASE
//	   REFERENCE_USAGE [ , ... ]
//	-- For DATABASE, FUNCTION, or SCHEMA
//	   USAGE [ , ... ]
//	-- For SEMANTIC VIEW
//	   { REFERENCES | SELECT } [ , ... ]
//	-- For TABLE
//	   EVOLVE SCHEMA [ , ... ]
//	-- For EXTERNAL TABLE, ICEBERG TABLE, TABLE, or VIEW
//	   SELECT [ , ... ]
//	-- For TAG
//	   READ
func (v *Validator) ParseGrantPrivToShare() bool {
	return true
}

// ParseGrantPrivsToUser validates the Snowflake `GRANT <privileges> TO USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-privilege-user
//
// Syntax:
//
//	GRANT {  { globalPrivileges         | ALL [ PRIVILEGES ] } ON ACCOUNT
//	       | { accountObjectPrivileges  | ALL [ PRIVILEGES ] } ON { USER | RESOURCE MONITOR | WAREHOUSE | COMPUTE POOL | DATABASE | INTEGRATION | CONNECTION | FAILOVER GROUP | REPLICATION GROUP | EXTERNAL VOLUME } <object_name>
//	       | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { SCHEMA <schema_name> | ALL SCHEMAS IN DATABASE <db_name> }
//	       | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON { <object_type> <object_name> | ALL <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> } }
//	      }
//	  TO [ USER ] <user_name> [ WITH GRANT OPTION ]
//
//	Where:
//
//	globalPrivileges ::=
//	  { ATTACH POLICY | AUDIT | BIND SERVICE ENDPOINT
//	    | APPLY { { AGGREGATION | AUTHENTICATION | JOIN | MASKING | PACKAGES | PASSWORD | PROJECTION | ROW ACCESS | SESSION } POLICY | TAG }
//	    | EXECUTE { ALERT | DATA METRIC FUNCTION | MANAGED ALERT | MANAGED TASK | TASK }
//	    | IMPORT SHARE
//	    | MANAGE { ACCOUNT SUPPORT CASES | EVENT SHARING | GRANTS | LISTING AUTO FULFILLMENT | ORGANIZATION SUPPORT CASES | USER SUPPORT CASES | WAREHOUSES }
//	    | MODIFY { LOG LEVEL | TRACE LEVEL | SESSION LOG LEVEL | SESSION TRACE LEVEL }
//	    | MONITOR { EXECUTION | SECURITY | USAGE }
//	    | OVERRIDE SHARE RESTRICTIONS | PURCHASE DATA EXCHANGE LISTING | RESOLVE ALL | READ SESSION }
//	  [ , ... ]
//
//	accountObjectPrivileges ::=
//	  -- per object type: { APPLYBUDGET | FAILOVER | IMPORTED PRIVILEGES | MODIFY | MONITOR | OPERATE | REPLICATE | USAGE | USE_ANY_ROLE } [ , ... ]
//
//	schemaPrivileges ::=
//	  ADD SEARCH OPTIMIZATION | APPLYBUDGET | MODIFY | MONITOR | USAGE [ , ... ]
//
//	schemaObjectPrivileges ::=
//	  -- per object type, e.g. SELECT / INSERT / USAGE / REFERENCES / APPLY / READ / WRITE / OPERATE / MONITOR / DELETE / UPDATE / TRUNCATE / etc.
func (v *Validator) ParseGrantPrivsToUser() bool {
	return true
}

// ParseGrantRole validates the Snowflake `GRANT ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-role
//
// Syntax:
//
//	GRANT ROLE <name> TO { ROLE <parent_role_name> | USER <user_name> }
func (v *Validator) ParseGrantRole() bool {
	return true
}

// ParseGrantServiceRole validates the Snowflake `GRANT SERVICE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-service-role
//
// Syntax:
//
//	GRANT SERVICE ROLE <name> TO
//	{
//	  ROLE <role_name>                     |
//	  APPLICATION ROLE <application_role_name>  |
//	  DATABASE ROLE <database_role_name>
//	}
func (v *Validator) ParseGrantServiceRole() bool {
	return true
}

// ParseRevokeApplicationRole validates the Snowflake `REVOKE APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-application-role
//
// Syntax:
//
//	REVOKE APPLICATION ROLE <name> FROM { ROLE <parent_role_name> | APPLICATION ROLE <application_role> | APPLICATION <application> }
func (v *Validator) ParseRevokeApplicationRole() bool {
	return true
}

// ParseRevokeCaller validates the Snowflake `REVOKE CALLER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-caller
//
// Syntax:
//
//	REVOKE CALLER <object_privilege> [ , <object_privilege> ... ]
//	  ON <object_type> <object_name>
//	  FROM { ROLE | DATABASE ROLE } <grantee_name>
//
//	REVOKE ALL CALLER PRIVILEGES
//	  ON <object_type> <object_name>
//	  FROM { ROLE | DATABASE ROLE } <grantee_name>
//
//	REVOKE INHERITED CALLER <object_privilege> [ , <object_privilege> ... ]
//	  ON ALL <object_type_plural>
//	  IN { ACCOUNT | DATABASE <db_name> | SCHEMA <schema_name> | APPLICATION <app_name> | APPLICATION PACKAGE <app_pkg_name> }
//	  FROM { ROLE | DATABASE ROLE } <grantee_name>
//
//	REVOKE ALL INHERITED CALLER PRIVILEGES
//	  ON ALL <object_type_plural>
//	  IN { ACCOUNT | DATABASE <db_name> | SCHEMA <schema_name> | APPLICATION <app_name> | APPLICATION PACKAGE <app_pkg_name> }
//	  FROM { ROLE | DATABASE ROLE } <grantee_name>
func (v *Validator) ParseRevokeCaller() bool {
	return true
}

// ParseRevokeDatabaseRole validates the Snowflake `REVOKE DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-database-role
//
// Syntax:
//
//	REVOKE DATABASE ROLE <name> FROM { ROLE | DATABASE ROLE } <parent_role_name>
//
//	REVOKE DATABASE ROLE <name> FROM APPLICATION <app_name>
func (v *Validator) ParseRevokeDatabaseRole() bool {
	return true
}

// ParseRevokeDatabaseRoleFromShare validates the Snowflake `REVOKE DATABASE ROLE FROM SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-database-role-share
//
// Syntax:
//
//	REVOKE DATABASE ROLE <name>
//	  FROM SHARE <share_name>
func (v *Validator) ParseRevokeDatabaseRoleFromShare() bool {
	return true
}

// ParseRevokePrivsFromRole validates the Snowflake `REVOKE <privileges> FROM ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-privilege
//
// Syntax:
//
//	-- Account roles:
//	REVOKE [ GRANT OPTION FOR ]
//	    {
//	       { globalPrivileges         | ALL [ PRIVILEGES ] } ON ACCOUNT
//	     | { accountObjectPrivileges  | ALL [ PRIVILEGES ] } ON { RESOURCE MONITOR | WAREHOUSE | COMPUTE POOL | DATABASE | INTEGRATION | CONNECTION | FAILOVER GROUP | REPLICATION GROUP | EXTERNAL VOLUME } <object_name>
//	     | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { SCHEMA <schema_name> | ALL SCHEMAS IN DATABASE <db_name> }
//	     | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { FUTURE SCHEMAS IN DATABASE <db_name> }
//	     | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON { <object_type> <object_name> | ALL <object_type_plural> IN SCHEMA <schema_name> }
//	     | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON FUTURE <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> }
//	    }
//	  FROM [ ROLE ] <role_name> [ RESTRICT | CASCADE ]
//
//	-- Database roles:
//	REVOKE [ GRANT OPTION FOR ]
//	    {
//	       { CREATE SCHEMA | MODIFY | MONITOR | USAGE } [ , ... ] } ON DATABASE <object_name>
//	       { globalPrivileges         | ALL [ PRIVILEGES ] } ON ACCOUNT
//	     | { accountObjectPrivileges  | ALL [ PRIVILEGES ] } ON { RESOURCE MONITOR | WAREHOUSE | COMPUTE POOL | DATABASE | INTEGRATION | EXTERNAL VOLUME } <object_name>
//	     | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { SCHEMA <schema_name> | ALL SCHEMAS IN DATABASE <db_name> }
//	     | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { FUTURE SCHEMAS IN DATABASE <db_name> }
//	     | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON { <object_type> <object_name> | ALL <object_type_plural> IN SCHEMA <schema_name> }
//	     | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON FUTURE <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> }
//	    }
//	  FROM DATABASE ROLE <database_role_name> [ RESTRICT | CASCADE ]
func (v *Validator) ParseRevokePrivsFromRole() bool {
	return true
}

// ParseRevokePrivsFromApplication validates the Snowflake `REVOKE <privileges> FROM APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-privilege-application
//
// Syntax:
//
//	REVOKE {  { globalPrivileges } ON ACCOUNT
//	        | { accountObjectPrivileges  | ALL [ PRIVILEGES ] } ON { USER | RESOURCE MONITOR | WAREHOUSE | COMPUTE POOL | DATABASE | INTEGRATION | CONNECTION | FAILOVER GROUP | REPLICATION GROUP | EXTERNAL VOLUME } <object_name>
//	        | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { SCHEMA <schema_name> | ALL SCHEMAS IN DATABASE <db_name> }
//	        | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON { <object_type> <object_name> | ALL <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> }
//	       }
//	     FROM APPLICATION <name>
//
//	Where:
//
//	globalPrivileges ::=
//	  { CREATE { COMPUTE POOL | DATABASE | WAREHOUSE } | BIND SERVICE ENDPOINT | EXECUTE MANAGED TASK | MANAGE WAREHOUSES | READ SESSION } [ , ... ]
//
//	accountObjectPrivileges ::=
//	  -- per object type: { APPLYBUDGET | FAILOVER | IMPORTED PRIVILEGES | MODIFY | MONITOR | OPERATE | REPLICATE | USAGE | USE_ANY_ROLE | ... } [ , ... ]
//
//	schemaPrivileges ::=
//	  ADD SEARCH OPTIMIZATION
//	  | CREATE { ALERT | EXTERNAL TABLE | FILE FORMAT | FUNCTION | IMAGE REPOSITORY | MATERIALIZED VIEW | PIPE | PROCEDURE
//	      | { AGGREGATION | MASKING | PASSWORD | PROJECTION | ROW ACCESS | SESSION } POLICY
//	      | SECRET | SEMANTIC VIEW | SEQUENCE | SERVICE | SNAPSHOT | STAGE | STREAM | TAG | TABLE | TASK | VIEW }
//	  | MODIFY | MONITOR | USAGE
//	  [ , ... ]
//
//	schemaObjectPrivileges ::=
//	  -- per object type, e.g. SELECT / INSERT / USAGE / REFERENCES / APPLY / READ / WRITE / OPERATE / MONITOR / etc.
func (v *Validator) ParseRevokePrivsFromApplication() bool {
	return true
}

// ParseRevokePrivsFromApplicationRole validates the Snowflake `REVOKE <privileges> FROM APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-privilege-application-role
//
// Syntax:
//
//	REVOKE [ GRANT OPTION FOR ]
//	    {
//	    | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { SCHEMA <schema_name> | ALL SCHEMAS IN DATABASE <db_name> }
//	    | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { FUTURE SCHEMAS IN DATABASE <db_name> }
//	    | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON { <object_type> <object_name> | ALL <object_type_plural> IN SCHEMA <schema_name> }
//	    | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON FUTURE <object_type_plural> IN { DATABASE <db_name> | SCHEMA <schema_name> }
//	    }
//	  FROM APPLICATION ROLE <name> [ RESTRICT | CASCADE ]
//
//	Where:
//
//	schemaPrivileges ::=
//	  {
//	    ADD SEARCH OPTIMIZATION
//	    | CREATE { ALERT | EXTERNAL TABLE | FILE FORMAT | FUNCTION | IMAGE REPOSITORY | MATERIALIZED VIEW | PIPE | PROCEDURE
//	        | { AGGREGATION | MASKING | PASSWORD | PROJECTION | ROW ACCESS | SESSION } POLICY
//	        | SECRET | SEMANTIC VIEW | SEQUENCE | SERVICE | SNAPSHOT | STAGE | STREAM | TAG | TABLE | TASK | VIEW }
//	    | MODIFY | MONITOR | USAGE
//	  }
//	  [ , ... ]
//
//	schemaObjectPrivileges ::=
//	  -- per object type, e.g. SELECT / INSERT / USAGE / REFERENCES / APPLY / READ / WRITE / OPERATE / MONITOR / etc.
func (v *Validator) ParseRevokePrivsFromApplicationRole() bool {
	return true
}

// ParseRevokePrivFromShare validates the Snowflake `REVOKE <privilege> FROM SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-privilege-share
//
// Syntax:
//
//	REVOKE objectPrivilege ON
//	     {  DATABASE <name>
//	      | SCHEMA <name>
//	      | SEMANTIC VIEW <name>
//	      | { TABLE <name> | ALL TABLES IN SCHEMA <schema_name> }
//	      | { EXTERNAL TABLE <name> | ALL EXTERNAL TABLES IN SCHEMA <schema_name> }
//	      | { ICEBERG TABLE <name> | ALL ICEBERG TABLES IN SCHEMA <schema_name> }
//	      | { DYNAMIC TABLE <name> | ALL DYNAMIC TABLES IN SCHEMA <schema_name> }
//	      | { VIEW <name> | ALL VIEWS IN SCHEMA <schema_name> }  }
//	  FROM SHARE <share_name>
//
//	Where:
//
//	objectPrivilege ::=
//	-- For DATABASE
//	   REFERENCE_USAGE [ , ... ]
//	-- For DATABASE, FUNCTION, or SCHEMA
//	   USAGE [ , ... ]
//	-- For SEMANTIC VIEW
//	   { REFERENCES | SELECT } [ , ... ]
//	-- For TABLE
//	   EVOLVE SCHEMA [ , ... ]
//	-- For EXTERNAL TABLE, ICEBERG TABLE, TABLE, or VIEW
//	   SELECT [ , ... ]
//	-- For TAG
//	   READ
func (v *Validator) ParseRevokePrivFromShare() bool {
	return true
}

// ParseRevokePrivsFromUser validates the Snowflake `REVOKE <privileges> FROM USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-privilege-user
//
// Syntax:
//
//	REVOKE [ GRANT OPTION FOR ]
//	    {
//	       { globalPrivileges         | ALL [ PRIVILEGES ] } ON ACCOUNT
//	     | { accountObjectPrivileges  | ALL [ PRIVILEGES ] } ON { RESOURCE MONITOR | WAREHOUSE | COMPUTE POOL | DATABASE | INTEGRATION | CONNECTION | FAILOVER GROUP | REPLICATION GROUP | EXTERNAL VOLUME } <object_name>
//	     | { schemaPrivileges         | ALL [ PRIVILEGES ] } ON { SCHEMA <schema_name> | ALL SCHEMAS IN DATABASE <db_name> }
//	     | { schemaObjectPrivileges   | ALL [ PRIVILEGES ] } ON { <object_type> <object_name> | ALL <object_type_plural> IN SCHEMA <schema_name> }
//	    }
//	  FROM [ USER ] <user_name> [ RESTRICT | CASCADE ]
//
//	Where:
//
//	globalPrivileges ::=
//	  { ATTACH POLICY | AUDIT | BIND SERVICE ENDPOINT
//	    | APPLY { { AGGREGATION | AUTHENTICATION | JOIN | MASKING | PACKAGES | PASSWORD | PROJECTION | ROW ACCESS | SESSION } POLICY | TAG }
//	    | EXECUTE { ALERT | DATA METRIC FUNCTION | MANAGED ALERT | MANAGED TASK | TASK }
//	    | IMPORT SHARE
//	    | MANAGE { ACCOUNT SUPPORT CASES | EVENT SHARING | GRANTS | LISTING AUTO FULFILLMENT | ORGANIZATION SUPPORT CASES | USER SUPPORT CASES | WAREHOUSES }
//	    | MODIFY { LOG LEVEL | TRACE LEVEL | SESSION LOG LEVEL | SESSION TRACE LEVEL }
//	    | MONITOR { EXECUTION | SECURITY | USAGE }
//	    | OVERRIDE SHARE RESTRICTIONS | PURCHASE DATA EXCHANGE LISTING | RESOLVE ALL | READ SESSION }
//	  [ , ... ]
//
//	accountObjectPrivileges ::=
//	  -- per object type: { APPLYBUDGET | FAILOVER | IMPORTED PRIVILEGES | MODIFY | MONITOR | OPERATE | REPLICATE | USAGE | USE_ANY_ROLE } [ , ... ]
//
//	schemaPrivileges ::=
//	  ADD SEARCH OPTIMIZATION | APPLYBUDGET | MODIFY | MONITOR | USAGE [ , ... ]
//
//	schemaObjectPrivileges ::=
//	  -- per object type, e.g. SELECT / INSERT / USAGE / REFERENCES / APPLY / READ / WRITE / OPERATE / MONITOR / DELETE / UPDATE / TRUNCATE / etc.
func (v *Validator) ParseRevokePrivsFromUser() bool {
	return true
}

// ParseRevokeRole validates the Snowflake `REVOKE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-role
//
// Syntax:
//
//	REVOKE ROLE <name> FROM { ROLE <parent_role_name> | USER <user_name> }
func (v *Validator) ParseRevokeRole() bool {
	return true
}

// ParseRevokeServiceRole validates the Snowflake `REVOKE SERVICE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-service-role
//
// Syntax:
//
//	REVOKE SERVICE ROLE <name> FROM
//	{
//	  ROLE <role_name>                     |
//	  APPLICATION ROLE <application_role_name>  |
//	  DATABASE ROLE <database_role_name>
//	}
func (v *Validator) ParseRevokeServiceRole() bool {
	return true
}
