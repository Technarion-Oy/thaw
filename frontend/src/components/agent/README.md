# components/agent

UI for Snowflake **AGENT** objects (Cortex AI agents).

- `CreateAgentModal.tsx` — CREATE AGENT form: name + `OR REPLACE`/`IF NOT EXISTS`,
  case control, comment, an optional **Profile** (display_name / avatar / color,
  assembled into the `PROFILE` JSON object — the avatar field has a **Browse…**
  button that opens the shared `StageFilePicker` and fills it with a `@stage/file`
  image reference), and a Monaco **Specification** editor (YAML/JSON, sent via
  `FROM SPECIFICATION $THAW$ … $THAW$`). Renders a live SQL preview via
  `BuildCreateAgentSql`. The PROFILE avatar is documented only as an "image file
  name or identifier", so the field stays free-text — the stage browser is a
  convenience, not the only accepted form.
- `AgentPropertiesModal.tsx` — covers every `ALTER AGENT` option:
  - **Comment** → `SET COMMENT` (no UNSET exists for agents).
  - **Profile** → `SET PROFILE = '<json>'` (edited as three fields; the avatar
    field has the same internal-stage **Browse…** picker).
  - **Specification (live version)** → `MODIFY LIVE VERSION SET SPECIFICATION =
    $THAW$ … $THAW$`, loaded via `DescribeAgent` (the `agent_spec` column, which `SHOW
    AGENTS` omits) and edited in a Monaco editor. Saving replaces the whole spec.
  - A raw `SHOW AGENTS` Properties dump for the remaining columns.

  Agents have no `RENAME`, `UNSET`, or `TAG`, so those are intentionally absent.
- `profile.ts` — shared `buildProfileJson` / `parseProfileJson` helpers for the
  `PROFILE` JSON object used by both modals.

Backend: `internal/agent` (builder), `internal/app/agent.go` (`AlterAgent`,
`DescribeAgent`). `GET_DDL` works via the `CORTEX_AGENT` object type, so View
Definition / Compare are available for agents.

See also: `components/externalagent` (the version-based External Agent type).
