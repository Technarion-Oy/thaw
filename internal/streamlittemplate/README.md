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
  concurrency cap). **Never errors on network/rate-limit failure**: those return
  a `Catalog{Degraded: true}` carrying `embeddedTemplateNames` (names only) plus
  a human-readable `Note`, so the picker stays usable and the feature is purely
  additive.
- `DownloadTemplate(ctx, name, destDir) error` — scaffolds one folder into
  `destDir`, preserving its relative structure. Fetches **only the chosen
  folder's files** via the GitHub git-tree API (one request) + raw file
  downloads. Refuses a non-empty destination. Writes the repo's Apache-2.0
  `LICENSE` (best-effort download, bundled fallback header if unreachable) and a
  `NOTICE` provenance line for attribution; the template's own `README.md` is
  kept.

## Types

- `Template{Name, Description}` — one deployable app folder.
- `Catalog{Templates, Degraded, Note}` — the list result; `Degraded` signals the
  built-in fallback is being shown.
- `RepoURL` — the source repo URL, reused by the IPC layer / UI attribution.

## Gotchas

- **Attribution is required** (Apache-2.0). The scaffolded folder always gets a
  `LICENSE` + `NOTICE`, and the picker UI shows a visible credit line — the repo
  won't appear in `THIRD_PARTY_NOTICES.md` (that is generated from Go module
  deps), so the in-UI credit is the attribution surface.
- **Path safety**: `validTemplateName` rejects traversal/hidden/excluded names,
  and `safeJoin` rejects any tree entry that would escape `destDir`.
- Base URLs (`githubAPIBase`, `rawBase`) are package vars so tests can point them
  at an `httptest` server; the network paths are covered without live GitHub.

## IPC

Exposed via `internal/app/streamlit.go`: `App.ListStreamlitTemplates` and
`App.CreateStreamlitFromTemplate(templateName, destDir)`. Neither needs a
Snowflake connection — scaffolding is local; deployment is a separate step.
