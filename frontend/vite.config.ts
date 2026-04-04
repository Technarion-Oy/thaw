import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import obfuscator from "javascript-obfuscator";

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

        chunk.code = obfuscator
          .obfuscate(chunk.code, {
            compact: true,

            // Control-flow flattening restructures if/switch/for blocks into
            // switch-based dispatch loops, making static analysis hard.
            // Threshold 0.3 (vs the default 0.75) processes ~30 % of
            // functions: enough obfuscation while keeping peak V8 heap
            // within the 6 GB budget on the macOS arm64 CI runner.
            // Each flattened function multiplies AST node count several
            // times, so higher thresholds OOM on 7 GB runners when
            // Terser is also live in the same Node process.
            controlFlowFlattening: true,
            controlFlowFlatteningThreshold: 0.3,

            // Dead-code injection inserts unreachable branches so decompilers
            // cannot cleanly reconstruct the original logic structure.
            deadCodeInjection: true,
            deadCodeInjectionThreshold: 0.2,

            // Replace all local identifier names with _0x<hex> sequences.
            identifierNamesGenerator: "hexadecimal",

            // Do NOT rename globals (window, document, React, …) — doing so
            // breaks runtime references that must resolve in the global scope.
            renameGlobals: false,

            // String array: all string literals are extracted into a shared
            // encoded array; accesses go through an indirection function
            // rather than inline literals, defeating simple string search.
            stringArray: true,
            rotateStringArray: true,
            shuffleStringArray: true,
            stringArrayCallsTransform: true,
            stringArrayCallsTransformThreshold: 0.75,
            stringArrayEncoding: ["base64"],
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
      }
    },
  };
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), appObfuscatorPlugin()],

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
