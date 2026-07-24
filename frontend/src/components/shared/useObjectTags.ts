// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { useCallback, useEffect, useState } from "react";
import { GetObjectTagReferences, ListAccountTags } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { quoteIdent } from "./ObjectNameCaseControl";
import { quoteTextLit } from "./sqlEscape";
import type { EditableTag } from "./TagsRow";
import { parseAllowedValues } from "../tag/allowedValues";

// A tag-name dropdown option (the `nameOptions` shape TagsRow accepts): the value
// is the quoted FQN inserted verbatim into `SET TAG <fqn> = …`, the label is the
// readable dotted name, and allowedValues (from SHOW TAGS' allowed_values column)
// turns the value field into a whitelist dropdown when non-empty.
export interface TagNameOption { value: string; label: string; allowedValues?: string[] }

// selfLevel folds a browser kind onto the object domain that
// INFORMATION_SCHEMA.TAG_REFERENCES reports in its LEVEL column for a tag applied
// directly to the object — the client-side mirror of the backend tagReferenceDomain
// fold (internal/app/tag.go). A tag whose LEVEL matches this is directly applied
// (removable here); anything else is inherited from a container (schema / database
// / account) and shown read-only, since it must be unset where it was applied.
function selfLevel(kind: string): string {
  switch (kind.trim().toUpperCase()) {
    case "DYNAMIC TABLE":
    case "EXTERNAL TABLE":
    case "ICEBERG TABLE":
    case "HYBRID TABLE":
    case "EVENT TABLE":
      return "TABLE";
    case "MATERIALIZED VIEW":
      return "VIEW";
    case "EXTERNAL FUNCTION":
    case "DATA METRIC FUNCTION":
      return "FUNCTION";
    default:
      return kind.trim().toUpperCase();
  }
}

// parseObjectTags maps a GetObjectTagReferences result (TAG_DATABASE / TAG_SCHEMA /
// TAG_NAME / TAG_VALUE / LEVEL) into TagsRow chips. The chip key is the quoted FQN
// handed straight back to `UNSET TAG`; a tag inherited from a higher level is kept
// for context as a non-removable "(inherited)" chip.
export function parseObjectTags(res: snowflake.QueryResult | null, kind: string): EditableTag[] {
  const cols = (res?.columns ?? []).map((c) => c.toLowerCase());
  const ci = (n: string) => cols.indexOf(n);
  const dbI = ci("tag_database"), scI = ci("tag_schema"), nmI = ci("tag_name"),
    vlI = ci("tag_value"), lvI = ci("level");
  const self = selfLevel(kind);
  return (res?.rows ?? []).map((row): EditableTag => {
    const tdb = dbI >= 0 ? String(row[dbI] ?? "") : "";
    const tsc = scI >= 0 ? String(row[scI] ?? "") : "";
    const tnm = nmI >= 0 ? String(row[nmI] ?? "") : "";
    const qualified = [tdb, tsc, tnm].filter(Boolean).map(quoteIdent).join(".");
    const level = lvI >= 0 ? String(row[lvI] ?? "").toUpperCase() : "";
    const inherited = level !== "" && level !== self;
    return {
      key: qualified,
      name: tnm,
      value: vlI >= 0 ? String(row[vlI] ?? "") : "",
      removable: !inherited,
      suffix: inherited ? " (inherited)" : "",
    };
  });
}

// tagNameOptions turns a ListAccountTags (SHOW TAGS IN ACCOUNT) result into the
// searchable tag-name dropdown options: the quoted FQN as value (so a mixed-case
// tag name round-trips through the ALTER statement), the dotted name as label, and
// the parsed allowed-values whitelist. Returns undefined for an empty / unreadable
// catalog so TagsRow falls back to a free-text name field.
function tagNameOptions(res: snowflake.QueryResult | null): TagNameOption[] | undefined {
  const cols = res?.columns ?? [];
  const iName = cols.indexOf("name");
  const iDb = cols.indexOf("database_name");
  const iSc = cols.indexOf("schema_name");
  const iAllowed = cols.indexOf("allowed_values");
  if (iName < 0) return undefined;
  const opts = (res?.rows ?? []).map((r) => {
    const nm = String(r[iName]);
    const db = iDb >= 0 && r[iDb] != null ? String(r[iDb]) : "";
    const sc = iSc >= 0 && r[iSc] != null ? String(r[iSc]) : "";
    const parts = [db, sc, nm].filter(Boolean);
    const allowedValues = iAllowed >= 0 && r[iAllowed] != null ? parseAllowedValues(String(r[iAllowed])) : [];
    return { value: parts.map(quoteIdent).join("."), label: parts.join("."), allowedValues };
  });
  return opts.length ? opts : undefined;
}

