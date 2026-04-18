// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import type * as monacoLib from "monaco-editor";

export function getSnowflakeSnippets(monaco: typeof monacoLib): monacoLib.languages.CompletionItem[] {
  const range = { startLineNumber: 0, startColumn: 0, endLineNumber: 0, endColumn: 0 }; // Placeholder, Monaco fills it

  return [
    // 3.1. Main Block Structure
    {
      label: "block",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "DECLARE",
        "  ${1:-- variable and cursor declarations}",
        "BEGIN",
        "  ${2:-- Snowflake Scripting and SQL statements}",
        "EXCEPTION",
        "  WHEN ${3:exception_name} THEN",
        "    ${4:statement;}",
        "  WHEN OTHER THEN",
        "    ${5:statement;}",
        "END;",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Main Snowflake Scripting block structure (DECLARE, BEGIN, EXCEPTION, END)",
      range: range as any,
    },
    {
      label: "declare",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "DECLARE",
        "  ${1:-- variable and cursor declarations}",
        "BEGIN",
        "  ${2:-- Snowflake Scripting and SQL statements}",
        "END;",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting block (DECLARE, BEGIN, END)",
      range: range as any,
    },

    // 3.2. Variable Declarations
    {
      label: "var",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "${1:variable_name} ${2:type} DEFAULT ${3:expression};",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting variable declaration",
      range: range as any,
    },
    {
      label: "declare_var",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "${1:variable_name} ${2:type};",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting variable declaration (type only, no initializer)",
      range: range as any,
    },
    {
      label: "let_typed",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "LET ${1:variable_name} ${2:type} ${3|DEFAULT,:=|} ${4:expression};",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "LET variable declaration with explicit type",
      range: range as any,
    },
    {
      label: "let",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "LET ${1:variable_name} ${2|DEFAULT,:=|} ${3:expression};",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "LET variable declaration (type inferred)",
      range: range as any,
    },

    // 3.3. Conditional Statements
    {
      label: "if",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "IF (${1:condition}) THEN",
        "  ${2:-- true statements}",
        "ELSEIF (${3:condition_2}) THEN",
        "  ${4:-- elseif statements}",
        "ELSE",
        "  ${5:-- fallback statements}",
        "END IF;",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting IF-THEN-ELSEIF-ELSE statement",
      range: range as any,
    },
    {
      label: "case",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "CASE (${1:expression})",
        "  WHEN ${2:value_1} THEN",
        "    ${3:statement;}",
        "  WHEN ${4:value_2} THEN",
        "    ${5:statement;}",
        "  ELSE",
        "    ${6:statement;}",
        "END CASE;",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting CASE statement",
      range: range as any,
    },

    // 3.4. Loops
    {
      label: "for",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "FOR ${1:counter_variable} IN ${2:start} TO ${3:end} DO",
        "  ${4:statement;}",
        "END FOR ${5:label};",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting FOR loop",
      range: range as any,
    },
    {
      label: "for_reverse",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "FOR ${1:counter_variable} IN REVERSE ${2:start} TO ${3:end} DO",
        "  ${4:statement;}",
        "END FOR ${5:label};",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting FOR REVERSE loop",
      range: range as any,
    },
    {
      label: "while",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "WHILE (${1:condition}) DO",
        "  ${2:statement;}",
        "END WHILE ${3:label};",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting WHILE loop",
      range: range as any,
    },
    {
      label: "repeat",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "REPEAT",
        "  ${1:statement;}",
        "UNTIL (${2:condition})",
        "END REPEAT ${3:label};",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting REPEAT loop",
      range: range as any,
    },
    {
      label: "loop",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "LOOP",
        "  ${1:statement;}",
        "END LOOP ${2:label};",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting LOOP statement",
      range: range as any,
    },

    // 3.5. Cursors and Resultsets
    {
      label: "cursor_lifecycle",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "-- 1. Declare (Must be in DECLARE block)",
        "-- ${1:cursor_name} CURSOR FOR ${2:query};",
        "",
        "-- 2. Open",
        "OPEN ${1:cursor_name};",
        "",
        "-- 3. Fetch",
        "FETCH ${1:cursor_name} INTO ${3:variables};",
        "",
        "-- 4. Close",
        "CLOSE ${1:cursor_name};",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting cursor lifecycle (OPEN, FETCH, CLOSE)",
      range: range as any,
    },
    {
      label: "resultset",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "${1:rs_name} RESULTSET DEFAULT (EXECUTE IMMEDIATE '${2:SELECT ...}');",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting RESULTSET declaration",
      range: range as any,
    },
    {
      label: "execute_immediate",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: ["EXECUTE IMMEDIATE $$", "  ${1:sql_statement}", "$$;"].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Execute a SQL string dynamically (dollar-quoted block)",
      range: range as any,
    },
    {
      label: "execute_immediate_using",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "EXECUTE IMMEDIATE '${1:sql_statement}' USING (${2:variable});",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Execute a SQL string dynamically with bound variables",
      range: range as any,
    },

    // 3.6. Asynchronous Jobs
    {
      label: "async_job",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "ASYNC ${1:SELECT ...};",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting ASYNC job execution",
      range: range as any,
    },
    {
      label: "await_job",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "AWAIT ${1:job_id_or_resultset};",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting AWAIT job completion",
      range: range as any,
    },
    {
      label: "cancel_job",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "CANCEL ${1:job_id_or_resultset};",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Snowflake Scripting CANCEL job execution",
      range: range as any,
    },

    // 3.7. Generic DDL Statements
    {
      label: "create",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "CREATE OR REPLACE ${1|TABLE,VIEW,SCHEMA,DATABASE,STAGE,PROCEDURE,FUNCTION|}",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Create or replace a database object",
      range: range as any,
    },
    {
      label: "alter",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "ALTER ${1|TABLE,VIEW,SCHEMA,DATABASE,USER,ROLE,WAREHOUSE|}",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Alter a database object",
      range: range as any,
    },
    {
      label: "drop",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "DROP ${1|TABLE,VIEW,SCHEMA,DATABASE,USER,ROLE,WAREHOUSE|} IF EXISTS",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Safely drop a database object",
      range: range as any,
    },
    {
      label: "describe",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "DESCRIBE ${1|TABLE,VIEW,SCHEMA,DATABASE,USER,ROLE,WAREHOUSE|}",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Describe the structure of a database object",
      range: range as any,
    },
  ];
}

