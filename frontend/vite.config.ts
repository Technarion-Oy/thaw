import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import obfuscator from "javascript-obfuscator";
import { writeFileSync } from "fs";
import { join } from "path";

// ── App-code obfuscation plugin ───────────────────────────────────────────────
//
// Runs after Terser has minified every chunk.  Only app-authored chunks are
// processed; vendor and worker chunks are explicitly skipped because:
//
//   • Monaco uses internal postMessage protocols that break under identifier
//     or property-key mangling.
//   • Worker bundles communicate via structured-clone messages with fixed
//     field names; renaming those fields would break the handshake.
//   • Third-party vendor code is already minified and obfuscating it yields
//     no IP-protection benefit while ballooning build time and bundle size.
//
// manualChunks (below) guarantees that all node_modules output chunks have
// filenames that start with "vendor", giving the skip test a stable anchor.
function appObfuscatorPlugin(): Plugin {
  return {
    name: "thaw:obfuscate-app",
    apply: "build",
    enforce: "post",
    generateBundle(_opts, bundle) {
      for (const [, chunk] of Object.entries(bundle)) {
        if (chunk.type !== "chunk") continue;

        // chunk.fileName includes the output dir (e.g. "assets/vendor-xlsx-….js"),
        // so the skip tests must run against the *basename* — an earlier "^vendor"
        // test silently never matched and every vendor chunk was being obfuscated
        // (inflating each ~5×, e.g. xlsx 0.4 MB → 2.4 MB in the binary).
        const base = chunk.fileName.split("/").pop() ?? chunk.fileName;

        // Skip vendor chunks — manualChunks ensures all node_modules output
        // files start with "vendor".  Obfuscating third-party code yields no
        // IP-protection benefit, balloons the binary, and risks breaking libs
        // that rely on stable property names.
        if (base.startsWith("vendor")) continue;

        // Skip worker bundles — Vite derives the output filename from the
        // source module name, so yamlWorker → yamlWorker-[hash].js and
        // editor.worker → editor.worker-[hash].js.
        if (/[Ww]orker/.test(base)) continue;

        // Belt-and-suspenders: skip any chunk that still contains Monaco
        // modules regardless of filename (e.g. dynamic imports that escaped
        // manualChunks grouping).
        if (
          chunk.moduleIds.some(
            (id) =>
              id.includes("monaco-editor") || id.includes("monaco-yaml"),
          )
        )
          continue;

        try {
          chunk.code = obfuscator
            .obfuscate(chunk.code, {
              compact: true,

              // Control-flow flattening restructures if/switch/for blocks into
              // switch-based dispatch loops, making static analysis hard.
              // Disabled: each flattened function multiplies AST node count
              // several times; combined with Terser + string-array transforms
              // this OOMs the V8 heap even at low thresholds.  String-array
              // encoding (below) provides the primary IP protection instead.
              controlFlowFlattening: false,

              // Dead-code injection inserts unreachable branches — disabled
              // for the same heap-budget reasons as controlFlowFlattening.
              deadCodeInjection: false,

              // Replace all local identifier names with _0x<hex> sequences.
              identifierNamesGenerator: "hexadecimal",

              // Do NOT rename globals (window, document, React, …) — doing so
              // breaks runtime references that must resolve in the global scope.
              renameGlobals: false,

              // String array: all string literals are extracted into a shared
              // encoded array; accesses go through an indirection function
              // rather than inline literals, defeating simple string search.
              //
              // RC4 encoding is used instead of base64.  The base64 path in
              // javascript-obfuscator calls encodeURIComponent internally,
              // which throws "URI malformed" for any string literal that
              // contains characters outside Latin-1 (lone surrogates, emoji,
              // multi-byte codepoints from SQL regexes / Monaco token rules).
              // RC4 operates on raw bytes and has no such restriction.
              stringArray: true,
              rotateStringArray: true,
              shuffleStringArray: true,
              stringArrayCallsTransform: true,
              stringArrayCallsTransformThreshold: 0.75,
              stringArrayEncoding: ["rc4"],
              stringArrayIndexShift: true,
              stringArrayRotate: true,
              stringArrayShuffle: true,
              stringArrayThreshold: 0.75,
              stringArrayWrappersCount: 2,
              stringArrayWrappersChainedCalls: true,
              stringArrayWrappersParametersMaxCount: 4,
              stringArrayWrappersType: "function",

              // Disabled — would break the app or cause unacceptable size growth:
              //   selfDefending     – eval-loop watchdog breaks in WKWebView's
              //                       strict CSP environment.
              //   splitStrings      – multiplies bundle size; SQL keyword strings
              //                       alone would add hundreds of KB.
              //   transformObjectKeys – mangles React prop names (className, …).
              //   unicodeEscapeSequence – makes bundles ~3× larger for no gain.
              selfDefending: false,
              splitStrings: false,
              transformObjectKeys: false,
              unicodeEscapeSequence: false,
            })
            .getObfuscatedCode();
        } catch (err) {
          // Surface the chunk name so it can be investigated, but do not
          // abort the build — leave the Terser-minified code in place.
          console.warn(
            `[thaw:obfuscate-app] skipped "${chunk.fileName}": ${err}`,
          );
        }
      }
    },
  };
}

// ── .gitkeep restore plugin ───────────────────────────────────────────────────
//
// Vite's emptyOutDir (the default) deletes every file in dist/ before writing
// the new bundle — including the committed frontend/dist/.gitkeep placeholder.
// That file must survive so Go's //go:embed all:frontend/dist directive never
// sees an empty directory on a fresh checkout (binding generation runs before
// the frontend build).  This plugin re-creates the empty file after each build.
function restoreGitkeepPlugin(): Plugin {
  return {
    name: "thaw:restore-gitkeep",
    apply: "build",
    closeBundle() {
      writeFileSync(join(__dirname, "dist/.gitkeep"), "");
    },
  };
}

