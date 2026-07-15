// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from "vitest";
import {
  parseColumnType,
  validateNumeric,
  validateJson,
  validateUuid,
  validateHex,
  validateVector,
  validateValue,
  snippetsFor,
  formatJson,
} from "./insertCellTypes";

describe("parseColumnType", () => {
  it("classifies numeric types and extracts precision/scale", () => {
    expect(parseColumnType("NUMBER(10,2)")).toMatchObject({ base: "NUMBER", family: "numeric", precision: 10, scale: 2 });
    expect(parseColumnType("INTEGER")).toMatchObject({ base: "INTEGER", family: "numeric" });
    expect(parseColumnType("FLOAT")).toMatchObject({ family: "numeric" });
  });

  it("classifies text types and extracts length", () => {
    expect(parseColumnType("VARCHAR(50)")).toMatchObject({ base: "VARCHAR", family: "text", length: 50 });
    expect(parseColumnType("STRING")).toMatchObject({ family: "text" });
  });

  it("classifies boolean, date, time and timestamp families", () => {
    expect(parseColumnType("BOOLEAN").family).toBe("boolean");
    expect(parseColumnType("DATE").family).toBe("date");
    expect(parseColumnType("TIME(9)").family).toBe("time");
    expect(parseColumnType("TIMESTAMP_NTZ(9)").family).toBe("timestamp");
    expect(parseColumnType("TIMESTAMP_TZ(9)").family).toBe("timestamptz");
    expect(parseColumnType("TIMESTAMP_LTZ(9)").family).toBe("timestamptz");
  });

  it("classifies semi-structured, binary, geo, uuid families", () => {
    expect(parseColumnType("VARIANT").family).toBe("json");
    expect(parseColumnType("OBJECT").family).toBe("json");
    expect(parseColumnType("ARRAY").family).toBe("json");
    expect(parseColumnType("BINARY(100)")).toMatchObject({ family: "binary", length: 100 });
    expect(parseColumnType("GEOGRAPHY").family).toBe("geo");
    expect(parseColumnType("GEOMETRY").family).toBe("geo");
    expect(parseColumnType("UUID").family).toBe("uuid");
  });

  it("parses vector element type and dimension", () => {
    expect(parseColumnType("VECTOR(FLOAT, 4)")).toMatchObject({
      base: "VECTOR",
      family: "vector",
      elementType: "FLOAT",
      dimension: 4,
    });
  });

  it("falls back to 'other' for unmodelled types", () => {
    expect(parseColumnType("FILE").family).toBe("other");
    expect(parseColumnType("MY_UDT").family).toBe("other");
  });
});

describe("validateNumeric", () => {
  const num = parseColumnType("NUMBER(10,2)");
  it("accepts empty and valid numbers", () => {
    expect(validateNumeric("", num)).toBeNull();
    expect(validateNumeric("42", num)).toBeNull();
    expect(validateNumeric("-3.14", num)).toBeNull();
    expect(validateNumeric("1e5", parseColumnType("FLOAT"))).toBeNull();
  });
  it("rejects non-numbers", () => {
    expect(validateNumeric("abc", num)).not.toBeNull();
  });
  it("enforces declared scale", () => {
    expect(validateNumeric("1.234", num)).not.toBeNull();
    expect(validateNumeric("1.23", num)).toBeNull();
  });
});

describe("validateJson", () => {
  it("accepts valid JSON and empty", () => {
    expect(validateJson("")).toBeNull();
    expect(validateJson('{"a": 1}')).toBeNull();
    expect(validateJson("[1, 2, 3]")).toBeNull();
  });
  it("rejects malformed JSON", () => {
    expect(validateJson("{a: 1}")).not.toBeNull();
  });
});

describe("validateUuid / validateHex", () => {
  it("validates uuid form", () => {
    expect(validateUuid("123e4567-e89b-12d3-a456-426614174000")).toBeNull();
    expect(validateUuid("not-a-uuid")).not.toBeNull();
  });
  it("validates hex form", () => {
    expect(validateHex("DEADBEEF")).toBeNull();
    expect(validateHex("XYZ")).not.toBeNull();
    expect(validateHex("ABC")).not.toBeNull(); // odd length
  });
});

describe("validateVector", () => {
  const vec = parseColumnType("VECTOR(FLOAT, 4)");
  it("accepts a correctly-sized numeric list", () => {
    expect(validateVector("[1.0, 2.0, 3.0, 4.0]", vec)).toBeNull();
  });
  it("rejects the wrong dimension", () => {
    expect(validateVector("[1, 2, 3]", vec)).not.toBeNull();
  });
  it("rejects non-numeric elements", () => {
    expect(validateVector("[1, x, 3, 4]", vec)).not.toBeNull();
  });
  it("rejects a non-bracketed value", () => {
    expect(validateVector("1, 2, 3, 4", vec)).not.toBeNull();
  });
});

describe("snippetsFor", () => {
  it("returns JSON scaffolds for semi-structured types and produces valid JSON where applicable", () => {
    const obj = snippetsFor(parseColumnType("OBJECT"));
    const arr = snippetsFor(parseColumnType("ARRAY"));
    const variant = snippetsFor(parseColumnType("VARIANT"));
    expect(obj.length).toBeGreaterThan(0);
    expect(arr.length).toBeGreaterThan(0);
    expect(variant.length).toBeGreaterThan(0);
    // The object/array scaffolds are all parseable JSON.
    for (const s of [...obj, ...arr]) expect(() => JSON.parse(s.value)).not.toThrow();
  });

  it("returns WKT/GeoJSON templates for geospatial types", () => {
    const geo = snippetsFor(parseColumnType("GEOGRAPHY"));
    expect(geo.some((s) => s.value.startsWith("POINT"))).toBe(true);
    expect(geo.some((s) => s.value.includes('"type": "Point"'))).toBe(true);
  });

  it("returns nothing for types without helpers", () => {
    expect(snippetsFor(parseColumnType("NUMBER"))).toHaveLength(0);
    expect(snippetsFor(parseColumnType("VARCHAR(50)"))).toHaveLength(0);
  });
});

describe("formatJson", () => {
  it("pretty-prints valid JSON with two-space indentation", () => {
    expect(formatJson('{"a":1}')).toBe('{\n  "a": 1\n}');
  });
  it("returns null for empty or invalid JSON", () => {
    expect(formatJson("")).toBeNull();
    expect(formatJson("{not json")).toBeNull();
  });
});

describe("validateValue dispatch", () => {
  it("routes to the right validator by family", () => {
    expect(validateValue("abc", parseColumnType("NUMBER"))).not.toBeNull();
    expect(validateValue("{bad", parseColumnType("VARIANT"))).not.toBeNull();
    expect(validateValue("hello", parseColumnType("VARCHAR(50)"))).toBeNull();
  });
});
