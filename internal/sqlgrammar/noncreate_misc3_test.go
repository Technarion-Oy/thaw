package sqlgrammar

import "testing"

// -- DML (dml.go) --

func TestParseDelete(t *testing.T) {
	assertValid(t, (*Validator).ParseDelete,
		`DELETE FROM my_table`,
		`DELETE FROM db.schema.t WHERE id = 1`,
		`DELETE FROM t USING other o WHERE t.id = o.id`,
	)
	assertInvalid(t, (*Validator).ParseDelete,
		``,
		`SELECT 1`,
		`DELETE my_table`,
	)
}

func TestParseInsert(t *testing.T) {
	assertValid(t, (*Validator).ParseInsert,
		`INSERT INTO t VALUES (1, 2)`,
		`INSERT OVERWRITE INTO t (a, b) VALUES (1, 2)`,
		`INSERT INTO db.s.t SELECT * FROM other`,
	)
	assertInvalid(t, (*Validator).ParseInsert,
		``,
		`UPDATE t SET a = 1`,
		`INSERT INTO t`,
	)
}

func TestParseInsertMultiTable(t *testing.T) {
	assertValid(t, (*Validator).ParseInsertMultiTable,
		`INSERT ALL INTO t1 VALUES (a) SELECT a FROM src`,
		`INSERT OVERWRITE ALL INTO t1 INTO t2 SELECT * FROM src`,
		`INSERT FIRST WHEN a > 0 THEN INTO t1 ELSE INTO t2 SELECT a FROM src`,
	)
	assertInvalid(t, (*Validator).ParseInsertMultiTable,
		``,
		`INSERT INTO t VALUES (1)`,
		`INSERT ALL`,
	)
}

func TestParseMerge(t *testing.T) {
	assertValid(t, (*Validator).ParseMerge,
		`MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET t.a = s.a`,
		`MERGE INTO db.s.t USING (SELECT * FROM x) src ON t.k = src.k WHEN NOT MATCHED THEN INSERT (a) VALUES (src.a)`,
		`MERGE INTO target USING source ON target.id = source.id WHEN MATCHED THEN DELETE`,
	)
	assertInvalid(t, (*Validator).ParseMerge,
		``,
		`MERGE t USING s ON x`,
		`MERGE INTO t USING s`,
	)
}

func TestParseSelect(t *testing.T) {
	assertValid(t, (*Validator).ParseSelect,
		`SELECT 1`,
		`SELECT a, b FROM t WHERE a > 1`,
		`SELECT DISTINCT col FROM db.s.t ORDER BY col`,
		`SELECT ALL * FROM t`,
		`SELECT TOP 5 a, b FROM t`,
		`SELECT * FROM t`,
		`SELECT EXTRACT(YEAR FROM dt) AS y FROM t`,        // FROM nested in a function call
		`SELECT a FROM t WHERE x IN ( SELECT id FROM u )`, // boundary keyword nested in a subquery
		`SELECT a, count(*) FROM t GROUP BY a HAVING count(*) > 1 ORDER BY a LIMIT 10`, // full clause stack
		`SELECT a FROM t QUALIFY row_number() OVER ( ORDER BY a ) = 1`,
		`SELECT a FROM t1 JOIN t2 ON t1.id = t2.id`, // permissive FROM body (joins)
		`SELECT 1 UNION SELECT 2`,                   // set operator
		`SELECT 1 UNION ALL SELECT 2 ORDER BY 1`,    // trailing ORDER BY on the union
		`SELECT 1 INTERSECT SELECT 2 EXCEPT SELECT 3`,
		`SELECT a FROM t OFFSET 5 ROWS FETCH NEXT 10 ROWS ONLY`,
		`SELECT a FROM t FOR UPDATE`,
	)
	assertInvalid(t, (*Validator).ParseSelect,
		``,
		`INSERT INTO t VALUES (1)`,
		`SELECT`,                  // no projection — 0 columns is not allowed
		`SELECT FROM t`,           // empty projection before FROM — 0 columns is not allowed
		`SELECT a FROM`,           // dangling FROM with no table
		`SELECT a FROM t GROUP a`, // GROUP without BY
	)
}

func TestParseTruncateTable(t *testing.T) {
	assertValid(t, (*Validator).ParseTruncateTable,
		`TRUNCATE TABLE my_table`,
		`TRUNCATE my_table`,
		`TRUNCATE TABLE IF EXISTS db.s.t`,
	)
	assertInvalid(t, (*Validator).ParseTruncateTable,
		``,
		`DROP TABLE foo`,
		`TRUNCATE TABLE`,
	)
}

