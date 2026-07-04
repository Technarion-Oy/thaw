// Object kinds Snowflake's GET_DDL cannot render. Mirror of the guard in
// internal/snowflake/client.go GetObjectDDL — keep the two lists in sync.
// Every frontend DDL entry point (sidebar hover, editor cmd/ctrl-hover, View
// Definition, comparison) must skip these so it doesn't fire a doomed GET_DDL,
// which the gosnowflake driver logs as error noise on every attempt.
export const DDL_UNSUPPORTED_KINDS = new Set<string>([
  "IMAGE REPOSITORY", "SERVICE", "GATEWAY", "PACKAGES POLICY", "MODEL",
  "MODEL MONITOR", "DATASET", "CORTEX SEARCH SERVICE", "EXTERNAL AGENT", "MCP SERVER",
]);

// A nullish/unknown kind is treated as supported (not in the blocklist) — matches
// the old inline `objKind !== "..."` chains, which showed the item for undefined.
export const kindSupportsDdl = (kind: string | null | undefined): boolean =>
  !kind || !DDL_UNSUPPORTED_KINDS.has(kind.toUpperCase().trim());
