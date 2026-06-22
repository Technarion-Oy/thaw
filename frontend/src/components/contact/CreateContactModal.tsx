// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect } from "react";
import { Form, Input, Checkbox, Alert, Select, Radio } from "antd";
import { ContactsOutlined } from "@ant-design/icons";
import { BuildCreateContactSql, ExecDDL, ListUsers } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";

// A contact carries exactly one contact method; these match the Go
// contact.Method* constants the builder switches on.
type Method = "users" | "email" | "url";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateContactModal({ db, schema, onClose, onSuccess }: Props) {
  const [name, setName] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [orReplace, setOrReplace] = useState(false);
  const [ifNotExists, setIfNotExists] = useState(false);
  const [method, setMethod] = useState<Method>("email");
  const [users, setUsers] = useState<string[]>([]);
  const [email, setEmail] = useState("");
  const [url, setUrl] = useState("");
  const [comment, setComment] = useState("");

  // Snowflake users for the USERS-method dropdown ("use drop down to add
  // snowflake users"). Best-effort: a SHOW USERS failure (insufficient
  // privileges) leaves the list empty but the Select stays free-typeable.
  const [userOptions, setUserOptions] = useState<string[]>([]);
  const [loadingUsers, setLoadingUsers] = useState(false);

  useEffect(() => {
    setLoadingUsers(true);
    ListUsers()
      .then((list) => setUserOptions((list ?? []).map((u) => u.name).filter(Boolean)))
      .catch(() => {})
      .finally(() => setLoadingUsers(false));
  }, []);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () =>
      BuildCreateContactSql(db, schema, {
        name,
        caseSensitive,
        orReplace,
        ifNotExists,
        method,
        users,
        email,
        url,
        comment,
      } as any),
    [db, schema, name, caseSensitive, orReplace, ifNotExists, method, users, email, url, comment],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const methodFilled =
    (method === "users" && users.length > 0) ||
    (method === "email" && email.trim().length > 0) ||
    (method === "url" && url.trim().length > 0);
  const canSubmit = name.trim().length > 0 && methodFilled;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<ContactsOutlined />}
      title="Create Contact"
      subtitle={`${db}.${schema}`}
      width={640}
      error={error}
      errorTitle="Contact creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="A contact names a notification target — a set of Snowflake users, an email distribution list, or a URL — used by alerts and other notification-based features. A contact has exactly one method."
        />

        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Contact name" required style={{ marginBottom: 4 }}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="MY_CONTACT" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={orReplace}
              onChange={(e) => { setOrReplace(e.target.checked); if (e.target.checked) setIfNotExists(false); }}
            >
              OR REPLACE
            </Checkbox>
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={ifNotExists}
              onChange={(e) => { setIfNotExists(e.target.checked); if (e.target.checked) setOrReplace(false); }}
            >
              IF NOT EXISTS
            </Checkbox>
          </Form.Item>
        </div>

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={name}
            caseSensitive={caseSensitive}
            onCaseSensitiveChange={setCaseSensitive}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Contact method" required style={itemStyle}>
          <Radio.Group value={method} onChange={(e) => setMethod(e.target.value)} optionType="button" buttonStyle="solid">
            <Radio value="email">Email distribution list</Radio>
            <Radio value="users">Snowflake users</Radio>
            <Radio value="url">URL</Radio>
          </Radio.Group>
        </Form.Item>

        {method === "email" && (
          <Form.Item label="Email distribution list" required style={itemStyle} help="A valid email address, which can be a distribution list.">
            <Input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="support@example.com" />
          </Form.Item>
        )}

        {method === "users" && (
          <Form.Item label="Snowflake users" required style={itemStyle} help="One or more Snowflake users to notify.">
            {/* mode="tags" (not "multiple") so a user name can still be typed
                manually when SHOW USERS is unavailable (insufficient privileges
                leaves the options list empty). */}
            <Select
              mode="tags"
              showSearch
              loading={loadingUsers}
              value={users}
              onChange={(v) => setUsers(v)}
              placeholder="Select or type users…"
              options={userOptions.map((u) => ({ value: u, label: u }))}
              notFoundContent={loadingUsers ? "Loading…" : "No users found"}
              style={{ width: "100%" }}
            />
          </Form.Item>
        )}

        {method === "url" && (
          <Form.Item label="URL" required style={itemStyle} help="A URL that can be used to contact people about an object.">
            <Input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/oncall" />
          </Form.Item>
        )}

        <Form.Item label="Comment" style={itemStyle}>
          <Input.TextArea value={comment} onChange={(e) => setComment(e.target.value)} rows={2} placeholder="Optional description" />
        </Form.Item>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
