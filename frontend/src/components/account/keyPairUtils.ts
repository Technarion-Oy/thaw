// SPDX-License-Identifier: GPL-3.0-or-later

// Pure helpers for the key-pair auth UI (KeyPairAuthModal), split out so they
// can be unit-tested without loading the React/antd component.

/**
 * stripPem reduces a pasted public key to the bare base64 payload Snowflake
 * expects: it drops the -----BEGIN/-----END----- header/footer lines and all
 * whitespace, so an admin can paste either a full PEM file received from a user
 * or an already-stripped key and get the same base64 string either way.
 *
 * It only normalises shape — it does not validate the base64 charset. The
 * backend builder (`internal/users.BuildAlterUserPropertySQL`, property
 * `rsaPublicKey`/`rsaPublicKey2`) is the authority that rejects non-base64
 * input before it reaches SQL.
 */
export function stripPem(s: string): string {
  return s
    .split(/\r?\n/)
    .filter((l) => !l.trim().startsWith("-----"))
    .join("")
    .replace(/\s+/g, "");
}
