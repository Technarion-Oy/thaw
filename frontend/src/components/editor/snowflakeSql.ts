// SPDX-License-Identifier: GPL-3.0-or-later

import { SNOWFLAKE_DATA_TYPE_NAMES } from "../../generated/snowflakeDataTypes";

// ─── Built-in function catalogue ──────────────────────────────────────────────
// Single source of truth for both the Monarch `@builtins` tokenizer rule below
// and the "Built-in Functions" group in the Code Snippets browser (which renders
// these keys as cascading sub-categories). Keep names uppercase.
export const BUILTIN_FUNCTION_CATEGORIES: Record<string, string[]> = {
  Aggregate: [
    "COUNT", "SUM", "AVG", "MIN", "MAX", "VARIANCE", "VAR_POP", "VAR_SAMP",
    "STDDEV", "STDDEV_POP", "STDDEV_SAMP", "MEDIAN", "MODE",
    "APPROX_COUNT_DISTINCT", "APPROX_PERCENTILE", "LISTAGG",
    "ARRAY_AGG", "OBJECT_AGG", "BOOLAND_AGG", "BOOLOR_AGG", "BOOLXOR_AGG",
  ],
  Window: [
    "ROW_NUMBER", "RANK", "DENSE_RANK", "NTILE", "LEAD", "LAG",
    "FIRST_VALUE", "LAST_VALUE", "NTH_VALUE", "RATIO_TO_REPORT",
    "PERCENT_RANK", "CUME_DIST",
  ],
  String: [
    "CONCAT", "CONCAT_WS", "UPPER", "LOWER", "TRIM", "LTRIM", "RTRIM",
    "LENGTH", "LEN", "SUBSTR", "SUBSTRING", "REPLACE", "REGEXP_REPLACE",
    "REGEXP_LIKE", "REGEXP_SUBSTR", "REGEXP_INSTR", "REGEXP_COUNT",
    "SPLIT", "SPLIT_PART", "STRTOK", "STRTOK_TO_ARRAY", "CONTAINS",
    "STARTSWITH", "ENDSWITH", "CHARINDEX", "POSITION", "INITCAP",
    "REPEAT", "SPACE", "REVERSE", "LPAD", "RPAD", "ASCII", "CHR",
    "UNICODE", "SOUNDEX", "EDITDISTANCE", "JAROWINKLER_SIMILARITY",
    "FORMAT_NUMBER", "TO_VARCHAR",
  ],
  "Date & Time": [
    "DATEADD", "DATEDIFF", "DATE_TRUNC", "DATE_PART", "EXTRACT",
    "TO_DATE", "TO_TIME", "TO_TIMESTAMP", "TO_TIMESTAMP_NTZ",
    "TO_TIMESTAMP_LTZ", "TO_TIMESTAMP_TZ",
    "YEAR", "MONTH", "DAY", "HOUR", "MINUTE", "SECOND", "QUARTER",
    "DAYOFWEEK", "DAYOFWEEKISO", "DAYOFYEAR", "WEEK", "WEEKOFYEAR",
    "WEEKISO", "YEAROFWEEK", "YEAROFWEEKISO",
    "LAST_DAY", "NEXT_DAY", "ADD_MONTHS", "MONTHS_BETWEEN", "TRUNC",
    "TIME_FROM_PARTS", "DATE_FROM_PARTS", "TIMESTAMP_FROM_PARTS",
    "CONVERT_TIMEZONE",
  ],
  "Conversion & Cast": [
    "TO_CHAR", "TO_NUMBER", "TO_DECIMAL", "TO_DOUBLE", "TO_BOOLEAN",
    "TO_BINARY", "TRY_TO_DATE", "TRY_TO_TIME", "TRY_TO_TIMESTAMP",
    "TRY_TO_NUMBER", "TRY_TO_DECIMAL", "TRY_TO_DOUBLE", "TRY_TO_BOOLEAN",
    "CAST", "TRY_CAST", "CONVERT",
  ],
  "Conditional & NULL": [
    "COALESCE", "NULLIF", "IFNULL", "NVL", "NVL2", "ZEROIFNULL",
    "DECODE", "GREATEST", "LEAST", "EQUAL_NULL",
  ],
  Math: [
    "ABS", "CEIL", "CEILING", "FLOOR", "ROUND", "MOD", "POWER", "POW",
    "SQRT", "SQUARE", "EXP", "LN", "LOG", "LOG2", "LOG10", "SIGN",
    "FACTORIAL", "UNIFORM", "NORMAL", "RANDOM", "RANDSTR",
    "SEQ1", "SEQ2", "SEQ4", "SEQ8",
    "SIN", "COS", "TAN", "ASIN", "ACOS", "ATAN", "ATAN2",
    "DEGREES", "RADIANS", "PI", "HAVERSINE", "WIDTH_BUCKET",
    "REGR_SLOPE", "REGR_INTERCEPT", "REGR_R2",
  ],
  "Semi-structured / JSON": [
    "PARSE_JSON", "TO_JSON", "TO_ARRAY", "TO_OBJECT",
    "ARRAY_CONSTRUCT", "OBJECT_CONSTRUCT",
    "ARRAY_SIZE", "ARRAY_CONTAINS", "ARRAY_APPEND", "ARRAY_CAT",
    "ARRAY_COMPACT", "ARRAY_DISTINCT", "ARRAY_FLATTEN", "ARRAY_INTERSECTION",
    "ARRAY_POSITION", "ARRAY_PREPEND", "ARRAY_REMOVE", "ARRAY_SLICE",
    "ARRAY_SORT", "ARRAY_TO_STRING", "ARRAYS_OVERLAP",
    "OBJECT_KEYS", "OBJECT_INSERT", "OBJECT_DELETE", "OBJECT_PICK",
    "FLATTEN", "GET", "GET_PATH", "IS_ARRAY", "IS_OBJECT",
    "IS_NULL_VALUE", "JSON_EXTRACT_PATH_TEXT", "TYPEOF", "CHECK_JSON", "CHECK_XML",
    "STRIP_NULL_VALUE",
    "AS_CHAR", "AS_VARCHAR", "AS_ARRAY", "AS_OBJECT",
    "AS_INTEGER", "AS_DOUBLE", "AS_DECIMAL", "AS_DATE", "AS_TIME",
  ],
  "Hash & Crypto": [
    "HASH", "MD5", "SHA1", "SHA2",
    "HEX_ENCODE", "HEX_DECODE_STRING",
    "BASE64_ENCODE", "BASE64_DECODE_STRING",
  ],
  "System & Table": [
    "GENERATOR", "RESULT_SCAN", "VALIDATE",
    "SYSTEM$CANCEL_QUERY", "SYSTEM$CLUSTERING_INFORMATION",
  ],
  Geospatial: [
    // HAVERSINE lives in Math (already present). See Snowflake geospatial docs.
    "TO_GEOGRAPHY", "TO_GEOMETRY", "TRY_TO_GEOGRAPHY", "TRY_TO_GEOMETRY",
    "ST_GEOGFROMGEOHASH", "ST_GEOGPOINTFROMGEOHASH", "ST_GEOGRAPHYFROMWKB",
    "ST_GEOGRAPHYFROMWKT", "ST_GEOMETRYFROMWKB", "ST_GEOMETRYFROMWKT",
    "ST_GEOMFROMGEOHASH", "ST_GEOMPOINTFROMGEOHASH",
    "ST_ASGEOJSON", "ST_ASWKB", "ST_ASBINARY", "ST_ASEWKB", "ST_ASWKT",
    "ST_ASTEXT", "ST_ASEWKT", "ST_GEOHASH",
    "ST_MAKELINE", "ST_MAKEGEOMPOINT", "ST_GEOMPOINT", "ST_MAKEPOINT",
    "ST_POINT", "ST_MAKEPOLYGON", "ST_POLYGON", "ST_MAKEPOLYGONORIENTED",
    "ST_DIMENSION", "ST_ENDPOINT", "ST_POINTN", "ST_SRID", "ST_STARTPOINT",
    "ST_X", "ST_XMAX", "ST_XMIN", "ST_Y", "ST_YMAX", "ST_YMIN",
    "ST_AREA", "ST_AZIMUTH", "ST_CONTAINS", "ST_COVEREDBY", "ST_COVERS",
    "ST_DISJOINT", "ST_DISTANCE", "ST_DWITHIN", "ST_HAUSDORFFDISTANCE",
    "ST_INTERSECTS", "ST_LENGTH", "ST_NPOINTS", "ST_NUMPOINTS",
    "ST_PERIMETER", "ST_WITHIN",
    "ST_BUFFER", "ST_CENTROID", "ST_COLLECT", "ST_DIFFERENCE", "ST_ENVELOPE",
    "ST_INTERPOLATE", "ST_INTERSECTION", "ST_INTERSECTION_AGG", "ST_SETSRID",
    "ST_SIMPLIFY", "ST_SYMDIFFERENCE", "ST_TRANSFORM", "ST_UNION",
    "ST_UNION_AGG", "ST_ISVALID",
  ],
};

