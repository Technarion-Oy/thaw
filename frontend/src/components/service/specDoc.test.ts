// SPDX-License-Identifier: GPL-3.0-or-later
import { describe, expect, it } from "vitest";
import { dumpSpec, parseSpec, setIn, specProblems, validateGatewaySpec, validateServiceSpec } from "./specDoc";

describe("spec form ⇄ YAML", () => {
  it("round-trips, keeping keys the form doesn't know", () => {
    const text = "spec:\n  containers:\n    - name: main\n      image: /db/sc/repo/img:1\n      futureKey: 7\n";
    const r = parseSpec(text);
    expect("doc" in r && dumpSpec(r.doc)).toBe(text);
  });
  it("refuses templates and non-mappings, accepts empty", () => {
    expect(parseSpec("image: {{ img }}")).toHaveProperty("error");
    expect(parseSpec("- a")).toHaveProperty("error");
    expect(parseSpec("")).toEqual({ doc: {} });
  });
  it("setIn deletes empty values and prunes empty mappings, never array items", () => {
    const d = setIn({ spec: { containers: [{ resources: { requests: { cpu: 1 } } }] } },
      ["spec", "containers", 0, "resources", "requests", "cpu"], "");
    expect(d).toEqual({ spec: { containers: [{}] } });
    expect(setIn({}, ["a", "nvidia.com/gpu"], 1)).toEqual({ a: { "nvidia.com/gpu": 1 } });
    expect(setIn({}, ["l", 0, "x"], 1)).toEqual({ l: [{ x: 1 }] });
  });
});

describe("service spec checks", () => {
  const ok = { spec: { containers: [{ name: "main", image: "/a/b/c/d" }] } };
  it("passes a minimal spec", () => expect(validateServiceSpec(ok)).toEqual([]));
  it("catches cross-field mistakes", () => {
    const p = validateServiceSpec({
      spec: {
        containers: [{
          name: "Main", image: "x",
          resources: { requests: { "nvidia.com/gpu": 1 } },
          volumeMounts: [{ name: "nope", mountPath: "/d" }],
          mountSessionToken: false, sessionTokenConfig: { path: "/t" },
        }],
        endpoints: [{ name: "api", portRange: "1-2", public: true }, { name: "web", port: 1, public: true, protocol: "TCP", corsSettings: { "Access-Control-Allow-Origin": ["*"] } }],
        volumes: [{ name: "m", source: "memory" }],
      },
      serviceRoles: [{ name: "r", endpoints: ["missing"] }],
    }).join("\n");
    for (const s of ["lowercase", "GPU limit", '"nope"', "sessionTokenConfig", "TCP and not public", "must use HTTP", '"*"', "size is required", '"missing"'])
      expect(p).toContain(s);
  });
  it("rejects service-only keys in a job", () => {
    const p = validateServiceSpec({ ...ok, spec: { ...ok.spec, endpoints: [{ name: "e", port: 1 }] } }, true);
    expect(p).toContain("Jobs don't support endpoints.");
  });
});

describe("gateway spec checks", () => {
  const t = (w: number) => ({ type: "endpoint", value: "d.s.svc!ep", weight: w });
  it("requires weights to sum to 100", () => {
    expect(validateGatewaySpec({ spec: { type: "traffic_split", targets: [t(50), t(50)] } })).toEqual([]);
    expect(validateGatewaySpec({ spec: { type: "traffic_split", targets: [t(50), t(40)] } })[0]).toContain("sum to 90");
  });
  it("checks shadow traffic shape", () => {
    expect(validateGatewaySpec({ spec: { type: "shadow_traffic", primary: [{ type: "endpoint", value: "d.s.svc!ep" }], shadow: [t(10)] } })).toEqual([]);
    expect(validateGatewaySpec({ spec: { type: "shadow_traffic", primary: [], shadow: [] } })).toHaveLength(2);
  });
});

it("specProblems skips unparseable text", () => {
  expect(specProblems("service", "spec: [")).toEqual([]);
  expect(specProblems("inference", "output: {}")).toEqual(["output.stage_location is required."]);
});
