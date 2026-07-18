// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Core IPC & App Lifecycle

import { useEffect, useState } from "react";
import { Typography } from "antd";
import { LockOutlined, WarningOutlined } from "@ant-design/icons";
import { GetSecretStorageInfo } from "../../../wailsjs/go/app/App";
import { secrets } from "../../../wailsjs/go/models";

const { Text } = Typography;

let cached: secrets.Info | null = null;

/**
 * SecretStorageIndicator shows where Thaw actually stores secrets on this OS,
 * as reported by the backend (App.GetSecretStorageInfo) — never guessed from
 * the frontend platform. When the OS secure store is available it renders a
 * subtle "Stored in <store>" hint; when the plaintext file fallback is active
 * it renders a warning-styled line naming the file.
 *
 * `noun` names what is stored (e.g. "API key", "password"), defaulting to a
 * generic "secret".
 */
export function SecretStorageIndicator({ noun = "secret" }: { noun?: string }) {
  const [info, setInfo] = useState<secrets.Info | null>(cached);

  useEffect(() => {
    if (cached) return;
    let alive = true;
    GetSecretStorageInfo()
      .then((i) => {
        cached = i;
        if (alive) setInfo(i);
      })
      .catch(() => {
        /* leave unset — no indicator rather than a wrong one */
      });
    return () => {
      alive = false;
    };
  }, []);

  if (!info) return null;

  // The storage location is the subject (not `noun`) so the copy reads
  // correctly whether the caller passes a singular ("API key") or plural
  // ("registry & proxy passwords") noun.
  if (info.secure) {
    return (
      <Text type="secondary" style={{ fontSize: 12 }}>
        <LockOutlined style={{ marginRight: 4 }} />
        {info.label} holds your {noun}.
      </Text>
    );
  }

  return (
    <Text type="warning" style={{ fontSize: 12 }}>
      <WarningOutlined style={{ marginRight: 4 }} />
      No OS secure store is available — a local file holds your {noun}
      {info.detail ? (
        <>
          {" "}
          (<Text code style={{ fontSize: 12 }}>{info.detail}</Text>, permissions 0600).
        </>
      ) : (
        " (permissions 0600)."
      )}
    </Text>
  );
}
