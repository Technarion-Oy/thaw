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
  server: {
    watch: {
      // Wails regenerates these files during dev-mode startup; ignore them so
      // Vite doesn't trigger a full page reload when runtime.js is written.
      ignored: ["**/wailsjs/**"],
    },
  },
});
