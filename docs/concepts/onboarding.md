# Onboarding

Welcome to Thaw. This guide gets you from a fresh clone to making your first change.

## 1. Install prerequisites

| Tool | Version |
|------|---------|
| Go | ≥ 1.22 |
| Node.js | ≥ 20 |
| Wails CLI | ≥ 2.9 (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`) |

Run `wails doctor` to confirm your toolchain is healthy.

## 2. Set up the project

```bash
go mod tidy
cd frontend && npm install && cd ..
```

## 3. Run in development

```bash
wails dev
```

Both the Go backend and the Vite/React frontend hot-reload. The first run regenerates `frontend/wailsjs/` from your Go structs.

## 4. Build a production binary

```bash
wails build      # output in build/bin/
```

The production frontend build minifies (Terser) and obfuscates (`javascript-obfuscator`); the build script allocates 6 GB of Node heap.

## 5. Learn the codebase

Read these, in order:

1. [Architecture](architecture.md) — the big picture and the IPC bridge.
2. [Patterns](patterns.md) — the thin-delegator pattern and how to add IPC methods, feature flags, and stores.
3. The `README.md` in the specific `internal/<pkg>/` or `frontend/src/<dir>/` folder you'll be touching.
4. [Gotchas](gotchas.md) — before you debug anything weird.

### Finding the right place to make a change

1. **Consult the semantic map.** Read `GetCodebaseSemanticMap()` in `internal/architecture/semantic_map.go` (generated) to find the domain that owns the area you're changing. Restrict new files to that domain's directories.
2. **Use the per-module READMEs** as your map of responsibilities.

## 6. Make your first change

A typical "add a backend feature surfaced in the UI" loop:

1. Add the real logic + a `Build*Sql`/`Parse*` to the right `internal/<domain>` package, with unit tests.
2. Add a thin-delegator method on `*App` in `internal/app/<domain>.go`.
3. `wails generate module` to regenerate bindings.
4. Wire the new binding into a component; subscribe to any emitted events.
5. Update the relevant docs **in the same PR** (see [CONTRIBUTING](../../CONTRIBUTING.md)).
6. `go test ./...`, `cd frontend && npx tsc --noEmit && npm test`, lint, then open a PR.

## 7. Conventions to internalize

- **Branch** off `main` with a `feat/`/`fix/`/`chore/` prefix; **never** amend, rebase, or force-push.
- **Commit** with Conventional Commits (`feat`/`fix`/`perf` trigger releases).
- **Docs travel with code** — update README/FEATURES/CLAUDE/GEMINI/module-README/concepts as part of the change.
- **Never edit** `frontend/wailsjs/` or `internal/architecture/semantic_map.go` by hand (both generated).

Full workflow rules: [`CONTRIBUTING.md`](../../CONTRIBUTING.md). Test details: [Testing](testing.md).
