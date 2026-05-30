# Contributing to Thaw

Thanks for contributing! Thaw is a native desktop Snowflake manager built with **Wails v2** (Go backend + React/TypeScript frontend embedded as a single binary).

This guide covers **how to branch, commit, open a PR, and write code & docs**. For the conceptual map of the codebase, read the [`docs/concepts/`](docs/concepts/) guides and the per-module `README.md` in each `internal/<pkg>/` and `frontend/src/<dir>/` folder.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | ≥ 1.22 | `brew install go` |
| Node.js | ≥ 20 | `brew install node` |
| Wails CLI | ≥ 2.9 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |

Verify with `wails doctor`. First-time setup:

```bash
go mod tidy
cd frontend && npm install && cd ..
wails dev          # hot-reload backend + frontend
```

---

## Branching

All changes go on a feature branch — never commit directly to `main`. Use a conventional prefix that matches the change:

| Prefix | Use for |
|--------|---------|
| `feat/` | new user-facing feature |
| `fix/` | bug fix |
| `perf/` | performance improvement |
| `refactor/` | internal restructure, no behavior change |
| `chore/`, `docs/`, `style/`, `test/`, `build/`, `ci/` | non-release maintenance |

```bash
git checkout -b feat/my-new-feature
```

### Git history rules

**Never alter branch history.** Do not use `git commit --amend`, `git rebase`, or `git push --force`. Always create new commits for updates. When a pre-commit hook fails the commit did **not** happen, so fix, re-stage, and create a **new** commit (never `--amend`).

---

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/). The type determines whether a release is cut and which version component bumps:

| Commit type | Release | Version bump |
|-------------|---------|--------------|
| `feat` | ✅ | **minor** (0.X.0) |
| `feat!` / `BREAKING CHANGE` footer | ✅ | **major** (X.0.0) |
| `fix`, `perf` | ✅ | **patch** (0.0.X) |
| `refactor`, `chore`, `docs`, `style`, `test`, `build`, `ci` | ❌ | no release |

Write the subject in the imperative ("add column context menu", not "added"). Use `add` for a wholly new feature, `update` for an enhancement, `fix` for a bug fix. Focus the body on the **why**, not the what.

---

## Pull requests

Create PRs with the GitHub CLI, targeting the upstream repo:

```bash
git push -u origin feat/my-new-feature
gh pr create --repo Technarion-Oy/thaw --base main \
  --title "feat: my new feature" --body "Description…"
```

Keep PR titles short (< 70 chars); put detail in the body. Review the full diff since the branch diverged from `main`, not just the latest commit.

---

## Documentation is part of the change

**Every change that adds, removes, or modifies a user-facing feature, internal package, frontend component/store, or architectural pattern MUST update the relevant docs in the same commit or PR.** Do not defer docs to a follow-up — outdated docs mislead both humans and LLM agents.

| File | Update when you change… |
|------|--------------------------|
| `README.md` | feature descriptions, project-structure tree, SQL validation list, keyboard shortcuts |
| `FEATURES.md` | the feature list; if toggleable, also "Feasible Optional Features" |
| `CLAUDE.md` | architecture tree, Zustand store list, key patterns, critical gotchas |
| `GEMINI.md` | architecture overview, engineering standards, common workflows |
| `internal/<pkg>/README.md` | a backend package's responsibility, types, or gotchas |
| `frontend/src/<dir>/README.md` | a frontend folder's components/state |
| `docs/concepts/*.md` | a cross-cutting pattern, the architecture, or onboarding flow |

The generated API reference under `docs/backend/` and `docs/frontend/` is produced by `make docs` (TypeDoc + gomarkdoc) — do not hand-edit it.

---

## Codebase navigation & the semantic map

Before proposing new features, refactoring, or writing new files, consult the codebase semantic map in [`internal/architecture/semantic_map.go`](internal/architecture/README.md):

1. Read `GetCodebaseSemanticMap()` and locate the target domain for your change.
2. Restrict new files and modifications to the directories that domain owns.
3. Do not invent new architectural folders unless explicitly asked.

