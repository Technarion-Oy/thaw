# Thaw — Claude Code Guide

Thaw is a native desktop Snowflake manager built with **Wails v2** (Go backend + React/TypeScript frontend embedded as a single binary).

This file is a concise index. Detailed documentation lives in dedicated files — **read the relevant one before working in an area**:

| Doc | Use it for |
|-----|-----------|
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Branching, commits, PRs, the docs-with-code rule, build/test/lint commands |
| [`docs/concepts/architecture.md`](docs/concepts/architecture.md) | The Wails IPC bridge, backend package map, frontend layout, data flow |
| [`docs/concepts/onboarding.md`](docs/concepts/onboarding.md) | Getting started, navigating the codebase, where to add things |
| [`docs/concepts/patterns.md`](docs/concepts/patterns.md) | Thin delegators, IPC methods, feature flags, stores, sidebar keys, the SQL editor |
| [`docs/concepts/gotchas.md`](docs/concepts/gotchas.md) | **Critical traps — read before debugging** |
| [`docs/concepts/testing.md`](docs/concepts/testing.md) | Unit, race, frontend, integration tests |
| `internal/<pkg>/README.md` | What a backend package does, its types, builders, parsers, gotchas |
| `frontend/src/<dir>/README.md` | What a frontend folder's components/stores do |
| `FEATURES.md` | The full user-facing feature list |

---

## Codebase Navigation & Architecture

Before proposing new features, refactoring, or writing new files, you MUST consult the codebase semantic map.

1. Open and read `internal/architecture/semantic_map.go`.
2. Locate the target domain for the request inside `GetCodebaseSemanticMap()`.
3. Restrict your file creation and modification to the directories that domain specifies.
4. Do not invent new architectural folders unless explicitly instructed.
5. Then read the `README.md` of the specific `internal/<pkg>/` or `frontend/src/<dir>/` you'll touch.

### Maintaining the semantic map

The map in `internal/architecture/semantic_map.go` is **generated** — do not edit it by hand.

- **Go packages** (`internal/*/`): add `// thaw:domain: <Domain Name>` anywhere in a `.go` file (canonical place: `doc.go`) → generator outputs the package dir path.
- **Root-level Go files** (`main.go`): add `// thaw:file-domain: <Domain Name>` → generator outputs the file path.
- **TypeScript / TSX**: add `// @thaw-domain: <Domain Name>` → generator outputs the file path.
- **Regenerate**: `go generate ./internal/architecture/` (or `go run scripts/gen_semantic_map.go`). CI's `TestSemanticMapAccuracy` fails if an annotated path no longer exists.

## Codebase Vector Database

A ChromaDB vector index of all `.go`, `.ts`, and `.tsx` files lives at `.chroma_db/` (not committed; see `.gitignore`). Collection `thaw_codebase`, model `models/gemini-embedding-2` @ 768 dims, cosine. **Before writing code for a non-trivial task, query it** to locate relevant existing files/functions and avoid duplicate implementations.

Query from Python:
```python
import chromadb, os
from google import genai
from google.genai import types

client = genai.Client(api_key=os.environ["GEMINI_API_KEY"])
col = chromadb.PersistentClient(path=".chroma_db").get_collection("thaw_codebase")

def search(query: str, n: int = 8):
    vec = client.models.embed_content(
        model="models/gemini-embedding-2", contents=query,
        config=types.EmbedContentConfig(task_type="RETRIEVAL_QUERY", output_dimensionality=768),
    ).embeddings[0].values
    r = col.query(query_embeddings=[vec], n_results=n)
    return [{"file": m["file_path"], "text": d} for m, d in zip(r["metadatas"][0], r["documents"][0])]
```

Refresh after significant code changes:
```bash
cd scripts && GEMINI_API_KEY=... .venv/bin/python embed_codebase.py --reset
```
`--reset` rebuilds from scratch (preferred; UUID IDs mean appends don't dedupe).

---

## Build & test commands

```bash
# Frontend
cd frontend && npx tsc --noEmit      # type-check (fast)
cd frontend && npm run build         # full frontend build
cd frontend && npm test              # vitest

# App
wails build                          # frontend + Go binary
wails generate module                # regenerate frontend/wailsjs/ after changing internal/app/ signatures
go mod tidy

# Go tests
go test ./...                        # all
go test -race ./...                  # race detector
go test -tags integration ./internal/integration/...   # live Snowflake required

# Docs
make docs                            # regenerate TypeDoc + gomarkdoc reference
make docs-serve                      # serve docs/ at http://localhost:4000
```

See [`docs/concepts/testing.md`](docs/concepts/testing.md) for lint/security gates and [`CONTRIBUTING.md`](CONTRIBUTING.md) for the full workflow.

---

## Architecture at a glance

```
thaw/
├── main.go              # //go:embed all:frontend/dist + app.Run(assets)
├── internal/
│   ├── app/             # Wails-bound App struct (package app) — ALL IPC methods (thin delegators).
│   │                    #   app.go=struct+lifecycle, run.go=wails.Run wiring, menu.go=native menu,
│   │                    #   <domain>.go=IPC bindings. See internal/app/README.md.
│   ├── snowflake/       # gosnowflake driver wrapper, pool, object-cache, result helpers
│   ├── sqleditor/       # SQL diagnostics & JOIN engine (its own Wails-bound Service)
│   ├── <domain>/        # SQL builders + parsers per object type (backup, table, column, pipe,
│   │                    #   stage, secret, warehouse, tasks, objects, queryhistory, dbtproject, …)
│   ├── migration/, snowpark/, mcp/   # larger features (Service pattern / MCP servers)
│   └── ai, config, gitrepo, filesystem, sfconfig, logger, crashreport, telemetry, …  # infrastructure
└── frontend/src/
    ├── pages/           # top-level pages (QueryPage)
    ├── components/      # feature UI by domain (editor/, results/, layout/, notebook/, …)
    ├── store/           # ~14 Zustand stores
    └── wailsjs/         # auto-generated IPC bindings (DO NOT EDIT)
```

**IPC flow**: component → `wailsjs/go/app/App.ts` (or `wailsjs/go/sqleditor/Service.ts`) → Wails runtime → Go methods on `App` / bound `Service` → `internal/` domain packages. Full detail: [`docs/concepts/architecture.md`](docs/concepts/architecture.md). Every package/folder has a `README.md`.

---

## Documentation is part of every change

When you add, remove, or modify a user-facing feature, internal package, frontend component/store, or architectural pattern, update the relevant docs **in the same commit/PR**: `README.md`, `FEATURES.md`, `CLAUDE.md`, `GEMINI.md`, the affected `internal/<pkg>/README.md` or `frontend/src/<dir>/README.md`, and any `docs/concepts/*.md` it touches. The generated reference under `docs/backend/` and `docs/frontend/` is produced by `make docs` — never hand-edit it. Do not defer docs to a follow-up PR.

## Tech stack

Wails v2.11 · Go 1.22 · gosnowflake v2.0 · React 18 + TypeScript 5.6 · Vite 5 · Ant Design 5 · Monaco · TanStack Table v8 · Zustand 5 · xterm.js.
