package sqlgrammar

import (
	"strings"

	"thaw/internal/sqltok"
)

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
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		func() bool { return v.phrase("APPLICATION", "ROLE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("TO") },
		// { ROLE | APPLICATION ROLE | APPLICATION | USER } <name>
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Choice(
						func() bool { return v.phrase("APPLICATION", "ROLE") },
						func() bool { return v.MatchWord("APPLICATION") },
						func() bool { return v.MatchWord("ROLE") },
						func() bool { return v.MatchWord("USER") },
					)
				},
				v.parseIdentPath,
			)
		},
	)
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
	// Consume the privilege/securable span up to (but not including) the TO
	// boundary keyword, which must appear at paren-depth 0.
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		// GRANT [ALL] [INHERITED] CALLER ... — require the CALLER keyword.
		func() bool { return v.Optional(func() bool { return v.MatchWord("ALL") }) },
		func() bool { return v.Optional(func() bool { return v.MatchWord("INHERITED") }) },
		func() bool { return v.MatchWord("CALLER") },
		func() bool { return consumeToBoundary("TO") },
		func() bool { return v.MatchWord("TO") },
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Choice(
						func() bool { return v.phrase("DATABASE", "ROLE") },
						func() bool { return v.MatchWord("ROLE") },
						func() bool { return v.MatchWord("APPLICATION") },
					)
				},
				v.parseIdentPath,
			)
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		func() bool { return v.phrase("DATABASE", "ROLE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("TO") },
		// { DATABASE ROLE | ROLE | USER | APPLICATION } <name>
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Choice(
						func() bool { return v.phrase("DATABASE", "ROLE") },
						func() bool { return v.MatchWord("ROLE") },
						func() bool { return v.MatchWord("USER") },
						func() bool { return v.MatchWord("APPLICATION") },
					)
				},
				v.parseIdentPath,
			)
		},
	)
}

// ParseGrantDatabaseRoleToShare validates the Snowflake `GRANT DATABASE ROLE TO SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-database-role-share
//
// Syntax:
//
//	GRANT DATABASE ROLE <name>
//	  TO SHARE <share_name>
func (v *Validator) ParseGrantDatabaseRoleToShare() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		func() bool { return v.phrase("DATABASE", "ROLE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("TO") },
		func() bool { return v.MatchWord("SHARE") },
		v.parseIdentPath,
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		func() bool { return v.MatchWord("OWNERSHIP") },
		func() bool { return v.MatchWord("ON") },
		// securable span up to TO — free-form.
		func() bool { return consumeToBoundary("TO") },
		func() bool { return v.MatchWord("TO") },
		// { ROLE | DATABASE ROLE } <name>
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Choice(
						func() bool { return v.phrase("DATABASE", "ROLE") },
						func() bool { return v.MatchWord("ROLE") },
					)
				},
				v.parseIdentPath,
			)
		},
		// [ { REVOKE | COPY } CURRENT GRANTS ]
		func() bool {
			return v.Optional(func() bool {
				return v.Sequence(
					v.wordsValue("REVOKE", "COPY"),
					func() bool { return v.phrase("CURRENT", "GRANTS") },
				)
			})
		},
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		// require at least one token of privilege span before TO.
		func() bool {
			if v.AtEnd() || strings.EqualFold(v.Peek().Text(v.src), "TO") {
				v.expect("privileges")
				return false
			}
			return true
		},
		func() bool { return consumeToBoundary("TO") },
		func() bool { return v.MatchWord("TO") },
		// TO [ ROLE ] <name>  |  TO DATABASE ROLE <name>
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Optional(func() bool {
						return v.Choice(
							func() bool { return v.phrase("DATABASE", "ROLE") },
							func() bool { return v.MatchWord("ROLE") },
						)
					})
				},
				v.parseIdentPath,
			)
		},
		// [ WITH GRANT OPTION ]
		func() bool {
			return v.Optional(func() bool { return v.phrase("WITH", "GRANT", "OPTION") })
		},
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		func() bool {
			if v.AtEnd() || strings.EqualFold(v.Peek().Text(v.src), "TO") {
				v.expect("privileges")
				return false
			}
			return true
		},
		func() bool { return consumeToBoundary("TO") },
		func() bool { return v.MatchWord("TO") },
		// TO APPLICATION <name>  (the APPLICATION ROLE form is a separate command)
		func() bool { return v.MatchWord("APPLICATION") },
		func() bool {
			return v.Optional(func() bool { return v.MatchWord("ROLE") })
		},
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool { return v.phrase("WITH", "GRANT", "OPTION") })
		},
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		func() bool {
			if v.AtEnd() || strings.EqualFold(v.Peek().Text(v.src), "TO") {
				v.expect("privileges")
				return false
			}
			return true
		},
		func() bool { return consumeToBoundary("TO") },
		func() bool { return v.MatchWord("TO") },
		func() bool { return v.phrase("APPLICATION", "ROLE") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool { return v.phrase("WITH", "GRANT", "OPTION") })
		},
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		// objectPrivilege ON ... up to the TO SHARE boundary.
		func() bool {
			if v.AtEnd() || strings.EqualFold(v.Peek().Text(v.src), "TO") {
				v.expect("privilege")
				return false
			}
			return true
		},
		func() bool { return consumeToBoundary("TO") },
		func() bool { return v.MatchWord("TO") },
		func() bool { return v.MatchWord("SHARE") },
		v.parseIdentPath,
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		func() bool {
			if v.AtEnd() || strings.EqualFold(v.Peek().Text(v.src), "TO") {
				v.expect("privileges")
				return false
			}
			return true
		},
		func() bool { return consumeToBoundary("TO") },
		func() bool { return v.MatchWord("TO") },
		// TO [ USER ] <name>
		func() bool { return v.Optional(func() bool { return v.MatchWord("USER") }) },
		v.parseIdentPath,
		func() bool {
			return v.Optional(func() bool { return v.phrase("WITH", "GRANT", "OPTION") })
		},
	)
}

