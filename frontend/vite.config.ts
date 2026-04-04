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

        // Skip vendor chunks — manualChunks ensures all node_modules output
        // files start with "vendor".
        if (/^vendor/.test(chunk.fileName)) continue;

        // Skip worker bundles — Vite derives the output filename from the
        // source module name, so yamlWorker → yamlWorker-[hash].js and
        // editor.worker → editor.worker-[hash].js.
        if (/[Ww]orker/.test(chunk.fileName)) continue;

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

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), appObfuscatorPlugin(), restoreGitkeepPlugin()],

  resolve: {
    alias: {
      "@": "/src",
    },
  },

  // Pre-bundle Monaco and its YAML plugin so Vite doesn't transform thousands
  // of ESM files on first request in dev mode.
  optimizeDeps: {
    include: ["monaco-editor", "monaco-yaml"],
  },

  build: {
    // Never emit source maps in the production bundle.  Source maps embed the
    // original TypeScript/JSX source, which would be readable inside the
    // shipped binary even after garble has obfuscated the Go layer.
    sourcemap: false,

    // Switch to Terser for aggressive, multi-pass minification.  esbuild (the
    // Vite default) is faster but does not support multiple compression passes
    // or fine-grained compress/mangle control.
    minify: "terser",
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
        manualChunks(id) {
          if (!id.includes("node_modules")) return undefined;
          // Isolate Monaco and its YAML worker in their own named chunk so
          // the skip pattern (/^vendor/) catches them by filename prefix.
          if (
            id.includes("monaco-editor") ||
            id.includes("monaco-yaml")
          ) {
            return "vendor-monaco";
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
