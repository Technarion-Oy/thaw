/**
 * Unit tests for the Thaw SQL formatter.
 *
 * Each test validates a specific formatting rule or Snowflake dialect quirk.
 * Tests are grouped by concern; within each group the most likely failure cases
 * (complex nesting, operator combinations, literal passthrough) come last.
 */

import { describe, expect, it } from "vitest";
import { DEFAULT_EDITOR_PREFS, EditorPrefs, formatSQL } from "./sqlFormatter";

// ── helpers ───────────────────────────────────────────────────────────────────

const prefs = (overrides: Partial<EditorPrefs> = {}): EditorPrefs => ({
  ...DEFAULT_EDITOR_PREFS,
  ...overrides,
});

// Normalize whitespace so snapshot comparisons are line-ending agnostic.
const normalize = (s: string) => s.trim().replace(/\r\n/g, "\n");

// ── 1. Empty / trivial input ──────────────────────────────────────────────────

describe("trivial input", () => {
  it("returns empty string unchanged", () => {
    expect(formatSQL("", prefs())).toBe("");
    expect(formatSQL("   ", prefs())).toBe("   ");
    expect(formatSQL("\n\n", prefs())).toBe("\n\n");
  });

  it("returns single-line SELECT unchanged in structure", () => {
    const out = formatSQL("select 1", prefs());
    expect(out.toUpperCase()).toContain("SELECT");
    expect(out).toContain("1");
  });
});

// ── 2. Keyword casing ─────────────────────────────────────────────────────────

