// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { snowflakeMonarchLanguage, thawDarkTheme, thawLightTheme } from "./snowflakeSql";
import { configureMonacoYaml } from "monaco-yaml";
import YamlWorker from "./yamlWorker?worker";
import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";

// Import the bundled dbt JSON schemas.
// resolveJsonModule: true in tsconfig makes these available as plain objects.
import dbtProjectSchema from "../../schemas/dbt/dbt_project-latest.json";
import dbtYmlFilesSchema from "../../schemas/dbt/dbt_yml_files-latest.json";
import packagesSchema from "../../schemas/dbt/packages-latest.json";
import selectorsSchema from "../../schemas/dbt/selectors-latest.json";

// Set up MonacoEnvironment **before** any editor or language worker is created.
// The YAML worker is served via Vite's ?worker bundling; the editor worker
// handles all other labels (css, json, typescript, …).
(self as unknown as Record<string, unknown>).MonacoEnvironment = {
  getWorker(_: string, label: string): Worker {
    if (label === "yaml") return new YamlWorker();
    return new EditorWorker();
  },
};

let registered = false;

export function ensureMonacoSetup(monaco: unknown): void {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const m = monaco as any;
  if (registered) return;
  registered = true;

  // ── SQL tokenizer & language configuration ────────────────────────────────
  m.languages.setMonarchTokensProvider("sql", snowflakeMonarchLanguage as any);

  // Declare SQL comment characters so editor.action.commentLine knows to use "--".
  m.languages.setLanguageConfiguration("sql", {
    comments: {
      lineComment: "--",
      blockComment: ["/*", "*/"],
    },
    brackets: [["(", ")"], ["[", "]"]],
    autoClosingPairs: [
      { open: "(", close: ")" },
      { open: "[", close: "]" },
      { open: "'", close: "'" },
      { open: '"', close: '"' },
    ],
    surroundingPairs: [
      { open: "(", close: ")" },
      { open: "[", close: "]" },
      { open: "'", close: "'" },
      { open: '"', close: '"' },
    ],
  });

  m.editor.defineTheme("thaw-dark",  thawDarkTheme  as any);
  m.editor.defineTheme("thaw-light", thawLightTheme as any);

  // ── dbt JSON Schema validation for YAML files ─────────────────────────────
  //
  // configureMonacoYaml wires the YAML language service (running in a web
  // worker) to provide validation, hover docs, and autocompletion driven by
  // the bundled dbt-jsonschema schemas.
  //
  // fileMatch patterns are tested against each model's URI string.  SqlEditor
  // passes the tab's real file path as the Monaco model path when in YAML
  // mode, so the URI looks like "/Users/…/dbt_project.yml" — the **/name.yml
  // glob patterns resolve correctly against absolute paths.
  //
  // Schema priority (highest → lowest):
  //   1. dbt_project.yml  → dbt_project-latest.json
  //   2. packages.yml / dependencies.yml → packages-latest.json
  //   3. selectors.yml    → selectors-latest.json
  //   4. everything else *.yml → dbt_yml_files-latest.json   (covers model
  //      configs, sources, seeds, snapshots, exposures, metrics, …)
  //
  // All schemas are bundled locally — no network request at runtime.
  configureMonacoYaml(m, {
    enableSchemaRequest: false,
    hover: true,
    completion: true,
    validate: true,
    format: true,
    schemas: [
      {
        uri: "dbt-jsonschema://dbt_project",
        fileMatch: ["**/dbt_project.yml", "**/dbt_project.yaml"],
        schema: dbtProjectSchema as object,
      },
      {
        uri: "dbt-jsonschema://packages",
        fileMatch: [
          "**/packages.yml",
          "**/packages.yaml",
          "**/dependencies.yml",
          "**/dependencies.yaml",
        ],
        schema: packagesSchema as object,
      },
      {
        uri: "dbt-jsonschema://selectors",
        fileMatch: ["**/selectors.yml", "**/selectors.yaml"],
        schema: selectorsSchema as object,
      },
      {
        // Catch-all for all other dbt YAML files: model configs, sources,
        // seeds, snapshots, analyses, exposures, metrics, etc.
        uri: "dbt-jsonschema://dbt_yml_files",
        fileMatch: ["**/*.yml", "**/*.yaml"],
        schema: dbtYmlFilesSchema as object,
      },
    ],
  });
}