// ─── Python / Snowpark snippets ───────────────────────────────────────────────

export function getPythonSnippets(monaco: typeof monacoLib): monacoLib.languages.CompletionItem[] {
  const range = { startLineNumber: 0, startColumn: 0, endLineNumber: 0, endColumn: 0 };
  return [
    {
      label: "hello_world",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: 'print("Hello, World!")',
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Print Hello World",
      range: range as any,
    },
    {
      label: "imports",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "import pandas as pd",
        "import numpy as np",
        "from snowflake.snowpark.functions import col, lit, when",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Common Snowpark imports",
      range: range as any,
    },
    {
      label: "load_table",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: 'df = session.table("${1:DB.SCHEMA.TABLE_NAME}")',
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Load a Snowflake table into a Snowpark DataFrame",
      range: range as any,
    },
    {
      label: "show_df",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: "${1:df}.show()",
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Print DataFrame rows to output",
      range: range as any,
    },
    {
      label: "select_cols",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: '${1:df}.select(col("${2:col1}"), col("${3:col2}"))',
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Select specific columns from a DataFrame",
      range: range as any,
    },
    {
      label: "filter_df",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: '${1:df}.filter(col("${2:column_name}") ${3|==,!=,>,<,>=,<=|} "${4:value}")',
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Filter a Snowpark DataFrame by a column condition",
      range: range as any,
    },
    {
      label: "groupby",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        '${1:df}.group_by(col("${2:group_col}"))',
        '    .agg({"${3:value_col}": "${4|avg,sum,count,min,max|}"})',
        ".show()",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Group by and aggregate a Snowpark DataFrame",
      range: range as any,
    },
    {
      label: "join_df",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: '${1:df1}.join(${2:df2}, ${1:df1}["${3:key}"] == ${2:df2}["${3:key}"], "${4|inner,left,right,full|}")',
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Join two Snowpark DataFrames",
      range: range as any,
    },
    {
      label: "write_table",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: '${1:df}.write.save_as_table("${2:DB.SCHEMA.TABLE_NAME}", mode="${3|append,overwrite,truncate,errorifexists|}")',
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Write a Snowpark DataFrame to a Snowflake table",
      range: range as any,
    },
    {
      label: "run_sql",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: 'df = session.sql("${1:SELECT * FROM MYTABLE}")\ndf.show()',
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Run a SQL statement via the Snowpark session",
      range: range as any,
    },
    {
      label: "create_session",
      kind: monaco.languages.CompletionItemKind.Snippet,
      insertText: [
        "from snowflake.snowpark import Session",
        "",
        "connection_params = {",
        '    "account": "${1:account}",',
        '    "user": "${2:user}",',
        '    "password": "${3:password}",',
        '    "role": "${4:role}",',
        '    "warehouse": "${5:warehouse}",',
        '    "database": "${6:database}",',
        '    "schema": "${7:schema}",',
        "}",
        "",
        "session = Session.builder.configs(connection_params).create()",
      ].join("\n"),
      insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
      documentation: "Create a Snowpark session manually (Thaw auto-creates one on kernel start)",
      range: range as any,
    },
  ];
}