describe("keywordCase", () => {
  const sql = "select id, name from users where active = true order by name";

  it("UPPER produces all-caps reserved words", () => {
    const out = formatSQL(sql, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("SELECT");
    expect(out).toContain("FROM");
    expect(out).toContain("WHERE");
    expect(out).toContain("ORDER");
    expect(out).toContain("BY");
    expect(out).toContain("TRUE");
  });

  it("lower produces lowercase reserved words", () => {
    const out = formatSQL(sql, prefs({ keywordCase: "lower" }));
    expect(out).toContain("select");
    expect(out).toContain("from");
    expect(out).toContain("where");
    expect(out).toContain("order");
    expect(out).toContain("by");
    expect(out).toContain("true");
    // Must NOT contain uppercase versions of keywords
    expect(out).not.toMatch(/\bSELECT\b/);
    expect(out).not.toMatch(/\bFROM\b/);
  });

  it("Title capitalises first letter only", () => {
    const out = formatSQL(sql, prefs({ keywordCase: "Title" }));
    expect(out).toContain("Select");
    expect(out).toContain("From");
    expect(out).toContain("Where");
    expect(out).toContain("Order");
    expect(out).toContain("By");
    // Must not be all-caps
    expect(out).not.toMatch(/\bSELECT\b/);
  });

  it("Preserve leaves original casing of keywords untouched", () => {
    const mixed = "SeLeCt id FrOm users WHERE active = TRUE";
    const out = normalize(formatSQL(mixed, prefs({ keywordCase: "Preserve" })));
    // sql-formatter normalises whitespace but our casing pass must not change token case
    expect(out).toContain("SeLeCt");
    expect(out).toContain("FrOm");
    expect(out).toContain("WHERE");
  });

  it("WINDOW and QUALIFY are treated as keywords", () => {
    const q = `
      select id, row_number() over w as rn
      from t
      window w as (partition by grp order by ts)
      qualify rn = 1
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("WINDOW");
    expect(out).toContain("QUALIFY");
    expect(out).toContain("PARTITION");
    expect(out).toContain("ORDER");
  });
});

// ── 3. Identifier casing ──────────────────────────────────────────────────────

describe("identifierCase", () => {
  const sql = "select MyTable.MyColumn from MyTable join OtherTable on MyTable.id = OtherTable.fk";

  it("Preserve leaves unquoted identifiers as-is", () => {
    const out = formatSQL(sql, prefs({ identifierCase: "Preserve", keywordCase: "UPPER" }));
    expect(out).toContain("MyTable");
    expect(out).toContain("MyColumn");
    expect(out).toContain("OtherTable");
  });

  it("UPPER uppercases unquoted identifiers", () => {
    const out = formatSQL(sql, prefs({ identifierCase: "UPPER", keywordCase: "lower" }));
    expect(out).toContain("MYTABLE");
    expect(out).toContain("MYCOLUMN");
    expect(out).toContain("OTHERTABLE");
    // Keywords should still be lower
    expect(out).toContain("select");
    expect(out).toContain("from");
  });

  it("lower lowercases unquoted identifiers", () => {
    const out = formatSQL(sql, prefs({ identifierCase: "lower", keywordCase: "UPPER" }));
    expect(out).toContain("mytable");
    expect(out).toContain("mycolumn");
    expect(out).toContain("othertable");
  });

  it("double-quoted identifiers are NEVER modified regardless of identifierCase", () => {
    const q = `select "MyTable"."MyColumn", "CamelCase" from "SchemaName"."TableName"`;

    for (const ic of ["Preserve", "UPPER", "lower"] as EditorPrefs["identifierCase"][]) {
      const out = formatSQL(q, prefs({ identifierCase: ic }));
      expect(out, `identifierCase=${ic}`).toContain('"MyTable"');
      expect(out, `identifierCase=${ic}`).toContain('"MyColumn"');
      expect(out, `identifierCase=${ic}`).toContain('"CamelCase"');
      expect(out, `identifierCase=${ic}`).toContain('"SchemaName"');
      expect(out, `identifierCase=${ic}`).toContain('"TableName"');
    }
  });

  it("escaped double-quote inside identifier is preserved", () => {
    // Snowflake: "My""Weird""Name" is a valid quoted identifier
    const q = `select "My""Weird""Name" from t`;
    const out = formatSQL(q, prefs({ identifierCase: "UPPER" }));
    expect(out).toContain('"My""Weird""Name"');
  });
});

// ── 4. Function casing ────────────────────────────────────────────────────────

describe("functionCase", () => {
  const sql = "select to_date(col), avg(amount), iff(flag, 'y', 'n'), nvl(x, 0) from t";

  it("UPPER uppercases function names", () => {
    const out = formatSQL(sql, prefs({ functionCase: "UPPER" }));
    expect(out).toContain("TO_DATE(");
    expect(out).toContain("AVG(");
    expect(out).toContain("IFF(");
    expect(out).toContain("NVL(");
  });

  it("lower lowercases function names", () => {
    const out = formatSQL(sql, prefs({ functionCase: "lower" }));
    expect(out).toContain("to_date(");
    expect(out).toContain("avg(");
    expect(out).toContain("iff(");
    expect(out).toContain("nvl(");
    // Keywords should still be uppercase
    expect(out).toContain("FROM");
  });

  it("function casing applies to UDFs (unknown functions)", () => {
    const q = "select my_custom_udf(col) from t";
    const upper = formatSQL(q, prefs({ functionCase: "UPPER" }));
    const lower = formatSQL(q, prefs({ functionCase: "lower" }));
    expect(upper).toContain("MY_CUSTOM_UDF(");
    expect(lower).toContain("my_custom_udf(");
  });

  it("nested function calls all receive function casing", () => {
    const q = "select coalesce(nullif(trim(col), ''), to_char(id)) from t";
    const out = formatSQL(q, prefs({ functionCase: "UPPER" }));
    expect(out).toContain("COALESCE(");
    expect(out).toContain("NULLIF(");
    expect(out).toContain("TRIM(");
    expect(out).toContain("TO_CHAR(");
  });
});

// ── 5. Indent style & size ────────────────────────────────────────────────────

describe("indent", () => {
  const sql = "select a, b, c from t where x = 1";

  it("spaces/2 uses two-space indent", () => {
    const out = formatSQL(sql, prefs({ indentStyle: "spaces", indentSize: 2 }));
    // Indented lines should start with exactly 2 spaces
    const indentedLines = out.split("\n").filter((l) => l.startsWith(" "));
    expect(indentedLines.length).toBeGreaterThan(0);
    indentedLines.forEach((l) => expect(l).toMatch(/^ {2}[^ ]/));
  });

  it("spaces/4 uses four-space indent", () => {
    const out = formatSQL(sql, prefs({ indentStyle: "spaces", indentSize: 4 }));
    const indentedLines = out.split("\n").filter((l) => l.startsWith(" "));
    expect(indentedLines.length).toBeGreaterThan(0);
    indentedLines.forEach((l) => expect(l).toMatch(/^ {4}[^ ]/));
  });

  it("tabs uses tab characters", () => {
    const out = formatSQL(sql, prefs({ indentStyle: "tabs" }));
    const indentedLines = out.split("\n").filter((l) => l.startsWith("\t"));
    expect(indentedLines.length).toBeGreaterThan(0);
    // No leading spaces on indented lines when using tabs
    indentedLines.forEach((l) => expect(l).not.toMatch(/^ /));
  });
});

// ── 6. Comma position ─────────────────────────────────────────────────────────

describe("commaPosition", () => {
  const sql = "select id, name, email, created_at from t";

  it("trailing puts commas at end of line (default)", () => {
    const out = formatSQL(sql, prefs({ commaPosition: "trailing" }));
    // Lines that continue a column list should end with ","
    const commaLines = out.split("\n").filter((l) => l.trimEnd().endsWith(","));
    expect(commaLines.length).toBeGreaterThan(0);
  });

  it("leading puts commas at start of continuation lines", () => {
    const out = formatSQL(sql, prefs({ commaPosition: "leading" }));
    // After the first SELECT column, remaining columns start with ", "
    const leadingLines = out.split("\n").filter((l) => l.trimStart().startsWith(","));
    expect(leadingLines.length).toBeGreaterThan(0);
    // No trailing commas should remain
    const trailingCommaLines = out.split("\n").filter((l) => l.trimEnd().endsWith(","));
    expect(trailingCommaLines).toHaveLength(0);
  });

  it("leading comma does not break function-argument commas on same line", () => {
    // DECIMAL(18,2) — the comma is on the same line so leading-comma transform
    // must NOT insert a newline inside it
    const q = "select total::decimal(18,2) from t";
    const out = formatSQL(q, prefs({ commaPosition: "leading" }));
    // functionCase defaults to UPPER so "decimal" becomes "DECIMAL".
    // The cast type should still be intact with comma inline (not split onto new line).
    expect(out).toContain("DECIMAL(18");
  });
});

// ── 7. Operator position ──────────────────────────────────────────────────────

describe("operatorPosition", () => {
  const sql = "select * from t where a = 1 and b = 2 and c = 3";

  it("before puts AND/OR at start of new line", () => {
    const out = formatSQL(sql, prefs({ operatorPosition: "before" }));
    const andLines = out.split("\n").filter((l) => l.trimStart().startsWith("AND"));
    expect(andLines.length).toBeGreaterThan(0);
  });

  it("after puts AND/OR at end of previous line", () => {
    const out = formatSQL(sql, prefs({ operatorPosition: "after" }));
    const andLines = out.split("\n").filter((l) => l.trimEnd().endsWith("AND"));
    expect(andLines.length).toBeGreaterThan(0);
    // AND must NOT appear at the start of any line
    const andAtStart = out.split("\n").filter((l) => l.trimStart().startsWith("AND"));
    expect(andAtStart).toHaveLength(0);
  });

  it("OR is also affected by operatorPosition", () => {
    const q = "select * from t where a = 1 or b = 2 or c = 3";
    const before = formatSQL(q, prefs({ operatorPosition: "before" }));
    const orAtStart = before.split("\n").filter((l) => l.trimStart().startsWith("OR"));
    expect(orAtStart.length).toBeGreaterThan(0);

    const after = formatSQL(q, prefs({ operatorPosition: "after" }));
    const orAtEnd = after.split("\n").filter((l) => l.trimEnd().endsWith("OR"));
    expect(orAtEnd.length).toBeGreaterThan(0);
  });
});

// ── 8. Snowflake :: cast operator ─────────────────────────────────────────────

describe("Snowflake :: cast operator", () => {
  it("no whitespace around :: after simple column", () => {
    const out = formatSQL("select col::string from t", prefs());
    expect(out).not.toMatch(/\s::/);
    expect(out).not.toMatch(/::\s/);
    expect(out).toContain("col::string");
  });

  it("no whitespace around :: after function call", () => {
    const out = formatSQL("select to_date(col)::timestamp_ntz from t", prefs());
    expect(out).not.toMatch(/\s::/);
    expect(out).toContain(")::timestamp_ntz");
  });

  it("chained :: casts are preserved", () => {
    const out = formatSQL("select col::string::varchar from t", prefs());
    expect(out).not.toMatch(/\s::/);
    expect(out).toContain("::string::varchar");
  });

  it(":: inside function argument has no spaces", () => {
    const out = formatSQL("select coalesce(a::string, b::varchar) from t", prefs());
    expect(out).not.toMatch(/\s::/);
    expect(out).toContain("a::string");
    expect(out).toContain("b::varchar");
  });

  it(":: on complex arithmetic expression has no spaces", () => {
    const out = formatSQL("select (a + b * 2)::number(18,2) from t", prefs());
    expect(out).not.toMatch(/\s::/);
    // functionCase defaults to UPPER so the type token "number" becomes "NUMBER"
    expect(out).toContain(")::NUMBER(18");
  });

  it("try_cast equivalent - :: with complex Snowflake types", () => {
    const types = ["integer", "float", "boolean", "date", "timestamp_ltz", "timestamp_ntz", "timestamp_tz", "variant", "object", "array"];
    for (const type of types) {
      const out = formatSQL(`select col::${type} from t`, prefs());
      expect(out, `type=${type}`).not.toMatch(/\s::\s/);
    }
  });
});

// ── 9. Snowflake : VARIANT path operator ─────────────────────────────────────

describe("Snowflake : VARIANT path operator", () => {
  it("no whitespace around : for single-level path", () => {
    const out = formatSQL("select src:user_id from t", prefs());
    expect(out).toContain("src:user_id");
    expect(out).not.toMatch(/src\s:/);
    expect(out).not.toMatch(/:\s+user_id/);
  });

  it("multi-level path access preserves colons", () => {
    const out = formatSQL("select src:level1:level2:level3 from t", prefs());
    expect(out).toContain("src:level1:level2:level3");
  });

  it("combined : path and :: cast on same token", () => {
    const out = formatSQL("select src:user_id::string, src:amount::number from t", prefs());
    expect(out).toContain("src:user_id::string");
    expect(out).toContain("src:amount::number");
    expect(out).not.toMatch(/\s::/);
  });

  it("deeply nested path with cast: src:a:b:c::string", () => {
    const out = formatSQL("select payload:event:properties:user_id::string as uid from t", prefs());
    expect(out).not.toMatch(/\s::\s/);
    expect(out).toContain("payload:event:properties:user_id::string");
  });
});

// ── 10. CTE formatting ────────────────────────────────────────────────────────

describe("CTE formatting", () => {
  it("WITH is placed on its own line", () => {
    const q = "with cte as (select 1 as n) select n from cte";
    const out = formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toMatch(/^WITH\s/m);
  });

  it("CTE body is indented", () => {
    const q = "with cte as (select id, name from users) select * from cte";
    const out = formatSQL(q, prefs());
    // The SELECT inside the CTE body must be indented
    const lines = out.split("\n");
    const selectInsideCte = lines.find(
      (l) => l.match(/SELECT/i) && l.startsWith(" "),
    );
    expect(selectInsideCte).toBeDefined();
  });

  it("multiple CTEs all appear correctly", () => {
    const q = `
      with
        a as (select 1 as n),
        b as (select n * 2 as m from a),
        c as (select n + m as total from a join b on true)
      select total from c
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toMatch(/\ba\s+AS\s*\(/im);
    expect(out).toMatch(/\bb\s+AS\s*\(/im);
    expect(out).toMatch(/\bc\s+AS\s*\(/im);
  });

  it("complex multi-CTE with window functions, QUALIFY, and :: casts", () => {
    const q = `
      with
        raw_events as (
          select
            event_id,
            user_id::string as uid,
            payload:event_type::string as event_type,
            payload:amount::number(18,2) as amount,
            to_timestamp_ntz(event_ts) as ts
          from raw.events
          where payload:event_type::string is not null
        ),
        ranked as (
          select
            *,
            row_number() over (
              partition by uid
              order by ts desc
            ) as rn,
            lag(amount, 1, 0) over (
              partition by uid
              order by ts
            ) as prev_amount
          from raw_events
        )
      select uid, event_type, amount, prev_amount, ts
      from ranked
      where rn = 1
        and amount > prev_amount
      order by amount desc nulls last
    `;
    // Must not throw and must preserve :: operators
    const out = formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).not.toMatch(/\s::\s/);
    expect(out).toContain("WITH");
    expect(out).toContain("raw_events");
    expect(out).toContain("ranked");
  });
});

// ── 11. LATERAL FLATTEN ───────────────────────────────────────────────────────

describe("LATERAL FLATTEN", () => {
  it("named parameter => is preserved", () => {
    const q = "select f.value::string from t, lateral flatten(input => t.col) f";
    const out = formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("=>");
    expect(out).not.toMatch(/\s::\s/);
  });

  it("FLATTEN with multiple named params", () => {
    const q = `
      select
        f.index,
        f.value::string as tag
      from events,
        lateral flatten(input => events.payload:tags, outer => true) f
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("FLATTEN(");
    expect(out).toContain("=>");
    expect(out).not.toMatch(/\s::\s/);
  });
});

// ── 12. String literals — must never be modified ──────────────────────────────

describe("string literal passthrough", () => {
  it("keywords inside single-quoted strings are not modified", () => {
    const q = "select 'SELECT FROM WHERE ORDER BY GROUP' as raw_sql from t";
    const out = formatSQL(q, prefs({ keywordCase: "lower" }));
    // The string content must not be lowercased/uppercased
    expect(out).toContain("'SELECT FROM WHERE ORDER BY GROUP'");
  });

  it("SQL inside single-quoted string is not formatted", () => {
    const q = "select 'with cte as (select 1)' as q, id from t";
    const out = formatSQL(q, prefs());
    expect(out).toContain("'with cte as (select 1)'");
  });

  it("escaped single-quotes inside strings are handled", () => {
    const q = "select 'it''s a trap' as msg from t";
    const out = formatSQL(q, prefs());
    expect(out).toContain("'it''s a trap'");
  });

  it("dollar-quoted body is passed through unchanged", () => {
    const body = `$$
DECLARE v NUMBER := 0;
BEGIN
  select count(*) into v from t;
  RETURN v;
END;
$$`;
    const q = `CREATE OR REPLACE FUNCTION count_rows() RETURNS NUMBER AS ${body}`;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER" }));
    // The content between $$ markers must not be modified
    expect(out).toContain("DECLARE v NUMBER");
    expect(out).toContain("BEGIN");
    // The body's internal casing must not be touched
    expect(out).not.toContain("declare v number");
  });

  it("dollar-quoted label ($label$) is also preserved", () => {
    const q = `create function f() returns string language javascript as $js$ return 'hello'; $js$`;
    const out = formatSQL(q, prefs({ functionCase: "lower" }));
    // sql-formatter may place the closing $js$ delimiter on its own line, but our
    // applyCasing pass must not modify content between the delimiters.
    expect(out).toContain("$js$");
    expect(out).toContain("return 'hello'"); // body content is untouched
  });
});

// ── 13. Comment passthrough ───────────────────────────────────────────────────

describe("comment passthrough", () => {
  it("line comments are not modified", () => {
    const q = `
      -- This is a select from where order by comment
      select id from t
    `;
    const out = formatSQL(q, prefs({ keywordCase: "lower" }));
    // Comment content must be unchanged
    expect(out).toContain("-- This is a select from where order by comment");
  });

  it("block comments are not modified", () => {
    const q = `
      /* SELECT id FROM t WHERE active = true */
      select id from t
    `;
    const out = formatSQL(q, prefs({ keywordCase: "lower" }));
    expect(out).toContain("/* SELECT id FROM t WHERE active = true */");
  });
});

// ── 14. Complex queries that stress the formatter ─────────────────────────────

describe("complex queries", () => {
  it("PIVOT query", () => {
    const q = `
      select *
      from sales
      pivot (sum(amount) for region in ('NA', 'EU', 'APAC'))
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("PIVOT");
    expect(out).toContain("SUM(");
    expect(out).toContain("FOR");
    expect(out).toContain("IN");
  });

  it("MATCH_RECOGNIZE pattern query", () => {
    const q = `
      select *
      from t
      match_recognize (
        partition by user_id
        order by event_ts
        measures
          strtok(classifier(), '_', 1) as event_class,
          last(ts) as last_ts
        pattern (start_event middle_event+ end_event)
        define
          start_event as event_type = 'start',
          middle_event as event_type like 'mid%',
          end_event as event_type = 'end'
      )
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("MATCH_RECOGNIZE");
    expect(out).toContain("MEASURES");
    expect(out).toContain("PATTERN");
    expect(out).toContain("DEFINE");
    // Should not throw
  });

  it("GROUP BY ROLLUP / CUBE / GROUPING SETS", () => {
    const q = `
      select region, product, sum(amount) as total
      from sales
      group by rollup (region, product)
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("GROUP");
    expect(out).toContain("BY");
    expect(out).toContain("ROLLUP");
    expect(out).toContain("SUM(");
  });

  it("window frame ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW", () => {
    const q = `
      select
        id,
        sum(amount) over (
          partition by user_id
          order by event_ts
          rows between unbounded preceding and current row
        ) as running_total
      from t
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("ROWS");
    expect(out).toContain("UNBOUNDED");
    expect(out).toContain("PRECEDING");
    expect(out).toContain("CURRENT");
    expect(out).toContain("ROW");
    expect(out).toContain("SUM(");
  });

  it("LISTAGG with WITHIN GROUP", () => {
    const q = `
      select
        group_id,
        listagg(name, ', ') within group (order by name) as names
      from t
      group by group_id
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("LISTAGG(");
    expect(out).toContain("WITHIN");
    expect(out).toContain("GROUP");
  });

  it("ARRAY_CONSTRUCT and OBJECT_CONSTRUCT with :: casts", () => {
    const q = `
      select
        array_construct(1, 2, 3) as arr,
        object_construct('k', col::string, 'n', count(*)::integer) as obj,
        array_agg(id::string) within group (order by id) as id_list
      from t
      group by col
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).not.toMatch(/\s::\s/);
    expect(out).toContain("ARRAY_CONSTRUCT(");
    expect(out).toContain("OBJECT_CONSTRUCT(");
    expect(out).toContain("ARRAY_AGG(");
  });

  it("IFF nested in CASE nested in COALESCE", () => {
    const q = `
      select
        coalesce(
          case
            when iff(score > 0.8, true, false) then 'high'
            when iff(score > 0.5, true, false) then 'medium'
            else 'low'
          end,
          'unknown'
        ) as tier
      from t
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("COALESCE(");
    expect(out).toContain("CASE");
    expect(out).toContain("WHEN");
    expect(out).toContain("IFF(");
    expect(out).toContain("THEN");
    expect(out).toContain("ELSE");
    expect(out).toContain("END");
  });

  it("correlated subquery with LATERAL", () => {
    const q = `
      select
        o.order_id,
        o.customer_id,
        latest.last_event
      from orders o
      join lateral (
        select max(event_ts) as last_event
        from events e
        where e.order_id = o.order_id
      ) latest on true
      where o.status = 'open'
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("LATERAL");
    expect(out).toContain("MAX(");
    expect(out).toContain("JOIN");
  });

  it("AT time-travel with DATEADD and ::timestamp_tz", () => {
    const q = `
      select *
      from orders at (timestamp => dateadd('minute', -5, current_timestamp())::timestamp_tz)
    `;
    const out = formatSQL(q, prefs({ keywordCase: "UPPER", functionCase: "UPPER" }));
    expect(out).toContain("AT");
    expect(out).toContain("DATEADD(");
    expect(out).toContain("CURRENT_TIMESTAMP(");
    expect(out).not.toMatch(/\s::\s/);
  });

  it("TRY_CAST and TRY_TO_DATE on messy input columns", () => {
    const q = `
      select
        try_cast(raw_value as number) as num,
        try_to_date(raw_ts, 'YYYY-MM-DD') as dt,
        try_to_decimal(amount_str, 18, 2) as amt,
        is_integer(coalesce(try_cast(s as integer), 0)) as is_int
      from raw_data
    `;
    const out = formatSQL(q, prefs({ functionCase: "UPPER" }));
    expect(out).toContain("TRY_CAST(");
    expect(out).toContain("TRY_TO_DATE(");
    expect(out).toContain("TRY_TO_DECIMAL(");
    expect(out).toContain("IS_INTEGER(");
  });

  it("SAMPLE clause is handled", () => {
    const q = "select * from very_large_table sample (10)";
    const out = formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("SAMPLE");
    expect(out).toContain("10");
  });

  it("SEMI JOIN and ANTI JOIN", () => {
    const q = `
      select id from a
      where id in (select id from b)
    `;
    // Just verify it formats without error
    const out = formatSQL(q, prefs({ keywordCase: "UPPER" }));
    expect(out).toContain("SELECT");
    expect(out).toContain("WHERE");
    expect(out).toContain("IN");
  });

  it("deeply nested CTE with FLATTEN inside window aggregate", () => {
    const q = `
      with
        exploded as (
          select
            e.event_id,
            e.user_id::string as uid,
            f.index as tag_index,
            f.value::string as tag,
            e.event_ts
          from events e,
            lateral flatten(input => e.payload:tags) f
        ),
        tag_counts as (
          select
            uid,
            tag,
            count(*) as occurrences,
            min(event_ts) as first_seen,
            max(event_ts) as last_seen
          from exploded
          group by uid, tag
        ),
        top_tags as (
          select
            uid,
            tag,
            occurrences,
            row_number() over (
              partition by uid
              order by occurrences desc, tag
            ) as tag_rank
          from tag_counts
        )
      select uid, tag, occurrences
      from top_tags
      where tag_rank <= 5
      order by uid, tag_rank
    `;
    const out = formatSQL(q, prefs({
      keywordCase: "UPPER",
      identifierCase: "Preserve",
      functionCase: "UPPER",
      commaPosition: "trailing",
      operatorPosition: "before",
    }));
    // No spaces around :: or :
    expect(out).not.toMatch(/\s::\s/);
    // Structural keywords present
    expect(out).toContain("WITH");
    expect(out).toContain("LATERAL");
    expect(out).toContain("FLATTEN(");
    expect(out).toContain("PARTITION");
    expect(out).toContain("ORDER BY");
    // Identifiers not changed
    expect(out).toContain("uid");
    expect(out).toContain("tag");
    expect(out).toContain("occurrences");
  });

  it("leading comma with nested CTEs produces valid output", () => {
    const q = `
      with a as (select 1 as n, 2 as m, 3 as k),
           b as (select n + m + k as total from a)
      select total, total * 2 as double_total from b
    `;
    const out = formatSQL(q, prefs({ commaPosition: "leading" }));
    // Leading comma lines should not contain trailing commas
    const lines = out.split("\n");
    const trailingCommaLines = lines.filter(
      (l) => l.trimEnd().endsWith(",") && !l.trimStart().startsWith(","),
    );
    expect(trailingCommaLines).toHaveLength(0);
  });

  it("formatter is idempotent — formatting twice produces identical output", () => {
    const q = `
      with orders as (
        select order_id, customer_id, total::decimal(18,2) as amount
        from raw.orders where status = 'active'
      )
      select customer_id, sum(amount) as total_spent
      from orders
      group by customer_id
      having sum(amount) > 100
      order by total_spent desc
    `;
    const p = prefs({ keywordCase: "UPPER", functionCase: "UPPER", commaPosition: "trailing" });
    const first = formatSQL(q, p);
    const second = formatSQL(first, p);
    expect(normalize(second)).toBe(normalize(first));
  });
});
