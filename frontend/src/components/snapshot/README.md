# components/snapshot

> Modals for Snowpark Container Services volume SNAPSHOT objects (issue #945).

| File | Purpose |
|---|---|
| `CreateSnapshotModal.tsx` | `CREATE SNAPSHOT` form on `CreateModalShell` with live `SqlPreview` (`BuildCreateSnapshotSql`): name, OR REPLACE / IF NOT EXISTS (mutually exclusive), **service** picker (`ListServicesInAccount`, the target schema's services first), **volume** (block volumes only, from `ListServiceVolumes`), **instance** (the instances that volume is mounted on; auto-picked when there is one), comment, `shared/TagInput`. An optional `service` prop pre-selects the source (used by the Service properties Volumes section). |
| `SnapshotPropertiesModal.tsx` | `DESCRIBE SNAPSHOT` row as read-only `InfoRow`s plus an editable COMMENT (`AlterSnapshot … SET COMMENT`; no UNSET exists, so blank sets `''`). |
| `RestoreSnapshotModal.tsx` | Name prompt for `UNDROP SNAPSHOT` (`UndropSnapshot`) — there is no history listing to pick from. |
| `blockVolumes.ts` | Pure helper: `SHOW SERVICE VOLUMES` rows → `block` volume name → instance ids. Never offers stage / local / memory volumes. Tested in `blockVolumes.test.ts`. |

## Entry points

- Object tree: **Create Object → Projects → Snapshot…**; the **Snapshots** group's
  context menu (**Create Snapshot…**, **Restore dropped snapshot…**); a snapshot's
  **Properties** / **Delete…** (`layout/Sidebar.tsx`).
- Container Services dialog → **Snapshots** tab (`containerservices/SnapshotsTab.tsx`).
- Service properties → Volumes → **Snapshot a block volume…**
  (`service/ServicePropertiesModal.tsx`).
