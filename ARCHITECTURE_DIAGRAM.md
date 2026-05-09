# Thaw Architecture Diagram — Agent Maintenance Guide

This document tells an LLM agent exactly how to keep `thaw_architecture.drawio` in sync with the codebase.

---

## When to update the diagram

| Change type | Update needed |
|-------------|---------------|
| New `internal/<package>` directory added | Add a node to the **Backend Services** layer |
| New Zustand store in `frontend/src/store/` | Add a cell inside `grp-stores` |
| New external service connected from Go backend | Add an external service node + edge |
| New major UI component/page in `frontend/src/` | Add a cell inside `grp-ui` |
| IPC layer restructured (new file alongside `app.go`) | Update the **Wails Bridge** layer |
| Internal package removed or merged | Remove the corresponding node and its edges |

---

## Diagram structure reference

The file is standard Draw.io XML (`mxfile > diagram > mxGraphModel > root > mxCell`).

### Cell ID conventions

| Prefix | Zone |
|--------|------|
| `layer-fe` | Frontend swimlane container |
| `layer-bridge` | Wails IPC Bridge swimlane container |
| `layer-be` | Backend Services swimlane container |
| `grp-stores` | Zustand stores sub-group (child of `layer-fe`) |
| `grp-ui` | UI components sub-group (child of `layer-fe`) |
| `fe-*` | Frontend component nodes |
| `st-*` | Zustand store nodes (children of `grp-stores`) |
| `ui-*` | UI component nodes (children of `grp-ui`) |
| `br-*` | Bridge layer nodes (children of `layer-bridge`) |
| `be-*` | Backend package nodes (children of `layer-be`) |
| `ext-*` | External service nodes (top-level, right column) |
| `e<N>` | Edges (connections) |

### Coordinate system

Coordinates for children are **relative to their parent container**.

| Container | Absolute position | Usable interior |
|-----------|-------------------|-----------------|
| `layer-fe` | x=20, y=20, w=1180, h=290 | x starts at 20, y starts at 40 (below header) |
| `grp-stores` | x=210, y=40 inside `layer-fe` | x starts at 10, y starts at 35 |
| `grp-ui` | x=760, y=40 inside `layer-fe` | x starts at 10, y starts at 35 |
| `layer-bridge` | x=20, y=340, w=1180, h=100 | y starts at 35 |
| `layer-be` | x=20, y=470, w=1180, h=250 | row 1 y=50, row 2 y=140 |
| External nodes | x=1250, right column | spaced ~80px apart vertically |

### Color palette

| Zone | fillColor | strokeColor |
|------|-----------|-------------|
| Frontend container | `#dae8fc` | `#6c8ebf` |
| Store cells | `#f0f0f0` | `#999999` |
| UI component cells | `#fff2cc` | `#d6b656` |
| Bridge container | `#e1d5e7` | `#9673a6` |
| Bridge nodes | `#f3e8ff` | `#9673a6` |
| Backend container | `#d5e8d4` | `#82b366` |
| Backend nodes | `#d5e8d4` | `#82b366` |

---

## Step-by-step agent process

### 1. Detect what changed

Read and diff these sources against the current diagram:

```bash
# New internal packages
ls internal/

# New Zustand stores
ls frontend/src/store/

# New UI pages / major components
ls frontend/src/pages/ frontend/src/components/

# New external service imports in app.go
grep -n '"github.com\|"golang.org\|openai\|google\|azure' app.go | head -40
```

### 2. Read the current diagram

```python
with open("thaw_architecture.drawio") as f:
    xml = f.read()
```

Parse it as XML and locate the `<root>` element. All `<mxCell>` nodes live directly under `<root>`.

### 3. Determine the next edge ID

Find the highest `e<N>` id already in use:

```python
import re
edge_ids = [int(m) for m in re.findall(r'id="e(\d+)"', xml)]
next_edge = max(edge_ids) + 1
```

### 4. Build the new XML snippets

#### Adding a backend package node (child of `layer-be`, row 2)

Place new nodes at the end of row 2 (y=140). Measure the rightmost existing node's x + width, then add 15px gap.

```xml
<mxCell id="be-mypkg" value="internal/mypkg&#xa;Short description&#xa;(key responsibilities)" style="rounded=1;whiteSpace=wrap;fillColor=#d5e8d4;strokeColor=#82b366;fontSize=9;" vertex="1" parent="layer-be">
  <mxGeometry x="<x>" y="140" width="175" height="55" as="geometry" />
</mxCell>
```

