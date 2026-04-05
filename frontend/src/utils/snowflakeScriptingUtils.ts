// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

/**
 * Extracts all script variables declared in a Snowflake SQL document via:
 * - DECLARE blocks
 * - LET/VAR assignments
 * - FOR ... IN loops
 */
export function extractDeclaredVariables(sql: string): Set<string> {
  const vars = new Set<string>();

  // 1. Match DECLARE blocks robustly (handles missing BEGIN while typing)
  const declareParts = sql.split(/\bDECLARE\b/gi);
  for (let i = 1; i < declareParts.length; i++) {
    // Take text up to BEGIN, END, or $$
    const blockContent = declareParts[i].split(/\b(?:BEGIN|END)\b|\$\$/gi)[0];
    const lines = blockContent.split(";");
    for (const line of lines) {
      const cleanLine = line
        .replace(/--.*/g, "")
        .replace(/\/\*[\s\S]*?\*\//g, "")
        .trim();

      if (!cleanLine) continue;

      // No '^' anchor, bypasses any invisible characters
      const wordMatch = cleanLine.match(/[a-zA-Z0-9_$]+/);
      if (wordMatch) {
        const varName = wordMatch[0].toUpperCase();
        // Exclude common keywords that might appear as the first word
        if (!["CURSOR", "EXCEPTION", "TYPE", "LET", "VAR"].includes(varName)) {
          vars.add(varName);
        }
      }
    }
  }

  // 2. Match LET / VAR assignments
  const inlineRegex = /\b(?:LET|VAR)\s+([a-zA-Z0-9_$]+)/gi;
  let match;
  while ((match = inlineRegex.exec(sql)) !== null) {
    vars.add(match[1].toUpperCase());
  }

  // 3. Match FOR loops
  const forRegex = /\bFOR\s+([a-zA-Z0-9_$]+)\s+IN\b/gi;
  while ((match = forRegex.exec(sql)) !== null) {
    vars.add(match[1].toUpperCase());
  }

  return vars;
}

/**
 * Determines if a colon prefix (:) is required for a variable at the given cursor offset.
 */
export function isColonRequired(sql: string, offset: number): boolean {
  const textBeforeCursor = sql.slice(0, offset);

  // 1. Guard check: If the user already typed a colon (e.g. `SELECT :are`), we 
  // don't need to add another one, otherwise Monaco will insert `::AREA_OF_CIRCLE`.
  const wordMatchStart = textBeforeCursor.search(/[a-zA-Z0-9_$]+$/);
  const posToEval = wordMatchStart >= 0 ? wordMatchStart : offset;
  const textBeforeWord = textBeforeCursor.slice(0, posToEval).trimEnd();
  
  if (textBeforeWord.endsWith(":")) {
    return false;
  }

  // 2. Clean the string backward to find the true statement start (ignoring comments/strings)
  const cleanText = textBeforeWord
    .replace(/--.*/g, " ")
    .replace(/\/\*[\s\S]*?\*\//g, " ")
    .replace(/'([^']*)'/g, " ")
    .replace(/"([^"]*)"/g, " ");

  // 3. Split by common Snowflake statement boundaries
  const segments = cleanText.split(/;|\bBEGIN\b|\bTHEN\b|\bELSE\b|\bDO\b|\bLOOP\b/i);
  const currentSegment = segments[segments.length - 1].trim();

  // If we are at the very beginning of a new statement, no colon is needed yet.
  if (!currentSegment) return false;

  // 4. Look at the very first word of the current statement segment
  // (Removed the '^' anchor so invisible spaces don't break keyword detection)
  const firstWordMatch = currentSegment.match(/[a-zA-Z0-9_$]+/);
  if (!firstWordMatch) return false;

  const firstWord = firstWordMatch[0].toUpperCase();

  // 5. If the statement starts with a standard SQL DQL/DML/DDL keyword, a colon is required
  const sqlKeywords = new Set([
    "SELECT", "INSERT", "UPDATE", "DELETE", "MERGE",
    "CREATE", "ALTER", "DROP", "TRUNCATE", "COPY",
    "CALL", "WITH", "SHOW", "DESCRIBE", "GRANT", "REVOKE",
  ]);

  return sqlKeywords.has(firstWord);
}
