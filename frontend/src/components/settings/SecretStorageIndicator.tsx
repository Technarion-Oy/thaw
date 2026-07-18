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

  if (info.secure) {
    return (
      <Text type="secondary" style={{ fontSize: 12 }}>
        <LockOutlined style={{ marginRight: 4 }} />
        Your {noun} is stored in {info.label}.
      </Text>
    );
  }

  return (
    <Text type="warning" style={{ fontSize: 12 }}>
      <WarningOutlined style={{ marginRight: 4 }} />
      No OS secure store is available — your {noun} is stored in a local file
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
