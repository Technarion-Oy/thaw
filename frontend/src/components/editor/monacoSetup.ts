// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { snowflakeMonarchLanguage, thawDarkTheme, thawLightTheme } from "./snowflakeSql";

let registered = false;

export function ensureMonacoSetup(monaco: any): void {
  if (registered) return;
  registered = true;
  monaco.languages.setMonarchTokensProvider("sql", snowflakeMonarchLanguage as any);
  monaco.editor.defineTheme("thaw-dark",  thawDarkTheme  as any);
  monaco.editor.defineTheme("thaw-light", thawLightTheme as any);
}
