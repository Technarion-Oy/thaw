// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties   holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useRef } from "react";
import { Modal, Form, Input, Select, Switch, Button, Alert, InputNumber, Radio, Checkbox, Divider, Typography } from "antd";
import {
  GetCurrentRegion,
  GetQuotedIdentifiersIgnoreCase,
  CreateStorageIntegration,
  CreateApiIntegration,
  CreateCatalogIntegration,
  CreateExternalAccessIntegration,
  CreateNotificationIntegration,
  CreateSecurityIntegration,
  BuildApiIntegrationPreviewSQL,
} from "../../../wailsjs/go/main/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";

const { TextArea } = Input;

interface Props {
  kind: string;
  onClose: () => void;
  onSuccess: () => void;
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
  return (
    <>
      {/* Allowed Prefixes — locked addonBefore for GitHub App, free TextArea for other modes */}
      {gitAuthMode === "GITHUB_APP" ? (
        <Form.Item
          name="githubAppPath"
          label="API Allowed Prefix"
          help="Optional — leave blank to allow all github.com repos, or enter an org/repo path"
        >
          <Input addonBefore="https://github.com/" placeholder="my-org/my-repo/" />
        </Form.Item>
      ) : (
        <Form.Item
          name="allowedPrefixes"
          label="API Allowed Prefixes"
          rules={[{ required: true, message: "Required" }]}
          help="One per line or comma-separated"
        >
          <TextArea rows={3} placeholder="https://example.com/" />
        </Form.Item>
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
  const githubAppPath         = Form.useWatch("githubAppPath",      form) as string   | undefined;
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

  // Live SQL preview for git_https_api — debounced call to Go backend
  const [sqlPreview, setSqlPreview] = useState("");
  const previewTimerRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    if (!isGitHttps) { setSqlPreview(""); return; }
    clearTimeout(previewTimerRef.current);
    previewTimerRef.current = setTimeout(async () => {
      try {
        const sql = await BuildApiIntegrationPreviewSQL({
          name:               nameValue,
          caseSensitive,
          provider:           "git_https_api",
          orReplace:          orReplace ?? false,
          ifNotExists:        ifNotExists ?? false,
          enabled:            enabledVal ?? true,
          githubAppPath:      githubAppPath ?? "",
          allowedPrefixes:    gitAllowedPrefixes ?? "",
          blockedPrefixes:    gitBlockedPrefixes ?? "",
          gitAuthMode:        gitAuthMode ?? "TOKEN",
          allowedAuthSecrets: gitAllowedSecrets ?? [],
          oauthClientId:      gitOauthClientId ?? "",
          oauthClientSecret:  gitOauthClientSecret ?? "",
          oauthTokenEndpoint: gitOauthTokenEndpoint ?? "",
          oauthScopes:        gitOauthScopes ?? "",
          usePrivateLink:     gitUsePrivateLink ?? false,
          tlsCertificates:    gitTlsCertificates ?? [],
          comment:            commentVal ?? "",
        } as any);
        setSqlPreview(sql ?? "");
      } catch {
        setSqlPreview("");
      }
    }, 300);
    return () => clearTimeout(previewTimerRef.current);
  }, [
    isGitHttps, nameValue, caseSensitive, orReplace, ifNotExists, enabledVal,
    githubAppPath, gitAllowedPrefixes, gitBlockedPrefixes, gitAllowedSecrets, gitAuthMode,
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
      if (kind === "STORAGE")
        await CreateStorageIntegration(vals as any);
      else if (kind === "API")
        await CreateApiIntegration(vals as any);
      else if (kind === "CATALOG")
        await CreateCatalogIntegration(vals as any);
      else if (kind === "EXTERNAL ACCESS")
        await CreateExternalAccessIntegration(vals as any);
      else if (kind === "NOTIFICATION")
        await CreateNotificationIntegration(vals as any);
      else if (kind === "SECURITY")
        await CreateSecurityIntegration(vals as any);
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
