package sqlgrammar

import "testing"

// Grammar fixes for issue #793 — valid Snowflake syntax the grammar rejected.

// D7: MERGE accepts an optional target alias ([AS] <alias>) before USING.
func TestIssue793_MergeTargetAlias(t *testing.T) {
	assertValid(t, (*Validator).ParseMerge,
		"MERGE INTO orders o USING customers c ON o.customer_id = c.id WHEN MATCHED THEN UPDATE SET o.amount = 0",
		"MERGE INTO orders AS o USING customers c ON o.customer_id = c.id WHEN MATCHED THEN DELETE",
		// The unaliased form must still parse (alias slot is optional).
		"MERGE INTO orders USING customers ON orders.customer_id = customers.id WHEN MATCHED THEN DELETE",
		// Qualified target with alias.
		"MERGE INTO mydb.public.orders o USING customers c ON o.id = c.id WHEN MATCHED THEN DELETE",
	)
}

// F: set operators accept the GA-2025 `BY NAME` modifier.
func TestIssue793_UnionByName(t *testing.T) {
	assertValid(t, (*Validator).ParseSelect,
		"SELECT a, b FROM t UNION ALL BY NAME SELECT b, a FROM u",
		"SELECT a FROM t UNION BY NAME SELECT a FROM u",
		"SELECT a FROM t INTERSECT BY NAME SELECT a FROM u",
		// Plain forms still parse.
		"SELECT a FROM t UNION ALL SELECT a FROM u",
		"SELECT a FROM t UNION SELECT a FROM u",
	)
}

// B17: a dangling USE (object-type keyword with no name) is incomplete and must
// not be accepted (the bare form previously swallowed DATABASE/ROLE/… as a name).
func TestIssue793_B17_UseDangling(t *testing.T) {
	assertInvalid(t, (*Validator).ParseUse, "USE DATABASE", "USE ROLE", "USE WAREHOUSE", "USE SCHEMA")
	// Named forms and a quoted object literally named DATABASE still parse.
	assertValid(t, (*Validator).ParseUse,
		"USE mydb", "USE DATABASE mydb", "USE SCHEMA myschema", "USE ROLE myrole",
		`USE "DATABASE"`,
	)
	// End to end via dispatch: no USE candidate accepts the dangling form.
	if topLevelOK("USE DATABASE") {
		t.Error("USE DATABASE should not fully parse through dispatch")
	}
}

// C: the documented LS/RM abbreviations are dispatched so their content is
// grammar-checked instead of passing silently.
func TestIssue793_C_LsRmDispatch(t *testing.T) {
	for _, sql := range []string{"LS @my_stage", "RM @my_stage/path/"} {
		if !New(sql).Recognized() {
			t.Errorf("%q should be recognized by the grammar dispatch", sql)
		}
		if !topLevelOK(sql) {
			t.Errorf("%q should parse", sql)
		}
	}
}

// F: CALL accepts the model-method form <model>!<method>(…).
func TestIssue793_CallModelMethod(t *testing.T) {
	assertValid(t, (*Validator).ParseCall,
		"CALL my_model!FORECAST(FORECASTING_PERIODS => 3)",
		"CALL my_db.my_schema.my_model!PREDICT(input_data => 1)",
		// Plain CALL still parses.
		"CALL my_proc(1, 2)",
		"CALL my_proc(1) INTO :result",
	)
}
