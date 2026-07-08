import { useState, useEffect, useCallback } from 'react';

interface UseLoadableOptions {
  /** Auto-refresh interval in ms. 0 or undefined disables. */
  intervalMs?: number;
  /** Error message shown on failure via App.useApp().message. */
  errorMessage?: string;
}

/**
 * Generic hook for fetching data with loading state, error handling, and optional auto-refresh.
 *
 * Usage:
 *   const { data, loading, reload } = useLoadable(() => labopsApi.devices(), { intervalMs: 10000 });
 */
export function useLoadable<T>(
  fetcher: () => Promise<T>,
  options: UseLoadableOptions = {},
) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await fetcher();
      setData(result);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)));
      if (options.errorMessage) {
        // We don't import App.useApp here to keep the hook generic.
        // Pages can handle error display via the error state.
        console.error(`${options.errorMessage}:`, err);
      }
    } finally {
      setLoading(false);
    }
  }, [fetcher, options.errorMessage]);

  useEffect(() => {
    load();
    if (options.intervalMs && options.intervalMs > 0) {
      const timer = window.setInterval(load, options.intervalMs);
      return () => window.clearInterval(timer);
    }
  }, [load, options.intervalMs]);

  return { data, loading, error, reload: load };
}

/**
 * Hook that fetches multiple independent requests, handling partial failures.
 * Returns an array of results in the same order; failed entries are null.
 */
export function useLoadableAll<T extends unknown[]>(
  fetchers: { [K in keyof T]: () => Promise<T[K]> },
  options: UseLoadableOptions = {},
) {
  type Result = { [K in keyof T]: T[K] | null };
  const [data, setData] = useState<Result | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const results = await Promise.all(
        fetchers.map((f) =>
          f().catch((err) => {
            console.error('useLoadableAll partial failure:', err);
            return null;
          }),
        ),
      );
      setData(results as Result);
    } finally {
      setLoading(false);
    }
  }, [fetchers]);

  useEffect(() => {
    load();
    if (options.intervalMs && options.intervalMs > 0) {
      const timer = window.setInterval(load, options.intervalMs);
      return () => window.clearInterval(timer);
    }
  }, [load, options.intervalMs]);

  return { data, loading, reload: load };
}