// ParseGrantRole validates the Snowflake `GRANT ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-role
//
// Syntax:
//
//	GRANT ROLE <name> TO { ROLE <parent_role_name> | USER <user_name> }
func (v *Validator) ParseGrantRole() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		func() bool { return v.MatchWord("ROLE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("TO") },
		// { ROLE <name> | USER <name> }
		func() bool {
			return v.Sequence(
				v.wordsValue("ROLE", "USER"),
				v.parseIdentPath,
			)
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("GRANT") },
		func() bool { return v.phrase("SERVICE", "ROLE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("TO") },
		// { ROLE | APPLICATION ROLE | DATABASE ROLE } <name>
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Choice(
						func() bool { return v.phrase("APPLICATION", "ROLE") },
						func() bool { return v.phrase("DATABASE", "ROLE") },
						func() bool { return v.MatchWord("ROLE") },
					)
				},
				v.parseIdentPath,
			)
		},
	)
}

// ParseRevokeApplicationRole validates the Snowflake `REVOKE APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-application-role
//
// Syntax:
//
//	REVOKE APPLICATION ROLE <name> FROM { ROLE <parent_role_name> | APPLICATION ROLE <application_role> | APPLICATION <application> }
func (v *Validator) ParseRevokeApplicationRole() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		func() bool { return v.phrase("APPLICATION", "ROLE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("FROM") },
		// { ROLE | APPLICATION ROLE | APPLICATION } <name>
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Choice(
						func() bool { return v.phrase("APPLICATION", "ROLE") },
						func() bool { return v.MatchWord("APPLICATION") },
						func() bool { return v.MatchWord("ROLE") },
					)
				},
				v.parseIdentPath,
			)
		},
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("ALL") }) },
		func() bool { return v.Optional(func() bool { return v.MatchWord("INHERITED") }) },
		func() bool { return v.MatchWord("CALLER") },
		func() bool { return consumeToBoundary("FROM") },
		func() bool { return v.MatchWord("FROM") },
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Choice(
						func() bool { return v.phrase("DATABASE", "ROLE") },
						func() bool { return v.MatchWord("ROLE") },
					)
				},
				v.parseIdentPath,
			)
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		func() bool { return v.phrase("DATABASE", "ROLE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("FROM") },
		// { ROLE | DATABASE ROLE | APPLICATION } <name>
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Choice(
						func() bool { return v.phrase("DATABASE", "ROLE") },
						func() bool { return v.MatchWord("ROLE") },
						func() bool { return v.MatchWord("APPLICATION") },
					)
				},
				v.parseIdentPath,
			)
		},
	)
}

