import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // Run test files next to source (*.test.ts)
    include: ["src/**/*.test.ts"],
    environment: "node",
    // No browser globals needed for the pure-function formatter tests
    globals: false,
  },
});
