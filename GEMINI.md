# Thaw — Google Antigravity & Gemini Guide

Thaw is a native desktop Snowflake manager built with **Wails v2** (Go backend + React/TypeScript frontend embedded as a single binary).

This file is a concise root rule kept well under Google Antigravity's 24 KB per-file rule cap so it is never truncated. Detailed domain rules live in `.agents/rules/` (auto-loaded by file glob), multi-step procedures live in `.agents/skills/` (loaded on demand via progressive disclosure), and deep documentation lives in the files below — **read the relevant one before working in an area**:

| Doc | Use it for |
|-----|-----------|
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Branching, Conventional Commits, PRs, docs-with-code rule, build/test/lint commands |
| [`docs/concepts/architecture.md`](docs/concepts/architecture.md) | Wails IPC bridge, backend package map, frontend layout, data flow |
| [`docs/concepts/onboarding.md`](docs/concepts/onboarding.md) | Getting started, navigating the codebase, where to add things |
| [`docs/concepts/patterns.md`](docs/concepts/patterns.md) | Thin delegators, IPC methods, feature flags, stores, sidebar keys, SQL editor, MCP |
| [`docs/concepts/gotchas.md`](docs/concepts/gotchas.md) | **Critical traps — read before debugging** |
| [`docs/concepts/testing.md`](docs/concepts/testing.md) | Unit, race, frontend, integration tests, generated-artifact guards, lint/security gates |
| `internal/<pkg>/README.md` | Backend package responsibility, types, SQL builders, parsers, gotchas |
| `frontend/src/<dir>/README.md` | Frontend folder components, hooks, and Zustand stores |
| [`FEATURES.md`](FEATURES.md) | Full user-facing feature catalogue and optional toggleable features |

---

## Antigravity Workspace Customizations (`.agents/`)

This repository ships shared Google Antigravity customizations under `.agents/`:

- **Glob-scoped rules** (`.agents/rules/`): automatically activated when working in matching paths:
  - `go-backend.md` (`internal/**/*.go, main.go`) — thin-delegator IPC, SQL quoting, slices/time/config/secrets, Wails/gosnowflake invariants.
  - `frontend-react.md` (`frontend/src/**/*.ts, frontend/src/**/*.tsx`) — Ant Design modal/theme rules, Monaco slim imports, WKWebView clipboard/selection guards, lazy chunks, Vitest mocking.
  - `sql-analysis.md` (`internal/sqltok/**/*.go, internal/sqlgrammar/**/*.go, internal/sqleditor/**/*.go`) — leaf-package boundaries, tokenizer-first parsing, grammar/diagnostics invariants.
- **On-demand skills** (`.agents/skills/`): activated automatically when relevant:
  - `thaw-ipc-feature` — adding a backend domain feature, Wails IPC binding, native menu item, or feature flag.
  - `thaw-object-kind` — adding or modifying a Snowflake object kind across registry, codegen, icons, and guards.
  - `thaw-sql-validator-tdd` — TDD workflow for Snowflake SQL grammar and editor diagnostics.
  - `thaw-mcp-tool` — adding or modifying MCP server tools, execution modes, and safety gates in `internal/mcp/`.
  - `thaw-pr-quality` — running pre-push quality/security/doc gates and opening a PR via `gh`.
  - `thaw-architecture-diagram` — updating `docs/concepts/thaw_architecture.drawio`.

---

## Codebase Navigation & Semantic Map

Before proposing new features, refactoring, or writing new files, you MUST consult the codebase semantic map:

1. Open and read `internal/architecture/semantic_map.go`.
2. Locate the target domain for the request inside `GetCodebaseSemanticMap()`.
3. Restrict file creation and modification to the directories that domain owns.
4. Do not invent new architectural folders unless explicitly instructed.
5. Read the `README.md` of each `internal/<pkg>/` or `frontend/src/<dir>/` folder you will touch.

### Maintaining the semantic map

`internal/architecture/semantic_map.go` is **generated** — never edit it by hand.

- **Go packages** (`internal/*/`): add `// thaw:domain: <Domain Name>` (canonical place: `doc.go`).
- **Root-level Go files** (`main.go`): add `// thaw:file-domain: <Domain Name>`.
- **TypeScript / TSX**: add `// @thaw-domain: <Domain Name>`.
- **Regenerate**: run `go generate ./internal/architecture/` (or `go run scripts/gen_semantic_map.go`). CI's `TestSemanticMapAccuracy` fails if any annotated path no longer exists on disk.

---

## Non-Negotiable Invariants

