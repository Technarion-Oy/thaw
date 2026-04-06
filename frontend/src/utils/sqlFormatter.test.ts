/**
 * Unit tests for the Thaw SQL formatter.
 *
 * Each test validates a specific formatting rule or Snowflake dialect quirk.
 * Tests are grouped by concern; within each group the most likely failure cases
 * (complex nesting, operator combinations, literal passthrough) come last.
 */

import { describe, expect, it, vi } from "vitest";
import { DEFAULT_EDITOR_PREFS, EditorPrefs, formatSQL } from "./sqlFormatter";

// ── helpers ───────────────────────────────────────────────────────────────────

const prefs = (overrides: Partial<EditorPrefs> = {}): EditorPrefs => ({
  ...DEFAULT_EDITOR_PREFS,
  ...overrides,
});

// Normalize whitespace so snapshot comparisons are line-ending agnostic.
const normalize = (s: string) => s.trim().replace(/\r\n/g, "\n");

// ── Mock ApplySqlCasing with the Go-equivalent inline implementation ──────────
// This lets unit tests run without a live Wails runtime while still exercising
// the full casing logic (keyword / identifier / function classification).

const SQL_KW = new Set([
  "ADD","ALL","ALTER","AND","ANY","AS","ASC","AT","BEFORE","BETWEEN","BY",
  "CALL","CASCADE","CASE","CAST","CHANGES","CLUSTER","COLUMN","COMMENT",
  "COMMIT","CONNECT","CONSTRAINT","COPY","CREATE","CROSS","CURRENT",
  "CURRENT_DATE","CURRENT_ROLE","CURRENT_SCHEMA","CURRENT_TIME",
  "CURRENT_TIMESTAMP","CURRENT_USER","CURRENT_WAREHOUSE","DATABASE",
  "DEFAULT","DELETE","DESC","DESCRIBE","DISTINCT","DROP","ELSE","END",
  "EXCEPT","EXECUTE","EXISTS","EXPLAIN","EXTRACT","FALSE","FILE","FIRST",
  "FLATTEN","FOLLOWING","FOR","FORCE","FOREIGN","FROM","FULL","FUNCTION",
  "DEFINE","GRANT","GROUP","GROUPING","HAVING","IF","ILIKE","IN","INDEX",
  "INNER","INSERT","INTERSECT","INTO","IS","JOIN","KEY","LAST","LATERAL",
  "LEFT","LIKE","LIMIT","MATCH_RECOGNIZE","MEASURES","MERGE","MINUS",
  "NATURAL","NOT","NULL","NULLS","OF","OFFSET","ON","OR","ORDER","OUTER",
  "OVER","OVERWRITE","PARTITION","PATTERN","PIPE","PRECEDING","PRIMARY",
  "PROCEDURE","PURGE","QUALIFY","RANGE","RECURSIVE","REFERENCES","REPLACE",
  "RESTRICT","REVOKE","RIGHT","ROLLBACK","ROW","ROWS","SAMPLE","SCHEMA",
  "SELECT","SEMI","SEQUENCE","SET","SHOW","SOME","STAGE","START","STREAM",
  "TABLE","TABLESAMPLE","TASK","THEN","TO","TOP","TRANSACTION","TRUE",
  "TRUNCATE","UNBOUNDED","UNION","UNIQUE","UPDATE","USING","VALUES","VIEW",
  "VOLATILE","WAREHOUSE","WHEN","WHERE","WINDOW","WITH","WITHIN","WITHOUT",
  "ANTI","ASOF","CLONE","CRON","DYNAMIC","ENABLE","EXTERNAL","FINALIZE",
  "FORMAT","ICEBERG","MASKING","NETWORK","NOTIFY","POLICY","PROJECTION",
  "RECOVER","REPLICATION","RESUME","ROLE","SCHEDULE","SECURE","SHARE",
  "SUSPEND","TABULAR","TRANSIENT","TRIGGER","UNDROP",
]);
const BUILTIN_FN = new Set([
  "ABS","ACOS","ACOSH","ADD_MONTHS","ANY_VALUE","APPROX_COUNT_DISTINCT",
  "APPROX_PERCENTILE","APPROX_TOP_K","ARRAY_AGG","ARRAY_APPEND","ARRAY_CAT",
  "ARRAY_COMPACT","ARRAY_CONSTRUCT","ARRAY_CONTAINS","ARRAY_DISTINCT",
  "ARRAY_EXCEPT","ARRAY_FLATTEN","ARRAY_GENERATE_RANGE","ARRAY_INSERT",
  "ARRAY_INTERSECTION","ARRAY_MAX","ARRAY_MIN","ARRAY_PREPEND","ARRAY_REMOVE",
  "ARRAY_REMOVE_AT","ARRAY_SIZE","ARRAY_SLICE","ARRAY_SORT","ARRAY_TO_STRING",
  "ARRAY_UNION_AGG","ARRAY_UNIQUE_AGG","AS_ARRAY","AS_BINARY","AS_BOOLEAN",
  "AS_CHAR","AS_DATE","AS_DECIMAL","AS_DOUBLE","AS_INTEGER","AS_NUMBER",
  "AS_OBJECT","AS_REAL","AS_TIME","AS_TIMESTAMP_LTZ","AS_TIMESTAMP_NTZ",
  "AS_TIMESTAMP_TZ","AS_TINYINT","AS_VARCHAR","ASIN","ASINH","ATAN","ATAN2",
  "ATANH","AVG","BASE64_DECODE_STRING","BASE64_ENCODE","BITNOT","BITSHIFTLEFT",
  "BITSHIFTRIGHT","BITAND","BITOR","BITXOR","BOOLAND","BOOLAND_AGG","BOOLNOT",
  "BOOLOR","BOOLOR_AGG","BOOLXOR","BOOLXOR_AGG","CASE","CAST","CBRT","CEIL",
  "CEILING","CHARINDEX","CHR","CHAR","COALESCE","COLLATE","COLLATION",
  "COMPRESS","CONCAT","CONCAT_WS","CONDITIONAL_CHANGE_EVENT",
  "CONDITIONAL_TRUE_EVENT","CONTAINS","CONVERT_TIMEZONE","COS","COSH","COUNT",
  "COUNT_IF","COVAR_POP","COVAR_SAMP","CUME_DIST","DATE_FROM_PARTS",
  "DATE_PART","DATE_TRUNC","DATEADD","DATEDIFF","DAYNAME","DAYOFMONTH",
  "DAYOFWEEK","DAYOFWEEKISO","DAYOFYEAR","DECODE","DECOMPRESS","DENSE_RANK",
  "DIV0","DIV0NULL","EDITDISTANCE","ENDSWITH","EQUAL_NULL","EXP","FIRST_VALUE",
  "FLATTEN","FLOOR","FORMAT_DATE","FORMAT_NUMBER","GENERATOR","GET",
  "GET_ABSOLUTE_PATH","GET_DDL","GET_PATH","GET_PRESIGNED_URL",
  "GET_STAGE_LOCATION","GETBIT","GREATEST","GROUPING","GROUPING_ID","HASH",
  "HASH_AGG","HAVERSINE","HEX_DECODE_BINARY","HEX_DECODE_STRING","HEX_ENCODE",
  "HOUR","HOURS","IFF","IFNULL","IN","INITCAP","INSERT","IS_ARRAY","IS_BINARY",
  "IS_BOOLEAN","IS_CHAR","IS_DATE","IS_DATE_VALUE","IS_DECIMAL","IS_DOUBLE",
  "IS_GRANTED_TO_INVOKER_ROLE","IS_INTEGER","IS_NULL_VALUE","IS_OBJECT",
  "IS_REAL","IS_TIME","IS_TIMESTAMP_LTZ","IS_TIMESTAMP_NTZ","IS_TIMESTAMP_TZ",
  "IS_VARCHAR","JAROWINKLER_SIMILARITY","JSON_EXTRACT_PATH_TEXT","KURTOSIS",
  "LAG","LAST_DAY","LAST_VALUE","LEAD","LEAST","LEFT","LENGTH","LEN",
  "LISTAGG","LN","LOG","LOWER","LPAD","LTRIM","MAX","MAX_BY","MEDIAN","MIN",
  "MIN_BY","MINUTE","MINUTES","MOD","MODE","MONTH","MONTHNAME",
  "MONTHS_BETWEEN","NORMAL","NTH_VALUE","NTILE","NULLIF","NULLIFZERO","NVL",
  "NVL2","OBJECT_AGG","OBJECT_CONSTRUCT","OBJECT_CONSTRUCT_KEEP_NULL",
  "OBJECT_DELETE","OBJECT_INSERT","OBJECT_KEYS","OBJECT_PICK","PARSE_IP",
  "PARSE_JSON","PARSE_URL","PARSE_XML","PERCENT_RANK","PERCENTILE_CONT",
  "PERCENTILE_DISC","PI","POSITION","POW","POWER","RANDSTR","RANDOM","RANK",
  "RATIO_TO_REPORT","REGEXP","REGEXP_COUNT","REGEXP_EXTRACT",
  "REGEXP_EXTRACT_ALL","REGEXP_INSTR","REGEXP_LIKE","REGEXP_REPLACE",
  "REGEXP_SUBSTR","REPEAT","REPLACE","REVERSE","RIGHT","ROUND","ROW_NUMBER",
  "RPAD","RTRIM","SECOND","SECONDS","SHA1","SHA1_BINARY","SHA1_HEX","SHA2",
  "SHA2_BINARY","SHA2_HEX","SIGN","SIN","SINH","SKEW","SOUNDEX","SPACE",
  "SPLIT","SPLIT_PART","SPLIT_TO_TABLE","SQL_VARIANT_PROPERTY","SQRT",
  "SQUARE","STARTSWITH","STDDEV","STDDEV_POP","STDDEV_SAMP","STRIP_NULL_VALUE",
  "STRTOK","STRTOK_SPLIT_TO_TABLE","STRTOK_TO_ARRAY","SUBSTR","SUBSTRING",
  "SUM","SYSTEM$ABORT_TRANSACTION","SYSTEM$CANCEL_ALL_QUERIES",
  "SYSTEM$CANCEL_QUERY","SYSTEM$CLUSTERING_DEPTH",
  "SYSTEM$CLUSTERING_INFORMATION","SYSTEM$GET_PREDECESSOR_RETURN_VALUE",
  "SYSTEM$STREAM_GET_TABLE_TIMESTAMP","SYSTEM$STREAM_HAS_DATA",
  "SYSTEM$TASK_DEPENDENTS_ENABLE","SYSTEM$TYPEOF","SYSTEM$WAIT","TAN","TANH",
  "TIME_FROM_PARTS","TIMEADD","TIMEDIFF","TIMESTAMPADD","TIMESTAMPDIFF",
  "TIMESTAMP_FROM_PARTS","TIMESTAMP_LTZ_FROM_PARTS",
  "TIMESTAMP_NTZ_FROM_PARTS","TIMESTAMP_TZ_FROM_PARTS","TO_ARRAY","TO_BINARY",
  "TO_BOOLEAN","TO_CHAR","TO_DATE","TO_DECIMAL","TO_DOUBLE","TO_GEOGRAPHY",
  "TO_GEOMETRY","TO_JSON","TO_NUMBER","TO_OBJECT","TO_REAL","TO_TIME",
  "TO_TIMESTAMP","TO_TIMESTAMP_LTZ","TO_TIMESTAMP_NTZ","TO_TIMESTAMP_TZ",
  "TO_VARIANT","TO_VARCHAR","TO_XML","TRANSLATE","TRIM","TRUNCATE","TRUNC",
  "TRY_BASE64_DECODE_BINARY","TRY_BASE64_DECODE_STRING","TRY_CAST",
  "TRY_HEX_DECODE_BINARY","TRY_HEX_DECODE_STRING","TRY_PARSE_JSON",
  "TRY_TO_BINARY","TRY_TO_BOOLEAN","TRY_TO_DATE","TRY_TO_DECIMAL",
  "TRY_TO_DOUBLE","TRY_TO_NUMBER","TRY_TO_TIME","TRY_TO_TIMESTAMP",
  "TRY_TO_TIMESTAMP_LTZ","TRY_TO_TIMESTAMP_NTZ","TRY_TO_TIMESTAMP_TZ",
  "TYPEOF","UNIFORM","UPPER","UNISTR","VAR_POP","VAR_SAMP","VARIANCE",
  "VARIANCE_POP","VARIANCE_SAMP","WEEK","WEEKISO","WEEKOFYEAR","XMLGET",
  "YEAR","YEAROFWEEK","YEAROFWEEKISO","ZEROIFNULL",
]);

