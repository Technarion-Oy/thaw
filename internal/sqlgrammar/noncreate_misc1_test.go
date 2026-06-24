package sqlgrammar

import "testing"

// --- transactions.go ---

func TestParseBegin(t *testing.T) {
	assertValid(t, (*Validator).ParseBegin,
		`BEGIN`,
		`BEGIN WORK`,
		`BEGIN TRANSACTION`,
		`BEGIN NAME my_txn`,
		`BEGIN TRANSACTION NAME my_txn`,
		`START TRANSACTION`,
		`START TRANSACTION NAME my_txn`,
	)
	assertInvalid(t, (*Validator).ParseBegin,
		``,
		`COMMIT`,
		`BEGIN NAME`,
		`START`,
	)
}

func TestParseCommit(t *testing.T) {
	assertValid(t, (*Validator).ParseCommit,
		`COMMIT`,
		`COMMIT WORK`,
	)
	assertInvalid(t, (*Validator).ParseCommit,
		``,
		`ROLLBACK`,
		`COMMIT NOW`,
	)
}

func TestParseRollback(t *testing.T) {
	assertValid(t, (*Validator).ParseRollback,
		`ROLLBACK`,
		`ROLLBACK WORK`,
	)
	assertInvalid(t, (*Validator).ParseRollback,
		``,
		`COMMIT`,
		`ROLLBACK NOW`,
	)
}

// --- session.go ---

func TestParseSet(t *testing.T) {
	assertValid(t, (*Validator).ParseSet,
		`SET v1 = 10`,
		`SET min_balance = 1000`,
		`SET (a, b) = (1, 2)`,
		`SET v = 'hello world'`,
	)
	assertInvalid(t, (*Validator).ParseSet,
		``,
		`UNSET v`,
		`SET v1`,
	)
}

func TestParseUnset(t *testing.T) {
	assertValid(t, (*Validator).ParseUnset,
		`UNSET v1`,
		`UNSET (a, b, c)`,
		`UNSET min_balance`,
	)
	assertInvalid(t, (*Validator).ParseUnset,
		``,
		`SET v1`,
		`UNSET`,
	)
}

func TestParseUse(t *testing.T) {
	assertValid(t, (*Validator).ParseUse,
		`USE mydb`,
		`USE ROLE myrole`,
		`USE WAREHOUSE wh1`,
		`USE DATABASE mydb`,
		`USE SCHEMA mydb.public`,
		`USE SECONDARY ROLES ALL`,
		`USE SECONDARY ROLES r1, r2`,
	)
	assertInvalid(t, (*Validator).ParseUse,
		``,
		`SET v`,
		`USE`,
	)
}

func TestParseUseDatabase(t *testing.T) {
	assertValid(t, (*Validator).ParseUseDatabase,
		`USE DATABASE mydb`,
		`USE mydb`,
		`USE DATABASE "My DB"`,
	)
	assertInvalid(t, (*Validator).ParseUseDatabase,
		``,
		`USE`,
		`SET x`,
	)
}

func TestParseUseRole(t *testing.T) {
	assertValid(t, (*Validator).ParseUseRole,
		`USE ROLE myrole`,
		`USE ROLE SYSADMIN`,
		`USE ROLE "My Role"`,
	)
	assertInvalid(t, (*Validator).ParseUseRole,
		``,
		`USE myrole`,
		`USE ROLE`,
	)
}

func TestParseUseSchema(t *testing.T) {
	assertValid(t, (*Validator).ParseUseSchema,
		`USE SCHEMA mydb.public`,
		`USE SCHEMA public`,
		`USE mydb.public`,
	)
	assertInvalid(t, (*Validator).ParseUseSchema,
		``,
		`USE`,
		`SET y`,
	)
}

func TestParseUseSecondaryRoles(t *testing.T) {
	assertValid(t, (*Validator).ParseUseSecondaryRoles,
		`USE SECONDARY ROLES ALL`,
		`USE SECONDARY ROLES NONE`,
		`USE SECONDARY ROLES r1, r2, r3`,
	)
	assertInvalid(t, (*Validator).ParseUseSecondaryRoles,
		``,
		`USE ROLE myrole`,
		`USE SECONDARY ROLES`,
	)
}

func TestParseUseWarehouse(t *testing.T) {
	assertValid(t, (*Validator).ParseUseWarehouse,
		`USE WAREHOUSE wh1`,
		`USE WAREHOUSE COMPUTE_WH`,
		`USE WAREHOUSE "My WH"`,
	)
	assertInvalid(t, (*Validator).ParseUseWarehouse,
		``,
		`USE wh1`,
		`USE WAREHOUSE`,
	)
}

// --- undrop.go ---

func TestParseUndropDatabase(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropDatabase,
		`UNDROP DATABASE mydb`,
		`UNDROP DATABASE my_db`,
		`UNDROP DATABASE "My DB"`,
	)
	assertInvalid(t, (*Validator).ParseUndropDatabase,
		``,
		`UNDROP TABLE t`,
		`UNDROP DATABASE`,
	)
}

