# Concepts

High-level, hand-written guides to how Thaw is put together. Unlike the generated API reference under [`docs/backend/`](../backend/app.md) and [`docs/frontend/`](../frontend/README.md), these documents explain the *why* and the cross-cutting patterns.

| Guide | What it covers |
|-------|----------------|
| [Architecture](architecture.md) | The Wails IPC bridge, backend package map, frontend store/component layout, data flow |
| [Onboarding](onboarding.md) | Getting a dev environment running, navigating the codebase, where to add things |
| [Patterns](patterns.md) | Reusable patterns: thin delegators, feature flags, IPC methods, stores, the SQL editor |
| [Gotchas](gotchas.md) | Non-obvious traps that have bitten us (driver logging, clipboard, async safety…) |
| [Testing](testing.md) | Unit, race, frontend, and integration test workflows |

For workflow rules (branching, commits, PRs, docs), see [`CONTRIBUTING.md`](../../CONTRIBUTING.md). For per-module detail, read the `README.md` inside each `internal/<pkg>/` and `frontend/src/<dir>/` folder.