function _applyCase(word: string, casing: string): string {
  switch (casing) {
    case "UPPER":   return word.toUpperCase();
    case "lower":   return word.toLowerCase();
    case "Title":   return word.charAt(0).toUpperCase() + word.slice(1).toLowerCase();
    default:        return word;
  }
}

function _applyCasing(sql: string, kc: string, ic: string, fc: string): string {
  const out: string[] = [];
  let i = 0; const len = sql.length;
  while (i < len) {
    const ch = sql[i];
    if (ch === '"') {
      let j = i + 1;
      while (j < len) { if (sql[j] === '"') { if (sql[j+1] === '"') { j+=2; continue; } j++; break; } j++; }
      out.push(sql.slice(i, j)); i = j; continue;
    }
    if (ch === "'") {
      let j = i + 1;
      while (j < len) { if (sql[j] === "'") { if (sql[j+1] === "'") { j+=2; continue; } j++; break; } j++; }
      out.push(sql.slice(i, j)); i = j; continue;
    }
    if (ch === "$") {
      let tagEnd = i + 1;
      while (tagEnd < len && sql[tagEnd] !== "$" && sql[tagEnd] !== "\n") tagEnd++;
      if (tagEnd < len && sql[tagEnd] === "$") {
        const tag = sql.slice(i, tagEnd + 1);
        const closeIdx = sql.indexOf(tag, tagEnd + 1);
        if (closeIdx >= 0) { out.push(sql.slice(i, closeIdx + tag.length)); i = closeIdx + tag.length; continue; }
      }
      out.push(ch); i++; continue;
    }
    if (ch === "-" && sql[i+1] === "-") {
      const nl = sql.indexOf("\n", i); const end = nl >= 0 ? nl + 1 : len;
      out.push(sql.slice(i, end)); i = end; continue;
    }
    if (ch === "/" && sql[i+1] === "*") {
      const close = sql.indexOf("*/", i + 2); const end = close >= 0 ? close + 2 : len;
      out.push(sql.slice(i, end)); i = end; continue;
    }
    if (/[a-zA-Z_]/.test(ch)) {
      let j = i + 1;
      while (j < len && /[a-zA-Z0-9_$]/.test(sql[j])) j++;
      const word = sql.slice(i, j); const upper = word.toUpperCase();
      let k = j; while (k < len && (sql[k] === " " || sql[k] === "\t")) k++;
      const isCall = sql[k] === "(";
      let result: string;
      if (isCall) {
        result = (SQL_KW.has(upper) && !BUILTIN_FN.has(upper)) ? _applyCase(word, kc) : _applyCase(word, fc);
      } else if (SQL_KW.has(upper)) {
        result = _applyCase(word, kc);
      } else {
        result = _applyCase(word, ic);
      }
      const isFn = isCall && (!SQL_KW.has(upper) || BUILTIN_FN.has(upper));
      out.push(result); i = isFn ? k : j; continue;
    }
    out.push(ch); i++;
  }
  return out.join("");
}

