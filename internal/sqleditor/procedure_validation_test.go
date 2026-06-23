package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateDataTypes_ProcedureMarkerPrecision(t *testing.T) {
	t.Run("Invalid parameter type has precise column position", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning")
		}
		w := warnings[0]
		if w.StartLineNumber != 1 {
			t.Errorf("Expected StartLineNumber=1, got %d", w.StartLineNumber)
		}
		// "BADTYPE" starts at column 28 (after "CREATE PROCEDURE my_proc(a ")
		expectedCol := strings.Index(sql, "BADTYPE") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
		if w.EndColumn != expectedCol+len("BADTYPE") {
			t.Errorf("Expected EndColumn=%d, got %d", expectedCol+len("BADTYPE"), w.EndColumn)
		}
	})

	t.Run("Invalid return type has precise column position", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS BADRETURN LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for invalid return type")
		}
		w := warnings[0]
		expectedCol := strings.Index(sql, "BADRETURN") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
		if w.EndColumn != expectedCol+len("BADRETURN") {
			t.Errorf("Expected EndColumn=%d, got %d", expectedCol+len("BADRETURN"), w.EndColumn)
		}
	})

	t.Run("Multiline procedure invalid param type has correct line number", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(\n  a INT,\n  b FAKETYPE\n) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning")
		}
		w := warnings[0]
		if w.StartLineNumber != 3 {
			t.Errorf("Expected StartLineNumber=3 for param on third line, got %d", w.StartLineNumber)
		}
	})

	t.Run("Procedure in second statement has correct offset", func(t *testing.T) {
		sql := "SELECT 1;\nCREATE PROCEDURE my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning in second statement")
		}
		w := warnings[0]
		if w.StartLineNumber != 2 {
			t.Errorf("Expected StartLineNumber=2 for procedure in second statement, got %d", w.StartLineNumber)
		}
	})

	t.Run("Multiple invalid params produce distinct markers", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a BADONE, b BADTWO) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) < 2 {
			t.Fatalf("Expected at least 2 data type warnings, got %d", len(warnings))
		}
		// Each marker should have a distinct StartColumn
		if warnings[0].StartColumn == warnings[1].StartColumn {
			t.Errorf("Expected distinct column positions for two invalid params, both got col %d", warnings[0].StartColumn)
		}
	})

	t.Run("Single-token parameter (name only, no type) does not crash", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		// processColumnDef requires >= 2 tokens; single-token param should be silently skipped.
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(w.Message, "a") && strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				t.Errorf("Single-token parameter name should not be validated as a type, got: %s", w.Message)
			}
		}
	})

	t.Run("Quoted parameter type is not validated", func(t *testing.T) {
		sql := `CREATE PROCEDURE my_proc(a "BADTYPE") RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") {
				t.Errorf("Quoted type should not be validated, got: %s", w.Message)
			}
		}
	})

	t.Run("Parameter DEFAULT with comma in string does not split incorrectly", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a VARCHAR DEFAULT 'x,y', b INT) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		// Both VARCHAR and INT are valid; no warnings expected.
		if len(warnings) > 0 {
			t.Errorf("Expected 0 data type warnings, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Parameter with block comment between name and type", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a /* comment */ BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE after block comment, not found")
		}
	})

	t.Run("Parameter with line comment between name and type", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(\n  a -- my param\n  BADTYPE\n) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE after line comment, not found")
		}
	})

	t.Run("Invalid return type on different line has correct line number", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc()\nRETURNS BADRETURN\nLANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for invalid return type")
		}
		w := warnings[0]
		if w.StartLineNumber != 2 {
			t.Errorf("Expected StartLineNumber=2 for return type on second line, got %d", w.StartLineNumber)
		}
	})

	t.Run("Parameter with NOT NULL constraint validates type correctly", func(t *testing.T) {
		// NOT NULL after the type should not confuse type extraction;
		// processColumnDef takes the second token as the type name.
		sql := "CREATE PROCEDURE my_proc(a BADTYPE NOT NULL) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") &&
				strings.Contains(strings.ToLower(w.Message), "badtype") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE with NOT NULL constraint, not found")
		}
	})

	t.Run("Parameterized invalid return type has correct column span", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS BADTYPE(10) LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for invalid parameterized return type")
		}
		w := warnings[0]
		expectedCol := strings.Index(sql, "BADTYPE") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
		// EndColumn should span only "BADTYPE", not "(10)"
		if w.EndColumn != expectedCol+len("BADTYPE") {
			t.Errorf("Expected EndColumn=%d, got %d", expectedCol+len("BADTYPE"), w.EndColumn)
		}
	})

	t.Run("Multiple invalid params on different lines have correct line numbers", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(\n  a BADONE,\n  b BADTWO\n) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) < 2 {
			t.Fatalf("Expected at least 2 data type warnings, got %d", len(warnings))
		}

		foundLine2 := false
		foundLine3 := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badone") && w.StartLineNumber == 2 {
				foundLine2 = true
			}
			if strings.Contains(strings.ToLower(w.Message), "badtwo") && w.StartLineNumber == 3 {
				foundLine3 = true
			}
		}
		if !foundLine2 {
			t.Error("Expected BADONE warning on line 2, not found")
		}
		if !foundLine3 {
			t.Error("Expected BADTWO warning on line 3, not found")
		}
	})

	t.Run("Unclosed paren produces no data type markers at all", func(t *testing.T) {
		// extractBalancedBlockPat returns "" for unclosed parens, so
		// no parameter type markers are produced (graceful failure).
		sql := "CREATE PROCEDURE my_proc(a BADTYPE, b ALSOBAD RETURNS VARCHAR LANGUAGE SQL AS $$ $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") ||
				strings.Contains(strings.ToLower(w.Message), "alsobad") {
				t.Errorf("Unclosed paren should produce no param type markers, got: %s", w.Message)
			}
		}
	})

	t.Run("Nested parens in DEFAULT do not break param type extraction", func(t *testing.T) {
		// extractBalancedBlockPat tracks paren depth, so COALESCE(NULL, 0)
		// must not prematurely close the parameter list. The second param
		// BADTYPE should still be validated.
		sql := "CREATE PROCEDURE my_proc(a NUMBER DEFAULT COALESCE(NULL, 0), b BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE after nested-paren DEFAULT, not found")
		}
	})

	t.Run("Duplicate RETURNS with second type invalid is caught by ValidateDataTypes", func(t *testing.T) {
		// reReturnsType uses FindAllStringSubmatchIndex, so both RETURNS types
		// are independently validated. The first (VARCHAR) is valid; the second
		// (BADRET) must still produce a warning.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR RETURNS BADRET LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badret") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADRET in second RETURNS clause, not found")
		}
	})

	t.Run("Trailing comma in parameter list does not produce false type warnings", func(t *testing.T) {
		// parseColumnDefs splits by commas; the trailing comma produces an empty
		// segment which processColumnDef must silently skip (0 tokens).
		sql := "CREATE PROCEDURE my_proc(a INT, b VARCHAR,) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 data type warnings for trailing comma, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Closing paren in single-quoted DEFAULT does not truncate param list", func(t *testing.T) {
		// extractBalancedBlockPat respects single-quoted strings, so ')' inside
		// a DEFAULT string literal must not close the parameter list. The second
		// param BADTYPE should still be validated.
		sql := "CREATE PROCEDURE my_proc(a VARCHAR DEFAULT ')', b BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE after paren-in-string DEFAULT, not found")
		}
	})

	t.Run("RETURNS TABLE does not produce unknown data type TABLE warning", func(t *testing.T) {
		// ValidateDataTypes filters out TABLE from RETURNS TABLE(...) to avoid
		// a false "Unknown data type 'TABLE'" warning.
		sql := "CREATE PROCEDURE my_proc() RETURNS TABLE(id INT, name VARCHAR) LANGUAGE SQL AS $$ BEGIN RETURN TABLE(SELECT 1, 'a'); END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "'table'") {
				t.Errorf("RETURNS TABLE should not produce unknown data type TABLE warning, got: %s", w.Message)
			}
		}
	})

	t.Run("RETURNS NULL ON NULL INPUT does not produce unknown data type NULL warning", func(t *testing.T) {
		// ValidateDataTypes filters out NULL from RETURNS NULL ON NULL INPUT to
		// avoid a false "Unknown data type 'NULL'" warning.
		sql := "CREATE PROCEDURE my_proc(a NUMBER) RETURNS VARCHAR LANGUAGE SQL RETURNS NULL ON NULL INPUT AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "'null'") {
				t.Errorf("RETURNS NULL ON NULL INPUT should not produce unknown data type NULL warning, got: %s", w.Message)
			}
		}
	})

	t.Run("RETURNS inside dollar-quoted body is not flagged", func(t *testing.T) {
		// The tokenizer treats the $$...$$ body as a single DollarQuoted token,
		// so the "RETURNS" inside it is never seen as a keyword. (The old
		// regex-based validator scanned rawText and falsely flagged it.)
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ RETURNS BADTYPE $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") {
				t.Errorf("RETURNS BADTYPE inside dollar-quoted body must not be flagged, got: %s", w.Message)
			}
		}
	})

	t.Run("cast shorthand inside dollar-quoted body is not flagged", func(t *testing.T) {
		// ::BADTYPE inside the $$...$$ body must not be flagged; the body is a
		// single token, not scanned for casts.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE JAVASCRIPT AS $$ var x = y::BADTYPE; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "badtype") {
				t.Errorf("::BADTYPE inside dollar-quoted body must not be flagged, got: %s", w.Message)
			}
		}
	})

	t.Run("RETURNS type on separate line has correct line number", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc()\nRETURNS BADRETURN\nLANGUAGE SQL\nAS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for invalid return type on second line")
		}
		w := warnings[0]
		if w.StartLineNumber != 2 {
			t.Errorf("Expected StartLineNumber=2 for RETURNS on second line, got %d", w.StartLineNumber)
		}
		// "BADRETURN" starts at column 9 on line 2 (after "RETURNS ")
		if w.StartColumn != 9 {
			t.Errorf("Expected StartColumn=9, got %d", w.StartColumn)
		}
	})

	t.Run("ValidateDataTypes catches invalid params in both multi-statement procedures", func(t *testing.T) {
		sql := "CREATE PROCEDURE p1(a BADONE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE PROCEDURE p2(b BADTWO) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		foundBadOne := false
		foundBadTwo := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "badone") {
				foundBadOne = true
			}
			if strings.Contains(msg, "badtwo") {
				foundBadTwo = true
			}
		}
		if !foundBadOne {
			t.Error("Expected unknown data type warning for BADONE in first procedure, not found")
		}
		if !foundBadTwo {
			t.Error("Expected unknown data type warning for BADTWO in second procedure, not found")
		}
	})

	t.Run("ValidateDataTypes catches invalid return types in both multi-statement procedures", func(t *testing.T) {
		sql := "CREATE PROCEDURE p1() RETURNS BADRET1 LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$;\nCREATE PROCEDURE p2() RETURNS BADRET2 LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		foundRet1 := false
		foundRet2 := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "badret1") {
				foundRet1 = true
			}
			if strings.Contains(msg, "badret2") {
				foundRet2 = true
			}
		}
		if !foundRet1 {
			t.Error("Expected unknown data type warning for BADRET1 in first procedure, not found")
		}
		if !foundRet2 {
			t.Error("Expected unknown data type warning for BADRET2 in second procedure, not found")
		}
	})

	t.Run("ValidateDataTypes produces zero markers for valid-only procedure", func(t *testing.T) {
		// Explicit check that no false positives are produced when all
		// parameter types and the return type are valid Snowflake types.
		sql := "CREATE PROCEDURE my_proc(a INT, b VARCHAR, c NUMBER(10,2), d BOOLEAN) RETURNS VARCHAR(255) LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 data type warnings for valid-only procedure, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("ValidateDataTypes produces zero markers for procedure with no parameters", func(t *testing.T) {
		// Empty parameter list must not produce any data type markers.
		sql := "CREATE PROCEDURE my_proc() RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 data type warnings for no-param procedure, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("First procedure invalid type does not affect second procedure valid types", func(t *testing.T) {
		// ValidateDataTypes must process each statement independently;
		// BADTYPE in p1 must not leak warnings into p2.
		sql := "CREATE PROCEDURE p1(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$;\nCREATE PROCEDURE p2(a INT, b VARCHAR) RETURNS NUMBER LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		// Exactly one warning expected: BADTYPE in p1.
		typeCount := 0
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				typeCount++
				if !strings.Contains(strings.ToLower(w.Message), "badtype") {
					t.Errorf("Expected data type warning for BADTYPE only, got: %s", w.Message)
				}
				if w.StartLineNumber != 1 {
					t.Errorf("Expected BADTYPE warning on line 1, got line %d", w.StartLineNumber)
				}
			}
		}
		if typeCount != 1 {
			t.Errorf("Expected exactly 1 unknown data type warning, got %d", typeCount)
		}
	})

	t.Run("Schema-qualified 3-part name with invalid param has correct marker", func(t *testing.T) {
		// reCreateProcExt uses _identPath which supports up to 3-part names.
		// Verify that parameter type validation still works for 3-part names
		// and markers have correct positions.
		sql := "CREATE PROCEDURE mydb.myschema.my_proc(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for schema-qualified name with BADTYPE")
		}
		w := warnings[0]
		expectedCol := strings.Index(sql, "BADTYPE") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
		if w.EndColumn != expectedCol+len("BADTYPE") {
			t.Errorf("Expected EndColumn=%d, got %d", expectedCol+len("BADTYPE"), w.EndColumn)
		}
	})

	t.Run("Interspersed valid and invalid params all invalid caught", func(t *testing.T) {
		// Four parameters: valid, invalid, valid, invalid. Both invalid types
		// must be independently flagged with correct positions.
		sql := "CREATE PROCEDURE my_proc(a INT, b BADONE, c VARCHAR, d BADTWO) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)

		foundBadOne := false
		foundBadTwo := false
		for _, w := range warnings {
			msg := strings.ToLower(w.Message)
			if strings.Contains(msg, "badone") {
				foundBadOne = true
			}
			if strings.Contains(msg, "badtwo") {
				foundBadTwo = true
			}
		}
		if !foundBadOne {
			t.Error("Expected unknown data type warning for BADONE, not found")
		}
		if !foundBadTwo {
			t.Error("Expected unknown data type warning for BADTWO, not found")
		}
	})

	t.Run("Quoted 3-part name with invalid param type still validated", func(t *testing.T) {
		// reCreateProcExt supports quoted identifiers in the path. Verify
		// parameter types are validated for fully-quoted 3-part names.
		sql := `CREATE PROCEDURE "MY_DB"."MY_SCHEMA"."MY_PROC"(a BADTYPE) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$`
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		found := false
		for _, w := range warnings {
			if strings.Contains(strings.ToLower(w.Message), "unknown data type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected unknown data type warning for BADTYPE with quoted 3-part name, not found")
		}
	})

	t.Run("Lowercase invalid param type produces marker with correct position", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a badtype) RETURNS VARCHAR LANGUAGE SQL AS $$ BEGIN RETURN 'ok'; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for lowercase badtype")
		}
		w := warnings[0]
		expectedCol := strings.Index(sql, "badtype") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
		if w.EndColumn != expectedCol+len("badtype") {
			t.Errorf("Expected EndColumn=%d, got %d", expectedCol+len("badtype"), w.EndColumn)
		}
	})

	t.Run("Lowercase valid params produce zero data type markers", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc(a int, b varchar, c float) RETURNS number LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) > 0 {
			t.Errorf("Expected 0 data type warnings for lowercase valid types, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("Mixed-case invalid return type produces marker", func(t *testing.T) {
		sql := "CREATE PROCEDURE my_proc() RETURNS BadReturn LANGUAGE SQL AS $$ BEGIN RETURN 1; END; $$"
		ranges := GetStatementRanges(sql)
		markers := ValidateDataTypes(sql, ranges)
		warnings := getWarnings(markers)
		if len(warnings) == 0 {
			t.Fatal("Expected at least one data type warning for mixed-case BadReturn")
		}
		w := warnings[0]
		expectedCol := strings.Index(sql, "BadReturn") + 1
		if w.StartColumn != expectedCol {
			t.Errorf("Expected StartColumn=%d, got %d", expectedCol, w.StartColumn)
		}
	})
}
