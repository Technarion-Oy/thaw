# components/gateway

> Modals for creating and managing Snowflake GATEWAY (Snowpark Container Services traffic-split) objects.

## Components

| File | Purpose |
|---|---|
| `CreateGatewayModal.tsx` | Create form with a live `CREATE GATEWAY` SQL preview. Fields: name, OR REPLACE / IF NOT EXISTS (mutually exclusive — selecting one clears the other), and the specification (`FROM SPECIFICATION $THAW$ … $THAW$`) in `service/SpecEditor` + `GatewaySpecForm`, pre-seeded with one 100% target row. Submit is gated on `specProblems("gateway", …)`. |
| `GatewayPropertiesModal.tsx` | `SHOW GATEWAYS` metadata (owner, gateway type, comment) plus the `DESCRIBE GATEWAY` ingress / PrivateLink URLs (with native-clipboard copy buttons), and the **editable** specification in `SpecEditor` + `GatewaySpecForm` (keyed on the loaded spec). Saving (disabled while the spec has problems) runs `ALTER GATEWAY … FROM SPECIFICATION` — the entire `ALTER GATEWAY` surface. |
| `GatewaySpecForm.tsx` | The gateway spec form (#958): **Traffic split** (≤ 5 target rows, each an `EndpointTargetPicker` + weight, live "sum to N / 100") or **Shadow traffic** (one primary picker, weighted shadow rows). Switching type rebuilds `spec`, carrying the first endpoint over. |
| `EndpointTargetPicker.tsx` | Row control: database → schema → service → endpoint searchable dropdowns; `value` / `onChange` carry the `db.schema.service!endpoint` reference (`""` while incomplete), parsed by `service/specDoc.parseEndpointRef`. Names that need quoting (mixed case, `.`, spaces) are offered and emitted double-quoted, so they round-trip. Services come from `ListObjects` (filtered to kind SERVICE); endpoints from `ListServiceEndpoints` (`SHOW ENDPOINTS IN SERVICE`). |

## Integration

- Create delegates to IPC `BuildCreateGatewaySql` / `ExecDDL`; properties delegate
  to `GetObjectProperties` (`SHOW GATEWAYS`), `DescribeGateway` (`DESCRIBE GATEWAY`
  → spec + ingress URLs that SHOW omits), and `AlterGateway` (the spec update).
- A `traffic_split` gateway splits ingress HTTP traffic across up to five service
  endpoints by weight (weights must sum to 100); a `shadow_traffic` gateway serves
  from one `primary` endpoint and mirrors a weighted share to `shadow` endpoints. Each `value` is a fully-qualified endpoint
  `db.schema.service!endpoint` that must already exist.
- **Updating the specification is the only mutation a gateway supports** — there is
  no `RENAME`, `SET COMMENT`, or `SET TAG`. To rename, recreate with
  `CREATE OR REPLACE`. `GET_DDL` does not support gateways, so there is no
  DDL-export / View Definition / comparison path.
- URLs are copied with the Wails native `ClipboardSetText` API (WKWebView blocks
  `navigator.clipboard`).