If the new package connects to an external service, add an edge:

```xml
<mxCell id="e<N>" style="edgeStyle=orthogonalEdgeStyle;rounded=0;exitX=1;exitY=0.5;exitDx=0;exitDy=0;" edge="1" source="be-mypkg" target="ext-<service>" parent="1">
  <mxGeometry relative="1" as="geometry" />
  <mxCell as="value" value="protocol description" />
</mxCell>
```

#### Adding a Zustand store (child of `grp-stores`)

Stores are arranged in a 3-column grid (150px wide, 40px tall, 15px gap). Add to the next available grid slot:

```xml
<mxCell id="st-mystore" value="myStore&#xa;(brief state description)" style="rounded=1;whiteSpace=wrap;fillColor=#f0f0f0;strokeColor=#999;fontSize=9;" vertex="1" parent="grp-stores">
  <mxGeometry x="<col_x>" y="<row_y>" width="150" height="40" as="geometry" />
</mxCell>
```

Grid columns start at x=10, 175, 340. Rows start at y=35, 90, 145.
If all 9 slots are full, extend `grp-stores` height by 55px and add a new row.

#### Adding an external service node

Place below the last existing external node. Each is ~80px below the previous.

```xml
<mxCell id="ext-myservice" value="Service Name&#xa;(brief description)" style="rounded=1;whiteSpace=wrap;fillColor=#fff9c4;strokeColor=#f0a500;fontStyle=1;fontSize=10;" vertex="1" parent="1">
  <mxGeometry x="1250" y="<y>" width="160" height="70" as="geometry" />
</mxCell>
```

### 5. Insert snippets into the XML

Insert new node snippets **before** the closing `</root>` tag. Insert new edge snippets **after** all node snippets, also before `</root>`.

### 6. Validate

- Confirm every `source` and `target` on edges references an existing cell `id`.
- Confirm no two cells share the same `id`.
- Confirm child cells reference the correct `parent` id.
- Open the file in Draw.io Desktop or app.diagrams.net to visually verify.

### 7. Commit and PR

Follow the project branching convention from `CLAUDE.md`:

```bash
git checkout -b docs/update-architecture-diagram
git add thaw_architecture.drawio
git commit -m "docs: Update system architecture diagram — <brief reason>"
git push -u origin docs/update-architecture-diagram
gh pr create --repo Technarion-Oy/thaw --base main \
  --title "docs: Update system architecture diagram" \
  --body "..."
```

Use the `docs:` commit prefix — this does not trigger a release.

---

## Quick reference: existing node IDs

### Backend nodes (`parent="layer-be"`)

| ID | Package |
|----|---------|
| `be-sf` | `internal/snowflake` |
| `be-sqled` | `internal/sqleditor` |
| `be-ai` | `internal/ai` |
| `be-ddl` | `internal/ddl` |
| `be-git` | `internal/gitrepo` |
| `be-fnmeta` | `internal/fnmeta` |
| `be-config` | `internal/config` |
| `be-fs` | `internal/filesystem` |
| `be-sfconf` | `internal/sfconfig` |
| `be-builders` | `internal/{fileformat,stage,integrations,procedure,pipe,secret,tasks}` |
| `be-infra` | `internal/{logger,telemetry,crashreport}` |

### External service nodes (`parent="1"`)

| ID | Service |
|----|---------|
| `ext-sf` | Snowflake Cloud |
| `ext-ai` | AI APIs (OpenAI / Google / Ollama / Azure) |
| `ext-git` | Git Remotes (GitHub / GitLab / Bitbucket) |
| `ext-fs` | Local Filesystem |
| `ext-sqlite` | SQLite |

### Edges

| ID | Connection |
|----|-----------|
| `e1` | Frontend layer → `br-wailsjs` (IPC calls) |
| `e2` | `br-wailsjs` → `br-ipc` |
| `e3` | `br-events` → Frontend layer (Wails events, dashed) |
| `e4` | `br-ipc` → `br-events` |
| `e5` | `br-ipc` → Backend layer |
| `e6` | `be-sf` → `ext-sf` |
| `e7` | `be-ai` → `ext-ai` |
| `e8` | `be-git` → `ext-git` |
| `e9` | `be-fs` → `ext-fs` |
| `e10` | `be-fnmeta` → `ext-sqlite` |
| `e11` | `be-ddl` → `ext-fs` (dashed) |
| `e12` | `be-fnmeta` → `ext-sf` (background sync, dashed) |
| `e13` | `be-config` → `ext-fs` (TOML + admin policy, dashed) |
