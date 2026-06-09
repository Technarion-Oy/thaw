# frontend/src/components/gitrepoobj

> Modals for managing Snowflake-native GIT REPOSITORY objects (CREATE, ALTER, commit filter) — distinct from the local git panel in `components/git/`.

## Responsibility

Provides the three modals that cover the full lifecycle of a Snowflake
`GIT REPOSITORY` object. SQL is built on the backend via `BuildCreateGitRepositorySql`
and `BuildModifyGitRepositorySql` and previewed in the modal before execution via
`ExecDDL`. All modals follow the debounced backend-generated SQL preview pattern used
elsewhere in the codebase (e.g. dbtproject modals).

## Files

| File | Purpose |
|---|---|
| `CreateGitRepositoryModal.tsx` | `CREATE GIT REPOSITORY` form. Collects name (with `ObjectNameCaseControl` for case-sensitivity), `OR REPLACE / IF NOT EXISTS`, origin URL, API integration (loaded from `ListApiIntegrations`), optional git credentials secret (loaded from `ListSecretsInAccount`), tags (name=value pairs), and comment. Embeds `CreateSecretModal` inline so the user can create a new secret without leaving the modal; after secret creation the secret list reloads and the new FQN is pre-selected. Live SQL preview is updated on every field change via `BuildCreateGitRepositorySql`. Submits via `ExecDDL(preview)`. |
| `ModifyGitRepositoryModal.tsx` | `ALTER GIT REPOSITORY` form. Loads current properties via `GetObjectProperties` on mount (origin URL shown read-only, since it cannot be changed post-creation). Editable fields: API integration, git credentials (with inline `CreateSecretModal`), and comment. SQL preview via `BuildModifyGitRepositorySql` (returns multiple statements joined by `\n\n`; each is executed sequentially via `ExecDDL`). |
| `SetGitCommitFilterModal.tsx` | Sets or clears the commit filter (a commit hash) on a GIT REPOSITORY object, used to view files at a specific version via the `commits/` folder. Loads the current filter hash via `GetGitCommitFilter` on mount. Submits via `SetGitCommitFilter`. |

## Patterns & integration

- **IPC**: `BuildCreateGitRepositorySql`, `BuildModifyGitRepositorySql`, `ExecDDL`, `GetObjectProperties`, `ListApiIntegrations`, `ListSecretsInAccount`, `GetQuotedIdentifiersIgnoreCase`, `SetGitCommitFilter`, `GetGitCommitFilter` — all from `wailsjs/go/app/App`.
- **SQL preview**: every field change triggers a backend SQL builder call; the preview `<pre>` block in the modal body always shows what will be executed before the user clicks Create/Apply. This avoids any SQL construction in the frontend.
- **Secret inline creation**: both Create and Modify modals mount `CreateSecretModal` (from `components/secret/`) conditionally; after `onSuccess(fqn)` the secrets dropdown reloads and the new FQN is auto-selected.
- **No store usage**: these modals are purely local-state. The Sidebar triggers them via context-menu handlers and provides `db`, `schema`, and (for Modify/Filter) `name` as props.

## Gotchas

- Origin URL is **read-only** in `ModifyGitRepositoryModal` — Snowflake does not allow changing it after creation.
- `BuildModifyGitRepositorySql` returns an array of SQL strings (one per altered property), not a single statement. The modal joins them with `\n\n` for display and executes them sequentially, stopping on first error.
- `GetQuotedIdentifiersIgnoreCase` is checked in `CreateGitRepositoryModal` to correctly compute whether the object name will be case-sensitive; this mirrors the pattern used in all other object-creation modals.
