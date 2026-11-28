#!/usr/bin/env node
// ESM script — updates the application version across build/config.yml and
// macOS Info*.plist files during semantic-release.
// Usage: node scripts/update-version.js <newVersion>

import { readFileSync, writeFileSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const rootDir = resolve(__dirname, "..");

const [, , newVersion] = process.argv;

if (!newVersion || !/^\d+\.\d+\.\d+$/.test(newVersion)) {
  console.error(`Error: invalid version "${newVersion}". Expected format: X.Y.Z`);
  process.exit(1);
}

// 1. Update info.version in build/config.yml (leaving top-level schema `version: '3'` untouched).
const configPath = resolve(rootDir, "build", "config.yml");
const configRaw = readFileSync(configPath, "utf8");
const infoVersionRe = /^([ \t]+version:\s*["'])(\d+\.\d+\.\d+)(["'])$/m;
const configMatch = configRaw.match(infoVersionRe);
if (!configMatch) {
  console.error(`Error: could not find indented info.version in ${configPath}`);
  process.exit(1);
}
const oldVersion = configMatch[2];
writeFileSync(
  configPath,
  configRaw.replace(infoVersionRe, `$1${newVersion}$3`),
  "utf8",
);
console.log(`build/config.yml: info.version ${oldVersion} → ${newVersion}`);

// 2. Update CFBundleVersion and CFBundleShortVersionString in macOS plists.
for (const plistRel of ["build/darwin/Info.plist", "build/darwin/Info.dev.plist"]) {
  const plistPath = resolve(rootDir, plistRel);
  let plistRaw = readFileSync(plistPath, "utf8");
  for (const key of ["CFBundleVersion", "CFBundleShortVersionString"]) {
    const re = new RegExp(`(<key>${key}</key>\\s*<string>)([^<]+)(</string>)`);
    if (!re.test(plistRaw)) {
      console.error(`Error: could not find ${key} in ${plistPath}`);
      process.exit(1);
    }
    plistRaw = plistRaw.replace(re, `$1${newVersion}$3`);
  }
  writeFileSync(plistPath, plistRaw, "utf8");
  console.log(`${plistRel}: CFBundleVersion/ShortVersionString → ${newVersion}`);
}
