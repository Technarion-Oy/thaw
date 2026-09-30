// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

export type LoginMethod = "cli" | "token" | "pat";

export interface RegistrySnippets {
  host: string;
  /** Login commands by method. Contain only the host and user — never a secret. */
  login: Record<LoginMethod, string>;
  defaultMethod: LoginMethod;
  push: string;
}

/**
 * Registry login + push/pull commands for a repository URL. The host comes from
 * the server-assigned URL (no account-name rebuilding). Shell-agnostic one-liners.
 */
export function registrySnippets(
  repositoryUrl: string,
  params: { user?: string; authenticator?: string } | null,
): RegistrySnippets {
  const host = repositoryUrl.split("/")[0];
  const user = params?.user?.trim() || "<user>";
  const img = `${repositoryUrl}/<image>:<tag>`;
  return {
    host,
    login: {
      cli: "snow spcs image-registry login",
      token: `snow spcs image-registry token --format=JSON | docker login ${host} -u 0sessiontoken --password-stdin`,
      pat: `docker login ${host} -u ${user}`,
    },
    defaultMethod: params?.authenticator === "programmatic_access_token" ? "pat" : "cli",
    push: `docker tag <image>:<tag> ${img}\ndocker push ${img}\ndocker pull ${img}`,
  };
}
