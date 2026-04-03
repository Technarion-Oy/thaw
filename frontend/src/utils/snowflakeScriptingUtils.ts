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
 * Extracts all declared variables from a Snowflake Scripting block.
 * Looks for DECLARE blocks and LET/VAR keywords.
 */
export function extractDeclaredVariables(sql: string): Set<string> {
  const vars = new Set<string>();
  
  // 1. Find all DECLARE ... BEGIN blocks and extract variables from DECLARE
  const declareRegex = /DECLARE\s+([\s\S]*?)\s+BEGIN/gi;
  let m: RegExpExecArray | null;
  while ((m = declareRegex.exec(sql)) !== null) {
    const declareBody = m[1];
    // Each line in declare body that isn't a comment or empty is likely a var
    // radius_of_circle FLOAT DEFAULT 3;
    // c1 CURSOR FOR ...;
    // Simple regex for name followed by type/CURSOR
    const lines = declareBody.split("\n");
    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith("--") || trimmed.startsWith("//")) continue;
      // Match identifier at start of line
      const nameMatch = trimmed.match(/^([a-zA-Z_]\w*)/);
      if (nameMatch) {
        vars.add(nameMatch[1].toUpperCase());
      }
    }
  }

  // 2. Find all inline LET/VAR declarations
  // let x := 1;
  // var y FLOAT;
  const inlineRegex = /\b(?:LET|VAR)\s+([a-zA-Z_]\w*)\b/gi;
  while ((m = inlineRegex.exec(sql)) !== null) {
    vars.add(m[1].toUpperCase());
  }

  return vars;
}

/**
 * Determines if the current position requires a colon prefix for variables.
 * In Snowflake Scripting:
 * - Inside SQL statements (SELECT, INSERT, etc.) -> YES
 * - Inside EXECUTE IMMEDIATE strings -> NO (it's part of the string)
 * - Inside procedural expressions (x := y + 1) -> NO
 * - In session variables (SET x = :y) -> YES
 */
export function isColonRequired(sql: string, offset: number): boolean {
  const textToCursor = sql.slice(0, offset);
  
  // Find if we are inside a dollar-quoted block (Scripting context)
  const dollarMatches = [...textToCursor.matchAll(/\$([a-zA-Z0-9_]*)\$/g)];
  const inScripting = dollarMatches.length % 2 !== 0;

  if (!inScripting) {
    // Outside scripting, check if we are in a SET command
    const lastSet = textToCursor.lastIndexOf("SET");
    const lastSemicolon = textToCursor.lastIndexOf(";");
    if (lastSet > lastSemicolon) {
      // We are in a SET command. Is it after the '='?
      const lastEq = textToCursor.lastIndexOf("=");
      return lastEq > lastSet;
    }
    return false;
  }

  // Inside scripting ($$).
  // Check if we are inside a SQL statement.
  // This is a heuristic: check the last "keyword" that isn't a scripting keyword.
  const keywords = textToCursor.match(/\b([a-zA-Z_]\w*)\b/gi) || [];
  if (keywords.length === 0) return false;

  const SQL_KEYWORDS = ["SELECT", "FROM", "INSERT", "UPDATE", "DELETE", "MERGE", "CREATE", "ALTER", "DROP", "TRUNCATE", "CALL", "LIMIT"];
  const SCRIPTING_KEYWORDS = ["DECLARE", "BEGIN", "END", "IF", "THEN", "ELSE", "ELSEIF", "CASE", "WHEN", "FOR", "WHILE", "LOOP", "REPEAT", "UNTIL", "DO", "RETURN", "RAISE", "EXCEPTION", "LET", "VAR"];

  // Search backwards for the last significant keyword
  for (let i = keywords.length - 1; i >= 0; i--) {
    const kw = keywords[i].toUpperCase();
    if (SQL_KEYWORDS.includes(kw)) return true;
    if (SCRIPTING_KEYWORDS.includes(kw)) return false;
  }

  return false;
}
