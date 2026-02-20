import { create } from "zustand";
import { ExecuteQuery } from "../../wailsjs/go/main/App";

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
  setRunning: (isRunning: boolean) => void;
  setError: (error: string | null) => void;
  // Sets the editor SQL, clears selection, and immediately runs the query.
  executeWith: (sql: string) => Promise<void>;
}

export const useQueryStore = create<QueryState>((set, get) => ({
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
  executeWith: async (sql) => {
    set({ sql, selectedSql: "", isRunning: true, error: null });
    try {
      const res = await ExecuteQuery(sql);
      get().setResult(res);
    } catch (e) {
      get().setError(String(e));
    } finally {
      set({ isRunning: false });
    }
  },
}));