func TestParseUndropDynamicTable(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropDynamicTable,
		`UNDROP DYNAMIC TABLE t1`,
		`UNDROP DYNAMIC TABLE db.sch.t1`,
		`UNDROP DYNAMIC TABLE "T"`,
	)
	assertInvalid(t, (*Validator).ParseUndropDynamicTable,
		``,
		`UNDROP TABLE t1`,
		`UNDROP DYNAMIC TABLE`,
	)
}

func TestParseUndropExternalVolume(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropExternalVolume,
		`UNDROP EXTERNAL VOLUME vol1`,
		`UNDROP EXTERNAL VOLUME my_vol`,
		`UNDROP EXTERNAL VOLUME "V"`,
	)
	assertInvalid(t, (*Validator).ParseUndropExternalVolume,
		``,
		`UNDROP VOLUME vol1`,
		`UNDROP EXTERNAL VOLUME`,
	)
}

func TestParseUndropIcebergTable(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropIcebergTable,
		`UNDROP ICEBERG TABLE t1`,
		`UNDROP ICEBERG TABLE db.sch.t1`,
		`UNDROP ICEBERG TABLE "T"`,
	)
	assertInvalid(t, (*Validator).ParseUndropIcebergTable,
		``,
		`UNDROP TABLE t1`,
		`UNDROP ICEBERG TABLE`,
	)
}

func TestParseUndropNotebook(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropNotebook,
		`UNDROP NOTEBOOK nb1`,
		`UNDROP NOTEBOOK db.sch.nb1`,
		`UNDROP NOTEBOOK "N"`,
	)
	assertInvalid(t, (*Validator).ParseUndropNotebook,
		``,
		`UNDROP TABLE nb1`,
		`UNDROP NOTEBOOK`,
	)
}

func TestParseUndropSchema(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropSchema,
		`UNDROP SCHEMA sch1`,
		`UNDROP SCHEMA mydb.sch1`,
		`UNDROP SCHEMA "S"`,
	)
	assertInvalid(t, (*Validator).ParseUndropSchema,
		``,
		`UNDROP TABLE sch1`,
		`UNDROP SCHEMA`,
	)
}

func TestParseUndropTable(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropTable,
		`UNDROP TABLE t1`,
		`UNDROP TABLE db.sch.t1`,
		`UNDROP TABLE "T"`,
	)
	assertInvalid(t, (*Validator).ParseUndropTable,
		``,
		`UNDROP VIEW t1`,
		`UNDROP TABLE`,
	)
}

func TestParseUndropView(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropView,
		`UNDROP VIEW v1`,
		`UNDROP VIEW db.sch.v1`,
		`UNDROP VIEW "V"`,
	)
	assertInvalid(t, (*Validator).ParseUndropView,
		``,
		`UNDROP TABLE v1`,
		`UNDROP VIEW`,
	)
}

func TestParseUndropObj(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropObj,
		`UNDROP TABLE t1`,
		`UNDROP SCHEMA mydb.sch1`,
		`UNDROP DATABASE mydb`,
	)
	assertInvalid(t, (*Validator).ParseUndropObj,
		``,
		`DROP TABLE t1`,
		`UNDROP`,
	)
}

func TestParseUndropAccount(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropAccount,
		`UNDROP ACCOUNT acct1`,
		`UNDROP ACCOUNT my_account`,
		`UNDROP ACCOUNT "A"`,
	)
	assertInvalid(t, (*Validator).ParseUndropAccount,
		``,
		`UNDROP TABLE acct1`,
		`UNDROP ACCOUNT`,
	)
}

func TestParseUndropSnapshot(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropSnapshot,
		`UNDROP SNAPSHOT snap1`,
		`UNDROP SNAPSHOT IDENTIFIER('snap1')`,
		`UNDROP SNAPSHOT snap1 RENAME TO snap2`,
	)
	assertInvalid(t, (*Validator).ParseUndropSnapshot,
		``,
		`UNDROP TABLE snap1`,
		`UNDROP SNAPSHOT`,
	)
}

func TestParseUndropStreamlit(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropStreamlit,
		`UNDROP STREAMLIT app1`,
		`UNDROP STREAMLIT db.sch.app1`,
		`UNDROP STREAMLIT "A"`,
	)
	assertInvalid(t, (*Validator).ParseUndropStreamlit,
		``,
		`UNDROP TABLE app1`,
		`UNDROP STREAMLIT`,
	)
}

func TestParseUndropTag(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropTag,
		`UNDROP TAG tag1`,
		`UNDROP TAG db.sch.tag1`,
		`UNDROP TAG "T"`,
	)
	assertInvalid(t, (*Validator).ParseUndropTag,
		``,
		`UNDROP TABLE tag1`,
		`UNDROP TAG`,
	)
}

func TestParseUndropType(t *testing.T) {
	assertValid(t, (*Validator).ParseUndropType,
		`UNDROP TYPE type1`,
		`UNDROP TYPE db.sch.type1`,
		`UNDROP TYPE "T"`,
	)
	assertInvalid(t, (*Validator).ParseUndropType,
		``,
		`UNDROP TABLE type1`,
		`UNDROP TYPE`,
	)
}
