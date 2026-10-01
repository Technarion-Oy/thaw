// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from "vitest";
import { blockVolumes } from "./blockVolumes";

describe("blockVolumes", () => {
  it("keeps only block volumes and collects their instances", () => {
    const got = blockVolumes([
      { volume_name: "data", volume_type: "BLOCK", instance_id: "0" },
      { volume_name: "data", volume_type: "block", instance_id: "1" },
      { volume_name: "data", volume_type: "block", instance_id: "1" },
      { volume_name: "logs", volume_type: "local", instance_id: "0" },
      { volume_name: "cache", volume_type: "memory", instance_id: "0" },
      { volume_name: "models", volume_type: "@stage", instance_id: "0" },
      { volume_name: "untyped", instance_id: "0" },
    ]);
    expect([...got.entries()]).toEqual([["data", ["0", "1"]]]);
  });
});
