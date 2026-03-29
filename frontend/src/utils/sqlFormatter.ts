/**
 * SQL formatter for the Thaw editor — Snowflake dialect.
 *
 * Uses sql-formatter for structural formatting (indentation, line breaks,
 * comma / operator placement, CTE layout) and applies a custom token-level
 * casing pass on top for separate keyword / identifier / function control.
 */

import { format as sfFormat } from "sql-formatter";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface EditorPrefs {
  keywordCase:      "UPPER" | "lower" | "Title" | "Preserve";
  identifierCase:   "Preserve" | "UPPER" | "lower";
  functionCase:     "UPPER" | "lower";
  indentStyle:      "spaces" | "tabs";
  indentSize:       2 | 4;
  commaPosition:    "trailing" | "leading";
  operatorPosition: "before" | "after";
}

export const DEFAULT_EDITOR_PREFS: EditorPrefs = {
  keywordCase:      "UPPER",
  identifierCase:   "Preserve",
  functionCase:     "UPPER",
  indentStyle:      "spaces",
  indentSize:       2,
  commaPosition:    "trailing",
  operatorPosition: "before",
};

// ── Snowflake keyword set (reserved words; canonical form = UPPERCASE) ────────
// These receive keywordCase treatment; everything else is an identifier.

const SQL_KEYWORDS = new Set([
  "ADD","ALL","ALTER","AND","ANY","AS","ASC","AT","BEFORE","BETWEEN","BY",
  "CALL","CASCADE","CASE","CAST","CHANGES","CLUSTER","COLUMN","COMMENT",
  "COMMIT","CONNECT","CONSTRAINT","COPY","CREATE","CROSS","CURRENT",
  "CURRENT_DATE","CURRENT_ROLE","CURRENT_SCHEMA","CURRENT_TIME",
  "CURRENT_TIMESTAMP","CURRENT_USER","CURRENT_WAREHOUSE","DATABASE",
  "DEFAULT","DELETE","DESC","DESCRIBE","DISTINCT","DROP","ELSE","END",
  "EXCEPT","EXECUTE","EXISTS","EXPLAIN","EXTRACT","FALSE","FILE","FIRST",
  "FLATTEN","FOLLOWING","FOR","FORCE","FOREIGN","FROM","FULL","FUNCTION",
  "GRANT","GROUP","GROUPING","HAVING","IF","ILIKE","IN","INDEX","INNER",
  "INSERT","INTERSECT","INTO","IS","JOIN","KEY","LAST","LATERAL","LEFT",
  "LIKE","LIMIT","MATCH_RECOGNIZE","MEASURES","MERGE","MINUS","NATURAL",
  "NOT","NULL","NULLS","OF","OFFSET","ON","OR","ORDER","OUTER","OVER",
  "OVERWRITE","PARTITION","PATTERN","PIPE","PRECEDING","PRIMARY","PROCEDURE",
  "PURGE","QUALIFY","RANGE","RECURSIVE","REFERENCES","REPLACE","RESTRICT",
  "REVOKE","RIGHT","ROLLBACK","ROW","ROWS","SAMPLE","SCHEMA","SELECT",
  "SEMI","SEQUENCE","SET","SHOW","SOME","STAGE","START","STREAM","TABLE",
  "TABLESAMPLE","TASK","THEN","TO","TOP","TRANSACTION","TRUE","TRUNCATE",
  "UNBOUNDED","UNION","UNIQUE","UPDATE","USING","VALUES","VIEW","VOLATILE",
  "WAREHOUSE","WHEN","WHERE","WINDOW","WITH","WITHIN","WITHOUT",
  // Snowflake-specific
  "ANTI","ASOF","CHANGES","CLONE","CONNECT","COPY","CRON","DYNAMIC",
  "ENABLE","EXTERNAL","FILE","FINALIZE","FORMAT","ICEBERG","MASKING",
  "NETWORK","NOTIFY","PIPE","POLICY","PROJECTION","PURGE","RECOVER",
  "REPLICATION","RESUME","REVOKE","ROLE","ROW","ROWS","SAMPLE","SCHEDULE",
  "SECURE","SHARE","STAGE","STREAM","SUSPEND","TABULAR","TASK","TRANSIENT",
  "TRIGGER","UNDROP","USING","VOLATILE","WAREHOUSE",
]);

