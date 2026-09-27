// SPDX-License-Identifier: GPL-3.0-or-later
import { describe, expect, it } from "vitest";
import { familyLabel, familyListText, rowsOf } from "./computePools";

describe("computePools helpers", () => {
  it("keys rows by lower-cased column", () => {
    expect(rowsOf({ columns: ["NAME", "vcpu"], rows: [["GPU_NV_S", 6]] } as never))
      .toEqual([{ name: "GPU_NV_S", vcpu: "6" }]);
  });
  it("shows GPU count and memory for a GPU family, nothing for a CPU one", () => {
    expect(familyLabel({ name: "GPU_NV_S", vcpu: "6", memory_gib: "27", gpu: "A10G", gpu_count: "1", gpu_memory_gib: "24", current_node_usage: "2" }))
      .toBe("GPU_NV_S · 6 vCPU · 27 GiB · 1× A10G (24 GiB) · 2 in use");
    expect(familyLabel({ name: "CPU_X64_XS", vcpu: "1", memory_gib: "6", gpu: "NONE", gpu_count: "0" }))
      .toBe("CPU_X64_XS · 1 vCPU · 6 GiB");
  });
  it("normalizes backup family lists", () => {
    expect(familyListText('["A","B"]')).toBe("A, B");
    expect(familyListText("")).toBe("");
  });
});
