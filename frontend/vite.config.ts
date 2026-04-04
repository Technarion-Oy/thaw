import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
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
  },
  server: {
    watch: {
      // Wails regenerates these files during dev-mode startup; ignore them so
      // Vite doesn't trigger a full page reload when runtime.js is written.
      ignored: ["**/wailsjs/**"],
    },
  },
});