export const PYTHON_SNIPPET_CATEGORIES: {
  header: string;
  labels: string[];
  titles?: Record<string, string>;
}[] = [
  {
    header: "Basics",
    labels: ["hello_world", "imports"],
    titles: {
      hello_world: "print('Hello, World!')",
      imports:     "common imports",
    },
  },
  {
    header: "DataFrames",
    labels: ["load_table", "show_df", "select_cols", "filter_df", "groupby", "join_df", "write_table"],
    titles: {
      load_table:  "load table → df",
      show_df:     "df.show()",
      select_cols: "df.select(cols)",
      filter_df:   "df.filter(condition)",
      groupby:     "df.group_by().agg()",
      join_df:     "df.join(other)",
      write_table: "df.write.save_as_table()",
    },
  },
  {
    header: "SQL & Session",
    labels: ["run_sql", "create_session"],
    titles: {
      run_sql:        "session.sql(...).show()",
      create_session: "Session.builder.create()",
    },
  },
];

/**
 * Category groups used to build the snippet submenu.
 *
 * `titles` is an optional per-label display name override.  When present it
 * is used as the Monaco menu item title instead of the raw label, so the
 * internal command ID (thaw.snippet.<label>) can stay a clean identifier
 * while the menu shows a more descriptive string.
 */
export const SNIPPET_CATEGORIES: {
  header: string;
  labels: string[];
  titles?: Record<string, string>;
}[] = [
  { header: "Block Structure", labels: ["block", "declare"] },
  {
    header: "DECLARE Variables",
    labels: ["var", "declare_var"],
    titles: {
      var:         "declare var",
      declare_var: "declare var (type only)",
    },
  },
  {
    header: "LET Variables",
    labels: ["let_typed", "let"],
    titles: {
      let_typed: "let (typed)",
      let:       "let",
    },
  },
  { header: "Conditionals",         labels: ["if", "case"] },
  { header: "Loops",                labels: ["for", "for_reverse", "while", "repeat", "loop"] },
  { header: "Cursors & Resultsets", labels: ["cursor_lifecycle", "resultset", "execute_immediate", "execute_immediate_using"] },
  { header: "Async Jobs",           labels: ["async_job", "await_job", "cancel_job"] },
  
  { 
    header: "DDL Statements", 
    labels: ["create", "alter", "drop", "describe"],
    titles: {
      create: "CREATE OR REPLACE ...",
      alter: "ALTER ...",
      drop: "DROP IF EXISTS ...",
      describe: "DESCRIBE ..."
    } 
  },
];