// THAW_FAST_BUILD=1 skips the production-only minify/obfuscation passes
// (Terser multi-pass + javascript-obfuscator).  Used by the CI build-check
// workflow, which only needs to verify the app compiles, links, and bundles —
// not produce a shipping artifact.  Release builds leave it unset.
const fastBuild = process.env.THAW_FAST_BUILD === "1";

// Packages reached only through lazy-loaded visualization modals (recharts'
// charts and @xyflow/@dagrejs graphs) plus their shared d3/victory transitive
// deps.  Grouped into a single on-demand "vendor-viz" chunk so none of them sit
// in the eager boot bundle.  Listed explicitly (rather than a broad /d3/ regex)
// so an unrelated future dependency can't accidentally land in this chunk.
const VIZ_DEPS = [
  "recharts",
  "victory-vendor",
  "internmap",
  "decimal.js-light",
  "d3-array",
  "d3-color",
  "d3-dispatch",
  "d3-drag",
  "d3-ease",
  "d3-format",
  "d3-interpolate",
  "d3-path",
  "d3-scale",
  "d3-selection",
  "d3-shape",
  "d3-time",
  "d3-time-format",
  "d3-timer",
  "d3-transition",
  "d3-zoom",
  "@xyflow",
  "@dagrejs",
];

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    // Obfuscation is the slowest, most memory-hungry pass — skip it entirely
    // in the fast CI build-check (its output is never shipped).
    ...(fastBuild ? [] : [appObfuscatorPlugin()]),
    restoreGitkeepPlugin(),
  ],

  resolve: {
    alias: {
      "@": "/src",
    },
  },

  // Pre-bundle Monaco and its YAML plugin so Vite doesn't transform thousands
  // of ESM files on first request in dev mode.  We import the *slim* editor
  // (editor.api + editor.all) rather than the full "monaco-editor" barrel, so
  // pre-bundling only the subpaths we actually use keeps `wails dev` cold start
  // fast and avoids a mid-session re-optimize page reload.
  optimizeDeps: {
    include: [
      "monaco-editor/esm/vs/editor/editor.api.js",
      "monaco-editor/esm/vs/editor/editor.all.js",
      "monaco-yaml",
    ],
  },

  build: {
    // Never emit source maps in the production bundle.  Source maps embed the
    // original TypeScript/JSX source, which would be readable inside the
    // shipped binary even after garble has obfuscated the Go layer.
    sourcemap: false,

    // Switch to Terser for aggressive, multi-pass minification.  esbuild (the
    // Vite default) is faster but does not support multiple compression passes
    // or fine-grained compress/mangle control.  The fast CI build-check falls
    // back to esbuild — it only needs to confirm the bundle builds, not ship it.
    minify: fastBuild ? "esbuild" : "terser",
    terserOptions: {
      compress: {
        // Strip all console calls and debugger statements so no internal
        // messages or breakpoints are present in the shipped binary.
        drop_console: true,
        drop_debugger: true,
        // Two passes: each pass unlocks further simplifications from the
        // previous one (dead branches, constant folding, inlining, …).
        // Reduced from 3 to keep Terser's peak RSS within budget when
        // javascript-obfuscator is also live in the same Node process.
        passes: 2,
      },
      // Mangle local variable and function names to short identifiers.
      mangle: true,
      format: {
        // Strip all comments, including third-party licence banners.
        comments: false,
      },
    },

    rollupOptions: {
      output: {
        // Explicit chunk groups give the obfuscator plugin reliable, stable
        // filenames to test against when deciding what to skip.
        //
        // Every node_modules chunk is named "vendor*", which (a) lets the
        // obfuscator skip it by filename and (b) lets Rollup load it only when
        // first reached.  Peeling the on-demand-only ecosystems into their own
        // "vendor-*" chunks means they are NOT pulled into the eager boot chunk:
        // combined with the React.lazy boundaries (terminal, notebook, chart /
        // ER / task-graph modals, Excel export) they load only when used.
        manualChunks(id) {
          if (!id.includes("node_modules")) return undefined;
          // Isolate Monaco and its YAML plugin in their own named chunk —
          // Monaco's postMessage protocols break under obfuscation, and the
          // /^vendor/ filename prefix keeps the obfuscator off it.
          if (
            id.includes("monaco-editor") ||
            id.includes("monaco-yaml") ||
            id.includes("monaco-worker-manager") ||
            id.includes("monaco-marker-data-provider") ||
            id.includes("monaco-languageserver-types")
          ) {
            return "vendor-monaco";
          }
          // xlsx — only needed for Excel export (lazy dynamic import).
          if (id.includes("/xlsx/")) return "vendor-xlsx";
          // xterm — only when the embedded terminal panel opens.
          if (id.includes("/@xterm/")) return "vendor-xterm";
          // leaflet — only when a GeoJSON cell's Map view is opened (lazy).
          if (id.includes("/leaflet/")) return "vendor-leaflet";
          // Visualization stack: recharts (Quick Chart / Warehouse Metering)
          // and @xyflow/@dagrejs (ER diagram / task graph), plus their shared
          // d3 / victory dependencies.  All reached only via lazy modals.
          if (VIZ_DEPS.some((p) => id.includes(`/node_modules/${p}/`))) {
            return "vendor-viz";
          }
          return "vendor";
        },
      },
    },
  },

  server: {
    watch: {
      // Wails regenerates these files during dev-mode startup; ignore them so
      // Vite doesn't trigger a full page reload when runtime.js is written.
      ignored: ["**/wailsjs/**"],
    },
  },
});
