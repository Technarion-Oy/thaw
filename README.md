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

## Requirements

Thaw ships as a **single self-contained binary** (Wails v2) — there is no separate runtime to
install for the app itself. You need a supported OS, the platform's web-rendering runtime, and a
Snowflake account. **Outbound HTTPS** access to your Snowflake account is required. Note that
**`git` is _not_ required** — Thaw's git integration uses an embedded Go git implementation
(`go-git`), not the system `git` binary.

### System requirements

| OS | Architectures | Runtime notes |
|----|---------------|---------------|
| **macOS** | `arm64` (Apple Silicon), `amd64` (Intel) | Minimum **macOS 10.13 High Sierra**. Builds are code-signed & notarized. |
| **Windows** | `amd64`, `arm64` | Requires the **[Microsoft Edge WebView2 runtime](https://developer.microsoft.com/microsoft-edge/webview2/)** (present on up-to-date Windows 10/11; installed automatically by the signed NSIS installer if missing). |
| **Linux** | `amd64` | Requires **GTK3** and **WebKit2GTK 4.1** runtime libraries — e.g. on Debian/Ubuntu `sudo apt-get install -y libgtk-3-0 libwebkit2gtk-4.1-0`. |

### Optional runtime dependencies (feature-specific)

These are only needed if you use the corresponding feature:

- **Snowpark notebooks** — a local **Python 3** install. Thaw creates a virtual environment and
  installs `snowflake-snowpark-python`, `notebook`, `ipython-sql`, and `sqlalchemy` into it.
- **dbt projects** — a local **dbt** install for the dbt integration.
- **AI inline completions** — an **OpenAI** or **Google AI Studio (Gemini)** API key, configured
  via **Tools → Configure AI Inline Completions…**.

### Snowflake requirements

Thaw works with any standard Snowflake account. To run queries and browse objects you need a
**role** and an active **warehouse** (both selectable from the toolbar).

**Supported authentication methods:**

| Authenticator | Description |
|---------------|-------------|
| `snowflake` | Username + password (optional TOTP passcode) |
| `username_password_mfa` | Password + MFA push notification |
| `externalbrowser` | Browser-based SSO |
| `okta` | Native Okta SSO (requires your Okta account URL) |
| `snowflake_jwt` | Key-pair / JWT (requires a PEM private-key path) |
| `oauth` | External OAuth token pass-through |
| `programmatic_access_token` | Snowflake Programmatic Access Token (PAT) |
| `oauth_authorization_code` | Browser-based OAuth2 authorization-code flow |
| `oauth_client_credentials` | Non-interactive OAuth2 client-credentials flow |
| `workload_identity` | Workload Identity Federation (AWS / Azure / GCP) |

Additional connection support:

- **Forward HTTP/HTTPS proxy** — host, port, user, password, protocol, and a no-proxy list.
- **Snowflake CLI profile reuse** — connection profiles in `~/.snowflake/config.toml` (or a
  custom path) can be read and managed from the connect dialog.
- **Edition-dependent features** — some capabilities depend on your Snowflake edition (for
  example data-retention / `MAX_DATA_EXTENSION_TIME_IN_DAYS` guidance and certain object types),
  and privileged operations require an appropriately privileged role.

### Cloud providers & regions

Thaw is **cloud- and region-agnostic** — there is no region allow-list and no per-cloud build.
It connects to whatever Snowflake account identifier you supply, so **any Snowflake region on
any supported cloud works**:

- Accounts hosted on **AWS, Azure, and GCP**, in **any Snowflake region**.
- Connectivity is fully determined by the **account identifier** — either the modern
  `myorg-account` org/account name or the legacy `locator.region` form (e.g.
  `xy12345.eu-north-1`). The region is encoded in the identifier; no separate region or cloud
  needs to be configured. For the current list, see Snowflake's
  [Supported Cloud Regions](https://docs.snowflake.com/en/user-guide/intro-regions).
- A cloud provider is only chosen explicitly for the **Workload Identity Federation**
  authenticator, where selecting `AWS` / `Azure` / `GCP` picks the CSP-native identity flow.
- **AWS PrivateLink / Azure Private Link / GCP Private Service Connect** accounts are supported —
  supply the PrivateLink account URL/host; outbound HTTPS plus the forward-proxy settings cover
  restricted-network setups.

### Configuration & data locations

- App preferences: `~/.config/thaw/config.json`
- Snowflake connection profiles: `~/.snowflake/config.toml`

---

## Developer workspace setup

> This section is for **contributors building Thaw from source**. If you just want to *run*
> Thaw, see [Requirements](#requirements) above and grab a release binary.

### Developer prerequisites

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