func TestParseUpdate(t *testing.T) {
	assertValid(t, (*Validator).ParseUpdate,
		`UPDATE t SET a = 1`,
		`UPDATE db.s.t SET a = 1, b = 2 WHERE id = 3`,
		`UPDATE t SET a = s.a FROM src s WHERE t.id = s.id`,
	)
	assertInvalid(t, (*Validator).ParseUpdate,
		``,
		`DELETE FROM t`,
		`UPDATE t`,
	)
}

func TestParseTruncateMaterializedView(t *testing.T) {
	assertValid(t, (*Validator).ParseTruncateMaterializedView,
		`TRUNCATE MATERIALIZED VIEW mv`,
		`TRUNCATE MATERIALIZED VIEW db.s.mv`,
		`TRUNCATE MATERIALIZED VIEW "MyView"`,
	)
	assertInvalid(t, (*Validator).ParseTruncateMaterializedView,
		``,
		`TRUNCATE TABLE t`,
		`TRUNCATE MATERIALIZED VIEW`,
	)
}

// -- GRANT / REVOKE (grant_revoke.go) --

func TestParseGrantApplicationRole(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantApplicationRole,
		`GRANT APPLICATION ROLE ar TO ROLE r`,
		`GRANT APPLICATION ROLE ar TO APPLICATION ROLE other`,
		`GRANT APPLICATION ROLE ar TO USER u`,
	)
	assertInvalid(t, (*Validator).ParseGrantApplicationRole,
		``,
		`GRANT ROLE r TO USER u`,
		`GRANT APPLICATION ROLE ar TO`,
	)
}

func TestParseGrantCaller(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantCaller,
		`GRANT CALLER SELECT ON TABLE t TO ROLE r`,
		`GRANT ALL CALLER PRIVILEGES ON TABLE t TO DATABASE ROLE dr`,
		`GRANT INHERITED CALLER SELECT ON ALL TABLES IN SCHEMA s TO APPLICATION app`,
	)
	assertInvalid(t, (*Validator).ParseGrantCaller,
		``,
		`GRANT SELECT ON TABLE t TO ROLE r`,
		`GRANT CALLER SELECT ON TABLE t TO`,
	)
}

func TestParseGrantDatabaseRole(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantDatabaseRole,
		`GRANT DATABASE ROLE dr TO ROLE r`,
		`GRANT DATABASE ROLE dr TO DATABASE ROLE other`,
		`GRANT DATABASE ROLE dr TO APPLICATION app`,
	)
	assertInvalid(t, (*Validator).ParseGrantDatabaseRole,
		``,
		`GRANT DATABASE ROLE dr TO SHARE sh`,
		`GRANT ROLE r TO USER u`,
	)
}

func TestParseGrantDatabaseRoleToShare(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantDatabaseRoleToShare,
		`GRANT DATABASE ROLE dr TO SHARE sh`,
		`GRANT DATABASE ROLE db.dr TO SHARE my_share`,
		`GRANT DATABASE ROLE "DR" TO SHARE "SH"`,
	)
	assertInvalid(t, (*Validator).ParseGrantDatabaseRoleToShare,
		``,
		`GRANT DATABASE ROLE dr TO ROLE r`,
		`GRANT DATABASE ROLE dr TO SHARE`,
	)
}

func TestParseGrantOwnership(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantOwnership,
		`GRANT OWNERSHIP ON TABLE t TO ROLE r`,
		`GRANT OWNERSHIP ON ALL TABLES IN SCHEMA s TO DATABASE ROLE dr`,
		`GRANT OWNERSHIP ON TABLE t TO ROLE r COPY CURRENT GRANTS`,
	)
	assertInvalid(t, (*Validator).ParseGrantOwnership,
		``,
		`GRANT OWNERSHIP TABLE t TO ROLE r`,
		`GRANT OWNERSHIP ON TABLE t TO USER u`,
	)
}

func TestParseGrantPrivsToRole(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantPrivsToRole,
		`GRANT SELECT ON TABLE t TO ROLE r`,
		`GRANT ALL PRIVILEGES ON SCHEMA s TO ROLE r WITH GRANT OPTION`,
		`GRANT USAGE ON DATABASE d TO DATABASE ROLE dr`,
	)
	assertInvalid(t, (*Validator).ParseGrantPrivsToRole,
		``,
		`GRANT TO ROLE r`,
		`REVOKE SELECT ON TABLE t FROM ROLE r`,
	)
}

func TestParseGrantPrivsToApplication(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantPrivsToApplication,
		`GRANT USAGE ON DATABASE d TO APPLICATION app`,
		`GRANT SELECT ON TABLE t TO APPLICATION my_app`,
		`GRANT ALL PRIVILEGES ON SCHEMA s TO APPLICATION app`,
	)
	assertInvalid(t, (*Validator).ParseGrantPrivsToApplication,
		``,
		`GRANT USAGE ON DATABASE d TO ROLE r`,
		`GRANT TO APPLICATION app`,
	)
}

