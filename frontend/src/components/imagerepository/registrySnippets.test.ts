// SPDX-License-Identifier: GPL-3.0-or-later
import { describe, expect, it } from "vitest";
import { registrySnippets } from "./registrySnippets";

const url = "org-acct.registry.snowflakecomputing.com/db/sch/repo";

describe("registrySnippets", () => {
  it("derives the host from the repository URL and embeds the URL in push commands", () => {
    const s = registrySnippets(url, { user: "ALICE", authenticator: "externalbrowser" });
    expect(s.host).toBe("org-acct.registry.snowflakecomputing.com");
    expect(s.login.pat).toBe("docker login org-acct.registry.snowflakecomputing.com -u ALICE");
    expect(s.login.token).toContain("-u 0sessiontoken --password-stdin");
    expect(s.push).toContain(`docker push ${url}/<image>:<tag>`);
    expect(s.defaultMethod).toBe("cli");
  });
  it("defaults to Docker with PAT and a <user> placeholder for PAT connections", () => {
    const s = registrySnippets(url, { user: "", authenticator: "programmatic_access_token" });
    expect(s.defaultMethod).toBe("pat");
    expect(s.login.pat).toMatch(/-u <user>$/);
  });
});
