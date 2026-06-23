package sqlgrammar

// GRANT / REVOKE commands — grammar-rule stubs for issue #556.
//
// Each function corresponds to one Snowflake command reference (see the per-
// function header comment for the command name and its documentation URL).
// These are STUBS: they return true unconditionally. The recursive-descent
// grammar bodies are to be implemented per the ParseCopyInto pattern in #556.

// ParseGrantApplicationRole validates the Snowflake `GRANT APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-application-role
func (v *Validator) ParseGrantApplicationRole() bool {
	return true
}

// ParseGrantCaller validates the Snowflake `GRANT CALLER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-caller
func (v *Validator) ParseGrantCaller() bool {
	return true
}

// ParseGrantDatabaseRole validates the Snowflake `GRANT DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-database-role
func (v *Validator) ParseGrantDatabaseRole() bool {
	return true
}

// ParseGrantDatabaseRoleToShare validates the Snowflake `GRANT DATABASE ROLE TO SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-database-role-share
func (v *Validator) ParseGrantDatabaseRoleToShare() bool {
	return true
}

// ParseGrantOwnership validates the Snowflake `GRANT OWNERSHIP` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-ownership
func (v *Validator) ParseGrantOwnership() bool {
	return true
}

// ParseGrantPrivsToRole validates the Snowflake `GRANT <privileges> TO ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-privilege
func (v *Validator) ParseGrantPrivsToRole() bool {
	return true
}

// ParseGrantPrivsToApplication validates the Snowflake `GRANT <privileges> TO APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-privilege-application
func (v *Validator) ParseGrantPrivsToApplication() bool {
	return true
}

// ParseGrantPrivsToApplicationRole validates the Snowflake `GRANT <privileges> TO APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-privilege-application-role
func (v *Validator) ParseGrantPrivsToApplicationRole() bool {
	return true
}

// ParseGrantPrivToShare validates the Snowflake `GRANT <privilege> TO SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-privilege-share
func (v *Validator) ParseGrantPrivToShare() bool {
	return true
}

// ParseGrantPrivsToUser validates the Snowflake `GRANT <privileges> TO USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-privilege-user
func (v *Validator) ParseGrantPrivsToUser() bool {
	return true
}

// ParseGrantRole validates the Snowflake `GRANT ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-role
func (v *Validator) ParseGrantRole() bool {
	return true
}

// ParseGrantServiceRole validates the Snowflake `GRANT SERVICE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/grant-service-role
func (v *Validator) ParseGrantServiceRole() bool {
	return true
}

// ParseRevokeApplicationRole validates the Snowflake `REVOKE APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-application-role
func (v *Validator) ParseRevokeApplicationRole() bool {
	return true
}

// ParseRevokeCaller validates the Snowflake `REVOKE CALLER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-caller
func (v *Validator) ParseRevokeCaller() bool {
	return true
}

// ParseRevokeDatabaseRole validates the Snowflake `REVOKE DATABASE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-database-role
func (v *Validator) ParseRevokeDatabaseRole() bool {
	return true
}

// ParseRevokeDatabaseRoleFromShare validates the Snowflake `REVOKE DATABASE ROLE FROM SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-database-role-share
func (v *Validator) ParseRevokeDatabaseRoleFromShare() bool {
	return true
}

// ParseRevokePrivsFromRole validates the Snowflake `REVOKE <privileges> FROM ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-privilege
func (v *Validator) ParseRevokePrivsFromRole() bool {
	return true
}

// ParseRevokePrivsFromApplication validates the Snowflake `REVOKE <privileges> FROM APPLICATION` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-privilege-application
func (v *Validator) ParseRevokePrivsFromApplication() bool {
	return true
}

// ParseRevokePrivsFromApplicationRole validates the Snowflake `REVOKE <privileges> FROM APPLICATION ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-privilege-application-role
func (v *Validator) ParseRevokePrivsFromApplicationRole() bool {
	return true
}

// ParseRevokePrivFromShare validates the Snowflake `REVOKE <privilege> FROM SHARE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-privilege-share
func (v *Validator) ParseRevokePrivFromShare() bool {
	return true
}

// ParseRevokePrivsFromUser validates the Snowflake `REVOKE <privileges> FROM USER` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-privilege-user
func (v *Validator) ParseRevokePrivsFromUser() bool {
	return true
}

// ParseRevokeRole validates the Snowflake `REVOKE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-role
func (v *Validator) ParseRevokeRole() bool {
	return true
}

// ParseRevokeServiceRole validates the Snowflake `REVOKE SERVICE ROLE` command.
// Reference: https://docs.snowflake.com/en/sql-reference/sql/revoke-service-role
func (v *Validator) ParseRevokeServiceRole() bool {
	return true
}
