#!/usr/bin/env node
// ESM script — updates wails.json productVersion.
// Usage: node scripts/update-version.js <newVersion>

import { readFileSync, writeFileSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));

const [, , newVersion] = process.argv;

if (!newVersion || !/^\d+\.\d+\.\d+$/.test(newVersion)) {
  console.error(`Error: invalid version "${newVersion}". Expected format: X.Y.Z`);
  process.exit(1);
}

const wailsJsonPath = resolve(__dirname, "..", "wails.json");
const raw = readFileSync(wailsJsonPath, "utf8");
const config = JSON.parse(raw);

const oldVersion = config?.info?.productVersion ?? "(unset)";
config.info.productVersion = newVersion;

writeFileSync(wailsJsonPath, JSON.stringify(config, null, 2) + "\n", "utf8");
console.log(`wails.json: productVersion ${oldVersion} → ${newVersion}`);