// ── Snowflake built-in functions (canonical form = UPPERCASE) ─────────────────
// Tokens followed by "(" that match this set receive functionCase treatment.
// Unknown function-like tokens (followed by "(") also get functionCase.

const BUILTIN_FUNCTIONS = new Set([
  "ABS","ACOS","ACOSH","ADD_MONTHS","ANY_VALUE","APPROX_COUNT_DISTINCT",
  "APPROX_PERCENTILE","APPROX_TOP_K","ARRAY_AGG","ARRAY_APPEND",
  "ARRAY_CAT","ARRAY_COMPACT","ARRAY_CONSTRUCT","ARRAY_CONTAINS",
  "ARRAY_DISTINCT","ARRAY_EXCEPT","ARRAY_FLATTEN","ARRAY_GENERATE_RANGE",
  "ARRAY_INSERT","ARRAY_INTERSECTION","ARRAY_MAX","ARRAY_MIN","ARRAY_PREPEND",
  "ARRAY_REMOVE","ARRAY_REMOVE_AT","ARRAY_SIZE","ARRAY_SLICE","ARRAY_SORT",
  "ARRAY_TO_STRING","ARRAY_UNION_AGG","ARRAY_UNIQUE_AGG","AS_ARRAY",
  "AS_BINARY","AS_BOOLEAN","AS_CHAR","AS_DATE","AS_DECIMAL","AS_DOUBLE",
  "AS_INTEGER","AS_NUMBER","AS_OBJECT","AS_REAL","AS_TIME","AS_TIMESTAMP_LTZ",
  "AS_TIMESTAMP_NTZ","AS_TIMESTAMP_TZ","AS_TINYINT","AS_VARCHAR","ASIN",
  "ASINH","ATAN","ATAN2","ATANH","AVG","BASE64_DECODE_STRING",
  "BASE64_ENCODE","BITNOT","BITSHIFTLEFT","BITSHIFTRIGHT","BITAND",
  "BITOR","BITXOR","BOOLAND","BOOLAND_AGG","BOOLNOT","BOOLOR",
  "BOOLOR_AGG","BOOLXOR","BOOLXOR_AGG",
  "CASE","CAST","CBRT","CEIL","CEILING","CHARINDEX","CHR","CHAR",
  "COALESCE","COLLATE","COLLATION","COMPRESS","CONCAT","CONCAT_WS",
  "CONDITIONAL_CHANGE_EVENT","CONDITIONAL_TRUE_EVENT","CONTAINS",
  "CONVERT_TIMEZONE","COS","COSH","COUNT","COUNT_IF","COVAR_POP",
  "COVAR_SAMP","CUME_DIST",
  "DATE_FROM_PARTS","DATE_PART","DATE_TRUNC","DATEADD","DATEDIFF",
  "DAYNAME","DAYOFMONTH","DAYOFWEEK","DAYOFWEEKISO","DAYOFYEAR",
  "DECODE","DECOMPRESS","DENSE_RANK","DIV0","DIV0NULL",
  "EDITDISTANCE","ENDSWITH","EQUAL_NULL","EXP",
  "FIRST_VALUE","FLATTEN","FLOOR","FORMAT_DATE","FORMAT_NUMBER",
  "GENERATOR","GET","GET_ABSOLUTE_PATH","GET_DDL","GET_PATH",
  "GET_PRESIGNED_URL","GET_STAGE_LOCATION","GETBIT","GREATEST",
  "GROUPING","GROUPING_ID",
  "HASH","HASH_AGG","HAVERSINE","HEX_DECODE_BINARY",
  "HEX_DECODE_STRING","HEX_ENCODE","HOUR","HOURS",
  "IFF","IFNULL","IN","INITCAP","INSERT","IS_ARRAY","IS_BINARY",
  "IS_BOOLEAN","IS_CHAR","IS_DATE","IS_DATE_VALUE","IS_DECIMAL",
  "IS_DOUBLE","IS_GRANTED_TO_INVOKER_ROLE","IS_INTEGER","IS_NULL_VALUE",
  "IS_OBJECT","IS_REAL","IS_TIME","IS_TIMESTAMP_LTZ","IS_TIMESTAMP_NTZ",
  "IS_TIMESTAMP_TZ","IS_VARCHAR","JAROWINKLER_SIMILARITY",
  "JSON_EXTRACT_PATH_TEXT",
  "KURTOSIS","LAG","LAST_DAY","LAST_VALUE","LEAD","LEAST","LEFT",
  "LENGTH","LEN","LISTAGG","LN","LOG","LOWER","LPAD","LTRIM",
  "MAX","MAX_BY","MEDIAN","MIN","MIN_BY","MINUTE","MINUTES","MOD",
  "MODE","MONTH","MONTHNAME","MONTHS_BETWEEN",
  "NORMAL","NTH_VALUE","NTILE","NULLIF","NULLIFZERO","NVL","NVL2",
  "OBJECT_AGG","OBJECT_CONSTRUCT","OBJECT_CONSTRUCT_KEEP_NULL",
  "OBJECT_DELETE","OBJECT_INSERT","OBJECT_KEYS","OBJECT_PICK",
  "PARSE_IP","PARSE_JSON","PARSE_URL","PARSE_XML","PERCENT_RANK",
  "PERCENTILE_CONT","PERCENTILE_DISC","PI","POSITION","POW","POWER",
  "RANDSTR","RANDOM","RANK","RATIO_TO_REPORT","REGEXP","REGEXP_COUNT",
  "REGEXP_EXTRACT","REGEXP_EXTRACT_ALL","REGEXP_INSTR","REGEXP_LIKE",
  "REGEXP_REPLACE","REGEXP_SUBSTR","REPEAT","REPLACE","REVERSE",
  "RIGHT","ROUND","ROW_NUMBER","RPAD","RTRIM",
  "SECOND","SECONDS","SHA1","SHA1_BINARY","SHA1_HEX","SHA2","SHA2_BINARY",
  "SHA2_HEX","SIGN","SIN","SINH","SKEW","SOUNDEX","SPACE","SPLIT",
  "SPLIT_PART","SPLIT_TO_TABLE","SQL_VARIANT_PROPERTY","SQRT",
  "SQUARE","STARTSWITH","STDDEV","STDDEV_POP","STDDEV_SAMP",
  "STRIP_NULL_VALUE","STRTOK","STRTOK_SPLIT_TO_TABLE","STRTOK_TO_ARRAY",
  "SUBSTR","SUBSTRING","SUM","SYSTEM$ABORT_TRANSACTION",
  "SYSTEM$CANCEL_ALL_QUERIES","SYSTEM$CANCEL_QUERY","SYSTEM$CLUSTERING_DEPTH",
  "SYSTEM$CLUSTERING_INFORMATION","SYSTEM$GET_PREDECESSOR_RETURN_VALUE",
  "SYSTEM$STREAM_GET_TABLE_TIMESTAMP","SYSTEM$STREAM_HAS_DATA",
  "SYSTEM$TASK_DEPENDENTS_ENABLE","SYSTEM$TYPEOF","SYSTEM$WAIT",
  "TAN","TANH","TIME_FROM_PARTS","TIMEADD","TIMEDIFF","TIMESTAMPADD",
  "TIMESTAMPDIFF","TIMESTAMP_FROM_PARTS","TIMESTAMP_LTZ_FROM_PARTS",
  "TIMESTAMP_NTZ_FROM_PARTS","TIMESTAMP_TZ_FROM_PARTS",
  "TO_ARRAY","TO_BINARY","TO_BOOLEAN","TO_CHAR","TO_DATE","TO_DECIMAL",
  "TO_DOUBLE","TO_GEOGRAPHY","TO_GEOMETRY","TO_JSON","TO_NUMBER",
  "TO_OBJECT","TO_REAL","TO_TIME","TO_TIMESTAMP","TO_TIMESTAMP_LTZ",
  "TO_TIMESTAMP_NTZ","TO_TIMESTAMP_TZ","TO_VARIANT","TO_VARCHAR",
  "TO_XML","TRANSLATE","TRIM","TRUNCATE","TRUNC","TRY_BASE64_DECODE_BINARY",
  "TRY_BASE64_DECODE_STRING","TRY_CAST","TRY_HEX_DECODE_BINARY",
  "TRY_HEX_DECODE_STRING","TRY_PARSE_JSON","TRY_TO_BINARY","TRY_TO_BOOLEAN",
  "TRY_TO_DATE","TRY_TO_DECIMAL","TRY_TO_DOUBLE","TRY_TO_NUMBER",
  "TRY_TO_TIME","TRY_TO_TIMESTAMP","TRY_TO_TIMESTAMP_LTZ",
  "TRY_TO_TIMESTAMP_NTZ","TRY_TO_TIMESTAMP_TZ","TYPEOF",
  "UNIFORM","UPPER","UNISTR",
  "VAR_POP","VAR_SAMP","VARIANCE","VARIANCE_POP","VARIANCE_SAMP",
  "WEEK","WEEKISO","WEEKOFYEAR",
  "XMLGET","YEAR","YEAROFWEEK","YEAROFWEEKISO","ZEROIFNULL",
]);

