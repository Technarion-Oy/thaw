// Copyright (c) 2026 Technarion Oy. All rights reserved.

/**
 * Extracts all script variables declared in a Snowflake SQL document via:
 * - DECLARE blocks
 * - LET/VAR assignments
 * - FOR ... IN loops
 * * Scoped to the current block (e.g., inside $$ ... $$) and only parses
 * declarations that appear *before* the cursor.
 */
export function extractDeclaredVariables(sql: string, cursorOffset: number): Set<string> {
  const vars = new Set<string>();

  // 1. Determine if the cursor is actually inside a $$ ... $$ block.
  // We count the number of $$ tags before the cursor. If it's an odd number (1, 3, 5),
  // the block is open. If even (0, 2, 4), the block is closed and we are in standard SQL.
  const textBeforeCursor = sql.substring(0, cursorOffset);
  const dollarMatches = textBeforeCursor.match(/\$\$/g);
  const isInsideBlock = dollarMatches && dollarMatches.length % 2 !== 0;

  // If we are outside a block, return empty set so variables don't leak into standard SQL
  if (!isInsideBlock) {
    return vars;
  }

  // 2. Isolate the text from the start of the current block up to the cursor.
  const blockStart = sql.lastIndexOf("$$", cursorOffset - 1);
  const textToScan = sql.substring(blockStart, cursorOffset);

  // 3. Match DECLARE blocks
  const declareParts = textToScan.split(/\bDECLARE\b/gi);
  for (let i = 1; i < declareParts.length; i++) {
    // Only take text up to a BEGIN or END keyword
    const blockContent = declareParts[i].split(/\b(?:BEGIN|END)\b/gi)[0];
    const lines = blockContent.split(";");
    for (const line of lines) {
      const cleanLine = line
        .replace(/--.*/g, "")
        .replace(/\/\*[\s\S]*?\*\//g, "")
        .trim();

      if (!cleanLine) continue;

      const wordMatch = cleanLine.match(/[a-zA-Z0-9_$]+/);
      if (wordMatch) {
        const varName = wordMatch[0].toUpperCase();
        if (!["CURSOR", "EXCEPTION", "TYPE", "LET", "VAR"].includes(varName)) {
          vars.add(varName);
        }
      }
    }
  }

  // 4. Match LET / VAR assignments
  const inlineRegex = /\b(?:LET|VAR)\s+([a-zA-Z0-9_$]+)/gi;
  let match;
  while ((match = inlineRegex.exec(textToScan)) !== null) {
    vars.add(match[1].toUpperCase());
  }

  // 5. Match FOR loops
  const forRegex = /\bFOR\s+([a-zA-Z0-9_$]+)\s+IN\b/gi;
  while ((match = forRegex.exec(textToScan)) !== null) {
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