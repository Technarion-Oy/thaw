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
import { getSnowflakeSnippets } from "./snowflakeSnippets";
import { configureMonacoYaml } from "monaco-yaml";
import YamlWorker from "./yamlWorker?worker";
import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";
import { loader } from "@monaco-editor/react";
import * as monacoLib from "monaco-editor";

// Import the bundled dbt JSON schemas.
// resolveJsonModule: true in tsconfig makes these available as plain objects.
import dbtProjectSchema from "../../schemas/dbt/dbt_project-latest.json";
import dbtYmlFilesSchema from "../../schemas/dbt/dbt_yml_files-latest.json";
import packagesSchema from "../../schemas/dbt/packages-latest.json";
import selectorsSchema from "../../schemas/dbt/selectors-latest.json";

// ── Use locally bundled Monaco instead of CDN ─────────────────────────────────
// By default @monaco-editor/loader fetches Monaco from jsDelivr at runtime.
// In a desktop Wails app (WKWebView) that means CDN Monaco (UMD/AMD bundle)
// while monaco-yaml and the editor workers are built from the local ESM package
// — two different module instances that cannot communicate via postMessage.
// Telling the loader to use the local package fixes the mismatch.
loader.config({ monaco: monacoLib });

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

  // ── Snowflake Scripting Snippets ──────────────────────────────────────────
  m.languages.registerCompletionItemProvider("sql", {
    provideCompletionItems: () => ({
      suggestions: getSnowflakeSnippets(m),
    }),
  });

  // ── Compatibility shim: monaco-worker-manager@2.x ↔ Monaco v0.55.x ────────
  //
  // monaco-yaml@5.4.1 uses monaco-worker-manager@2.0.1, which calls
  //   monaco.editor.createWebWorker({ createData, label, moduleId })
  // That was the old Monaco API.  In v0.55.x the standalone createWebWorker was
  // updated: it now expects opts.worker (a Worker or Promise<Worker>).  When
  // opts.worker is absent the implementation cannot locate the worker and falls
  // back silently to a local EditorWorker stub — YAML completions/hover/
  // validation never reach the actual yaml.worker bundle.
  //
  // Fix: intercept legacy-style calls (opts.moduleId / opts.createData present
  // but opts.worker absent) and bridge them to the new API:
  //   1. Obtain the real Worker via MonacoEnvironment.getWorker (returns our
  //      Vite-bundled YamlWorker for label "yaml").
  //   2. Post two bootstrap messages to the raw Worker before handing it off:
  //      – "ignore": consumed by monaco-worker-manager/worker.js's outer
  //        self.onmessage, which then installs the vs/common/initialize.js
  //        handler that waits for the *next* message.
  //      – createData: consumed by that new handler, which calls start() and
  //        builds the WebWorkerServer — after this the worker can respond to
  //        the RPC $initialize message that WebWorkerClient sends.
  //   3. Pass the Promise<Worker> as opts.worker to the original createWebWorker
  //      so Monaco's WebWorkerClient/$initialize handshake completes normally.
  //
  // This keeps monaco-yaml working without upgrading or forking either package.
  const origCreateWebWorker = (m.editor.createWebWorker as Function).bind(m.editor);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  m.editor.createWebWorker = (opts: any) => {
    if (opts && !opts.worker && (opts.moduleId != null || opts.createData != null)) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const env = (self as any).MonacoEnvironment;
      const workerPromise: Promise<Worker> = Promise.resolve(
        env.getWorker("workerMain.js", opts.label ?? "anonymous") as Worker
      ).then((w: Worker) => {
        w.postMessage("ignore");          // triggers outer self.onmessage → installs real handler
        w.postMessage(opts.createData ?? null); // triggers real handler → calls start() → WebWorkerServer ready
        return w;
      });
      return origCreateWebWorker({
        worker: workerPromise,
        keepIdleModels: opts.keepIdleModels ?? false,
      });
    }
    return origCreateWebWorker(opts);
  };

  // ── dbt JSON Schema validation for YAML files ─────────────────────────────
  //
  // configureMonacoYaml wires the YAML language service (running in a web
  // worker) to provide validation, hover docs, and autocompletion driven by
  // the bundled dbt-jsonschema schemas.
  //
  // fileMatch patterns are tested against each model's URI string.  SqlEditor
  // passes the tab's real file path prefixed with "file://" as the Monaco
  // model path when in YAML mode, so the URI looks like
  // "file:///Users/…/dbt_project.yml" — the **/name.yml glob patterns
  // resolve correctly against absolute file URIs.
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
