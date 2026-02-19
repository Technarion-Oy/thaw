import { create } from "zustand";

export interface QueryResult {
  columns: string[];
  rows: unknown[][];
  rowsAffected: number;
}

interface QueryState {
  sql: string;
  selectedSql: string;
  result: QueryResult | null;
  isRunning: boolean;
  error: string | null;
  setSql: (sql: string) => void;
  setSelectedSql: (selected: string) => void;
  setResult: (result: QueryResult) => void;
  setRunning: (running: boolean) => void;
  setError: (error: string | null) => void;
}

export const useQueryStore = create<QueryState>((set) => ({
  sql: "SELECT CURRENT_USER(), CURRENT_WAREHOUSE(), CURRENT_DATABASE();",
  selectedSql: "",
  result: null,
  isRunning: false,
  error: null,
  setSql: (sql) => set({ sql }),
  setSelectedSql: (selectedSql) => set({ selectedSql }),
  setResult: (result) => set({ result, error: null }),
  setRunning: (isRunning) => set({ isRunning }),
  setError: (error) => set({ error, isRunning: false }),
}));
