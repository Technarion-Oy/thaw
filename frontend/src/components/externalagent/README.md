# components/externalagent

UI for Snowflake **EXTERNAL AGENT** objects.

An external agent registers a third-party / generative-AI application for use
with AI Observability. Unlike a native AGENT it has no inline specification — it
is version-based.

- `CreateExternalAgentModal.tsx` — CREATE EXTERNAL AGENT form: name +
  `OR REPLACE`/`IF NOT EXISTS`, case control, an optional initial version name
  (`WITH VERSION`), and a comment. Live SQL preview via
  `BuildCreateExternalAgentSql`.
- `ExternalAgentPropertiesModal.tsx` — covers every `ALTER EXTERNAL AGENT`
  option:
  - **Comment** → `SET COMMENT` (no UNSET exists).
  - **Add version…** → `ADD VERSION <name>`.
  - An Overview (owner, default version), the raw `versions` JSON, and a raw
    `SHOW EXTERNAL AGENTS` Properties dump.

  External agents have no `RENAME`, `UNSET`, or `TAG`, so those are absent.

Backend: `internal/externalagent` (builder), `internal/app/externalagent.go`
(`AlterExternalAgent`). `GET_DDL` does **not** support external agents, so View
Definition / Compare are excluded for this kind.

See also: `components/agent` (the specification-based native Agent type).
