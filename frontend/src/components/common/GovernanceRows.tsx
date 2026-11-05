// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration
//
// Table-level data-governance attach/detach rows (row access, aggregation,
// join, storage lifecycle policies) shared by the base-table PropertiesModal
// and DynamicTablePropertiesModal. Attachments aren't in SHOW output (they live
// in POLICY_REFERENCES), so these are editors, not current-value rows.

import { SECTION_HEAD, ActionRow } from "../shared/PropertyRows";
import * as dtc from "../dynamictable/dynamicTableClauses";
import { setJoinPolicy, replaceRowAccessPolicy } from "./tableClauses";

export default function GovernanceRows({ apply, joinPolicy }: {
  apply: (clause: string) => Promise<void>;
  /** ALTER TABLE only — dynamic tables have no JOIN POLICY clause. */
  joinPolicy?: boolean;
}) {
  return (
    <>
      <div style={SECTION_HEAD}>Governance</div>
      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <tbody>
          <ActionRow
            label="Row Access Policy"
            fields={["policy", "on columns (a, b)"]}
            actions={[
              { label: "Add", clause: ([p, c]) => dtc.addRowAccessPolicy(p, c) },
              { label: "Drop", clause: ([p]) => `DROP ROW ACCESS POLICY ${dtc.sqlIdent(p)}` },
              { label: "Drop all", clause: () => "DROP ALL ROW ACCESS POLICIES", danger: true, confirm: "Drop every row access policy on this table?" },
            ]}
            run={apply}
          />
          {/* DROP …, ADD … in one statement so the table is never unprotected. */}
          <ActionRow
            label="Replace Row Access Policy"
            fields={["current policy", "new policy", "on columns (a, b)"]}
            actions={[{ label: "Replace", clause: ([o, n, c]) => replaceRowAccessPolicy(o, n, c), confirm: "Swap the row access policy?" }]}
            run={apply}
          />
          <ActionRow
            label="Aggregation Policy"
            fields={["policy", "entity key (optional)"]}
            actions={[
              { label: "Set", clause: ([p, k]) => dtc.setAggregationPolicy(p, k, false) },
              { label: "Set (force)", clause: ([p, k]) => dtc.setAggregationPolicy(p, k, true) },
              { label: "Unset", clause: () => "UNSET AGGREGATION POLICY", danger: true, confirm: "Unset the aggregation policy?" },
            ]}
            run={apply}
          />
          {joinPolicy && (
            <ActionRow
              label="Join Policy"
              fields={["policy"]}
              actions={[
                { label: "Set", clause: ([p]) => setJoinPolicy(p, false) },
                { label: "Set (force)", clause: ([p]) => setJoinPolicy(p, true) },
                { label: "Unset", clause: () => "UNSET JOIN POLICY", danger: true, confirm: "Unset the join policy?" },
              ]}
              run={apply}
            />
          )}
          <ActionRow
            label="Storage Lifecycle Policy"
            fields={["policy", "on columns (a, b)"]}
            actions={[
              { label: "Add", clause: ([p, c]) => dtc.addStorageLifecyclePolicy(p, c) },
              { label: "Drop", clause: () => "DROP STORAGE LIFECYCLE POLICY", danger: true, confirm: "Drop the storage lifecycle policy?" },
            ]}
            run={apply}
          />
        </tbody>
      </table>
    </>
  );
}
