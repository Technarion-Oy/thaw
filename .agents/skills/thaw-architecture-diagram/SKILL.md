---
name: thaw-architecture-diagram
description: >-
  Use this skill when updating the Draw.io system architecture diagram
  (`docs/concepts/thaw_architecture.drawio`) after adding or removing internal packages,
  Zustand stores, major UI components, or external services.
---

# Thaw Architecture Diagram Maintenance Workflow

Follow `docs/concepts/ARCHITECTURE_DIAGRAM.md` as the authoritative reference when updating `docs/concepts/thaw_architecture.drawio`.

## Steps
1. Read `docs/concepts/ARCHITECTURE_DIAGRAM.md` for exact cell ID conventions (`be-*`, `st-*`, `ui-*`, `br-*`, `ext-*`, `e<N>`), relative coordinates, and color palette.
2. Detect structural drift against the current diagram:
   ```bash
   ls internal/
   ls frontend/src/store/
   ls frontend/src/pages/ frontend/src/components/
   ```
3. Inspect `docs/concepts/thaw_architecture.drawio` to find the highest existing `e<N>` edge ID and the rightmost/bottommost node coordinates in the target container (`layer-be`, `grp-stores`, `grp-ui`, `layer-bridge`, or external column `x=1250`).
4. Insert new node `<mxCell>` elements before `</root>` and new edge `<mxCell>` elements after all nodes (still before `</root>`), preserving valid XML escaping (`&#xa;` for line breaks).
5. Validate the XML before committing:
   - Parse cleanly with `python3 -c 'import xml.etree.ElementTree as ET; ET.parse("docs/concepts/thaw_architecture.drawio")'`.
   - Confirm every `id` is unique, every edge `source`/`target` exists, and every child `parent` exists.
6. Commit with a `docs:` Conventional Commit prefix (does not trigger a release).