// ── Token-level casing ────────────────────────────────────────────────────────

function applyCase(
  word: string,
  casing: "UPPER" | "lower" | "Title" | "Preserve",
): string {
  switch (casing) {
    case "UPPER":   return word.toUpperCase();
    case "lower":   return word.toLowerCase();
    case "Title":   return word.charAt(0).toUpperCase() + word.slice(1).toLowerCase();
    case "Preserve":return word;
  }
}

/**
 * Walk the formatted SQL string token by token and apply per-role casing.
 * Double-quoted identifiers, single-quoted strings, dollar-quoted strings,
 * and comments are passed through unchanged.
 */
function applyCasing(sql: string, prefs: EditorPrefs): string {
  const out: string[] = [];
  let i = 0;
  const len = sql.length;

  while (i < len) {
    const ch = sql[i];

    // ── Double-quoted identifier — never modify ────────────────────────────
    if (ch === '"') {
      let j = i + 1;
      while (j < len) {
        if (sql[j] === '"') {
          // Escaped quote "" inside identifier
          if (sql[j + 1] === '"') { j += 2; continue; }
          j++; break;
        }
        j++;
      }
      out.push(sql.slice(i, j));
      i = j;
      continue;
    }

    // ── Single-quoted string literal — never modify ────────────────────────
    if (ch === "'") {
      let j = i + 1;
      while (j < len) {
        if (sql[j] === "'") {
          if (sql[j + 1] === "'") { j += 2; continue; } // escaped ''
          j++; break;
        }
        j++;
      }
      out.push(sql.slice(i, j));
      i = j;
      continue;
    }

    // ── Dollar-quoted string $$ … $$ or $tag$ … $tag$ — never modify ──────
    if (ch === "$") {
      // Find the tag: $[optional_tag]$
      let tagEnd = i + 1;
      while (tagEnd < len && sql[tagEnd] !== "$" && sql[tagEnd] !== "\n") tagEnd++;
      if (tagEnd < len && sql[tagEnd] === "$") {
        const tag = sql.slice(i, tagEnd + 1); // e.g. "$$" or "$body$"
        const closeIdx = sql.indexOf(tag, tagEnd + 1);
        if (closeIdx >= 0) {
          out.push(sql.slice(i, closeIdx + tag.length));
          i = closeIdx + tag.length;
          continue;
        }
      }
      // Not a dollar-quoted string — fall through to normal char handling
      out.push(ch);
      i++;
      continue;
    }

    // ── Line comment -- … \n — never modify ───────────────────────────────
    if (ch === "-" && sql[i + 1] === "-") {
      const nl = sql.indexOf("\n", i);
      const end = nl >= 0 ? nl + 1 : len;
      out.push(sql.slice(i, end));
      i = end;
      continue;
    }

    // ── Block comment /* … */ — never modify ──────────────────────────────
    if (ch === "/" && sql[i + 1] === "*") {
      const close = sql.indexOf("*/", i + 2);
      const end = close >= 0 ? close + 2 : len;
      out.push(sql.slice(i, end));
      i = end;
      continue;
    }

    // ── Word token (identifier / keyword) ─────────────────────────────────
    if (/[a-zA-Z_]/.test(ch)) {
      let j = i + 1;
      while (j < len && /[a-zA-Z0-9_$]/.test(sql[j])) j++;
      const word = sql.slice(i, j);
      const upper = word.toUpperCase();

      // Peek past whitespace to determine if this is a function call.
      let k = j;
      while (k < len && (sql[k] === " " || sql[k] === "\t")) k++;
      const isCall = sql[k] === "(";

      let result: string;
      if (isCall) {
        // All function-call tokens get functionCase (known builtins or UDFs).
        // Exception: bare keywords like CASE, CAST, CONVERT that use "(" don't
        // need special handling — applyCase will just follow keyword rules.
        if (SQL_KEYWORDS.has(upper) && !BUILTIN_FUNCTIONS.has(upper)) {
          result = applyCase(word, prefs.keywordCase);
        } else {
          result = applyCase(word, prefs.functionCase);
        }
      } else if (SQL_KEYWORDS.has(upper)) {
        result = applyCase(word, prefs.keywordCase);
      } else {
        result = applyCase(word, prefs.identifierCase);
      }

      out.push(result);
      i = j;
      continue;
    }

    // Everything else: numbers, operators, whitespace, punctuation — pass through.
    out.push(ch);
    i++;
  }

  return out.join("");
}