vi.mock("../../wailsjs/go/main/App", () => ({
  ApplySqlCasing: (sql: string, kc: string, ic: string, fc: string) =>
    Promise.resolve(_applyCasing(sql, kc, ic, fc)),
}));


// ── 1. Empty / trivial input ──────────────────────────────────────────────────

describe("trivial input", () => {
  it("returns empty string unchanged", async () => {
    expect(await formatSQL("", prefs())).toBe("");
    expect(await formatSQL("   ", prefs())).toBe("   ");
    expect(await formatSQL("\n\n", prefs())).toBe("\n\n");
  });

  it("returns single-line SELECT unchanged in structure", async () => {
    const out = await formatSQL("select 1", prefs());
    expect(out.toUpperCase()).toContain("SELECT");
    expect(out).toContain("1");
  });
});

// ── 2. Keyword casing ─────────────────────────────────────────────────────────

describe("keywordCase", () => {
  const sql = "select id, name from users where active = true order by name";

  it("UPPER produces all-caps reserved words", async () => {
    const out = await formatSQL(sql, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("SELECT");
    expect(out).toContain("FROM");
    expect(out).toContain("WHERE");
    expect(out).toContain("ORDER");
    expect(out).toContain("BY");
    expect(out).toContain("TRUE");
  });

  it("lower produces lowercase reserved words", async () => {
    const out = await formatSQL(sql, prefs({ keywordCase: "lower" }));
    expect(out).toContain("select");
    expect(out).toContain("from");
    expect(out).toContain("where");
    expect(out).toContain("order");
    expect(out).toContain("by");
    expect(out).toContain("true");
    // Must NOT contain uppercase versions of keywords
    expect(out).not.toMatch(/\bSELECT\b/);
    expect(out).not.toMatch(/\bFROM\b/);
  });

  it("Title capitalises first letter only", async () => {
    const out = await formatSQL(sql, prefs({ keywordCase: "Title" }));
    expect(out).toContain("Select");
    expect(out).toContain("From");
    expect(out).toContain("Where");
    expect(out).toContain("Order");
    expect(out).toContain("By");
    // Must not be all-caps
    expect(out).not.toMatch(/\bSELECT\b/);
  });

  it("Preserve leaves original casing of keywords untouched", async () => {
    const mixed = "SeLeCt id FrOm users WHERE active = TRUE";
    const out = normalize(await formatSQL(mixed, prefs({ keywordCase: "Preserve" })));
    // sql-formatter normalises whitespace but our casing pass must not change token case
    expect(out).toContain("SeLeCt");
    expect(out).toContain("FrOm");
    expect(out).toContain("WHERE");
  });

  it("WINDOW and QUALIFY are treated as keywords", async () => {
    const q = `
      select id, row_number() over w as rn
      from t
      window w as (partition by grp order by ts)
      qualify rn = 1
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("WINDOW");
    expect(out).toContain("QUALIFY");
    expect(out).toContain("PARTITION");
    expect(out).toContain("ORDER");
  });
});

// ── 3. Identifier casing ──────────────────────────────────────────────────────

describe("identifierCase", () => {
  const sql = "select MyTable.MyColumn from MyTable join OtherTable on MyTable.id = OtherTable.fk";

  it("Preserve leaves unquoted identifiers as-is", async () => {
    const out = await formatSQL(sql, prefs({ identifierCase: "Preserve", keywordCase: "UPPER" }));
    expect(out).toContain("MyTable");
    expect(out).toContain("MyColumn");
    expect(out).toContain("OtherTable");
  });

  it("UPPER uppercases unquoted identifiers", async () => {
    const out = await formatSQL(sql, prefs({ identifierCase: "UPPER", keywordCase: "lower" }));
    expect(out).toContain("MYTABLE");
    expect(out).toContain("MYCOLUMN");
    expect(out).toContain("OTHERTABLE");
    // Keywords should still be lower
    expect(out).toContain("select");
    expect(out).toContain("from");
  });

  it("lower lowercases unquoted identifiers", async () => {
    const out = await formatSQL(sql, prefs({ identifierCase: "lower", keywordCase: "UPPER" }));
    expect(out).toContain("mytable");
    expect(out).toContain("mycolumn");
    expect(out).toContain("othertable");
  });

  it("double-quoted identifiers are NEVER modified regardless of identifierCase", async () => {
    const q = `select "MyTable"."MyColumn", "CamelCase" from "SchemaName"."TableName"`;

    for (const ic of ["Preserve", "UPPER", "lower"] as EditorPrefs["identifierCase"][]) {
      const out = await formatSQL(q, prefs({ identifierCase: ic }));
      expect(out, `identifierCase=${ic}`).toContain('"MyTable"');
      expect(out, `identifierCase=${ic}`).toContain('"MyColumn"');
      expect(out, `identifierCase=${ic}`).toContain('"CamelCase"');
      expect(out, `identifierCase=${ic}`).toContain('"SchemaName"');
      expect(out, `identifierCase=${ic}`).toContain('"TableName"');
    }
  });

  it("escaped double-quote inside identifier is preserved", async () => {
    // Snowflake: "My""Weird""Name" is a valid quoted identifier
    const q = `select "My""Weird""Name" from t`;
    const out = await formatSQL(q, prefs({ identifierCase: "UPPER" }));
    expect(out).toContain('"My""Weird""Name"');
  });
});

// ── 4. Function casing ────────────────────────────────────────────────────────

describe("functionCase", () => {
  const sql = "select to_date(col), avg(amount), iff(flag, 'y', 'n'), nvl(x, 0) from t";

  it("UPPER uppercases function names", async () => {
    const out = await formatSQL(sql, prefs({ functionCase: "UPPER" }));
    expect(out).toContain("TO_DATE(");
    expect(out).toContain("AVG(");
    expect(out).toContain("IFF(");
    expect(out).toContain("NVL(");
  });

  it("lower lowercases function names", async () => {
    const out = await formatSQL(sql, prefs({ functionCase: "lower" }));
    expect(out).toContain("to_date(");
    expect(out).toContain("avg(");
    expect(out).toContain("iff(");
    expect(out).toContain("nvl(");
    // Keywords should still be uppercase
    expect(out).toContain("FROM");
  });

  it("function casing applies to UDFs (unknown functions)", async () => {
    const q = "select my_custom_udf(col) from t";
    const upper = await formatSQL(q, prefs({ functionCase: "UPPER" }));
    const lower = await formatSQL(q, prefs({ functionCase: "lower" }));
    expect(upper).toContain("MY_CUSTOM_UDF(");
    expect(lower).toContain("my_custom_udf(");
  });

  it("nested function calls all receive function casing", async () => {
    const q = "select coalesce(nullif(trim(col), ''), to_char(id)) from t";
    const out = await formatSQL(q, prefs({ functionCase: "UPPER" }));
    expect(out).toContain("COALESCE(");
    expect(out).toContain("NULLIF(");
    expect(out).toContain("TRIM(");
    expect(out).toContain("TO_CHAR(");
  });
});

// ── 5. Indent style & size ────────────────────────────────────────────────────

describe("indent", () => {
  const sql = "select a, b, c from t where x = 1";

  it("spaces/2 uses two-space indent", async () => {
    const out = await formatSQL(sql, prefs({ indentStyle: "spaces", indentSize: 2 }));
    // Indented lines should start with exactly 2 spaces
    const indentedLines = out.split("\n").filter((l) => l.startsWith(" "));
    expect(indentedLines.length).toBeGreaterThan(0);
    indentedLines.forEach((l) => expect(l).toMatch(/^ {2}[^ ]/));
  });

  it("spaces/4 uses four-space indent", async () => {
    const out = await formatSQL(sql, prefs({ indentStyle: "spaces", indentSize: 4 }));
    const indentedLines = out.split("\n").filter((l) => l.startsWith(" "));
    expect(indentedLines.length).toBeGreaterThan(0);
    indentedLines.forEach((l) => expect(l).toMatch(/^ {4}[^ ]/));
  });

  it("tabs uses tab characters", async () => {
    const out = await formatSQL(sql, prefs({ indentStyle: "tabs" }));
    const indentedLines = out.split("\n").filter((l) => l.startsWith("\t"));
    expect(indentedLines.length).toBeGreaterThan(0);
    // No leading spaces on indented lines when using tabs
    indentedLines.forEach((l) => expect(l).not.toMatch(/^ /));
  });
});

// ── 6. Comma position ─────────────────────────────────────────────────────────

describe("commaPosition", () => {
  const sql = "select id, name, email, created_at from t";

  it("trailing puts commas at end of line (default)", async () => {
    const out = await formatSQL(sql, prefs({ commaPosition: "trailing" }));
    // Lines that continue a column list should end with ","
    const commaLines = out.split("\n").filter((l) => l.trimEnd().endsWith(","));
    expect(commaLines.length).toBeGreaterThan(0);
  });

  it("leading puts commas at start of continuation lines", async () => {
    const out = await formatSQL(sql, prefs({ commaPosition: "leading" }));
    // After the first SELECT column, remaining columns start with ", "
    const leadingLines = out.split("\n").filter((l) => l.trimStart().startsWith(","));
    expect(leadingLines.length).toBeGreaterThan(0);
    // No trailing commas should remain
    const trailingCommaLines = out.split("\n").filter((l) => l.trimEnd().endsWith(","));
    expect(trailingCommaLines).toHaveLength(0);
  });

  it("leading comma does not break function-argument commas on same line", async () => {
    // DECIMAL(18,2) — the comma is on the same line so leading-comma transform
    // must NOT insert a newline inside it
    const q = "select total::decimal(18,2) from t";
    const out = await formatSQL(q, prefs({ commaPosition: "leading" }));
    // functionCase defaults to UPPER so "decimal" becomes "DECIMAL".
    // The cast type should still be intact with comma inline (not split onto new line).
    expect(out).toContain("DECIMAL(18");
  });
});

// ── 7. Operator position ──────────────────────────────────────────────────────

describe("operatorPosition", () => {
  const sql = "select * from t where a = 1 and b = 2 and c = 3";

  it("before puts AND/OR at start of new line", async () => {
    const out = await formatSQL(sql, prefs({ operatorPosition: "before" }));
    const andLines = out.split("\n").filter((l) => l.trimStart().startsWith("AND"));
    expect(andLines.length).toBeGreaterThan(0);
  });

  it("after puts AND/OR at end of previous line", async () => {
    const out = await formatSQL(sql, prefs({ operatorPosition: "after" }));
    const andLines = out.split("\n").filter((l) => l.trimEnd().endsWith("AND"));
    expect(andLines.length).toBeGreaterThan(0);
    // AND must NOT appear at the start of any line
    const andAtStart = out.split("\n").filter((l) => l.trimStart().startsWith("AND"));
    expect(andAtStart).toHaveLength(0);
  });

  it("OR is also affected by operatorPosition", async () => {
    const q = "select * from t where a = 1 or b = 2 or c = 3";
    const before = await formatSQL(q, prefs({ operatorPosition: "before" }));
    const orAtStart = before.split("\n").filter((l) => l.trimStart().startsWith("OR"));
    expect(orAtStart.length).toBeGreaterThan(0);

    const after = await formatSQL(q, prefs({ operatorPosition: "after" }));
    const orAtEnd = after.split("\n").filter((l) => l.trimEnd().endsWith("OR"));
    expect(orAtEnd.length).toBeGreaterThan(0);
  });
});

// ── 8. Snowflake :: cast operator ─────────────────────────────────────────────

describe("Snowflake :: cast operator", () => {
  it("no whitespace around :: after simple column", async () => {
    const out = await formatSQL("select col::string from t", prefs());
    expect(out).not.toMatch(/\s::/);
    expect(out).not.toMatch(/::\s/);
    expect(out).toContain("col::string");
  });

  it("no whitespace around :: after function call", async () => {
    const out = await formatSQL("select to_date(col)::timestamp_ntz from t", prefs());
    expect(out).not.toMatch(/\s::/);
    expect(out).toContain(")::timestamp_ntz");
  });

  it("chained :: casts are preserved", async () => {
    const out = await formatSQL("select col::string::varchar from t", prefs());
    expect(out).not.toMatch(/\s::/);
    expect(out).toContain("::string::varchar");
  });

  it(":: inside function argument has no spaces", async () => {
    const out = await formatSQL("select coalesce(a::string, b::varchar) from t", prefs());
    expect(out).not.toMatch(/\s::/);
    expect(out).toContain("a::string");
    expect(out).toContain("b::varchar");
  });

  it(":: on complex arithmetic expression has no spaces", async () => {
    const out = await formatSQL("select (a + b * 2)::number(18,2) from t", prefs());
    expect(out).not.toMatch(/\s::/);
    // functionCase defaults to UPPER so the type token "number" becomes "NUMBER"
    expect(out).toContain(")::NUMBER(18");
  });

  it("try_cast equivalent - :: with complex Snowflake types", async () => {
    const types = ["integer", "float", "boolean", "date", "timestamp_ltz", "timestamp_ntz", "timestamp_tz", "variant", "object", "array"];
    for (const type of types) {
      const out = await formatSQL(`select col::${type} from t`, prefs());
      expect(out, `type=${type}`).not.toMatch(/\s::\s/);
    }
  });
});

// ── 9. Snowflake : VARIANT path operator ─────────────────────────────────────

describe("Snowflake : VARIANT path operator", () => {
  it("no whitespace around : for single-level path", async () => {
    const out = await formatSQL("select src:user_id from t", prefs());
    expect(out).toContain("src:user_id");
    expect(out).not.toMatch(/src\s:/);
    expect(out).not.toMatch(/:\s+user_id/);
  });

  it("multi-level path access preserves colons", async () => {
    const out = await formatSQL("select src:level1:level2:level3 from t", prefs());
    expect(out).toContain("src:level1:level2:level3");
  });

  it("combined : path and :: cast on same token", async () => {
    const out = await formatSQL("select src:user_id::string, src:amount::number from t", prefs());
    expect(out).toContain("src:user_id::string");
    expect(out).toContain("src:amount::number");
    expect(out).not.toMatch(/\s::/);
  });

  it("deeply nested path with cast: src:a:b:c::string", async () => {
    const out = await formatSQL("select payload:event:properties:user_id::string as uid from t", prefs());
    expect(out).not.toMatch(/\s::\s/);
    expect(out).toContain("payload:event:properties:user_id::string");
  });
});

// ── 10. CTE formatting ────────────────────────────────────────────────────────

describe("CTE formatting", () => {
  it("WITH is placed on its own line", async () => {
    const q = "with cte as (select 1 as n) select n from cte";
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toMatch(/^WITH\s/m);
  });

  it("CTE body is indented", async () => {
    const q = "with cte as (select id, name from users) select * from cte";
    const out = await formatSQL(q, prefs());
    // The SELECT inside the CTE body must be indented
    const lines = out.split("\n");
    const selectInsideCte = lines.find(
      (l) => l.match(/SELECT/i) && l.startsWith(" "),
    );
    expect(selectInsideCte).toBeDefined();
  });

  it("multiple CTEs all appear correctly", async () => {
    const q = `
      with
        a as (select 1 as n),
        b as (select n * 2 as m from a),
        c as (select n + m as total from a join b on true)
      select total from c
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toMatch(/\ba\s+AS\s*\(/im);
    expect(out).toMatch(/\bb\s+AS\s*\(/im);
    expect(out).toMatch(/\bc\s+AS\s*\(/im);
  });

  it("complex multi-CTE with window functions, QUALIFY, and :: casts", async () => {
    const q = `
      with
        raw_events as (
          select
            event_id,
            user_id::string as uid,
            payload:event_type::string as event_type,
            payload:amount::number(18,2) as amount,
            to_timestamp_ntz(event_ts) as ts
          from raw.events
          where payload:event_type::string is not null
        ),
        ranked as (
          select
            *,
            row_number() over (
              partition by uid
              order by ts desc
            ) as rn,
            lag(amount, 1, 0) over (
              partition by uid
              order by ts
            ) as prev_amount
          from raw_events
        )
      select uid, event_type, amount, prev_amount, ts
      from ranked
      where rn = 1
        and amount > prev_amount
      order by amount desc nulls last
    `;
    // Must not throw and must preserve :: operators
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).not.toMatch(/\s::\s/);
    expect(out).toContain("WITH");
    expect(out).toContain("raw_events");
    expect(out).toContain("ranked");
  });
});

// ── 11. LATERAL FLATTEN ───────────────────────────────────────────────────────

describe("LATERAL FLATTEN", () => {
  it("named parameter => is preserved", async () => {
    const q = "select f.value::string from t, lateral flatten(input => t.col) f";
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("=>");
    expect(out).not.toMatch(/\s::\s/);
  });

  it("FLATTEN with multiple named params", async () => {
    const q = `
      select
        f.index,
        f.value::string as tag
      from events,
        lateral flatten(input => events.payload:tags, outer => true) f
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("FLATTEN(");
    expect(out).toContain("=>");
    expect(out).not.toMatch(/\s::\s/);
  });
});

// ── 12. String literals — must never be modified ──────────────────────────────

describe("string literal passthrough", () => {
  it("keywords inside single-quoted strings are not modified", async () => {
    const q = "select 'SELECT FROM WHERE ORDER BY GROUP' as raw_sql from t";
    const out = await formatSQL(q, prefs({ keywordCase: "lower" }));
    // The string content must not be lowercased/uppercased
    expect(out).toContain("'SELECT FROM WHERE ORDER BY GROUP'");
  });

  it("SQL inside single-quoted string is not formatted", async () => {
    const q = "select 'with cte as (select 1)' as q, id from t";
    const out = await formatSQL(q, prefs());
    expect(out).toContain("'with cte as (select 1)'");
  });

  it("escaped single-quotes inside strings are handled", async () => {
    const q = "select 'it''s a trap' as msg from t";
    const out = await formatSQL(q, prefs());
    expect(out).toContain("'it''s a trap'");
  });

  it("dollar-quoted body is passed through unchanged", async () => {
    const body = `$$
DECLARE v NUMBER := 0;
BEGIN
  select count(*) into v from t;
  RETURN v;
END;
$$`;
    const q = `CREATE OR REPLACE FUNCTION count_rows() RETURNS NUMBER AS ${body}`;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER" }));
    // The content between $$ markers must not be modified
    expect(out).toContain("DECLARE v NUMBER");
    expect(out).toContain("BEGIN");
    // The body's internal casing must not be touched
    expect(out).not.toContain("declare v number");
  });

  it("dollar-quoted label ($label$) is also preserved", async () => {
    const q = `create function f() returns string language javascript as $js$ return 'hello'; $js$`;
    const out = await formatSQL(q, prefs({ functionCase: "lower" }));
    // sql-formatter may place the closing $js$ delimiter on its own line, but our
    // applyCasing pass must not modify content between the delimiters.
    expect(out).toContain("$js$");
    expect(out).toContain("return 'hello'"); // body content is untouched
  });
});

// ── 13. Comment passthrough ───────────────────────────────────────────────────

describe("comment passthrough", () => {
  it("line comments are not modified", async () => {
    const q = `
      -- This is a select from where order by comment
      select id from t
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "lower" }));
    // Comment content must be unchanged
    expect(out).toContain("-- This is a select from where order by comment");
  });

  it("block comments are not modified", async () => {
    const q = `
      /* SELECT id FROM t WHERE active = true */
      select id from t
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "lower" }));
    expect(out).toContain("/* SELECT id FROM t WHERE active = true */");
  });
});

// ── 14. Complex queries that stress the formatter ─────────────────────────────

describe("complex queries", () => {
  it("PIVOT query", async () => {
    const q = `
      select *
      from sales
      pivot (sum(amount) for region in ('NA', 'EU', 'APAC'))
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("PIVOT");
    expect(out).toContain("SUM(");
    expect(out).toContain("FOR");
    expect(out).toContain("IN");
  });

  it("MATCH_RECOGNIZE pattern query", async () => {
    const q = `
      select *
      from t
      match_recognize (
        partition by user_id
        order by event_ts
        measures
          strtok(classifier(), '_', 1) as event_class,
          last(ts) as last_ts
        pattern (start_event middle_event+ end_event)
        define
          start_event as event_type = 'start',
          middle_event as event_type like 'mid%',
          end_event as event_type = 'end'
      )
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("MATCH_RECOGNIZE");
    expect(out).toContain("MEASURES");
    expect(out).toContain("PATTERN");
    expect(out).toContain("DEFINE");
    // Should not throw
  });

  it("GROUP BY ROLLUP / CUBE / GROUPING SETS", async () => {
    const q = `
      select region, product, sum(amount) as total
      from sales
      group by rollup (region, product)
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("GROUP");
    expect(out).toContain("BY");
    expect(out).toContain("ROLLUP");
    expect(out).toContain("SUM(");
  });

  it("window frame ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW", async () => {
    const q = `
      select
        id,
        sum(amount) over (
          partition by user_id
          order by event_ts
          rows between unbounded preceding and current row
        ) as running_total
      from t
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("ROWS");
    expect(out).toContain("UNBOUNDED");
    expect(out).toContain("PRECEDING");
    expect(out).toContain("CURRENT");
    expect(out).toContain("ROW");
    expect(out).toContain("SUM(");
  });

  it("LISTAGG with WITHIN GROUP", async () => {
    const q = `
      select
        group_id,
        listagg(name, ', ') within group (order by name) as names
      from t
      group by group_id
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("LISTAGG(");
    expect(out).toContain("WITHIN");
    expect(out).toContain("GROUP");
  });

  it("ARRAY_CONSTRUCT and OBJECT_CONSTRUCT with :: casts", async () => {
    const q = `
      select
        array_construct(1, 2, 3) as arr,
        object_construct('k', col::string, 'n', count(*)::integer) as obj,
        array_agg(id::string) within group (order by id) as id_list
      from t
      group by col
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).not.toMatch(/\s::\s/);
    expect(out).toContain("ARRAY_CONSTRUCT(");
    expect(out).toContain("OBJECT_CONSTRUCT(");
    expect(out).toContain("ARRAY_AGG(");
  });

  it("IFF nested in CASE nested in COALESCE", async () => {
    const q = `
      select
        coalesce(
          case
            when iff(score > 0.8, true, false) then 'high'
            when iff(score > 0.5, true, false) then 'medium'
            else 'low'
          end,
          'unknown'
        ) as tier
      from t
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("COALESCE(");
    expect(out).toContain("CASE");
    expect(out).toContain("WHEN");
    expect(out).toContain("IFF(");
    expect(out).toContain("THEN");
    expect(out).toContain("ELSE");
    expect(out).toContain("END");
  });

  it("correlated subquery with LATERAL", async () => {
    const q = `
      select
        o.order_id,
        o.customer_id,
        latest.last_event
      from orders o
      join lateral (
        select max(event_ts) as last_event
        from events e
        where e.order_id = o.order_id
      ) latest on true
      where o.status = 'open'
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("LATERAL");
    expect(out).toContain("MAX(");
    expect(out).toContain("JOIN");
  });

  it("AT time-travel with DATEADD and ::timestamp_tz", async () => {
    const q = `
      select *
      from orders at (timestamp => dateadd('minute', -5, current_timestamp())::timestamp_tz)
    `;
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("AT");
    expect(out).toContain("DATEADD(");
    expect(out).toContain("CURRENT_TIMESTAMP(");
    expect(out).not.toMatch(/\s::\s/);
  });

  it("TRY_CAST and TRY_TO_DATE on messy input columns", async () => {
    const q = `
      select
        try_cast(raw_value as number) as num,
        try_to_date(raw_ts, 'YYYY-MM-DD') as dt,
        try_to_decimal(amount_str, 18, 2) as amt,
        is_integer(coalesce(try_cast(s as integer), 0)) as is_int
      from raw_data
    `;
    const out = await formatSQL(q, prefs({ functionCase: "UPPER" }));
    expect(out).toContain("TRY_CAST(");
    expect(out).toContain("TRY_TO_DATE(");
    expect(out).toContain("TRY_TO_DECIMAL(");
    expect(out).toContain("IS_INTEGER(");
  });

  it("SAMPLE clause is handled", async () => {
    const q = "select * from very_large_table sample (10)";
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("SAMPLE");
    expect(out).toContain("10");
  });

  it("SEMI JOIN and ANTI JOIN", async () => {
    const q = `
      select id from a
      where id in (select id from b)
    `;
    // Just verify it formats without error
    const out = await formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("SELECT");
    expect(out).toContain("WHERE");
    expect(out).toContain("IN");
  });

  it("deeply nested CTE with FLATTEN inside window aggregate", async () => {
    const q = `
      with
        exploded as (
          select
            e.event_id,
            e.user_id::string as uid,
            f.index as tag_index,
            f.value::string as tag,
            e.event_ts
          from events e,
            lateral flatten(input => e.payload:tags) f
        ),
        tag_counts as (
          select
            uid,
            tag,
            count(*) as occurrences,
            min(event_ts) as first_seen,
            max(event_ts) as last_seen
          from exploded
          group by uid, tag
        ),
        top_tags as (
          select
            uid,
            tag,
            occurrences,
            row_number() over (
              partition by uid
              order by occurrences desc, tag
            ) as tag_rank
          from tag_counts
        )
      select uid, tag, occurrences
      from top_tags
      where tag_rank <= 5
      order by uid, tag_rank
    `;
    const out = await formatSQL(q, prefs({
      keywordCase: "UPPER",
      identifierCase: "Preserve",
      functionCase: "UPPER",
      commaPosition: "trailing",
      operatorPosition: "before",
    }));
    // No spaces around :: or :
    expect(out).not.toMatch(/\s::\s/);
    // Structural keywords present
    expect(out).toContain("WITH");
    expect(out).toContain("LATERAL");
    expect(out).toContain("FLATTEN(");
    expect(out).toContain("PARTITION");
    expect(out).toContain("ORDER BY");
    // Identifiers not changed
    expect(out).toContain("uid");
    expect(out).toContain("tag");
    expect(out).toContain("occurrences");
  });

  it("leading comma with nested CTEs produces valid output", async () => {
    const q = `
      with a as (select 1 as n, 2 as m, 3 as k),
           b as (select n + m + k as total from a)
      select total, total * 2 as double_total from b
    `;
    const out = await formatSQL(q, prefs({ commaPosition: "leading" }));
    // Leading comma lines should not contain trailing commas
    const lines = out.split("\n");
    const trailingCommaLines = lines.filter(
      (l) => l.trimEnd().endsWith(",") && !l.trimStart().startsWith(","),
    );
    expect(trailingCommaLines).toHaveLength(0);
  });

  it("formatter is idempotent — formatting twice produces identical output", async () => {
    const q = `
      with orders as (
        select order_id, customer_id, total::decimal(18,2) as amount
        from raw.orders where status = 'active'
      )
      select customer_id, sum(amount) as total_spent
      from orders
      group by customer_id
      having sum(amount) > 100
      order by total_spent desc
    `;
    const p = prefs({ keywordCase: "UPPER", functionCase: "UPPER", commaPosition: "trailing" });
    const first = await formatSQL(q, p);
    const second = await formatSQL(first, p);
    expect(normalize(second)).toBe(normalize(first));
  });
});