export interface UseObjectTagsArgs {
  // The object's browser kind / Snowflake domain, e.g. "STREAMLIT", "TASK",
  // "DYNAMIC TABLE" — passed to GetObjectTagReferences and folded to detect
  // inherited tags.
  kind: string;
  db: string;
  schema: string;
  name: string;
  // The comma-separated argument-type signature for callable objects
  // (functions / procedures, e.g. "NUMBER, VARCHAR"); required there so the
  // TAG_REFERENCES read resolves the overload, ignored for every other kind.
  args?: string;
  // Applies the given ALTER clause (`SET TAG …` / `UNSET TAG …`) via the object's
  // own domain-specific ALTER IPC method, e.g.
  // `(clause) => AlterStreamlit(db, schema, name, clause)`. Using each object's own
  // ALTER builder keeps the correct object keyword (and the argument signature for
  // callable objects) without a per-domain generic write path.
  alter: (clause: string) => Promise<void>;
  // When false, skips the tag read (for objects whose identity isn't ready yet).
  enabled?: boolean;
}

export interface UseObjectTags {
  tags: EditableTag[];
  // The account tag catalog for TagsRow's searchable name dropdown, or undefined
  // when the catalog is empty / unreadable (TagsRow then shows a free-text field).
  nameOptions: TagNameOption[] | undefined;
  setTag: (name: string, value: string) => Promise<void>;
  unsetTag: (key: string) => Promise<void>;
  reload: () => Promise<void>;
}

// useObjectTags is the shared read/write engine behind the Tags section of every
// object Properties modal. It reads the object's current tags from the immediate-
// consistency INFORMATION_SCHEMA.TAG_REFERENCES table function (so a SET/UNSET
// reflects at once), loads the account tag catalog for name suggestions with
// allowed-value enforcement, and applies each add/remove through the caller's
// domain-specific ALTER method. All reads are best-effort — a failed tag read or
// catalog load degrades gracefully (empty chips / free-text name) and never blocks
// the modal or the write path.
export function useObjectTags({ kind, db, schema, name, args = "", alter, enabled = true }: UseObjectTagsArgs): UseObjectTags {
  const [tags, setTags] = useState<EditableTag[]>([]);
  const [nameOptions, setNameOptions] = useState<TagNameOption[] | undefined>(undefined);

  const reload = useCallback(async () => {
    if (!enabled) { setTags([]); return; }
    try {
      const t = await GetObjectTagReferences(kind, db, schema, name, args);
      setTags(parseObjectTags(t, kind));
    } catch {
      setTags([]);
    }
  }, [kind, db, schema, name, args, enabled]);

  useEffect(() => { reload(); }, [reload]);

  // The account tag catalog (SHOW TAGS IN ACCOUNT) backs the name dropdown; loaded
  // once and best-effort, so accounts without governance access just get a
  // free-text name field.
  useEffect(() => {
    let live = true;
    ListAccountTags()
      .then((r) => { if (live) setNameOptions(tagNameOptions(r)); })
      .catch(() => { if (live) setNameOptions(undefined); });
    return () => { live = false; };
  }, []);

  const setTag = useCallback(async (tagName: string, value: string) => {
    // tagName is inserted verbatim — a quoted FQN from the catalog dropdown or a
    // free-typed name; the value is a quoted string literal.
    await alter(`SET TAG ${tagName} = ${quoteTextLit(value)}`);
    await reload();
  }, [alter, reload]);

  const unsetTag = useCallback(async (key: string) => {
    await alter(`UNSET TAG ${key}`);
    await reload();
  }, [alter, reload]);

  return { tags, nameOptions, setTag, unsetTag, reload };
}