// ── Main export ───────────────────────────────────────────────────────────────

/**
 * Format a SQL string using the given editor preferences.
 *
 * Structural formatting (line breaks, indentation, CTE layout, comma / operator
 * placement) is handled by sql-formatter with the Snowflake dialect.
 * Token-level casing is applied by a custom post-processor so keyword, identifier,
 * and function casing can be controlled independently.
 */
export function formatSQL(sql: string, prefs: EditorPrefs): string {
  if (!sql.trim()) return sql;

  try {
    let structured = sfFormat(sql, {
      language: "snowflake",
      // Always output UPPER from sql-formatter; our casing pass will convert.
      keywordCase:          "upper",
      tabWidth:             prefs.indentSize,
      useTabs:              prefs.indentStyle === "tabs",
      logicalOperatorNewline: prefs.operatorPosition === "before" ? "before" : "after",
      expressionWidth:      60,
      linesBetweenQueries:  1,
    });

    // sql-formatter removed commaPosition support in v15+. Apply leading
    // commas as a post-processing step: move each trailing comma to the
    // start of the following line so `col1,\n  col2` → `col1\n, col2`.
    if (prefs.commaPosition === "leading") {
      structured = structured.replace(/,\n\s*/g, "\n, ");
    }

    return applyCasing(structured, prefs);
  } catch {
    // If the formatter fails (e.g. on a partial/invalid statement), return the
    // original SQL unchanged so the editor never ends up with corrupted content.
    return sql;
  }
}
