# Thaw — Snowflake Manager

A native desktop application for Snowflake management: browse objects, run SQL queries, export
DDL to a git repository, and push changes through CI/CD workflows. Built with **Wails v2** (Go
backend + React/TypeScript frontend, shipped as a single binary).

**Stack:** Go · Wails v2 · React · Ant Design · Monaco Editor · TanStack Table

> Thaw is **free software** licensed under the **GNU General Public License v3.0 (or later)** —
> see [License](#license).

<!-- Screenshots: add app screenshots / GIFs here. -->

---

## What Thaw does

- **Connect to Snowflake** — account/user/password/warehouse/role, or key-pair auth via the
  Snowflake CLI profile manager (`~/.snowflake/config.toml`).
- **SQL editor** — a Monaco-based editor with Snowflake-dialect highlighting, multi-statement
  execution, diagnostics, autocomplete, hover DDL, formatting, and split views.
- **Object browser** — a two-pane sidebar tree over databases, schemas, and every object type,
  with context-menu actions and DDL export.
- **Notebooks & Snowpark** — Jupyter-style Python/SQL notebooks with a managed Snowpark
  environment.
- **Administration** — warehouses, users, roles, integrations, backup policies, query
  activity, and metering.
- **Git & dbt** — export DDL to a git repo, scaffold dbt projects, and drive schema migrations.
- **MCP server** — expose schema-browsing tools to AI agents over SSE/HTTP.

The complete, exhaustive feature catalogue lives in [`FEATURES.md`](FEATURES.md).

---

## Developer workspace setup

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | ≥ 1.22 | `brew install go` |
| Node.js | ≥ 20 | `brew install node` |
| Wails CLI | ≥ 2.9 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |

Verify your toolchain with `wails doctor`.

> **Linux** also needs CGO + GTK/WebKit headers:
> `sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev`

### Getting started

```bash
# 1. Install Go dependencies
go mod tidy

# 2. Install frontend dependencies
cd frontend && npm install && cd ..

# 3. Run in development mode (Go + React hot-reload)
wails dev
```

The first `wails dev` run regenerates `frontend/wailsjs/` from your Go structs.

To produce a production binary (output in `build/bin/`):

```bash
wails build
```

For the full build/test/architecture reference, see
**[`docs/TECHNICAL_INSTRUCTIONS.md`](docs/TECHNICAL_INSTRUCTIONS.md)**.

---

## Documentation

| Where | What |
|-------|------|
| [`docs/TECHNICAL_INSTRUCTIONS.md`](docs/TECHNICAL_INSTRUCTIONS.md) | Build, test, project structure, keyboard shortcuts, configuration |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | How to branch, commit, open PRs, and write code & docs |
| [`COLLABORATION.md`](COLLABORATION.md) | How outside contributors get involved; review process |
| [`docs/concepts/`](docs/concepts/) | [architecture](docs/concepts/architecture.md) · [onboarding](docs/concepts/onboarding.md) · [patterns](docs/concepts/patterns.md) · [gotchas](docs/concepts/gotchas.md) · [testing](docs/concepts/testing.md) |
| `internal/<pkg>/README.md` | Per-package backend reference |
| `frontend/src/<dir>/README.md` | Per-folder frontend reference |
| [`FEATURES.md`](FEATURES.md) | The complete feature catalogue |
| [`SECURITY.md`](SECURITY.md) | How to report a vulnerability |

---

## Contributing

Contributions are welcome. Please read [`CONTRIBUTING.md`](CONTRIBUTING.md) and
[`COLLABORATION.md`](COLLABORATION.md) first, follow the
[Code of Conduct](CODE_OF_CONDUCT.md), and note that a
[Contributor License Agreement](CLA.md) must be signed before your first PR can be merged
(the CLA bot will guide you automatically).

---

## License

Copyright © 2026 Technarion Oy and Thaw contributors.

Thaw is free software: you can redistribute it and/or modify it under the terms of the
**GNU General Public License** as published by the Free Software Foundation, either version 3
of the License, or (at your option) any later version.

Thaw is distributed in the hope that it will be useful, but **WITHOUT ANY WARRANTY**; without
even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
General Public License for more details.

You should have received a copy of the GNU General Public License along with Thaw. If not, see
<https://www.gnu.org/licenses/>. The full text is in the [`LICENSE`](LICENSE) file.
