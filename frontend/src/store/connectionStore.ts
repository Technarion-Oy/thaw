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
            }
          : null,
      }),
    },
  ),
);