func TestParseGrantPrivsToApplicationRole(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantPrivsToApplicationRole,
		`GRANT USAGE ON SCHEMA s TO APPLICATION ROLE ar`,
		`GRANT SELECT ON TABLE t TO APPLICATION ROLE ar WITH GRANT OPTION`,
		`GRANT ALL PRIVILEGES ON SCHEMA s TO APPLICATION ROLE my_role`,
	)
	assertInvalid(t, (*Validator).ParseGrantPrivsToApplicationRole,
		``,
		`GRANT USAGE ON SCHEMA s TO APPLICATION app`,
		`GRANT TO APPLICATION ROLE ar`,
	)
}

func TestParseGrantPrivToShare(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantPrivToShare,
		`GRANT USAGE ON DATABASE d TO SHARE sh`,
		`GRANT SELECT ON TABLE t TO SHARE my_share`,
		`GRANT REFERENCE_USAGE ON DATABASE d TO SHARE sh`,
	)
	assertInvalid(t, (*Validator).ParseGrantPrivToShare,
		``,
		`GRANT USAGE ON DATABASE d TO ROLE r`,
		`GRANT TO SHARE sh`,
	)
}

func TestParseGrantPrivsToUser(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantPrivsToUser,
		`GRANT AUDIT ON ACCOUNT TO USER u`,
		`GRANT IMPORT SHARE ON ACCOUNT TO u WITH GRANT OPTION`,
		`GRANT SELECT ON TABLE t TO USER my_user`,
	)
	assertInvalid(t, (*Validator).ParseGrantPrivsToUser,
		``,
		`GRANT TO USER u`,
		`REVOKE SELECT ON TABLE t FROM USER u`,
	)
}

func TestParseGrantRole(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantRole,
		`GRANT ROLE r TO ROLE parent`,
		`GRANT ROLE analyst TO USER alice`,
		`GRANT ROLE "R" TO ROLE "P"`,
	)
	assertInvalid(t, (*Validator).ParseGrantRole,
		``,
		`GRANT ROLE r TO SHARE sh`,
		`GRANT ROLE r TO`,
	)
}

func TestParseGrantServiceRole(t *testing.T) {
	assertValid(t, (*Validator).ParseGrantServiceRole,
		`GRANT SERVICE ROLE sr TO ROLE r`,
		`GRANT SERVICE ROLE sr TO APPLICATION ROLE ar`,
		`GRANT SERVICE ROLE sr TO DATABASE ROLE dr`,
	)
	assertInvalid(t, (*Validator).ParseGrantServiceRole,
		``,
		`GRANT SERVICE ROLE sr TO USER u`,
		`GRANT ROLE r TO ROLE p`,
	)
}

func TestParseRevokeApplicationRole(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokeApplicationRole,
		`REVOKE APPLICATION ROLE ar FROM ROLE r`,
		`REVOKE APPLICATION ROLE ar FROM APPLICATION ROLE other`,
		`REVOKE APPLICATION ROLE ar FROM APPLICATION app`,
	)
	assertInvalid(t, (*Validator).ParseRevokeApplicationRole,
		``,
		`GRANT APPLICATION ROLE ar TO ROLE r`,
		`REVOKE APPLICATION ROLE ar FROM`,
	)
}

func TestParseRevokeCaller(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokeCaller,
		`REVOKE CALLER SELECT ON TABLE t FROM ROLE r`,
		`REVOKE ALL CALLER PRIVILEGES ON TABLE t FROM DATABASE ROLE dr`,
		`REVOKE INHERITED CALLER SELECT ON ALL TABLES IN SCHEMA s FROM ROLE r`,
	)
	assertInvalid(t, (*Validator).ParseRevokeCaller,
		``,
		`REVOKE SELECT ON TABLE t FROM ROLE r`,
		`REVOKE CALLER SELECT ON TABLE t FROM`,
	)
}

func TestParseRevokeDatabaseRole(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokeDatabaseRole,
		`REVOKE DATABASE ROLE dr FROM ROLE r`,
		`REVOKE DATABASE ROLE dr FROM DATABASE ROLE other`,
		`REVOKE DATABASE ROLE dr FROM APPLICATION app`,
	)
	assertInvalid(t, (*Validator).ParseRevokeDatabaseRole,
		``,
		`REVOKE DATABASE ROLE dr FROM SHARE sh`,
		`GRANT DATABASE ROLE dr TO ROLE r`,
	)
}

