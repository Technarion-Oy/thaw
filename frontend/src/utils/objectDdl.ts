// Object kinds Snowflake's GET_DDL cannot render. The list is generated from the
// canonical Go registry (internal/objectkind — the kinds with no GET_DDL object
// type), the same source the backend's GetObjectDDL guard derives from, so the
// two can no longer drift.
// Every frontend DDL entry point (sidebar hover, editor cmd/ctrl-hover, View
// Definition, comparison) must skip these so it doesn't fire a doomed GET_DDL,
// which the gosnowflake driver logs as error noise on every attempt.
import { DDL_UNSUPPORTED_KINDS } from "../generated/objectKinds";

export { DDL_UNSUPPORTED_KINDS };

// A nullish/unknown kind is treated as supported (not in the blocklist) — matches
// the old inline `objKind !== "..."` chains, which showed the item for undefined.
export const kindSupportsDdl = (kind: string | null | undefined): boolean =>
  !kind || !DDL_UNSUPPORTED_KINDS.has(kind.toUpperCase().trim());
