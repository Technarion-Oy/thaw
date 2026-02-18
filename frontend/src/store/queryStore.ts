import { create } from "zustand";

export interface QueryResult {
  columns: string[];
  rows: unknown[][];
  rowsAffected: number;
}

interface QueryState {
  sql: string;
  result: QueryResult | null;
  isRunning: boolean;
  error: string | null;
  setSql: (sql: string) => void;
  setResult: (result: QueryResult) => void;
  setRunning: (running: boolean) => void;
  setError: (error: string | null) => void;
}

export const useQueryStore = create<QueryState>((set) => ({
  sql: "SELECT CURRENT_USER(), CURRENT_WAREHOUSE(), CURRENT_DATABASE();",
  result: null,
  isRunning: false,
  error: null,
  setSql: (sql) => set({ sql }),
  setResult: (result) => set({ result, error: null }),
  setRunning: (isRunning) => set({ isRunning }),
  setError: (error) => set({ error, isRunning: false }),
}));
