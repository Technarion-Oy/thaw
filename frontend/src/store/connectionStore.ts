import { create } from "zustand";

export interface ConnectionParams {
  account: string;
  user: string;
  password: string;
  role: string;
  warehouse: string;
  database: string;
  schema: string;
}

interface ConnectionState {
  isConnected: boolean;
  params: ConnectionParams | null;
  setConnected: (params: ConnectionParams) => void;
  disconnect: () => void;
}

export const useConnectionStore = create<ConnectionState>((set) => ({
  isConnected: false,
  params: null,
  setConnected: (params) => set({ isConnected: true, params }),
  disconnect: () => set({ isConnected: false, params: null }),
}));
