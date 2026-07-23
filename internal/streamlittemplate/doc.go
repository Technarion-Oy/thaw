// SPDX-License-Identifier: GPL-3.0-or-later

// Package streamlittemplate lists and downloads Streamlit app templates from the
// Snowflake-Labs/snowflake-demo-streamlit repository (Apache-2.0). Each top-level
// folder there is a self-contained Streamlit app (streamlit_app.py,
// environment.yml, README.md, optional assets/, data/, pages/), which makes
// "scaffold a template → deploy the folder" compose directly with the local
// deploy path in internal/snowflake (DeployStreamlit).
//
// The catalog is fetched at runtime from the GitHub API with an embedded
// names-only fallback for offline / rate-limited cases (ListTemplates returns a
// Catalog whose Degraded flag signals the fallback). DownloadTemplate scaffolds a
// single chosen folder into a local destination via the git-tree API + raw file
// downloads — never a full clone of the 30+ demos — and carries along the repo's
// Apache-2.0 LICENSE plus a NOTICE provenance line for attribution.
//
// thaw:domain: Object Browser & Administration
package streamlittemplate