// ParseRevokeDatabaseRoleFromShare validates the Snowflake `REVOKE DATABASE ROLE FROM SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-database-role-share
//
// Syntax:
//
//	REVOKE DATABASE ROLE <name>
//	  FROM SHARE <share_name>
func (v *Validator) ParseRevokeDatabaseRoleFromShare() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		func() bool { return v.phrase("DATABASE", "ROLE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("FROM") },
		func() bool { return v.MatchWord("SHARE") },
		v.parseIdentPath,
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		// [ GRANT OPTION FOR ]
		func() bool {
			return v.Optional(func() bool { return v.phrase("GRANT", "OPTION", "FOR") })
		},
		func() bool {
			if v.AtEnd() || strings.EqualFold(v.Peek().Text(v.src), "FROM") {
				v.expect("privileges")
				return false
			}
			return true
		},
		func() bool { return consumeToBoundary("FROM") },
		func() bool { return v.MatchWord("FROM") },
		// FROM [ ROLE ] <name>  |  FROM DATABASE ROLE <name>
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Optional(func() bool {
						return v.Choice(
							func() bool { return v.phrase("DATABASE", "ROLE") },
							func() bool { return v.MatchWord("ROLE") },
						)
					})
				},
				v.parseIdentPath,
			)
		},
		// [ RESTRICT | CASCADE ]
		func() bool {
			return v.Optional(v.wordsValue("RESTRICT", "CASCADE"))
		},
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		func() bool {
			return v.Optional(func() bool { return v.phrase("GRANT", "OPTION", "FOR") })
		},
		func() bool {
			if v.AtEnd() || strings.EqualFold(v.Peek().Text(v.src), "FROM") {
				v.expect("privileges")
				return false
			}
			return true
		},
		func() bool { return consumeToBoundary("FROM") },
		func() bool { return v.MatchWord("FROM") },
		func() bool { return v.MatchWord("APPLICATION") },
		func() bool { return v.Optional(func() bool { return v.MatchWord("ROLE") }) },
		v.parseIdentPath,
		func() bool {
			return v.Optional(v.wordsValue("RESTRICT", "CASCADE"))
		},
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		func() bool {
			return v.Optional(func() bool { return v.phrase("GRANT", "OPTION", "FOR") })
		},
		func() bool {
			if v.AtEnd() || strings.EqualFold(v.Peek().Text(v.src), "FROM") {
				v.expect("privileges")
				return false
			}
			return true
		},
		func() bool { return consumeToBoundary("FROM") },
		func() bool { return v.MatchWord("FROM") },
		func() bool { return v.phrase("APPLICATION", "ROLE") },
		v.parseIdentPath,
		func() bool {
			return v.Optional(v.wordsValue("RESTRICT", "CASCADE"))
		},
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		func() bool {
			if v.AtEnd() || strings.EqualFold(v.Peek().Text(v.src), "FROM") {
				v.expect("privilege")
				return false
			}
			return true
		},
		func() bool { return consumeToBoundary("FROM") },
		func() bool { return v.MatchWord("FROM") },
		func() bool { return v.MatchWord("SHARE") },
		v.parseIdentPath,
	)
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
	consumeToBoundary := func(boundary string) bool {
		depth := 0
		for !v.AtEnd() {
			t := v.Peek()
			if depth == 0 && t.Kind.IsIdentLike() && strings.EqualFold(t.Text(v.src), boundary) {
				return true
			}
			switch t.Kind {
			case sqltok.LParen:
				depth++
			case sqltok.RParen:
				if depth > 0 {
					depth--
				}
			}
			v.advance()
		}
		v.expect(boundary)
		return false
	}
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		func() bool {
			return v.Optional(func() bool { return v.phrase("GRANT", "OPTION", "FOR") })
		},
		func() bool {
			if v.AtEnd() || strings.EqualFold(v.Peek().Text(v.src), "FROM") {
				v.expect("privileges")
				return false
			}
			return true
		},
		func() bool { return consumeToBoundary("FROM") },
		func() bool { return v.MatchWord("FROM") },
		// FROM [ USER ] <name>
		func() bool { return v.Optional(func() bool { return v.MatchWord("USER") }) },
		v.parseIdentPath,
		func() bool {
			return v.Optional(v.wordsValue("RESTRICT", "CASCADE"))
		},
	)
}

// ParseRevokeRole validates the Snowflake `REVOKE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-role
//
// Syntax:
//
//	REVOKE ROLE <name> FROM { ROLE <parent_role_name> | USER <user_name> }
func (v *Validator) ParseRevokeRole() bool {
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		func() bool { return v.MatchWord("ROLE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("FROM") },
		// { ROLE <name> | USER <name> }
		func() bool {
			return v.Sequence(
				v.wordsValue("ROLE", "USER"),
				v.parseIdentPath,
			)
		},
	)
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
	return v.Sequence(
		func() bool { return v.MatchWord("REVOKE") },
		func() bool { return v.phrase("SERVICE", "ROLE") },
		v.parseIdentPath,
		func() bool { return v.MatchWord("FROM") },
		// { ROLE | APPLICATION ROLE | DATABASE ROLE } <name>
		func() bool {
			return v.Sequence(
				func() bool {
					return v.Choice(
						func() bool { return v.phrase("APPLICATION", "ROLE") },
						func() bool { return v.phrase("DATABASE", "ROLE") },
						func() bool { return v.MatchWord("ROLE") },
					)
				},
				v.parseIdentPath,
			)
		},
	)
}
