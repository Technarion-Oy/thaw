// SPDX-License-Identifier: GPL-3.0-or-later

package secrets

import "strings"

// Stable keys for Thaw's fixed secrets. These strings are the identity of a
// secret in the OS store — never change them without a migration.
//
// #nosec G101 // Justification: these are OS-store lookup keys/labels, not credential values.
const (
	KeyAIAPIKey           = "ai/api-key"                 // AIConfig.APIKey
	KeyGitHubClientSecret = "oauth/github-client-secret" // OAuthConfig.GithubClientSecret
	KeyGitLabClientSecret = "oauth/gitlab-client-secret" // OAuthConfig.GitlabClientSecret
	KeyPipProxyPassword   = "pip/proxy-password"         // PipRegistryConfig.ProxyPassword
)

// Prefixes for the dynamic, collection-valued secrets.
//
// #nosec G101 // Justification: these are OS-store lookup key prefixes, not credential values.
const (
	pipCredentialPrefix = "pip/credential/" // per pip registry URL
	mcpTokenPrefix      = "mcp/token/"      // per MCP session label
)

// PipCredentialKey returns the store key for a pip registry credential password
// (PipRegistryCredential.Password), keyed by the registry URL it applies to.
func PipCredentialKey(registry string) string { return pipCredentialPrefix + registry }

// IsPipCredentialKey reports whether key is a pip registry credential password.
func IsPipCredentialKey(key string) bool { return strings.HasPrefix(key, pipCredentialPrefix) }

// MCPTokenKey returns the store key for an MCP session auth token
// (MCPSessionCredential.Token), keyed by the session label.
func MCPTokenKey(label string) string { return mcpTokenPrefix + label }
