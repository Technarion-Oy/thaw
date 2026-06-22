# components/gateway

> Modals for creating and managing Snowflake GATEWAY (Snowpark Container Services traffic-split) objects.

## Components

| File | Purpose |
|---|---|
| `CreateGatewayModal.tsx` | Create form with a live `CREATE GATEWAY` SQL preview. Fields: name, OR REPLACE / IF NOT EXISTS (mutually exclusive — selecting one clears the other), and a Monaco YAML editor for the traffic-split specification (`FROM SPECIFICATION $THAW$ … $THAW$`), pre-seeded with a single-endpoint template, with an `EndpointTargetPicker` above it. |
| `GatewayPropertiesModal.tsx` | `SHOW GATEWAYS` metadata (owner, gateway type, comment) plus the `DESCRIBE GATEWAY` ingress / PrivateLink URLs (with native-clipboard copy buttons), and an **editable** Monaco YAML specification (also with an `EndpointTargetPicker`). Saving runs `ALTER GATEWAY … FROM SPECIFICATION` — the entire `ALTER GATEWAY` surface. |
| `EndpointTargetPicker.tsx` | Database → schema → service → endpoint searchable dropdowns + a weight, with an **Insert** button that drops a ready-made weighted `targets` entry (`db.schema.service!endpoint`) into the spec editor at the caret — so users don't hand-type fully-qualified endpoint references. Services come from `ListObjects` (filtered to kind SERVICE); endpoints from `ListServiceEndpoints` (`SHOW ENDPOINTS IN SERVICE`). |
| `insertSpecTarget.ts` | Shared helper that inserts a YAML target block into a Monaco editor at the caret (prefixing a newline when the current line is non-empty), with a fallback that appends to the spec text when the editor instance isn't ready. |

## Integration

- Create delegates to IPC `BuildCreateGatewaySql` / `ExecDDL`; properties delegate
  to `GetObjectProperties` (`SHOW GATEWAYS`), `DescribeGateway` (`DESCRIBE GATEWAY`
  → spec + ingress URLs that SHOW omits), and `AlterGateway` (the spec update).
- A gateway splits ingress HTTP traffic across up to five service endpoints by
  weight (weights must sum to 100). Each `value` is a fully-qualified endpoint
  `db.schema.service!endpoint` that must already exist.
- **Updating the specification is the only mutation a gateway supports** — there is
  no `RENAME`, `SET COMMENT`, or `SET TAG`. To rename, recreate with
  `CREATE OR REPLACE`. `GET_DDL` does not support gateways, so there is no
  DDL-export / View Definition / comparison path.
- URLs are copied with the Wails native `ClipboardSetText` API (WKWebView blocks
  `navigator.clipboard`).
