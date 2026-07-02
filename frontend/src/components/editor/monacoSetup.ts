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
// ── Targeted Monaco imports (NOT the full "monaco-editor" barrel) ─────────────
// `monaco-editor` (editor.main.js) eagerly pulls every language service
// (TypeScript/HTML/CSS/JSON) plus ~80 basic-languages, each referencing a web
// worker.  We only use SQL (custom Monarch), an inline Python grammar, and YAML
// (via monaco-yaml's own worker) — so the TS/CSS/HTML/JSON worker bundles
// (~9 MB) and the basic-language grammars are dead weight embedded in the binary.
//
//   • editor.api → the Monaco namespace (editor, languages, KeyMod, Range, …)
//   • editor.all → all editor *features* (find, folding, comment, suggest,
//                  hover, multicursor, …) WITHOUT any language service.
//
// This is exactly editor.main minus the language contributions.  All three
// Monaco value-importers (this file, SqlEditor, NotebookTab) must import from
// editor.api so Vite never resolves the full barrel.
import * as monacoLib from "monaco-editor/esm/vs/editor/editor.api.js";
import "monaco-editor/esm/vs/editor/editor.all.js";
import { registerFindWidgetTooltipFix } from "../../utils/monacoTooltipFix";

// ── Inline Python Monarch grammar ─────────────────────────────────────────────
// Defined inline instead of importing from monaco-editor/esm/vs/basic-languages/python/python.js
// because that file begins with ~70 side-effect imports (identical to _.contribution.js)
// which can interfere with module initialisation order in Vite's ESM bundle.
// The grammar below is a faithful port of Monaco's built-in Python tokeniser.
const PYTHON_MONARCH_LANGUAGE = {
  defaultToken: "",
  tokenPostfix: ".python",
  keywords: [
    "False", "None", "True", "_", "and", "as", "assert", "async", "await",
    "break", "case", "class", "continue", "def", "del", "elif", "else",
    "except", "exec", "finally", "for", "from", "global", "if", "import",
    "in", "is", "lambda", "match", "nonlocal", "not", "or", "pass", "print",
    "raise", "return", "try", "type", "while", "with", "yield",
    // builtins treated as keywords by Monaco's built-in grammar:
    "int", "float", "long", "complex", "hex", "abs", "all", "any", "apply",
    "basestring", "bin", "bool", "buffer", "bytearray", "callable", "chr",
    "classmethod", "cmp", "coerce", "compile", "complex", "delattr", "dict",
    "dir", "divmod", "enumerate", "eval", "execfile", "file", "filter",
    "format", "frozenset", "getattr", "globals", "hasattr", "hash", "help",
    "id", "input", "intern", "isinstance", "issubclass", "iter", "len",
    "locals", "list", "map", "max", "memoryview", "min", "next", "object",
    "oct", "open", "ord", "pow", "print", "property", "reversed", "range",
    "raw_input", "reduce", "reload", "repr", "reversed", "round", "self",
    "set", "setattr", "slice", "sorted", "staticmethod", "str", "sum",
    "super", "tuple", "type", "unichr", "unicode", "vars", "xrange", "zip",
    "__dict__", "__methods__", "__members__", "__class__", "__bases__",
    "__name__", "__mro__", "__subclasses__", "__init__", "__import__",
  ],
  brackets: [
    { open: "{", close: "}", token: "delimiter.curly" },
    { open: "[", close: "]", token: "delimiter.bracket" },
    { open: "(", close: ")", token: "delimiter.parenthesis" },
  ],
  // A Monarch tokenizer is a small state machine used purely for *syntax
  // highlighting* (it does not parse Python — it only classifies spans of text).
  // Each state is an ordered list of rules `[regex, tokenClass, nextState?]`:
  // for the text at the current cursor, Monaco tries the rules top-to-bottom,
  // the first regex that matches consumes that text and tags it with the given
  // token class (which the theme then colours, e.g. "keyword", "string",
  // "comment"), and the optional third element pushes/pops a state so multi-line
  // constructs like triple-quoted strings keep their highlighting across lines.
  // `@name` references another state/list; `@brackets`/`@keywords` reference the
  // bracket and keyword tables declared above.
  tokenizer: {
    // Top-level state: whitespace/comments, then numbers, then strings, then
    // punctuation, decorators (@foo), and finally words — classified as a
    // keyword when they appear in `keywords`, otherwise a plain identifier.
    root: [
      { include: "@whitespace" },
      { include: "@numbers" },
      { include: "@strings" },
      [/[,:;]/, "delimiter"],                 // separators
      [/[{}\[\]()]/, "@brackets"],            // brackets (matched via the brackets table)
      [/@[a-zA-Z_]\w*/, "tag"],               // decorators, e.g. @staticmethod
      // A word: `keyword` if it's in the keyword list above, else `identifier`.
      [/[a-zA-Z_]\w*/, { cases: { "@keywords": "keyword", "@default": "identifier" } }],
    ],
    // Whitespace, `#` line comments, and the opening of triple-quoted strings.
    // A triple quote switches into a dedicated state so the whole multi-line
    // block stays highlighted as a string until the closing triple quote.
    whitespace: [
      [/\s+/, "white"],                       // runs of spaces/tabs/newlines
      [/(#.*$)/, "comment"],                  // `# …` to end of line
      [/'''/, "string", "@endDocString"],     // start of '''…''' docstring
      [/"""/, "string", "@endDblDocString"],  // start of """…""" docstring
    ],
    // Inside a '''…''' block: stay here colouring text as string until '''.
    endDocString: [
      [/[^']+/, "string"],                    // any run without a single quote
      [/\\'/, "string"],                      // escaped quote stays inside
      [/'''/, "string", "@popall"],           // closing triple quote → exit
      [/'/, "string"],                        // a lone quote (not the closer)
    ],
    // Inside a """…""" block: same as above but for double quotes.
    endDblDocString: [
      [/[^"]+/, "string"],
      [/\\"/, "string"],
      [/"""/, "string", "@popall"],
      [/"/, "string"],
    ],
    // Numeric literals. Hex first (so 0x… isn't split), then int/float with an
    // optional fraction, exponent, and j/l (complex/long) suffixes.
    numbers: [
      [/-?0x([abcdef]|[ABCDEF]|\d)+[lL]?/, "number.hex"],       // 0x1F, -0xABl
      [/-?(\d*\.)?\d+([eE][+\-]?\d+)?[jJ]?[lL]?/, "number"],    // 42, 3.14, 1e-9, 2j
    ],
    // Opening of a single-line string. The opening quote is tagged
    // "string.escape" (the quote glyph) and we branch into a body state by
    // quote style: f-strings (f'…' / f"…") use the f* bodies so `{…}`
    // interpolations get their own highlighting; plain strings use the others.
    // A quote immediately at end-of-line (`'$`) is an empty/unterminated string,
    // so just pop.
    strings: [
      [/'$/, "string.escape", "@popall"],
      [/f'{1,3}/, "string.escape", "@fStringBody"],   // f'…' (1–3 opening quotes)
      [/'/, "string.escape", "@stringBody"],
      [/"$/, "string.escape", "@popall"],
      [/f"{1,3}/, "string.escape", "@fDblStringBody"], // f"…"
      [/"/, "string.escape", "@dblStringBody"],
    ],
    // Body of a single-quoted f-string. Runs of plain text are coloured string;
    // a `{` opens an interpolation handled by fStringDetail; `\.` is an escape;
    // a closing `'` ends the string. The `…$` variants pop at line end.
    fStringBody: [
      [/[^\\'\{\}]+$/, "string", "@popall"],   // text to end of line → close
      [/[^\\'\{\}]+/, "string"],               // text (no backslash/quote/brace)
      [/\{[^\}':!=]+/, "identifier", "@fStringDetail"], // `{expr` → interpolation
      [/\\./, "string"],                       // escape sequence, e.g. \n, \'
      [/'/, "string.escape", "@popall"],       // closing quote
      [/\\$/, "string"],                       // trailing line-continuation
    ],
    // Body of a plain single-quoted string (no `{…}` interpolation handling).
    stringBody: [
      [/[^\\']+$/, "string", "@popall"],
      [/[^\\']+/, "string"],
      [/\\./, "string"],
      [/'/, "string.escape", "@popall"],
      [/\\$/, "string"],
    ],
    // Body of a double-quoted f-string (mirror of fStringBody for `"`).
    fDblStringBody: [
      [/[^\\"\{\}]+$/, "string", "@popall"],
      [/[^\\"\{\}]+/, "string"],
      [/\{[^\}':!=]+/, "identifier", "@fStringDetail"],
      [/\\./, "string"],
      [/"/, "string.escape", "@popall"],
      [/\\$/, "string"],
    ],
    // Body of a plain double-quoted string (mirror of stringBody for `"`).
    dblStringBody: [
      [/[^\\"]+$/, "string", "@popall"],
      [/[^\\"]+/, "string"],
      [/\\./, "string"],
      [/"/, "string.escape", "@popall"],
      [/\\$/, "string"],
    ],
    // Inside an f-string `{…}` interpolation: a `:format`, `!a/!r/!s`
    // conversion, or `=` debug spec stays string-coloured; the closing `}`
    // ends the interpolation and pops back to the string body.
    fStringDetail: [
      [/[:][^}]+/, "string"],   // :format_spec
      [/[!][ars]/, "string"],   // !a / !r / !s conversion
      [/=/, "string"],          // f"{x=}" debug form
      [/\}/, "identifier", "@pop"],
    ],
  },
} as const;

const PYTHON_LANGUAGE_CONF = {
  comments: {
    lineComment: "#",
    blockComment: ["'''", "'''"] as [string, string],
  },
  brackets: [
    ["{", "}"],
    ["[", "]"],
    ["(", ")"],
  ] as [string, string][],
  autoClosingPairs: [
    { open: "{", close: "}" },
    { open: "[", close: "]" },
    { open: "(", close: ")" },
    { open: '"', close: '"', notIn: ["string"] },
    { open: "'", close: "'", notIn: ["string", "comment"] },
  ],
  surroundingPairs: [
    { open: "{", close: "}" },
    { open: "[", close: "]" },
    { open: "(", close: ")" },
    { open: '"', close: '"' },
    { open: "'", close: "'" },
  ],
  folding: {
    offSide: true,
    markers: {
      start: /^\s*#region\b/,
      end: /^\s*#endregion\b/,
    },
  },
};

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

  // Find-widget tooltip fix (issue #593). A global editor-creation hook, so it's
  // decoupled from any per-editor clipboard wiring and covers every Monaco mount.
  registerFindWidgetTooltipFix(m.editor);

  // ── Register the languages we use ─────────────────────────────────────────
  // The slim Monaco import (editor.api + editor.all, see top of file) drops the
  // basic-language contributions, so `sql` and `python` are no longer
  // auto-registered.  They MUST be registered before setMonarchTokensProvider /
  // setLanguageConfiguration, otherwise Monaco throws "Cannot set configuration
  // for unknown language sql" and takes down the whole editor.  (`yaml` is
  // registered by monaco-yaml's configureMonacoYaml below, so it is omitted.)
  for (const id of ["sql", "python"]) {
    if (!m.languages.getLanguages().some((l: { id: string }) => l.id === id)) {
      m.languages.register({ id });
    }
  }

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

  // ── Python tokenizer (eager, inline grammar) ─────────────────────────────
  // python.contribution.js registers a *lazy* factory that fires only when the
  // first Python model is created.  We register our own compiled tokenizer
  // synchronously here so that Python cells are highlighted on first render.
  // The inline grammar (defined at the top of this file) avoids importing
  // python.js directly, which starts with ~70 side-effect contrib imports that
  // can disrupt module initialisation order in Vite's ESM bundle.
  m.languages.setMonarchTokensProvider("python", PYTHON_MONARCH_LANGUAGE);
  m.languages.setLanguageConfiguration("python", PYTHON_LANGUAGE_CONF);

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
