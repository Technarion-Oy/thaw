// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { cloneElement, isValidElement } from "react";
import type { ReactElement, ReactNode } from "react";
import { Modal, Space, Typography, Button, Alert } from "antd";

const { Text } = Typography;

interface Props {
  /** Leading icon — rendered tinted in the title and plain on the Create button. */
  icon: ReactElement;
  /** Title text, e.g. "Create Secret". */
  title: string;
  /** Optional muted subtitle after the title, e.g. `${db}.${schema}`. */
  subtitle?: ReactNode;
  /** Error banner text; the banner is hidden when null/undefined. */
  error?: string | null;
  /** Heading of the error banner, e.g. "Secret creation failed". */
  errorTitle?: string;
  /** Called when the user dismisses the error banner. */
  onErrorClose?: () => void;
  /** Disables Cancel and shows the spinner on Create. */
  creating: boolean;
  /** Enables the Create button. Defaults to true. */
  canSubmit?: boolean;
  /** Label of the primary button. Defaults to "Create". */
  okText?: string;
  /** Icon on the primary button. Defaults to `icon`. */
  okIcon?: ReactElement;
  width?: number;
  /** `body.maxHeight`. Defaults to "80vh". */
  bodyMaxHeight?: string;
  /**
   * When true, ESC / backdrop-click / close-X are ignored while `creating` — for
   * modals whose submit runs a real side effect (e.g. a file upload) that
   * shouldn't be orphaned by dismissing the modal mid-flight. Defaults to false so
   * existing modals keep their normal dismiss-anytime behavior.
   */
  lockWhileBusy?: boolean;
  onClose: () => void;
  onSubmit: () => void;
  children: ReactNode;
}

/**
 * Shared chrome for object-creation modals: the tinted icon + title + subtitle
 * header, the Cancel / Create footer with loading + disabled wiring, and the
 * dismissible error banner. The form body is supplied as `children`.
 */
export default function CreateModalShell({
  icon,
  title,
  subtitle,
  error,
  errorTitle = "Creation failed",
  onErrorClose,
  creating,
  canSubmit = true,
  okText = "Create",
  okIcon,
  width = 600,
  bodyMaxHeight = "80vh",
  lockWhileBusy = false,
  onClose,
  onSubmit,
  children,
}: Props) {
  const titleIcon = isValidElement(icon)
    ? cloneElement(icon as ReactElement<{ style?: React.CSSProperties }>, {
        style: { color: "var(--link)", ...(icon.props as { style?: React.CSSProperties }).style },
      })
    : icon;

  return (
    <Modal
      open
      title={
        <Space size={6}>
          {titleIcon}
          <span>{title}</span>
          {subtitle != null && (
            <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
              {subtitle}
            </Text>
          )}
        </Space>
      }
      onCancel={() => { if (!(lockWhileBusy && creating)) onClose(); }}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={creating}>Cancel</Button>
          <Button
            type="primary"
            icon={okIcon ?? icon}
            onClick={onSubmit}
            disabled={!canSubmit}
            loading={creating}
          >
            {okText}
          </Button>
        </Space>
      }
      width={width}
      styles={{ body: { paddingTop: 16, maxHeight: bodyMaxHeight, overflowY: "auto" } }}
    >
      {error && (
        <Alert
          type="error"
          message={errorTitle}
          description={error}
          showIcon
          closable
          onClose={onErrorClose}
          style={{ marginBottom: 16 }}
        />
      )}
      {children}
    </Modal>
  );
}
