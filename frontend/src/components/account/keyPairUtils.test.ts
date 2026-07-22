// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from "vitest";
import { stripPem } from "./keyPairUtils";

describe("stripPem", () => {
  it("strips a full PEM to its base64 payload", () => {
    const pem = [
      "-----BEGIN PUBLIC KEY-----",
      "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A",
      "MIIBCgKCAQEArandombase64content",
      "-----END PUBLIC KEY-----",
    ].join("\n");
    expect(stripPem(pem)).toBe(
      "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArandombase64content",
    );
  });

  it("leaves an already-stripped single-line key unchanged", () => {
    expect(stripPem("MIIBIjANBgkq")).toBe("MIIBIjANBgkq");
  });

  it("removes interior whitespace and blank lines", () => {
    expect(stripPem("MIIB IjAN\n\n  Bgkq \t hkiG\n")).toBe("MIIBIjANBgkqhkiG");
  });

  it("handles CRLF line endings", () => {
    const pem = "-----BEGIN PUBLIC KEY-----\r\nMIIBIjAN\r\n-----END PUBLIC KEY-----\r\n";
    expect(stripPem(pem)).toBe("MIIBIjAN");
  });

  it("drops header/footer lines even with surrounding whitespace", () => {
    const pem = "  -----BEGIN PUBLIC KEY-----  \nMIIBIjAN\n  -----END PUBLIC KEY-----";
    expect(stripPem(pem)).toBe("MIIBIjAN");
  });

  it("returns an empty string for empty or whitespace-only input", () => {
    expect(stripPem("")).toBe("");
    expect(stripPem("   \n\t  ")).toBe("");
  });

  it("preserves trailing base64 padding", () => {
    expect(stripPem("MIIBIjAN==\n")).toBe("MIIBIjAN==");
  });
});
