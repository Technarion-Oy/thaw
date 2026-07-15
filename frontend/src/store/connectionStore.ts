// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Core IPC & App Lifecycle

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

export interface ConnectionParams {
  account: string;
  user: string;
  password: string;
  role: string;
  warehouse: string;
  database: string;
  schema: string;
  authenticator: string;
  passcode: string;
  oktaUrl: string;
  privateKeyPath: string;
  privateKeyPassphrase: string;
  // Token-based, OAuth2, and Workload Identity Federation authenticators.
  token: string;
  tokenFilePath: string;
  oauthClientId: string;
  oauthClientSecret: string;
  oauthTokenRequestUrl: string;
  oauthAuthorizationUrl: string;
  oauthRedirectUri: string;
  oauthScope: string;
  enableSingleUseRefreshTokens: boolean;
  workloadIdentityProvider: string;
  workloadIdentityEntraResource: string;
  workloadIdentityImpersonationPath: string;
  // Forward-proxy configuration.
  proxyHost: string;
  proxyPort: number;
  proxyUser: string;
  proxyPassword: string;
  proxyProtocol: string;
  noProxy: string;
}

interface ConnectionState {
  isConnected: boolean;
  params: ConnectionParams | null;
  setConnected: (params: ConnectionParams) => void;
  setIsConnected: (v: boolean) => void;
  disconnect: () => void;
}

export const useConnectionStore = create<ConnectionState>()(
  persist(
    (set) => ({
      isConnected: false,
      params: null,
      setConnected: (params) => set({ isConnected: true, params }),
      setIsConnected: (v) => set({ isConnected: v }),
      disconnect: () => set({ isConnected: false, params: null }),
    }),
    {
      name: "thaw-connection",
      storage: createJSONStorage(() => sessionStorage),
      partialize: (state) => ({
        isConnected: state.isConnected,
        params: state.params
          ? {
              ...state.params,
              // Never persist credentials to storage
              password: "",
              passcode: "",
              privateKeyPassphrase: "",
              token: "",
              oauthClientSecret: "",
              proxyPassword: "",
            }
          : null,
      }),
    },
  ),
);
