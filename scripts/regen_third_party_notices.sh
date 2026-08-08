#!/usr/bin/env bash
# Regenerate THIRD_PARTY_NOTICES.md, installing everything the generator needs
# first. Run it after changing dependencies in go.mod or frontend/package.json.
#
# Safe to run on any OS: the generator lists the Go modules once per shipped
# platform (darwin/linux/windows) and merges them, so the file it writes is
# byte-identical everywhere and matches what CI verifies.
#
# Renovate invokes this as a postUpgradeTasks command (see .github/renovate.json)
# so dependency PRs keep the notices in sync. It has to be a script rather than a
# chain of commands in the config: Renovate executes post-upgrade commands with
# `shell: false` and splits them on whitespace, so `cd frontend && npm ci` tries
# to spawn a binary literally named "cd" (Error: spawn cd ENOENT).
set -euo pipefail

cd "$(dirname "$0")/.."

# main.go has `//go:embed all:frontend/dist`, and the `go list` the generator
# runs refuses to resolve that pattern when the directory does not exist.
mkdir -p frontend/dist
touch frontend/dist/.keep

# The generator reads each package's version and LICENSE out of
# frontend/node_modules, so it has to exist and match the current lockfile.
# Without it, `npm ls` reports every dependency as missing, the generator skips
# them all, and the notices file silently loses its entire frontend section.
(cd frontend && npm ci)

go run scripts/gen_third_party_notices.go