The map is **generated** — do not edit it by hand. To change it, annotate the relevant files and regenerate:

- Go packages (`internal/*/`): `// thaw:domain: <Domain Name>` (canonical place: `doc.go`)
- Root-level Go files (`main.go`): `// thaw:file-domain: <Domain Name>`
- TypeScript/TSX: `// @thaw-domain: <Domain Name>`
- Regenerate: `go generate ./internal/architecture/` — CI's `TestSemanticMapAccuracy` fails if an annotated path no longer exists.

---

## Key engineering patterns

These are summarized here; full detail lives in [`docs/concepts/patterns.md`](docs/concepts/patterns.md) and [`docs/concepts/gotchas.md`](docs/concepts/gotchas.md).

### Adding a Go → frontend IPC method

1. Add a public method on `*App` (receiver `a *App`) in the `internal/app/<domain>.go` file matching its domain (query methods → `query.go`, object listing → `objects.go`). All files in `internal/app/` are `package app`, so methods bind regardless of file.
2. Run `wails generate module` to regenerate `frontend/wailsjs/`.
3. Import from `"../../../wailsjs/go/app/App"` in the component.

### Thin-delegator pattern

`*App` methods should be **thin delegators**: a nil-check (`apperrors.ErrNotConnected`), a single call into a domain package, and a return. All real logic — SQL string building, `snowflake.QueryResult` parsing, validation, key generation — lives in `internal/<domain>` packages so it is unit-testable without a live connection.

```go
// internal/app/warehouse.go — thin delegator
func (a *App) GetWarehouseMeteringHistory(wh, start, end string) ([]warehouse.WarehouseMeteringRow, error) {
    if a.client == nil { return nil, apperrors.ErrNotConnected }
    return warehouse.GetMeteringHistory(a.ctx, a.client, wh, start, end)
}
```

Domain packages expose `Build*Sql(...) (string, error)` builders and `Parse*(res *snowflake.QueryResult)` parsers; shared parsing helpers live in `internal/snowflake/result.go`.

### Adding a feature flag

Feature flags live in `internal/config/config.go` (`FeatureFlags`) and surface via **View → Enabled Features…**. All flags default to enabled (an `Initialized` sentinel prevents Go's zero-value `false` from disabling features on fresh installs). Steps: add the bool field + set it in `DefaultFeatureFlags()`, bump `flagsVersion`, add `MigrateFlags()` logic, run `wails generate module`, add a `<FlagRow>` in `FeatureFlagsModal.tsx`, then gate the feature in its component. See [`internal/config/README.md`](internal/config/README.md).

---

## Building, testing & quality gates

```bash
# Type-check frontend (fast, no emit)
cd frontend && npx tsc --noEmit

# Full builds
cd frontend && npm run build      # frontend only
wails build                       # frontend + Go binary → build/bin/

# Regenerate Wails bindings after changing internal/app/ signatures
wails generate module

# Tests
go test ./...                     # all Go unit tests
go test -race ./...               # with race detector
cd frontend && npm test           # frontend (vitest)

# Docs
make docs                         # regenerate TypeDoc + gomarkdoc reference
```

Integration tests live in `internal/integration/` and require a live Snowflake connection (gated behind the `integration` build tag).

### Lint & security (run before pushing)

```bash
golangci-lint run ./...
govulncheck ./...
gosec -exclude=G104,G115,G122,G201,G204,G301,G304,G306,G703 \
      -exclude-dir=frontend -exclude-dir=internal/integration ./...
```

These also run weekly in CI (`.github/workflows/`).

---

## Security

Never introduce command injection, XSS, SQL injection, or other OWASP-top-10 issues. Build all SQL through the domain-package `Build*Sql` builders (which quote identifiers via `internal/snowflake` helpers) — never concatenate user input into SQL inline in the frontend. Git tokens are never written to disk; the AI API key is written `0600`.
