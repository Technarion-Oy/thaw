import { describe, it, expect, beforeEach } from "vitest";
import { useObjectStore } from "./objectStore";

// removeObject backs the hover path's stale-entry self-heal (#918): when
// GET_DDL reports an object gone, it is evicted so the cmd/ctrl link stops
// coming back instead of re-firing a doomed fetch on every re-hover.
describe("objectStore.removeObject", () => {
  beforeEach(() => {
    useObjectStore.getState().addObjects("DB", "S", [
      { name: "ORDERS", kind: "TABLE" },
      { name: "ORDERS", kind: "STREAM" },
      { name: "CUSTOMERS", kind: "TABLE" },
    ]);
    useObjectStore.getState().addObjects("DB", "OTHER", [{ name: "ORDERS", kind: "TABLE" }]);
  });

  const names = () => useObjectStore.getState().objects.map((o) => `${o.db}.${o.schema}.${o.name}:${o.kind}`);

  it("removes only the named object of that kind", () => {
    useObjectStore.getState().removeObject("DB", "S", "ORDERS", "TABLE");
    // The same-named STREAM in the same schema must survive.
    expect(names()).toEqual([
      "DB.S.ORDERS:STREAM",
      "DB.S.CUSTOMERS:TABLE",
      "DB.OTHER.ORDERS:TABLE",
    ]);
  });

  it("leaves the store untouched when nothing matches", () => {
    const before = names();
    useObjectStore.getState().removeObject("DB", "S", "ORDERS", "VIEW");
    useObjectStore.getState().removeObject("DB", "NOPE", "ORDERS", "TABLE");
    expect(names()).toEqual(before);
  });
});
