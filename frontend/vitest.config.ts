import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // Run test files next to source (*.test.ts)
    include: ["src/**/*.test.ts"],
    environment: "node",
    // No browser globals needed for the pure-function formatter tests
    globals: false,
    // Vitest stubs CSS imports to an empty string by default. The object-icon
    // coverage test reads styles/global.css (as ?raw) to verify every kind's
    // colour variable is actually declared, so CSS has to be processed rather
    // than stubbed. No other test imports CSS.
    css: true,
  },
});
