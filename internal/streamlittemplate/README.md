# internal/streamlittemplate

Lists and downloads Streamlit app **templates** from
[`Snowflake-Labs/snowflake-demo-streamlit`](https://github.com/Snowflake-Labs/snowflake-demo-streamlit)
(Apache-2.0) so a user can scaffold a new local app, then deploy it with the
Phase 1 local-deploy path (`internal/snowflake` `DeployStreamlit`).

## What it does

Every top-level folder of the demo repo is a self-contained Streamlit app
(`streamlit_app.py`, `environment.yml`, `README.md`, optional `assets/`,
`data/`, `pages/`). This package fetches that catalog at runtime and scaffolds a
single chosen folder locally — never a full clone of the 30+ demos.

- `ListTemplates(ctx) Catalog` — the deployable top-level folders (excludes
  `shared_assets` and hidden entries), each with a one-line `Description` taken
  from its `README.md` first paragraph (best-effort, fetched in parallel with a
  concurrency cap). The contents endpoint is **paginated** (`per_page=100`, up to
  `contentsMaxPages`), so a repo that outgrows one page can't yield a quietly
  incomplete catalog; running past the page cap is an error, not a truncation.
  **Never errors on network/rate-limit failure**: those return
  a `Catalog{Degraded: true}` carrying `embeddedTemplateNames` (names only) plus
  a human-readable `Note`, so the picker stays usable and the feature is purely
  additive.
- `DownloadTemplate(ctx, name, destDir) error` — scaffolds one folder into
  `destDir`, preserving its relative structure. Fetches **only the chosen
  folder's files** via the GitHub git-tree API (one request) + raw file
  downloads (fetched with bounded parallelism, `errgroup.SetLimit(8)`, matching
  `ListTemplates`' description fetch). Refuses a non-empty destination, and
  **rolls back on failure** (`rollbackScaffold`): a folder it created is removed,
  a pre-existing empty folder is emptied again — otherwise a half-written app
  would trip the empty-destination rule and block every retry into that folder.
  Writes the repo's Apache-2.0
  `LICENSE` (best-effort download, bundled fallback header if unreachable) and a
  `NOTICE` provenance line for attribution; the template's own `README.md` is
  kept.

## Types

- `Template{Name, Description}` — one deployable app folder.
- `Catalog{Templates, Degraded, Note}` — the list result; `Degraded` signals the
  built-in fallback is being shown.
- `Repo`, `RepoURL`, `License` — the `owner/name` slug, source URL, and SPDX
  license of the upstream repo. Every attribution surface reads these rather than
  restating the strings: the `NOTICE` written into a scaffolded folder, the UI
  credit line, and the `source` block on the MCP `list_streamlit_templates` /
  `create_streamlit_from_template` results.

## Gotchas

- **Attribution is required** (Apache-2.0). The scaffolded folder always gets a
  `LICENSE` + `NOTICE`, and the picker UI shows a visible credit line — the repo
  won't appear in `THIRD_PARTY_NOTICES.md` (that is generated from Go module
  deps), so the in-UI credit is the attribution surface. The MCP tools are a
  second such surface and carry the same credit in their results (see
  `internal/mcp/streamlit_tools.go`).
- **A template's own `LICENSE`/`NOTICE` is never overwritten.** An existing
  `LICENSE` is left alone, and an existing `NOTICE` is *appended to* with Thaw's
  provenance lines rather than replaced — Apache-2.0 §4(d) requires a downstream
  copy to carry the upstream `NOTICE` contents forward.
- **Rate limiting** is detected for both of GitHub's forms: the primary limit
  (403 + `X-RateLimit-Remaining: 0`) and the secondary/abuse limit (403 or 429,
  usually with `Retry-After`, or a body that says so) — `rateLimitMessage` maps
  them all to one "rate limit exceeded" error, which `ListTemplates` turns into
  its `Degraded` note. A plain 403 (e.g. a private repo) stays a generic error.
- **Path safety**: `validTemplateName` rejects traversal/hidden/excluded names,
  and `safeJoin` rejects any tree entry that would escape `destDir`.
- Base URLs (`githubAPIBase`, `rawBase`) are package vars so tests can point them
  at an `httptest` server; the network paths are covered without live GitHub —
  including the failure modes (rate-limit 403 → `Degraded` catalog / clear
  download error, truncated tree, template-not-found) and space-containing folder
  names (URL escaping).

## IPC

Exposed via `internal/app/streamlit.go`: `App.ListStreamlitTemplates` and
`App.CreateStreamlitFromTemplate(templateName, destDir)`. Neither needs a
Snowflake connection — scaffolding is local; deployment is a separate step.

The same two entry points are exposed to external AI clients as the MCP tools
`list_streamlit_templates` and `create_streamlit_from_template`
(`internal/mcp/streamlit_tools.go`); the scaffolder is workspace-gated there, so
`destDir` must resolve inside the session's workspace root.
