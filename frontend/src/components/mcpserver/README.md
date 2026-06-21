# components/mcpserver

UI for Snowflake **MCP SERVER** objects (Model Context Protocol).

An MCP server exposes Snowflake tools — Cortex Search, Cortex Analyst, SQL
execution, Cortex agents, and generic UDFs / procedures — to MCP clients through
a single YAML specification.

## Components

- **`CreateMCPServerModal.tsx`** — name + `OR REPLACE` / `IF NOT EXISTS` (mutually
  exclusive) + case control + a Monaco YAML editor for the tools specification.
  Calls `BuildCreateMCPServerSql` for the live preview and `ExecDDL` to run it.
- **`MCPServerPropertiesModal.tsx`** — **read-only**. MCP servers have no `ALTER`
  statement, so this panel only displays: the SHOW metadata (owner, comment), the
  `server_spec` from `DescribeMCPServer` in a read-only Monaco JSON viewer, and
  the remaining SHOW property pairs. To change a server, recreate it with
  `CREATE OR REPLACE`.

There is no `GET_DDL` for MCP servers, so View Definition / Compare / Rename are
not offered in the sidebar context menu.

See also: `components/agent` (the specification-based Agent type).