const BUILTINS_FLAT: string[] = Object.values(BUILTIN_FUNCTION_CATEGORIES).flat();

// ─── Monarch tokenizer ────────────────────────────────────────────────────────
// Produces granular token types so the custom themes below can assign distinct
// colours to DML, DDL, clause, control-flow, functions, types, etc.

export const snowflakeMonarchLanguage = {
  defaultToken:  "identifier",
  tokenPostfix:  ".sql",
  ignoreCase:    true,   // keywords compared case-insensitively

  // ── Keyword groups ──────────────────────────────────────────────────────────

  dml: [
    "SELECT", "INSERT", "UPDATE", "DELETE", "MERGE", "UPSERT", "CALL", "EXECUTE",
  ],

  clause: [
    "FROM", "WHERE", "ON", "USING", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER",
    "CROSS", "FULL", "NATURAL", "LATERAL", "GROUP", "ORDER", "HAVING", "QUALIFY",
    "LIMIT", "OFFSET", "FETCH", "NEXT", "ONLY", "UNION", "INTERSECT", "EXCEPT",
    "MINUS", "BY", "PARTITION", "OVER", "ROWS", "RANGE", "WINDOW", "UNBOUNDED",
    "PRECEDING", "FOLLOWING", "INTO", "VALUES", "WITH", "RECURSIVE",
    "EXCLUDE", "AT", "BEFORE", "AFTER",
  ],

  ddl: [
    "CREATE", "ALTER", "DROP", "TRUNCATE", "RENAME", "REPLACE", "COPY",
    "GET", "PUT", "REMOVE", "LIST", "CLONE", "UNDROP",
  ],

  object: [
    "TABLE", "VIEW", "SCHEMA", "DATABASE", "WAREHOUSE", "ROLE", "STAGE",
    "STREAM", "TASK", "PIPE", "SEQUENCE", "PROCEDURE", "FUNCTION", "FORMAT",
    "INTEGRATION", "POLICY", "MASKING", "NETWORK", "USER", "INDEX",
    "CONSTRAINT", "PRIMARY", "KEY", "FOREIGN", "REFERENCES", "UNIQUE",
    "DEFAULT", "IDENTITY", "AUTOINCREMENT", "ENABLE", "DISABLE",
    "ENFORCE", "NOVALIDATE", "COLUMN", "TAG", "TRANSIENT", "STAGED", "SCHEDULE",
  ],

  dcl: [
    "GRANT", "REVOKE", "SHOW", "DESCRIBE", "DESC", "USE", "SET", "UNSET",
    "COMMENT", "EXPLAIN", "BEGIN", "START", "TRANSACTION", "COMMIT",
    "ROLLBACK", "SAVEPOINT",
  ],

  logic: [
    "AND", "OR", "NOT", "IN", "BETWEEN", "LIKE", "ILIKE", "REGEXP", "RLIKE",
    "IS", "EXISTS", "ANY", "ALL", "SOME", "AS", "DISTINCT",
  ],

  control: [
    "CASE", "WHEN", "THEN", "ELSE", "END", "IF", "IFF", "ELSEIF",
  ],

  scripting: [
    "DECLARE", "BEGIN", "EXCEPTION", "END", "EXIT", "RETURN", "CONTINUE",
    "RAISE", "OPEN", "FETCH", "CLOSE", "DEFAULT", "RESULTSET",
    "ASYNC", "AWAIT", "CANCEL", "LET", "VAR",
  ],

  scripting_loop: [
    "FOR", "IN", "REVERSE", "TO", "DO", "LOOP", "WHILE", "REPEAT", "UNTIL",
  ],

  // Special constants (coloured separately from NULL-the-keyword)
  constants: [
    "NULL", "TRUE", "FALSE", "UNKNOWN",
    "CURRENT_DATE", "CURRENT_TIME", "CURRENT_TIMESTAMP",
    "CURRENT_USER", "CURRENT_ROLE", "CURRENT_ACCOUNT",
    "CURRENT_REGION", "CURRENT_WAREHOUSE", "CURRENT_DATABASE", "CURRENT_SCHEMA",
    "SYSDATE", "NOW", "LOCALTIME", "LOCALTIMESTAMP",
  ],

  // Data types sourced from the generated registry artifact (single source of
  // truth: internal/snowflake/datatypes.go).  Multi-word entries such as
  // "DOUBLE PRECISION" never match the single-word tokenizer rule, so they are
  // harmless; "PRECISION" alone is intentionally no longer highlighted.
  datatypes: [...SNOWFLAKE_DATA_TYPE_NAMES],

  // Built-in functions — sourced from BUILTIN_FUNCTION_CATEGORIES (single source
  // of truth, also drives the Code Snippets "Built-in Functions" sub-categories).
  builtins: BUILTINS_FLAT,

  // ── Tokenizer rules ─────────────────────────────────────────────────────────

  tokenizer: {
    root: [
      // Whitespace
      [/[ \t\r\n]+/, "white"],

      // Stage URI (file:///path, s3://bucket, azure://…) in PUT/GET. Match the
      // scheme + `://…` whole so it never reaches the `//` line-comment rule
      // below. Monarch tries rules top-to-bottom, so this must precede it.
      [/[a-z][a-z0-9+.-]*:\/\/[^\s]*/i, "string"],

      // Line comment (Snowflake: both -- and //)
      [/(--|\/\/).*$/, "comment"],

      // Block comment
      [/\/\*/, { token: "comment.block", next: "@blockComment" }],

      // Single-quoted string literal
      [/'/, { token: "string", next: "@stringSingle" }],

      // Dollar-quoted markers (Snowflake Scripting / JS / Python bodies)
      // We treat these as delimiters so the content inside is highlighted as normal SQL.
      [/\$[a-zA-Z0-9_]*\$/, "delimiter.dollar"],

      // Double-quoted identifier
      [/"/, { token: "identifier.quoted", next: "@quotedIdent" }],

      // Hex number
      [/\b0x[\da-fA-F]+\b/, "number.hex"],

      // Float
      [/\b\d+\.\d+(?:[eE][+-]?\d+)?\b/, "number.float"],

      // Integer
      [/\b\d+\b/, "number"],

      // Keywords and identifiers — order of @cases matters: most specific first
      [
        /[a-zA-Z_]\w*/,
        {
          cases: {
            "@dml":       "keyword.dml",
            "@clause":    "keyword.clause",
            "@ddl":       "keyword.ddl",
            "@object":    "keyword.object",
            "@dcl":       "keyword.dcl",
            "@logic":     "keyword.logic",
            "@control":   "keyword.control",
            "@scripting": "keyword.scripting",
            "@scripting_loop": "keyword.scripting.loop",
            "@constants": "constant",
            "@datatypes": "type",
            "@builtins":  "predefined",
            "@default":   "identifier",
          },
        },
      ],

      // Type-cast operator (Snowflake: value::type)
      [/::/, "operator"],

      // Comparison / equality operators
      [/[!<>]=?|<>/, "operator"],

      // String concatenation
      [/\|\|/, "operator"],

      // Arithmetic
      [/[+\-*/%^]/, "operator"],

      // Named-parameter marker
      [/[?]/, "operator"],

      // Punctuation
      [/[;,]/, "delimiter"],
      [/\./, "delimiter.dot"],
      [/[()[\]{}]/, "delimiter.parenthesis"],
    ],

    blockComment: [
      [/[^*/]+/, "comment.block"],
      [/\*\//, { token: "comment.block", next: "@pop" }],
      [/[*/]/, "comment.block"],
    ],

    stringSingle: [
      [/[^']+/, "string"],
      [/''/, "string"],
      [/'/, { token: "string", next: "@pop" }],
    ],

    quotedIdent: [
      [/[^"]+/, "identifier.quoted"],
      [/""/, "identifier.quoted"],
      [/"/, { token: "identifier.quoted", next: "@pop" }],
    ],
  },
} as const;

// Context (session) functions — the callable subset of `constants` (excludes
// NULL/TRUE/FALSE/UNKNOWN). Shared by the Code Snippets modal and the editor's
// right-click "Built-in Functions" submenu so there's one source.
export const CONTEXT_FUNCTIONS: string[] = snowflakeMonarchLanguage.constants.filter(
  (c) => c.startsWith("CURRENT_") || ["SYSDATE", "NOW", "LOCALTIME", "LOCALTIMESTAMP"].includes(c),
);

// Canonical ordered category list (Context first, then the builtins by group).
// Single shape consumed by both the Code Snippets modal and the editor's
// right-click "Built-in Functions" submenu — assemble it once, here.
export const FUNCTION_CATEGORIES: { name: string; fns: string[] }[] = [
  { name: "Context", fns: CONTEXT_FUNCTIONS },
  ...Object.entries(BUILTIN_FUNCTION_CATEGORIES).map(([name, fns]) => ({ name, fns })),
];

// ─── Dark theme ───────────────────────────────────────────────────────────────
// GitHub dark palette: coral DML, azure clauses, peach DDL, lavender control,
// gold functions, mint types, amber constants, pale-blue strings.

export const thawDarkTheme = {
  base:    "vs-dark" as const,
  inherit: true,
  rules: [
    // DML — coral/red, bold  (SELECT, INSERT, UPDATE, DELETE, MERGE …)
    { token: "keyword.dml",      foreground: "ff7b72", fontStyle: "bold" },
    // Clause — azure blue      (FROM, WHERE, JOIN, GROUP, ORDER, OVER …)
    { token: "keyword.clause",   foreground: "79c0ff" },
    // DDL — peach/orange, bold  (CREATE, ALTER, DROP, TRUNCATE, COPY …)
    { token: "keyword.ddl",      foreground: "ffa657", fontStyle: "bold" },
    // Object keywords — same peach (TABLE, VIEW, SCHEMA, WAREHOUSE …)
    { token: "keyword.object",   foreground: "ffa657" },
    // DCL — same peach           (GRANT, REVOKE, SHOW, DESCRIBE, USE …)
    { token: "keyword.dcl",      foreground: "ffa657" },
    // Logic — azure (AND, OR, NOT, IN, BETWEEN, LIKE, AS, DISTINCT …)
    { token: "keyword.logic",    foreground: "79c0ff" },
    // Control flow — lavender   (CASE, WHEN, THEN, ELSE, END, IFF …)
    { token: "keyword.control",  foreground: "d2a8ff" },
    // Scripting — lavender, bold (DECLARE, BEGIN, EXCEPTION, END …)
    { token: "keyword.scripting", foreground: "d2a8ff", fontStyle: "bold" },
    // Scripting loops — lavender
    { token: "keyword.scripting.loop", foreground: "d2a8ff" },
    // Generic keyword fallback — matches any keyword.* token not caught above
    { token: "keyword",          foreground: "79c0ff" },
    // Constants — amber          (NULL, TRUE, FALSE, CURRENT_DATE …)
    { token: "constant",         foreground: "ffa657", fontStyle: "italic" },
    // Built-in functions — gold  (COUNT, SUM, DATEADD, COALESCE …)
    { token: "predefined",       foreground: "e3b341" },
    // Data types — mint green   (VARCHAR, NUMBER, TIMESTAMP, VARIANT …)
    { token: "type",             foreground: "7ee787" },
    // String literals — pale blue
    { token: "string",           foreground: "a5d6ff" },
    // f-string delimiters (Python string.escape tokens) — same pale blue
    { token: "string.escape",    foreground: "a5d6ff" },
    // Numbers — same azure as clauses (digits are never confused with words)
    { token: "number",           foreground: "79c0ff" },
    { token: "number.float",     foreground: "79c0ff" },
    { token: "number.hex",       foreground: "79c0ff" },
    // Comments — muted gray, italic
    { token: "comment",          foreground: "6e7681", fontStyle: "italic" },
    { token: "comment.block",    foreground: "6e7681", fontStyle: "italic" },
    // Decorators (Python @tag tokens) — gold, same as built-in functions
    { token: "tag",              foreground: "e3b341" },
    // Quoted identifiers — near white (same as plain identifiers)
    { token: "identifier.quoted", foreground: "e6edf3" },
    { token: "identifier",        foreground: "e6edf3" },
    // Operators and punctuation — muted
    { token: "operator",          foreground: "8b949e" },
    { token: "delimiter",         foreground: "8b949e" },
    { token: "delimiter.dot",     foreground: "8b949e" },
    { token: "delimiter.parenthesis", foreground: "8b949e" },
  ],
  colors: {
    "editor.background":                  "#0d1117",
    "editor.foreground":                  "#f0f6fc",
    "editor.lineHighlightBackground":     "#161b22",
    "editor.selectionBackground":         "#264f78",
    "editor.inactiveSelectionBackground": "#1c2b3a",
    "editorLineNumber.foreground":        "#6e7681",
    "editorLineNumber.activeForeground":  "#e6edf3",
    "editorCursor.foreground":            "#58a6ff",
    "editorIndentGuide.background1":      "#21262d",
    "editorIndentGuide.activeBackground1":"#30363d",
    "editor.findMatchBackground":         "#9e6a0340",
    "editor.findMatchHighlightBackground":"#1f6feb40",
  },
} as const;

// ─── Light theme ──────────────────────────────────────────────────────────────
// VS Code light palette: blue DML, purple DDL, brown functions, teal types.

export const thawLightTheme = {
  base:    "vs" as const,
  inherit: true,
  rules: [
    // DML — deep blue, bold
    { token: "keyword.dml",      foreground: "0550ae", fontStyle: "bold" },
    // Clause — deep blue
    { token: "keyword.clause",   foreground: "0550ae" },
    // DDL — purple, bold
    { token: "keyword.ddl",      foreground: "8250df", fontStyle: "bold" },
    // Object — purple
    { token: "keyword.object",   foreground: "8250df" },
    // DCL — purple
    { token: "keyword.dcl",      foreground: "8250df" },
    // Logic — deep blue
    { token: "keyword.logic",    foreground: "0550ae" },
    // Control flow — purple
    { token: "keyword.control",  foreground: "8250df" },
    // Scripting — purple, bold
    { token: "keyword.scripting", foreground: "8250df", fontStyle: "bold" },
    // Scripting loops — purple
    { token: "keyword.scripting.loop", foreground: "8250df" },
    // Generic keyword fallback — matches any keyword.* token not caught above
    { token: "keyword",          foreground: "0550ae" },
    // Constants — dark azure
    { token: "constant",         foreground: "0969da", fontStyle: "italic" },
    // Built-in functions — dark amber/brown
    { token: "predefined",       foreground: "953800" },
    // Data types — teal
    { token: "type",             foreground: "0a7266" },
    // String literals — dark red
    { token: "string",           foreground: "a31515" },
    // f-string delimiters (Python string.escape tokens) — same dark red
    { token: "string.escape",    foreground: "a31515" },
    // Numbers — dark green
    { token: "number",           foreground: "098658" },
    { token: "number.float",     foreground: "098658" },
    { token: "number.hex",       foreground: "098658" },
    // Comments — muted green, italic
    { token: "comment",          foreground: "6a737d", fontStyle: "italic" },
    { token: "comment.block",    foreground: "6a737d", fontStyle: "italic" },
    // Decorators (Python @tag tokens) — dark amber, same as built-in functions
    { token: "tag",              foreground: "953800" },
    // Identifiers
    { token: "identifier.quoted", foreground: "1f2328" },
    { token: "identifier",        foreground: "1f2328" },
    // Operators and punctuation
    { token: "operator",          foreground: "1f2328" },
    { token: "delimiter",         foreground: "1f2328" },
    { token: "delimiter.dot",     foreground: "1f2328" },
    { token: "delimiter.parenthesis", foreground: "1f2328" },
  ],
  colors: {
    "editor.background":                  "#ffffff",
    "editor.foreground":                  "#1f2328",
    "editor.lineHighlightBackground":     "#f6f8fa",
    "editor.selectionBackground":         "#add6ff",
    "editor.inactiveSelectionBackground": "#d0e8ff",
    "editorLineNumber.foreground":        "#636c76",
    "editorLineNumber.activeForeground":  "#1f2328",
    "editorCursor.foreground":            "#0969da",
  },
} as const;