func TestParseRevokeDatabaseRoleFromShare(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokeDatabaseRoleFromShare,
		`REVOKE DATABASE ROLE dr FROM SHARE sh`,
		`REVOKE DATABASE ROLE db.dr FROM SHARE my_share`,
		`REVOKE DATABASE ROLE "DR" FROM SHARE "SH"`,
	)
	assertInvalid(t, (*Validator).ParseRevokeDatabaseRoleFromShare,
		``,
		`REVOKE DATABASE ROLE dr FROM ROLE r`,
		`REVOKE DATABASE ROLE dr FROM SHARE`,
	)
}

func TestParseRevokePrivsFromRole(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokePrivsFromRole,
		`REVOKE SELECT ON TABLE t FROM ROLE r`,
		`REVOKE GRANT OPTION FOR ALL PRIVILEGES ON SCHEMA s FROM ROLE r CASCADE`,
		`REVOKE USAGE ON DATABASE d FROM DATABASE ROLE dr RESTRICT`,
	)
	assertInvalid(t, (*Validator).ParseRevokePrivsFromRole,
		``,
		`REVOKE FROM ROLE r`,
		`GRANT SELECT ON TABLE t TO ROLE r`,
	)
}

func TestParseRevokePrivsFromApplication(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokePrivsFromApplication,
		`REVOKE USAGE ON DATABASE d FROM APPLICATION app`,
		`REVOKE GRANT OPTION FOR SELECT ON TABLE t FROM APPLICATION app CASCADE`,
		`REVOKE ALL PRIVILEGES ON SCHEMA s FROM APPLICATION my_app`,
	)
	assertInvalid(t, (*Validator).ParseRevokePrivsFromApplication,
		``,
		`REVOKE USAGE ON DATABASE d FROM ROLE r`,
		`REVOKE FROM APPLICATION app`,
	)
}

func TestParseRevokePrivsFromApplicationRole(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokePrivsFromApplicationRole,
		`REVOKE USAGE ON SCHEMA s FROM APPLICATION ROLE ar`,
		`REVOKE GRANT OPTION FOR SELECT ON TABLE t FROM APPLICATION ROLE ar RESTRICT`,
		`REVOKE ALL PRIVILEGES ON SCHEMA s FROM APPLICATION ROLE my_role`,
	)
	assertInvalid(t, (*Validator).ParseRevokePrivsFromApplicationRole,
		``,
		`REVOKE USAGE ON SCHEMA s FROM APPLICATION app`,
		`REVOKE FROM APPLICATION ROLE ar`,
	)
}

func TestParseRevokePrivFromShare(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokePrivFromShare,
		`REVOKE USAGE ON DATABASE d FROM SHARE sh`,
		`REVOKE SELECT ON TABLE t FROM SHARE my_share`,
		`REVOKE REFERENCE_USAGE ON DATABASE d FROM SHARE sh`,
	)
	assertInvalid(t, (*Validator).ParseRevokePrivFromShare,
		``,
		`REVOKE USAGE ON DATABASE d FROM ROLE r`,
		`REVOKE FROM SHARE sh`,
	)
}

func TestParseRevokePrivsFromUser(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokePrivsFromUser,
		`REVOKE AUDIT ON ACCOUNT FROM USER u`,
		`REVOKE GRANT OPTION FOR IMPORT SHARE ON ACCOUNT FROM u CASCADE`,
		`REVOKE SELECT ON TABLE t FROM USER my_user RESTRICT`,
	)
	assertInvalid(t, (*Validator).ParseRevokePrivsFromUser,
		``,
		`REVOKE FROM USER u`,
		`GRANT SELECT ON TABLE t TO USER u`,
	)
}

func TestParseRevokeRole(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokeRole,
		`REVOKE ROLE r FROM ROLE parent`,
		`REVOKE ROLE analyst FROM USER alice`,
		`REVOKE ROLE "R" FROM ROLE "P"`,
	)
	assertInvalid(t, (*Validator).ParseRevokeRole,
		``,
		`REVOKE ROLE r FROM SHARE sh`,
		`REVOKE ROLE r FROM`,
	)
}

func TestParseRevokeServiceRole(t *testing.T) {
	assertValid(t, (*Validator).ParseRevokeServiceRole,
		`REVOKE SERVICE ROLE sr FROM ROLE r`,
		`REVOKE SERVICE ROLE sr FROM APPLICATION ROLE ar`,
		`REVOKE SERVICE ROLE sr FROM DATABASE ROLE dr`,
	)
	assertInvalid(t, (*Validator).ParseRevokeServiceRole,
		``,
		`REVOKE SERVICE ROLE sr FROM USER u`,
		`REVOKE ROLE r FROM ROLE p`,
	)
}
