# internal/architecture

> Generated codebase domain map used by LLM agents to navigate the repo.

## Responsibility

Maintains the authoritative mapping from product domains (e.g. "SQL Editor & Diagnostics",
"Git Integration") to their backend Go package paths and frontend TypeScript file paths.
The map is the single source of truth that tells AI agents which directories to touch for
a given feature area, preventing accidental cross-domain coupling.

## Key files

| File | Purpose |
|---|---|
| `semantic_map.go` | **Generated** — do not edit by hand. Contains `GetCodebaseSemanticMap()` returning the domain JSON. |
| `generate.go` | `//go:generate` directive wiring `go generate ./internal/architecture/` to the generator script. |
| `semantic_map_test.go` | `TestSemanticMapAccuracy` — verifies every path in the map exists on disk; fails CI if the map is stale. |

## Key types & functions

- `GetCodebaseSemanticMap() string` — returns the full domain JSON; consumed by LLM tooling.
- `SemanticMap` (test-only) — struct used in `TestSemanticMapAccuracy` to unmarshal and validate all paths.

## Patterns & integration

Annotations drive the generator:
- Go packages: add `// thaw:domain: <Domain Name>` anywhere in a `.go` file in the package (canonical place: `doc.go`).
- Root-level Go files: add `// thaw:file-domain: <Domain Name>` to the file itself.
- TypeScript / TSX files: add `// @thaw-domain: <Domain Name>` anywhere in the file.

After adding or changing annotations, regenerate with:

```bash
go generate ./internal/architecture/
# or equivalently:
go run scripts/gen_semantic_map.go
```

## Gotchas

- `semantic_map.go` is **generated**. Editing it by hand will be overwritten on the next `go generate` run and will break `TestSemanticMapAccuracy` if paths drift.
- The test runs from `internal/architecture/`, so it resolves the project root as `../..`. Do not move the package.