- **Never hand-edit generated files**: `frontend/wailsjs/`, `frontend/src/generated/`, `internal/architecture/semantic_map.go`, `THIRD_PARTY_NOTICES.md`, `docs/backend/`, and `docs/frontend/`. Regenerate with the commands below.
- **Never delete `frontend/dist/.gitkeep`**: required for `//go:embed all:frontend/dist` during `wails generate module` on clean checkouts.
- **Never alter git history**: always work on a dedicated branch (`feat/`, `fix/`, `chore/`, `docs/`, etc. off `main`); **never** use `git commit --amend`, `git rebase`, or `git push --force`. Always create new commits.
- **Never tag manually**: do not create or push `v*` tags by hand — `manual-release.yml` runs `semantic-release` from Conventional Commit squash-merge PR titles (`feat` → minor, `fix`/`perf` → patch, `feat!`/`BREAKING CHANGE` → major; `refactor`/`chore`/`docs`/`style`/`test`/`build`/`ci` → no release).
- **Target upstream PRs cleanly**: `gh pr create --repo Technarion-Oy/thaw --base main --title "..." --body "..."`.

---

## Build, Test & Quality Commands

```bash
# Frontend
cd frontend && npx tsc --noEmit      # fast type-check
cd frontend && npm test              # vitest unit suite
cd frontend && npm run build         # full production build

# Wails bindings & app build
wails generate module                # regenerate frontend/wailsjs/ after changing internal/app/ or bound Service signatures
wails build                          # frontend + Go binary → build/bin/
go mod tidy

# Go tests
go test ./...                        # all unit tests + generated-artifact guards
go test -race ./...                  # with race detector
go test -tags integration ./internal/integration/...   # live Snowflake required

# Code generation (committed; CI fails if stale)
go generate ./internal/architecture/ # → internal/architecture/semantic_map.go
go generate ./internal/objectkind/   # → frontend/src/generated/objectKinds.ts
go generate ./internal/snowflake/    # → frontend/src/generated/snowflakeDataTypes.ts
bash scripts/regen_third_party_notices.sh   # → THIRD_PARTY_NOTICES.md after dep changes

# Lint & security gates (run before pushing)
golangci-lint run ./...
govulncheck ./...
gosec -exclude=G104,G115,G122,G201,G204,G301,G304,G306,G703 \
      -exclude-dir=frontend -exclude-dir=internal/integration ./...

# Generated API docs
make docs                            # TypeDoc + gomarkdoc → docs/frontend/ & docs/backend/
```

---

## Architecture at a Glance

```text
thaw/
├── main.go              # //go:embed all:frontend/dist + app.Run(assets)
├── .agents/             # Google Antigravity workspace rules (glob-scoped) & skills (on-demand)
├── internal/
│   ├── app/             # Wails-bound App struct (package app) — ALL IPC methods (thin delegators).
│   │                    #   app.go=struct+lifecycle, run.go=wails.Run wiring, menu.go=native menu,
│   │                    #   <domain>.go=IPC bindings. See internal/app/README.md.
│   ├── snowflake/       # gosnowflake driver wrapper, pool, object-cache, datatypes, collations, result helpers
│   ├── objectkind/      # canonical object-kind registry (leaf pkg); generates frontend/src/generated/objectKinds.ts
│   ├── sqltok/          # single-pass Snowflake SQL tokenizer & splitter (leaf pkg)
│   ├── sqlgrammar/      # recursive-descent Snowflake SQL grammar engine (imports only sqltok)
│   ├── sqleditor/       # SQL diagnostics, JOIN & autocomplete engine (separate Wails-bound Service)
│   ├── <domain>/        # SQL builders + parsers per Snowflake object type (backup, table, column, pipe,
│   │                    #   stage, secret, warehouse, tasks, objects, queryhistory, dbtproject, …)
│   ├── migration/, snowpark/, mcp/   # larger features (Service pattern / localhost MCP servers)
│   └── ai, config, secrets, gitrepo, filesystem, sfconfig, logger, crashreport, updater, …  # infrastructure
└── frontend/src/
    ├── pages/           # top-level pages (QueryPage)
    ├── components/      # feature UI by domain (editor/, results/, layout/, notebook/, …)
    ├── store/           # Zustand stores (~17 stores)
    ├── generated/       # Go-generated TS artifacts (objectKinds.ts, snowflakeDataTypes.ts — DO NOT EDIT)
    └── wailsjs/         # Wails auto-generated IPC bindings (DO NOT EDIT)
```

**IPC flow**: component → `wailsjs/go/app/App.ts` (or `wailsjs/go/sqleditor/Service.ts`) → Wails runtime → Go methods on `*App` / bound `Service` → `internal/` domain packages.

---

## Documentation Is Part of Every Change

When you add, remove, or modify a user-facing feature, internal package, frontend component/store, or architectural pattern, update the relevant docs **in the same commit/PR**: `README.md`, `FEATURES.md`, `CLAUDE.md`, `GEMINI.md`, `.agents/` (if a rule or workflow changed), the affected `internal/<pkg>/README.md` or `frontend/src/<dir>/README.md`, and any `docs/concepts/*.md` it touches. Never defer docs to a follow-up PR.

## Tech Stack

Wails v2.17 · Go 1.26 (toolchain 1.27) · gosnowflake v2.4 · React 19 + TypeScript 7 · Vite 8 · Ant Design 6 · Monaco · TanStack Table v9 · Zustand 5 · xterm.js 6.
