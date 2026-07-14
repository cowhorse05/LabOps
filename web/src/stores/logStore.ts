import { create } from 'zustand';

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

export interface LogEntry {
  id: number;
  timestamp: string;
  level: LogLevel;
  source: string;
  message: string;
  detail?: string;
}

const MAX_LOGS = 500;

interface LogState {
  logs: LogEntry[];
  push: (entry: Omit<LogEntry, 'id' | 'timestamp'>) => void;
  clear: () => void;
}

let nextId = 1;

export const useLogStore = create<LogState>((set) => ({
  logs: [],
  push: (entry) =>
    set((state) => {
      const log: LogEntry = {
        ...entry,
        id: nextId++,
        timestamp: new Date().toISOString(),
      };
      const logs = [...state.logs, log];
      if (logs.length > MAX_LOGS) {
        return { logs: logs.slice(logs.length - MAX_LOGS) };
      }
      return { logs };
    }),
  clear: () => set({ logs: [] }),
}));
