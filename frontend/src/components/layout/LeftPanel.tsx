// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import ExportPanel from "../export/ExportPanel";
import FileBrowser from "../files/FileBrowser";
import GitPanel from "../git/GitPanel";

export default function LeftPanel() {
  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <ExportPanel />
      <FileBrowser />
      <GitPanel />
    </div>
  );
}
