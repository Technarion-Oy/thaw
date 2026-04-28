// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties   holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useMemo } from "react";
import { Modal, Form, Input, Select, Switch, Button, Alert, InputNumber, Radio, Checkbox, Divider, Typography } from "antd";
import { ExecuteQuery, GetCurrentRegion, GetQuotedIdentifiersIgnoreCase } from "../../../wailsjs/go/main/App";
import ObjectNameCaseControl, { identToken } from "../shared/ObjectNameCaseControl";

const { TextArea } = Input;

interface Props {
  kind: string;
  onClose: () => void;
  onSuccess: () => void;
}

// ── SQL builder ───────────────────────────────────────────────────────────────

function sq(s: string): string {
  return "'" + s.replace(/'/g, "''") + "'";
}
function ident(s: string): string {
  return '"' + s.replace(/"/g, '""') + '"';
}

function buildStorageSql(vals: Record<string, unknown>): string {
  const name = identToken(String(vals.name ?? ""), Boolean(vals.caseSensitive));
  const provider = String(vals.provider ?? "S3");
  const enabled = vals.enabled !== false ? "TRUE" : "FALSE";
  const lines: string[] = [
    `CREATE STORAGE INTEGRATION ${name}`,
    `  TYPE = EXTERNAL_STAGE`,
    `  STORAGE_PROVIDER = '${provider}'`,
    `  ENABLED = ${enabled}`,
  ];
  if (provider === "S3" || provider === "S3GOV") {
    if (vals.awsRoleArn) lines.push(`  STORAGE_AWS_ROLE_ARN = ${sq(String(vals.awsRoleArn))}`);
    if (vals.allowedLocations) lines.push(`  STORAGE_ALLOWED_LOCATIONS = (${splitLocations(String(vals.allowedLocations))})`);
    if (vals.blockedLocations) lines.push(`  STORAGE_BLOCKED_LOCATIONS = (${splitLocations(String(vals.blockedLocations))})`);
    if (vals.awsExternalId) lines.push(`  STORAGE_AWS_EXTERNAL_ID = ${sq(String(vals.awsExternalId))}`);
    if (vals.usePrivatelink) lines.push(`  USE_PRIVATELINK_ENDPOINT = TRUE`);
  } else if (provider === "GCS") {
    if (vals.allowedLocations) lines.push(`  STORAGE_ALLOWED_LOCATIONS = (${splitLocations(String(vals.allowedLocations))})`);
    if (vals.blockedLocations) lines.push(`  STORAGE_BLOCKED_LOCATIONS = (${splitLocations(String(vals.blockedLocations))})`);
  } else if (provider === "AZURE") {
    if (vals.azureTenantId) lines.push(`  AZURE_TENANT_ID = ${sq(String(vals.azureTenantId))}`);
    if (vals.allowedLocations) lines.push(`  STORAGE_ALLOWED_LOCATIONS = (${splitLocations(String(vals.allowedLocations))})`);
    if (vals.blockedLocations) lines.push(`  STORAGE_BLOCKED_LOCATIONS = (${splitLocations(String(vals.blockedLocations))})`);
    if (vals.usePrivatelink) lines.push(`  USE_PRIVATELINK_ENDPOINT = TRUE`);
  }
  if (vals.comment) lines.push(`  COMMENT = ${sq(String(vals.comment))}`);
  return lines.join("\n");
}

function buildApiSql(vals: Record<string, unknown>): string {
  const provider = String(vals.provider ?? "aws_api_gateway");
  // Delegate git_https_api to its own builder with full mode support
  if (provider === "git_https_api") {
    return buildGitHttpsApiSql(vals);
  }
  const name = identToken(String(vals.name ?? ""), Boolean(vals.caseSensitive));
  const enabled = vals.enabled !== false ? "TRUE" : "FALSE";
  const lines: string[] = [
    `CREATE API INTEGRATION ${name}`,
    `  API_PROVIDER = ${provider}`,
    `  ENABLED = ${enabled}`,
  ];
  if (vals.allowedPrefixes) lines.push(`  API_ALLOWED_PREFIXES = (${splitLocations(String(vals.allowedPrefixes))})`);
  if (vals.blockedPrefixes) lines.push(`  API_BLOCKED_PREFIXES = (${splitLocations(String(vals.blockedPrefixes))})`);
  if (provider === "aws_api_gateway" || provider === "aws_private_api_gateway") {
    if (vals.awsRoleArn) lines.push(`  API_AWS_ROLE_ARN = ${sq(String(vals.awsRoleArn))}`);
    if (vals.apiKey) lines.push(`  API_KEY = ${sq(String(vals.apiKey))}`);
  } else if (provider === "azure_api_management") {
    if (vals.azureTenantId) lines.push(`  AZURE_TENANT_ID = ${sq(String(vals.azureTenantId))}`);
    if (vals.azureAdAppId) lines.push(`  AZURE_AD_APPLICATION_ID = ${sq(String(vals.azureAdAppId))}`);
    if (vals.apiKey) lines.push(`  API_KEY = ${sq(String(vals.apiKey))}`);
  } else if (provider === "google_api_gateway") {
    if (vals.googleAudience) lines.push(`  GOOGLE_AUDIENCE = ${sq(String(vals.googleAudience))}`);
  }
  if (vals.comment) lines.push(`  COMMENT = ${sq(String(vals.comment))}`);
  return lines.join("\n");
}

function buildGitHttpsApiSql(vals: Record<string, unknown>): string {
  const name = identToken(String(vals.name ?? ""), Boolean(vals.caseSensitive));
  const enabled = vals.enabled !== false ? "TRUE" : "FALSE";

  let createClause = "CREATE";
  if (vals.orReplace) createClause += " OR REPLACE";
  createClause += " API INTEGRATION";
  if (!vals.orReplace && vals.ifNotExists) createClause += " IF NOT EXISTS";

  const lines: string[] = [
    `${createClause} ${name}`,
    `  API_PROVIDER = git_https_api`,
  ];

  const mode = String(vals.gitAuthMode ?? "TOKEN");

  // For GitHub App, fall back to the base GitHub URL if the user left the field empty.
  const rawPrefixes = vals.allowedPrefixes ? String(vals.allowedPrefixes).trim() : "";
  const effectivePrefixes = mode === "GITHUB_APP" && !rawPrefixes ? "https://github.com/" : rawPrefixes;
  if (effectivePrefixes) {
    lines.push(`  API_ALLOWED_PREFIXES = (${splitLocations(effectivePrefixes)})`);
  }
  if (vals.blockedPrefixes) {
    lines.push(`  API_BLOCKED_PREFIXES = (${splitLocations(String(vals.blockedPrefixes))})`);
  }

  // ALLOWED_AUTHENTICATION_SECRETS applies to TOKEN and PRIVATELINK modes
  if (mode === "TOKEN" || mode === "PRIVATELINK") {
    const rawSecrets = vals.allowedAuthSecrets;
    if (rawSecrets !== undefined && rawSecrets !== null) {
      const secretArr = Array.isArray(rawSecrets)
        ? (rawSecrets as string[]).map(String)
        : [String(rawSecrets)];
      const filtered = secretArr.map((s) => s.trim()).filter(Boolean);
      if (filtered.length === 1 && (filtered[0].toUpperCase() === "ALL" || filtered[0].toUpperCase() === "NONE")) {
        lines.push(`  ALLOWED_AUTHENTICATION_SECRETS = ${filtered[0].toUpperCase()}`);
      } else if (filtered.length > 0) {
        lines.push(`  ALLOWED_AUTHENTICATION_SECRETS = (${filtered.join(", ")})`);
      }
    }
  }

  if (mode === "GITHUB_APP") {
    lines.push(`  API_USER_AUTHENTICATION = (`);
    lines.push(`    TYPE = SNOWFLAKE_GITHUB_APP`);
    lines.push(`  )`);
  } else if (mode === "OAUTH2") {
    const oauthParams: string[] = ["    TYPE = OAUTH2"];
    if (vals.oauthClientId) oauthParams.push(`    OAUTH_CLIENT_ID = ${sq(String(vals.oauthClientId))}`);
    if (vals.oauthClientSecret) oauthParams.push(`    OAUTH_CLIENT_SECRET = ${sq(String(vals.oauthClientSecret))}`);
    if (vals.oauthTokenEndpoint) oauthParams.push(`    OAUTH_TOKEN_ENDPOINT = ${sq(String(vals.oauthTokenEndpoint))}`);
    if (vals.oauthScopes) {
      const scopes = String(vals.oauthScopes)
        .split(",")
        .map((s) => sq(s.trim()))
        .filter((s) => s !== "''");
      if (scopes.length > 0) oauthParams.push(`    OAUTH_ALLOWED_SCOPES = (${scopes.join(", ")})`);
    }
    lines.push(`  API_USER_AUTHENTICATION = (`);
    oauthParams.forEach((l) => lines.push(l));
    lines.push(`  )`);
  } else if (mode === "PRIVATELINK") {
    lines.push(`  USE_PRIVATELINK_ENDPOINT = ${vals.usePrivateLink ? "TRUE" : "FALSE"}`);
    const rawCerts = vals.tlsCertificates;
    if (rawCerts !== undefined && rawCerts !== null) {
      const certArr = Array.isArray(rawCerts)
        ? (rawCerts as string[]).map(String)
        : [String(rawCerts)];
      const filtered = certArr.map((s) => s.trim()).filter(Boolean);
      if (filtered.length > 0) {
        lines.push(`  TLS_TRUSTED_CERTIFICATES = (${filtered.join(", ")})`);
      }
    }
  }

  lines.push(`  ENABLED = ${enabled}`);
  if (vals.comment) lines.push(`  COMMENT = ${sq(String(vals.comment))}`);
  return lines.join("\n");
}

function buildCatalogSql(vals: Record<string, unknown>): string {
  const name = identToken(String(vals.name ?? ""), Boolean(vals.caseSensitive));
  const source = String(vals.source ?? "GLUE");
  const enabled = vals.enabled !== false ? "TRUE" : "FALSE";
  const lines: string[] = [
    `CREATE CATALOG INTEGRATION ${name}`,
    `  CATALOG_SOURCE = ${source}`,
    `  ENABLED = ${enabled}`,
  ];
  if (source === "GLUE") {
    if (vals.glueAwsRoleArn) lines.push(`  GLUE_AWS_ROLE_ARN = ${sq(String(vals.glueAwsRoleArn))}`);
    if (vals.glueCatalogId) lines.push(`  GLUE_CATALOG_ID = ${sq(String(vals.glueCatalogId))}`);
    if (vals.glueRegion) lines.push(`  GLUE_REGION = ${sq(String(vals.glueRegion))}`);
    if (vals.catalogNamespace) lines.push(`  CATALOG_NAMESPACE = ${sq(String(vals.catalogNamespace))}`);
  } else if (source === "OBJECT_STORE") {
    if (vals.tableFormat) lines.push(`  TABLE_FORMAT = ${String(vals.tableFormat)}`);
  } else if (source === "POLARIS" || source === "ICEBERG_REST") {
    if (vals.catalogUri) lines.push(`  CATALOG_URI = ${sq(String(vals.catalogUri))}`);
    if (vals.catalogName) lines.push(`  CATALOG_NAME = ${sq(String(vals.catalogName))}`);
    if (vals.catalogNamespace) lines.push(`  CATALOG_NAMESPACE = ${sq(String(vals.catalogNamespace))}`);
    if (vals.catalogApiType) lines.push(`  CATALOG_API_TYPE = ${String(vals.catalogApiType)}`);
    if (vals.accessDelegationMode) lines.push(`  ACCESS_DELEGATION_MODE = ${String(vals.accessDelegationMode)}`);
    if (source === "POLARIS") {
      if (vals.oauthTokenUri) lines.push(`  OAUTH_TOKEN_URI = ${sq(String(vals.oauthTokenUri))}`);
      if (vals.oauthClientId) lines.push(`  OAUTH_CLIENT_ID = ${sq(String(vals.oauthClientId))}`);
      if (vals.oauthClientSecret) lines.push(`  OAUTH_CLIENT_SECRET = ${sq(String(vals.oauthClientSecret))}`);
      if (vals.oauthScopes) lines.push(`  OAUTH_ALLOWED_SCOPES = (${splitLocations(String(vals.oauthScopes))})`);
    } else {
      const authType = String(vals.icebergAuthType ?? "OAUTH");
      lines.push(`  AUTH_TYPE = ${authType}`);
      if (authType === "OAUTH") {
        if (vals.oauthTokenUri) lines.push(`  OAUTH_TOKEN_URI = ${sq(String(vals.oauthTokenUri))}`);
        if (vals.oauthClientId) lines.push(`  OAUTH_CLIENT_ID = ${sq(String(vals.oauthClientId))}`);
        if (vals.oauthClientSecret) lines.push(`  OAUTH_CLIENT_SECRET = ${sq(String(vals.oauthClientSecret))}`);
        if (vals.oauthScopes) lines.push(`  OAUTH_ALLOWED_SCOPES = (${splitLocations(String(vals.oauthScopes))})`);
      } else if (authType === "BEARER") {
        if (vals.bearerToken) lines.push(`  BEARER_TOKEN = ${sq(String(vals.bearerToken))}`);
      }
    }
    if (vals.prefix) lines.push(`  PREFIX = ${sq(String(vals.prefix))}`);
  } else if (source === "SAP_BDC") {
    if (vals.sapInvitationLink) lines.push(`  SAP_BDC_INVITATION_LINK = ${sq(String(vals.sapInvitationLink))}`);
    if (vals.accessDelegationMode) lines.push(`  ACCESS_DELEGATION_MODE = ${String(vals.accessDelegationMode)}`);
  }
  if (vals.refreshInterval) lines.push(`  REFRESH_INTERVAL_SECONDS = ${vals.refreshInterval}`);
  if (vals.comment) lines.push(`  COMMENT = ${sq(String(vals.comment))}`);
  return lines.join("\n");
}

function buildExternalAccessSql(vals: Record<string, unknown>): string {
  const name = identToken(String(vals.name ?? ""), Boolean(vals.caseSensitive));
  const enabled = vals.enabled !== false ? "TRUE" : "FALSE";
  const lines: string[] = [
    `CREATE EXTERNAL ACCESS INTEGRATION ${name}`,
  ];
  if (vals.allowedNetworkRules) {
    lines.push(`  ALLOWED_NETWORK_RULES = (${String(vals.allowedNetworkRules)})`);
  }
  if (vals.allowedApiAuthIntegrations) {
    lines.push(`  ALLOWED_API_AUTHENTICATION_INTEGRATIONS = (${String(vals.allowedApiAuthIntegrations)})`);
  }
  if (vals.allowedAuthSecrets) {
    const sec = String(vals.allowedAuthSecrets).trim().toLowerCase();
    if (sec === "all" || sec === "none") {
      lines.push(`  ALLOWED_AUTHENTICATION_SECRETS = ${sec.toUpperCase()}`);
    } else {
      lines.push(`  ALLOWED_AUTHENTICATION_SECRETS = (${String(vals.allowedAuthSecrets)})`);
    }
  }
  lines.push(`  ENABLED = ${enabled}`);
  if (vals.comment) lines.push(`  COMMENT = ${sq(String(vals.comment))}`);
  return lines.join("\n");
}

function buildNotificationSql(vals: Record<string, unknown>): string {
  const name = identToken(String(vals.name ?? ""), Boolean(vals.caseSensitive));
  const subtype = String(vals.subtype ?? "EMAIL");
  const lines: string[] = [`CREATE NOTIFICATION INTEGRATION ${name}`];

  if (subtype === "AZURE_STORAGE_QUEUE_INBOUND") {
    lines.push(`  TYPE = QUEUE`);
    lines.push(`  NOTIFICATION_PROVIDER = AZURE_STORAGE_QUEUE`);
    lines.push(`  DIRECTION = INBOUND`);
    if (vals.azureQueueUri) lines.push(`  AZURE_STORAGE_QUEUE_PRIMARY_URI = ${sq(String(vals.azureQueueUri))}`);
    if (vals.azureTenantId) lines.push(`  AZURE_TENANT_ID = ${sq(String(vals.azureTenantId))}`);
    if (vals.usePrivatelink) lines.push(`  USE_PRIVATELINK_ENDPOINT = TRUE`);
  } else if (subtype === "GCP_PUBSUB_INBOUND") {
    lines.push(`  TYPE = QUEUE`);
    lines.push(`  NOTIFICATION_PROVIDER = GCP_PUBSUB`);
    lines.push(`  DIRECTION = INBOUND`);
    if (vals.gcpSubName) lines.push(`  GCP_PUBSUB_SUBSCRIPTION_NAME = ${sq(String(vals.gcpSubName))}`);
  } else if (subtype === "AWS_SNS_OUTBOUND") {
    lines.push(`  TYPE = QUEUE`);
    lines.push(`  NOTIFICATION_PROVIDER = AWS_SNS`);
    lines.push(`  DIRECTION = OUTBOUND`);
    if (vals.awsSnsTopicArn) lines.push(`  AWS_SNS_TOPIC_ARN = ${sq(String(vals.awsSnsTopicArn))}`);
    if (vals.awsSnsRoleArn) lines.push(`  AWS_SNS_ROLE_ARN = ${sq(String(vals.awsSnsRoleArn))}`);
  } else if (subtype === "AZURE_EVENT_GRID_OUTBOUND") {
    lines.push(`  TYPE = QUEUE`);
    lines.push(`  NOTIFICATION_PROVIDER = AZURE_EVENT_GRID`);
    lines.push(`  DIRECTION = OUTBOUND`);
    if (vals.azureTopicEndpoint) lines.push(`  AZURE_EVENT_GRID_TOPIC_ENDPOINT = ${sq(String(vals.azureTopicEndpoint))}`);
    if (vals.azureTenantId) lines.push(`  AZURE_TENANT_ID = ${sq(String(vals.azureTenantId))}`);
  } else if (subtype === "GCP_PUBSUB_OUTBOUND") {
    lines.push(`  TYPE = QUEUE`);
    lines.push(`  NOTIFICATION_PROVIDER = GCP_PUBSUB`);
    lines.push(`  DIRECTION = OUTBOUND`);
    if (vals.gcpTopicName) lines.push(`  GCP_PUBSUB_TOPIC_NAME = ${sq(String(vals.gcpTopicName))}`);
  } else if (subtype === "EMAIL") {
    lines.push(`  TYPE = EMAIL`);
    if (vals.allowedRecipients) lines.push(`  ALLOWED_RECIPIENTS = (${splitLocations(String(vals.allowedRecipients))})`);
    if (vals.defaultRecipients) lines.push(`  DEFAULT_RECIPIENTS = (${splitLocations(String(vals.defaultRecipients))})`);
    if (vals.defaultSubject) lines.push(`  DEFAULT_SUBJECT = ${sq(String(vals.defaultSubject))}`);
  } else if (subtype === "WEBHOOK") {
    lines.push(`  TYPE = WEBHOOK`);
    if (vals.webhookUrl) lines.push(`  WEBHOOK_URL = ${sq(String(vals.webhookUrl))}`);
    if (vals.webhookSecret) lines.push(`  WEBHOOK_SECRET = ${sq(String(vals.webhookSecret))}`);
    if (vals.webhookBodyTemplate) lines.push(`  WEBHOOK_BODY_TEMPLATE = ${sq(String(vals.webhookBodyTemplate))}`);
    if (vals.webhookHeaders) lines.push(`  WEBHOOK_HEADERS = (${String(vals.webhookHeaders)})`);
  }
  lines.push(`  ENABLED = ${vals.enabled !== false ? "TRUE" : "FALSE"}`);
  if (vals.comment) lines.push(`  COMMENT = ${sq(String(vals.comment))}`);
  return lines.join("\n");
}

function buildSecuritySql(vals: Record<string, unknown>): string {
  const name = identToken(String(vals.name ?? ""), Boolean(vals.caseSensitive));
  const secType = String(vals.secType ?? "OAUTH");
  const enabled = vals.enabled !== false ? "TRUE" : "FALSE";
  const lines: string[] = [`CREATE SECURITY INTEGRATION ${name}`];

  if (secType === "API_AUTHENTICATION") {
    lines.push(`  TYPE = API_AUTHENTICATION`);
    const authType = String(vals.authType ?? "OAUTH2");
    lines.push(`  AUTH_TYPE = ${authType}`);
    if (authType === "AWS_IAM") {
      if (vals.awsRoleArn) lines.push(`  AWS_ROLE_ARN = ${sq(String(vals.awsRoleArn))}`);
    } else {
      const grant = String(vals.oauthGrant ?? "CLIENT_CREDENTIALS");
      lines.push(`  OAUTH_GRANT = ${grant}`);
      if (vals.oauthTokenEndpoint) lines.push(`  OAUTH_TOKEN_ENDPOINT = ${sq(String(vals.oauthTokenEndpoint))}`);
      if (vals.oauthClientId) lines.push(`  OAUTH_CLIENT_ID = ${sq(String(vals.oauthClientId))}`);
      if (vals.oauthClientSecret) lines.push(`  OAUTH_CLIENT_SECRET = ${sq(String(vals.oauthClientSecret))}`);
      if (vals.oauthScopes) lines.push(`  OAUTH_ALLOWED_SCOPES = (${splitLocations(String(vals.oauthScopes))})`);
    }
  } else if (secType === "EXTERNAL_OAUTH") {
    lines.push(`  TYPE = EXTERNAL_OAUTH`);
    if (vals.externalOauthType) lines.push(`  EXTERNAL_OAUTH_TYPE = ${String(vals.externalOauthType)}`);
    if (vals.issuer) lines.push(`  EXTERNAL_OAUTH_ISSUER = ${sq(String(vals.issuer))}`);
    if (vals.tokenUserMappingClaim) lines.push(`  EXTERNAL_OAUTH_TOKEN_USER_MAPPING_CLAIM = ${sq(String(vals.tokenUserMappingClaim))}`);
    if (vals.snowflakeUserMappingAttr) lines.push(`  EXTERNAL_OAUTH_SNOWFLAKE_USER_MAPPING_ATTRIBUTE = ${sq(String(vals.snowflakeUserMappingAttr))}`);
    if (vals.jwsKeysUrl) lines.push(`  EXTERNAL_OAUTH_JWS_KEYS_URL = ${sq(String(vals.jwsKeysUrl))}`);
    if (vals.audienceList) lines.push(`  EXTERNAL_OAUTH_AUDIENCE_LIST = (${splitLocations(String(vals.audienceList))})`);
    if (vals.anyRoleMode) lines.push(`  EXTERNAL_OAUTH_ANY_ROLE_MODE = ${String(vals.anyRoleMode)}`);
    if (vals.networkPolicy) lines.push(`  NETWORK_POLICY = ${ident(String(vals.networkPolicy))}`);
  } else if (secType === "OAUTH_PARTNER") {
    lines.push(`  TYPE = OAUTH`);
    if (vals.oauthClient) lines.push(`  OAUTH_CLIENT = ${String(vals.oauthClient)}`);
    if (vals.oauthRedirectUri) lines.push(`  OAUTH_REDIRECT_URI = ${sq(String(vals.oauthRedirectUri))}`);
    if (vals.oauthIssueRefreshTokens !== undefined) lines.push(`  OAUTH_ISSUE_REFRESH_TOKENS = ${vals.oauthIssueRefreshTokens ? "TRUE" : "FALSE"}`);
    if (vals.oauthRefreshTokenValidity) lines.push(`  OAUTH_REFRESH_TOKEN_VALIDITY = ${vals.oauthRefreshTokenValidity}`);
  } else if (secType === "OAUTH_CUSTOM") {
    lines.push(`  TYPE = OAUTH`);
    lines.push(`  OAUTH_CLIENT = CUSTOM`);
    if (vals.oauthClientType) lines.push(`  OAUTH_CLIENT_TYPE = ${String(vals.oauthClientType)}`);
    if (vals.oauthRedirectUri) lines.push(`  OAUTH_REDIRECT_URI = ${sq(String(vals.oauthRedirectUri))}`);
    if (vals.oauthIssueRefreshTokens !== undefined) lines.push(`  OAUTH_ISSUE_REFRESH_TOKENS = ${vals.oauthIssueRefreshTokens ? "TRUE" : "FALSE"}`);
    if (vals.oauthRefreshTokenValidity) lines.push(`  OAUTH_REFRESH_TOKEN_VALIDITY = ${vals.oauthRefreshTokenValidity}`);
    if (vals.networkPolicy) lines.push(`  NETWORK_POLICY = ${ident(String(vals.networkPolicy))}`);
  } else if (secType === "SAML2") {
    lines.push(`  TYPE = SAML2`);
    if (vals.samlIdpMetadataUrl) {
      lines.push(`  SAML2_IDP_METADATA_URL = ${sq(String(vals.samlIdpMetadataUrl))}`);
    } else {
      if (vals.samlIdpEntityId) lines.push(`  SAML2_IDP_ENTITY_ID = ${sq(String(vals.samlIdpEntityId))}`);
      if (vals.samlIdpSsoUrl) lines.push(`  SAML2_IDP_SSO_URL = ${sq(String(vals.samlIdpSsoUrl))}`);
      if (vals.samlIdpCert) lines.push(`  SAML2_IDP_CERTIFICATE = ${sq(String(vals.samlIdpCert))}`);
    }
    if (vals.samlAllowedUserDomains) lines.push(`  SAML2_ALLOWED_EMAIL_PATTERNS = (${splitLocations(String(vals.samlAllowedUserDomains))})`);
    if (vals.samlSignRequest !== undefined) lines.push(`  SAML2_SIGN_REQUEST = ${vals.samlSignRequest ? "TRUE" : "FALSE"}`);
    if (vals.samlForceAuthn !== undefined) lines.push(`  SAML2_FORCE_AUTHN = ${vals.samlForceAuthn ? "TRUE" : "FALSE"}`);
  } else if (secType === "SCIM") {
    lines.push(`  TYPE = SCIM`);
    if (vals.scimClient) lines.push(`  SCIM_CLIENT = ${sq(String(vals.scimClient))}`);
    if (vals.runAsRole) lines.push(`  RUN_AS_SERVICE_USER = ${ident(String(vals.runAsRole))}`);
    if (vals.networkPolicy) lines.push(`  NETWORK_POLICY = ${ident(String(vals.networkPolicy))}`);
    if (vals.syncPassword !== undefined) lines.push(`  SYNC_PASSWORD = ${vals.syncPassword ? "TRUE" : "FALSE"}`);
  }

  lines.push(`  ENABLED = ${enabled}`);
  if (vals.comment) lines.push(`  COMMENT = ${sq(String(vals.comment))}`);
  return lines.join("\n");
}

function splitLocations(s: string): string {
  return s.split(/[\n,]/).map((x) => x.trim()).filter(Boolean).map(sq).join(", ");
}

// ── Form sections ──────────────────────────────────────────────────────────────

function StorageForm({ provider }: { provider: string }) {
  return (
    <>
      {provider === "S3" || provider === "S3GOV" ? (
        <>
          <Form.Item name="awsRoleArn" label="AWS Role ARN" rules={[{ required: true, message: "Required" }]}>
            <Input placeholder="arn:aws:iam::123456789:role/my-role" />
          </Form.Item>
          <Form.Item name="allowedLocations" label="Allowed Locations" rules={[{ required: true, message: "Required" }]} help="One per line or comma-separated, e.g. s3://bucket/path/">
            <TextArea rows={3} placeholder="s3://my-bucket/path/" />
          </Form.Item>
          <Form.Item name="blockedLocations" label="Blocked Locations" help="Optional">
            <TextArea rows={2} placeholder="s3://my-bucket/blocked/" />
          </Form.Item>
          <Form.Item name="awsExternalId" label="AWS External ID">
            <Input />
          </Form.Item>
          <Form.Item name="usePrivatelink" label="Use PrivateLink Endpoint" valuePropName="checked">
            <Switch size="small" />
          </Form.Item>
        </>
      ) : provider === "GCS" ? (
        <>
          <Form.Item name="allowedLocations" label="Allowed Locations" rules={[{ required: true, message: "Required" }]} help="One per line or comma-separated, e.g. gcs://bucket/path/">
            <TextArea rows={3} placeholder="gcs://my-bucket/path/" />
          </Form.Item>
          <Form.Item name="blockedLocations" label="Blocked Locations" help="Optional">
            <TextArea rows={2} />
          </Form.Item>
        </>
      ) : provider === "AZURE" ? (
        <>
          <Form.Item name="azureTenantId" label="Azure Tenant ID" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="allowedLocations" label="Allowed Locations" rules={[{ required: true, message: "Required" }]} help="One per line or comma-separated, e.g. azure://account.blob.core.windows.net/container/">
            <TextArea rows={3} placeholder="azure://account.blob.core.windows.net/container/" />
          </Form.Item>
          <Form.Item name="blockedLocations" label="Blocked Locations" help="Optional">
            <TextArea rows={2} />
          </Form.Item>
          <Form.Item name="usePrivatelink" label="Use PrivateLink Endpoint" valuePropName="checked">
            <Switch size="small" />
          </Form.Item>
        </>
      ) : null}
    </>
  );
}

// Special options for the secrets tag-select
const GIT_SECRET_SPECIAL_OPTIONS = [
  { value: "ALL",  label: "ALL"  },
  { value: "NONE", label: "NONE" },
];

function GitHttpsApiForm({ gitAuthMode }: { gitAuthMode: string }) {
  const form = Form.useFormInstance();
  const allowedPrefixesVal = Form.useWatch("allowedPrefixes") as string | undefined;

  // When switching to GitHub App mode, pre-fill the base prefix so the user
  // only has to type the org/repo path.
  useEffect(() => {
    if (gitAuthMode === "GITHUB_APP") {
      const current = ((form.getFieldValue("allowedPrefixes") as string | undefined) ?? "").trim();
      if (!current) {
        form.setFieldValue("allowedPrefixes", "https://github.com/");
      }
    }
  }, [gitAuthMode, form]);

  // Warn when GitHub App mode has non-github.com prefixes
  const githubPrefixWarning = useMemo(() => {
    if (gitAuthMode !== "GITHUB_APP" || !allowedPrefixesVal) return false;
    const prefixes = allowedPrefixesVal.split(/[\n,]/).map((s) => s.trim()).filter(Boolean);
    return prefixes.some((p) => !p.startsWith("https://github.com/"));
  }, [gitAuthMode, allowedPrefixesVal]);

  return (
    <>
      <Form.Item
        name="allowedPrefixes"
        label="API Allowed Prefixes"
        rules={[
          // GitHub App always has https://github.com/ at minimum, so the field
          // is effectively optional — the SQL builder fills in the base URL.
          ...(gitAuthMode !== "GITHUB_APP" ? [{ required: true, message: "Required" }] : []),
          ...(gitAuthMode === "GITHUB_APP"
            ? [
                {
                  validator: (_: unknown, value: string) => {
                    const prefixes = (value ?? "")
                      .split(/[\n,]/)
                      .map((s: string) => s.trim())
                      .filter(Boolean);
                    const invalid = prefixes.filter(
                      (p: string) => !p.startsWith("https://github.com/")
                    );
                    return invalid.length > 0
                      ? Promise.reject("GitHub App prefixes must begin with https://github.com/")
                      : Promise.resolve();
                  },
                },
              ]
            : []),
        ]}
        help={
          gitAuthMode === "GITHUB_APP"
            ? "Optional — defaults to https://github.com/. Add specific org/repo paths on separate lines."
            : "One per line or comma-separated"
        }
      >
        <TextArea rows={3} placeholder={gitAuthMode === "GITHUB_APP" ? "https://github.com/my-org/my-repo/" : "https://example.com/"} />
      </Form.Item>

      {githubPrefixWarning && (
        <Alert
          type="warning"
          showIcon
          message="All prefixes must start with https://github.com/ for GitHub App authentication."
          style={{ marginBottom: 12, fontSize: 12 }}
        />
      )}

      <Form.Item name="blockedPrefixes" label="API Blocked Prefixes (optional)" help="One per line or comma-separated">
        <TextArea rows={2} />
      </Form.Item>

      <Form.Item name="gitAuthMode" label="Git Authentication Mode" initialValue="TOKEN" style={{ marginBottom: 8 }}>
        <Radio.Group>
          <Radio.Button value="TOKEN">Token / Secret</Radio.Button>
          <Radio.Button value="GITHUB_APP">GitHub App</Radio.Button>
          <Radio.Button value="OAUTH2">OAuth2</Radio.Button>
          <Radio.Button value="PRIVATELINK">Private Link</Radio.Button>
        </Radio.Group>
      </Form.Item>

      {(gitAuthMode === "TOKEN" || gitAuthMode === "PRIVATELINK") && (
        <Form.Item
          name="allowedAuthSecrets"
          label="Allowed Authentication Secrets (optional)"
          help='Enter "ALL", "NONE", or type secret names and press Enter'
        >
          <Select
            mode="tags"
            options={GIT_SECRET_SPECIAL_OPTIONS}
            placeholder='ALL, NONE, or secret names'
            maxTagCount="responsive"
          />
        </Form.Item>
      )}

      {gitAuthMode === "GITHUB_APP" && (
        <Alert
          type="info"
          showIcon
          message="GitHub App authentication is managed automatically by Snowflake. No additional credentials are required here."
          style={{ marginBottom: 12, fontSize: 12 }}
        />
      )}

      {gitAuthMode === "OAUTH2" && (
        <>
          <Form.Item name="oauthClientId" label="OAuth Client ID (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="oauthClientSecret" label="OAuth Client Secret (optional)">
            <Input.Password />
          </Form.Item>
          <Form.Item name="oauthTokenEndpoint" label="OAuth Token Endpoint (optional)">
            <Input placeholder="https://..." />
          </Form.Item>
          <Form.Item name="oauthScopes" label="OAuth Allowed Scopes (optional)" help="Comma-separated">
            <Input placeholder="read:user, repo" />
          </Form.Item>
        </>
      )}

      {gitAuthMode === "PRIVATELINK" && (
        <>
          <Form.Item name="usePrivateLink" label="Use PrivateLink Endpoint" valuePropName="checked">
            <Switch size="small" />
          </Form.Item>
          <Form.Item
            name="tlsCertificates"
            label="TLS Trusted Certificates (optional)"
            help="Secret names for TLS certificates — type names and press Enter"
          >
            <Select
              mode="tags"
              placeholder="Secret names"
              maxTagCount="responsive"
            />
          </Form.Item>
        </>
      )}
    </>
  );
}

function ApiForm({ provider, gitAuthMode }: { provider: string; gitAuthMode: string }) {
  if (provider === "git_https_api") {
    return <GitHttpsApiForm gitAuthMode={gitAuthMode} />;
  }
  return (
    <>
      <Form.Item name="allowedPrefixes" label="API Allowed Prefixes" rules={[{ required: true, message: "Required" }]} help="One per line or comma-separated">
        <TextArea rows={3} />
      </Form.Item>
      {(provider === "aws_api_gateway" || provider === "aws_private_api_gateway") && (
        <>
          <Form.Item name="awsRoleArn" label="API AWS Role ARN" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="apiKey" label="API Key (optional)">
            <Input />
          </Form.Item>
        </>
      )}
      {provider === "azure_api_management" && (
        <>
          <Form.Item name="azureTenantId" label="Azure Tenant ID" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="azureAdAppId" label="Azure AD Application ID" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="apiKey" label="API Key (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="blockedPrefixes" label="API Blocked Prefixes (optional)">
            <TextArea rows={2} />
          </Form.Item>
        </>
      )}
      {provider === "google_api_gateway" && (
        <>
          <Form.Item name="googleAudience" label="Google Audience" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="blockedPrefixes" label="API Blocked Prefixes (optional)">
            <TextArea rows={2} />
          </Form.Item>
        </>
      )}
    </>
  );
}

function CatalogForm({ source }: { source: string }) {
  return (
    <>
      {source === "GLUE" && (
        <>
          <Form.Item name="glueAwsRoleArn" label="Glue AWS Role ARN" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="glueCatalogId" label="Glue Catalog ID" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="glueRegion" label="Glue Region (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="catalogNamespace" label="Catalog Namespace (optional)">
            <Input />
          </Form.Item>
        </>
      )}
      {source === "OBJECT_STORE" && (
        <Form.Item name="tableFormat" label="Table Format" rules={[{ required: true, message: "Required" }]}>
          <Select options={[{ value: "ICEBERG", label: "ICEBERG" }, { value: "DELTA", label: "DELTA" }]} />
        </Form.Item>
      )}
      {source === "POLARIS" && (
        <>
          <Form.Item name="catalogUri" label="Catalog URI" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="catalogName" label="Catalog Name" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="catalogNamespace" label="Catalog Namespace (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="catalogApiType" label="Catalog API Type" initialValue="PUBLIC">
            <Select options={[{ value: "PUBLIC", label: "PUBLIC" }, { value: "PRIVATE", label: "PRIVATE" }]} />
          </Form.Item>
          <Form.Item name="accessDelegationMode" label="Access Delegation Mode (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="oauthTokenUri" label="OAuth Token URI (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="oauthClientId" label="OAuth Client ID" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="oauthClientSecret" label="OAuth Client Secret" rules={[{ required: true, message: "Required" }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="oauthScopes" label="OAuth Allowed Scopes" rules={[{ required: true, message: "Required" }]} help="Comma-separated">
            <Input />
          </Form.Item>
        </>
      )}
      {source === "ICEBERG_REST" && (
        <>
          <Form.Item name="catalogUri" label="Catalog URI" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="catalogName" label="Catalog Name (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="prefix" label="Prefix (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="catalogApiType" label="Catalog API Type" initialValue="PUBLIC">
            <Select options={["PUBLIC","PRIVATE","AWS_API_GATEWAY","AWS_PRIVATE_API_GATEWAY","AWS_GLUE","AWS_PRIVATE_GLUE"].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="accessDelegationMode" label="Access Delegation Mode (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="icebergAuthType" label="Auth Type" initialValue="OAUTH">
            <Select options={[{ value: "OAUTH", label: "OAUTH" }, { value: "BEARER", label: "BEARER" }, { value: "SIGV4", label: "SIGV4" }]} />
          </Form.Item>
          <Form.Item name="oauthTokenUri" label="OAuth Token URI (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="oauthClientId" label="OAuth Client ID (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="oauthClientSecret" label="OAuth Client Secret (optional)">
            <Input.Password />
          </Form.Item>
          <Form.Item name="bearerToken" label="Bearer Token (optional)">
            <Input.Password />
          </Form.Item>
        </>
      )}
      {source === "SAP_BDC" && (
        <>
          <Form.Item name="sapInvitationLink" label="SAP BDC Invitation Link" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="accessDelegationMode" label="Access Delegation Mode (optional)">
            <Input />
          </Form.Item>
        </>
      )}
      <Form.Item name="refreshInterval" label="Refresh Interval (seconds, optional)">
        <InputNumber min={1} style={{ width: "100%" }} />
      </Form.Item>
    </>
  );
}

function NotificationForm({ subtype }: { subtype: string }) {
  return (
    <>
      {subtype === "AZURE_STORAGE_QUEUE_INBOUND" && (
        <>
          <Form.Item name="azureQueueUri" label="Azure Storage Queue URI" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="azureTenantId" label="Azure Tenant ID" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="usePrivatelink" label="Use PrivateLink Endpoint" valuePropName="checked">
            <Switch size="small" />
          </Form.Item>
        </>
      )}
      {subtype === "GCP_PUBSUB_INBOUND" && (
        <Form.Item name="gcpSubName" label="GCP Pub/Sub Subscription Name" rules={[{ required: true, message: "Required" }]}>
          <Input />
        </Form.Item>
      )}
      {subtype === "AWS_SNS_OUTBOUND" && (
        <>
          <Form.Item name="awsSnsTopicArn" label="AWS SNS Topic ARN" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="awsSnsRoleArn" label="AWS SNS Role ARN" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
        </>
      )}
      {subtype === "AZURE_EVENT_GRID_OUTBOUND" && (
        <>
          <Form.Item name="azureTopicEndpoint" label="Azure Event Grid Topic Endpoint" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="azureTenantId" label="Azure Tenant ID" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
        </>
      )}
      {subtype === "GCP_PUBSUB_OUTBOUND" && (
        <Form.Item name="gcpTopicName" label="GCP Pub/Sub Topic Name" rules={[{ required: true, message: "Required" }]}>
          <Input />
        </Form.Item>
      )}
      {subtype === "EMAIL" && (
        <>
          <Form.Item name="allowedRecipients" label="Allowed Recipients (optional)" help="Comma-separated email addresses">
            <TextArea rows={2} />
          </Form.Item>
          <Form.Item name="defaultRecipients" label="Default Recipients (optional)" help="Comma-separated email addresses">
            <Input />
          </Form.Item>
          <Form.Item name="defaultSubject" label="Default Subject (optional)">
            <Input />
          </Form.Item>
        </>
      )}
      {subtype === "WEBHOOK" && (
        <>
          <Form.Item name="webhookUrl" label="Webhook URL" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="webhookSecret" label="Webhook Secret (optional)">
            <Input.Password />
          </Form.Item>
          <Form.Item name="webhookBodyTemplate" label="Body Template (optional)">
            <TextArea rows={3} />
          </Form.Item>
          <Form.Item name="webhookHeaders" label="Webhook Headers (optional)" help="KEY=VALUE pairs, comma-separated">
            <Input />
          </Form.Item>
        </>
      )}
    </>
  );
}

function SecurityForm({ secType }: { secType: string }) {
  return (
    <>
      {secType === "API_AUTHENTICATION" && (
        <>
          <Form.Item name="authType" label="Auth Type" initialValue="OAUTH2">
            <Select options={[{ value: "OAUTH2", label: "OAUTH2" }, { value: "AWS_IAM", label: "AWS_IAM" }]} />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(p, c) => p.authType !== c.authType}>
            {({ getFieldValue }) =>
              getFieldValue("authType") === "AWS_IAM" ? (
                <Form.Item name="awsRoleArn" label="AWS Role ARN" rules={[{ required: true, message: "Required" }]}>
                  <Input />
                </Form.Item>
              ) : (
                <>
                  <Form.Item name="oauthGrant" label="OAuth Grant" initialValue="CLIENT_CREDENTIALS">
                    <Select options={["CLIENT_CREDENTIALS","AUTHORIZATION_CODE","JWT_BEARER"].map((v) => ({ value: v, label: v }))} />
                  </Form.Item>
                  <Form.Item name="oauthTokenEndpoint" label="OAuth Token Endpoint">
                    <Input />
                  </Form.Item>
                  <Form.Item name="oauthClientId" label="OAuth Client ID" rules={[{ required: true, message: "Required" }]}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="oauthClientSecret" label="OAuth Client Secret">
                    <Input.Password />
                  </Form.Item>
                  <Form.Item name="oauthScopes" label="OAuth Allowed Scopes (optional)" help="Comma-separated">
                    <Input />
                  </Form.Item>
                </>
              )
            }
          </Form.Item>
        </>
      )}
      {secType === "EXTERNAL_OAUTH" && (
        <>
          <Form.Item name="externalOauthType" label="External OAuth Type" rules={[{ required: true, message: "Required" }]}>
            <Select options={["OKTA","AZURE","PING_FEDERATE","CUSTOM"].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="issuer" label="Issuer" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="tokenUserMappingClaim" label="Token User Mapping Claim" rules={[{ required: true, message: "Required" }]}>
            <Input placeholder="sub" />
          </Form.Item>
          <Form.Item name="snowflakeUserMappingAttr" label="Snowflake User Mapping Attribute" rules={[{ required: true, message: "Required" }]}>
            <Select options={["LOGIN_NAME","EMAIL_ADDRESS"].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="jwsKeysUrl" label="JWS Keys URL (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="audienceList" label="Audience List (optional)" help="Comma-separated">
            <Input />
          </Form.Item>
          <Form.Item name="anyRoleMode" label="Any Role Mode (optional)">
            <Select allowClear options={["DISABLE","ENABLE","ENABLE_FOR_PRIVILEGE"].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="networkPolicy" label="Network Policy (optional)">
            <Input />
          </Form.Item>
        </>
      )}
      {secType === "OAUTH_PARTNER" && (
        <>
          <Form.Item name="oauthClient" label="OAuth Client" rules={[{ required: true, message: "Required" }]}>
            <Select options={["LOOKER","TABLEAU_DESKTOP","TABLEAU_SERVER","POWER_BI"].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="oauthRedirectUri" label="OAuth Redirect URI">
            <Input />
          </Form.Item>
          <Form.Item name="oauthIssueRefreshTokens" label="Issue Refresh Tokens" valuePropName="checked" initialValue={true}>
            <Switch size="small" />
          </Form.Item>
          <Form.Item name="oauthRefreshTokenValidity" label="Refresh Token Validity (seconds, optional)">
            <InputNumber min={1} style={{ width: "100%" }} />
          </Form.Item>
        </>
      )}
      {secType === "OAUTH_CUSTOM" && (
        <>
          <Form.Item name="oauthClientType" label="Client Type" rules={[{ required: true, message: "Required" }]}>
            <Select options={[{ value: "CONFIDENTIAL", label: "CONFIDENTIAL" }, { value: "PUBLIC", label: "PUBLIC" }]} />
          </Form.Item>
          <Form.Item name="oauthRedirectUri" label="OAuth Redirect URI" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="oauthIssueRefreshTokens" label="Issue Refresh Tokens" valuePropName="checked" initialValue={true}>
            <Switch size="small" />
          </Form.Item>
          <Form.Item name="oauthRefreshTokenValidity" label="Refresh Token Validity (seconds, optional)">
            <InputNumber min={1} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item name="networkPolicy" label="Network Policy (optional)">
            <Input />
          </Form.Item>
        </>
      )}
      {secType === "SAML2" && (
        <>
          <Form.Item name="samlIdpMetadataUrl" label="IDP Metadata URL (optional, overrides manual fields)">
            <Input />
          </Form.Item>
          <Form.Item name="samlIdpEntityId" label="IDP Entity ID">
            <Input />
          </Form.Item>
          <Form.Item name="samlIdpSsoUrl" label="IDP SSO URL">
            <Input />
          </Form.Item>
          <Form.Item name="samlIdpCert" label="IDP Certificate">
            <TextArea rows={3} />
          </Form.Item>
          <Form.Item name="samlAllowedUserDomains" label="Allowed Email Patterns (optional)" help="Comma-separated">
            <Input />
          </Form.Item>
          <Form.Item name="samlSignRequest" label="Sign Request" valuePropName="checked">
            <Switch size="small" />
          </Form.Item>
          <Form.Item name="samlForceAuthn" label="Force AuthN" valuePropName="checked">
            <Switch size="small" />
          </Form.Item>
        </>
      )}
      {secType === "SCIM" && (
        <>
          <Form.Item name="scimClient" label="SCIM Client" rules={[{ required: true, message: "Required" }]}>
            <Select options={["OKTA","AZURE","GENERIC"].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="runAsRole" label="Run As Service User" rules={[{ required: true, message: "Required" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="networkPolicy" label="Network Policy (optional)">
            <Input />
          </Form.Item>
          <Form.Item name="syncPassword" label="Sync Password" valuePropName="checked">
            <Switch size="small" />
          </Form.Item>
        </>
      )}
    </>
  );
}

// ── Default provider detection ─────────────────────────────────────────────────

function defaultProviderForKind(kind: string, region: string): string {
  const r = region.toUpperCase();
  const isAzure = r.startsWith("AZURE_");
  const isGcp   = r.startsWith("GCP_");

  if (kind === "STORAGE") {
    if (isAzure) return "AZURE";
    if (isGcp) return "GCS";
    return "S3";
  }
  if (kind === "API") {
    if (isAzure) return "azure_api_management";
    if (isGcp) return "google_api_gateway";
    return "aws_api_gateway";
  }
  return "";
}

// ── Main modal ─────────────────────────────────────────────────────────────────

export default function CreateIntegrationModal({ kind, onClose, onSuccess }: Props) {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [error, setError]     = useState<string | null>(null);
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);

  // Provider / subtype state tracked as watched form values
  const provider    = Form.useWatch("provider",    form) as string  | undefined;
  const source      = Form.useWatch("source",      form) as string  | undefined;
  const subtype     = Form.useWatch("subtype",     form) as string  | undefined;
  const secType     = Form.useWatch("secType",     form) as string  | undefined;
  const nameValue   = (Form.useWatch("name",       form) as string  | undefined) ?? "";
  const gitAuthMode = Form.useWatch("gitAuthMode", form) as string  | undefined;
  const orReplace   = Form.useWatch("orReplace",   form) as boolean | undefined;
  const ifNotExists = Form.useWatch("ifNotExists", form) as boolean | undefined;

  // Additional watches for the live git_https_api SQL preview
  const gitAllowedPrefixes    = Form.useWatch("allowedPrefixes",   form) as string   | undefined;
  const gitBlockedPrefixes    = Form.useWatch("blockedPrefixes",    form) as string   | undefined;
  const gitAllowedSecrets     = Form.useWatch("allowedAuthSecrets", form) as string[] | undefined;
  const gitOauthClientId      = Form.useWatch("oauthClientId",      form) as string   | undefined;
  const gitOauthClientSecret  = Form.useWatch("oauthClientSecret",  form) as string   | undefined;
  const gitOauthTokenEndpoint = Form.useWatch("oauthTokenEndpoint", form) as string   | undefined;
  const gitOauthScopes        = Form.useWatch("oauthScopes",        form) as string   | undefined;
  const gitUsePrivateLink     = Form.useWatch("usePrivateLink",     form) as boolean  | undefined;
  const gitTlsCertificates    = Form.useWatch("tlsCertificates",    form) as string[] | undefined;
  const enabledVal            = Form.useWatch("enabled",            form) as boolean  | undefined;
  const commentVal            = Form.useWatch("comment",            form) as string   | undefined;

  const isGitHttps = kind === "API" && provider === "git_https_api";

  // Live SQL preview for git_https_api — recomputed whenever any relevant field changes
  const sqlPreview = useMemo(() => {
    if (!isGitHttps) return "";
    return buildApiSql({
      name:               nameValue,
      caseSensitive,
      provider:           "git_https_api",
      orReplace,
      ifNotExists,
      enabled:            enabledVal,
      allowedPrefixes:    gitAllowedPrefixes,
      blockedPrefixes:    gitBlockedPrefixes,
      gitAuthMode:        gitAuthMode ?? "TOKEN",
      allowedAuthSecrets: gitAllowedSecrets,
      oauthClientId:      gitOauthClientId,
      oauthClientSecret:  gitOauthClientSecret,
      oauthTokenEndpoint: gitOauthTokenEndpoint,
      oauthScopes:        gitOauthScopes,
      usePrivateLink:     gitUsePrivateLink,
      tlsCertificates:    gitTlsCertificates,
      comment:            commentVal,
    });
  }, [
    isGitHttps, nameValue, caseSensitive, orReplace, ifNotExists, enabledVal,
    gitAllowedPrefixes, gitBlockedPrefixes, gitAllowedSecrets, gitAuthMode,
    gitOauthClientId, gitOauthClientSecret, gitOauthTokenEndpoint, gitOauthScopes,
    gitUsePrivateLink, gitTlsCertificates, commentVal,
  ]);

  useEffect(() => {
    GetCurrentRegion()
      .then((region) => {
        const def = defaultProviderForKind(kind, region ?? "");
        if (def) {
          if (kind === "STORAGE") form.setFieldValue("provider", def);
          if (kind === "API")     form.setFieldValue("provider", def);
        }
      })
      .catch(() => {});
    Promise.resolve()
      .then(() => GetQuotedIdentifiersIgnoreCase())
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});
  }, [kind, form]);

  const submit = async () => {
    let vals: Record<string, unknown>;
    try {
      vals = await form.validateFields();
    } catch {
      return;
    }
    vals.caseSensitive = caseSensitive;
    setLoading(true);
    setError(null);
    try {
      let sql = "";
      if (kind === "STORAGE")          sql = buildStorageSql(vals);
      else if (kind === "API")         sql = buildApiSql(vals);
      else if (kind === "CATALOG")     sql = buildCatalogSql(vals);
      else if (kind === "EXTERNAL ACCESS") sql = buildExternalAccessSql(vals);
      else if (kind === "NOTIFICATION") sql = buildNotificationSql(vals);
      else if (kind === "SECURITY")    sql = buildSecuritySql(vals);
      await ExecuteQuery(sql);
      onSuccess();
      onClose();
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  const kindLabel = kind.charAt(0) + kind.slice(1).toLowerCase();

  return (
    <Modal
      open
      title={`Create ${kindLabel} Integration`}
      onCancel={onClose}
      width={620}
      footer={[
        <Button key="cancel" onClick={onClose} disabled={loading}>Cancel</Button>,
        <Button key="create" type="primary" loading={loading} onClick={submit}>Create</Button>,
      ]}
    >
      <div style={{ maxHeight: "65vh", overflowY: "auto", paddingRight: 8 }}>
        <Form form={form} layout="vertical" size="small">
          <Form.Item name="name" label="Integration Name" rules={[{ required: true, message: "Required" }]} style={{ marginBottom: 4 }}>
            <Input placeholder="MY_INTEGRATION" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 12 }}>
            <ObjectNameCaseControl
              name={nameValue}
              caseSensitive={caseSensitive}
              onCaseSensitiveChange={setCaseSensitive}
              quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
            />
          </Form.Item>

          <Form.Item name="enabled" label="Enabled" valuePropName="checked" initialValue={true}>
            <Switch size="small" />
          </Form.Item>

          {/* Kind-specific top-level selector */}
          {kind === "STORAGE" && (
            <Form.Item name="provider" label="Storage Provider" initialValue="S3">
              <Select options={[
                { value: "S3",     label: "Amazon S3" },
                { value: "S3GOV",  label: "Amazon S3 (GovCloud)" },
                { value: "GCS",    label: "Google Cloud Storage" },
                { value: "AZURE",  label: "Azure Blob Storage" },
              ]} />
            </Form.Item>
          )}
          {kind === "API" && (
            <Form.Item name="provider" label="API Provider" initialValue="aws_api_gateway">
              <Select options={[
                { value: "aws_api_gateway",          label: "AWS API Gateway" },
                { value: "aws_private_api_gateway",  label: "AWS Private API Gateway" },
                { value: "azure_api_management",     label: "Azure API Management" },
                { value: "google_api_gateway",       label: "Google API Gateway" },
                { value: "git_https_api",            label: "Git HTTPS API" },
              ]} />
            </Form.Item>
          )}
          {kind === "CATALOG" && (
            <Form.Item name="source" label="Catalog Source" initialValue="GLUE">
              <Select options={["GLUE","OBJECT_STORE","POLARIS","ICEBERG_REST","SAP_BDC"].map((v) => ({ value: v, label: v }))} />
            </Form.Item>
          )}
          {kind === "NOTIFICATION" && (
            <Form.Item name="subtype" label="Notification Type" initialValue="EMAIL">
              <Select options={[
                { value: "EMAIL",                       label: "Email" },
                { value: "WEBHOOK",                     label: "Webhook" },
                { value: "AZURE_STORAGE_QUEUE_INBOUND", label: "Azure Storage Queue (Inbound)" },
                { value: "GCP_PUBSUB_INBOUND",          label: "GCP Pub/Sub (Inbound)" },
                { value: "AWS_SNS_OUTBOUND",            label: "AWS SNS (Outbound)" },
                { value: "AZURE_EVENT_GRID_OUTBOUND",   label: "Azure Event Grid (Outbound)" },
                { value: "GCP_PUBSUB_OUTBOUND",         label: "GCP Pub/Sub (Outbound)" },
              ]} />
            </Form.Item>
          )}
          {kind === "SECURITY" && (
            <Form.Item name="secType" label="Security Integration Type" initialValue="OAUTH_PARTNER">
              <Select options={[
                { value: "API_AUTHENTICATION", label: "API Authentication" },
                { value: "EXTERNAL_OAUTH",     label: "External OAuth" },
                { value: "OAUTH_PARTNER",      label: "OAuth (Partner)" },
                { value: "OAUTH_CUSTOM",       label: "OAuth (Custom)" },
                { value: "SAML2",              label: "SAML2" },
                { value: "SCIM",               label: "SCIM" },
              ]} />
            </Form.Item>
          )}

          {/* OR REPLACE / IF NOT EXISTS — shown only for git_https_api */}
          {isGitHttps && (
            <Form.Item label="Create Options" style={{ marginBottom: 8 }}>
              <div style={{ display: "flex", gap: 16 }}>
                <Form.Item name="orReplace" valuePropName="checked" noStyle>
                  <Checkbox disabled={!!ifNotExists}>OR REPLACE</Checkbox>
                </Form.Item>
                <Form.Item name="ifNotExists" valuePropName="checked" noStyle>
                  <Checkbox disabled={!!orReplace}>IF NOT EXISTS</Checkbox>
                </Form.Item>
              </div>
            </Form.Item>
          )}

          {/* Type-specific fields */}
          {kind === "STORAGE"          && <StorageForm provider={provider ?? "S3"} />}
          {kind === "API"              && <ApiForm provider={provider ?? "aws_api_gateway"} gitAuthMode={gitAuthMode ?? "TOKEN"} />}
          {kind === "CATALOG"          && <CatalogForm source={source ?? "GLUE"} />}
          {kind === "EXTERNAL ACCESS"  && (
            <>
              <Form.Item name="allowedNetworkRules" label="Allowed Network Rules" rules={[{ required: true, message: "Required" }]} help="Comma-separated rule names">
                <TextArea rows={2} />
              </Form.Item>
              <Form.Item name="allowedApiAuthIntegrations" label="Allowed API Auth Integrations (optional)" help="Comma-separated">
                <Input />
              </Form.Item>
              <Form.Item name="allowedAuthSecrets" label="Allowed Authentication Secrets" help='"all", "none", or comma-separated secret names'>
                <Input placeholder="all" />
              </Form.Item>
            </>
          )}
          {kind === "NOTIFICATION"     && <NotificationForm subtype={subtype ?? "EMAIL"} />}
          {kind === "SECURITY"         && <SecurityForm secType={secType ?? "OAUTH_PARTNER"} />}

          <Form.Item name="comment" label="Comment (optional)">
            <Input />
          </Form.Item>
        </Form>

        {/* Live SQL preview for git_https_api */}
        {isGitHttps && sqlPreview && (
          <>
            <Divider style={{ margin: "8px 0" }} />
            <Typography.Text type="secondary" style={{ fontSize: 11 }}>SQL Preview</Typography.Text>
            <pre style={{
              fontSize: 11,
              fontFamily: "monospace",
              background: "rgba(0,0,0,0.04)",
              padding: 8,
              borderRadius: 4,
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
              marginTop: 4,
              marginBottom: 0,
            }}>{sqlPreview}</pre>
          </>
        )}

        {error && (
          <Alert
            type="error"
            message={error}
            style={{ marginTop: 8, fontSize: 12, fontFamily: "monospace" }}
          />
        )}
      </div>
    </Modal>
  );
}
