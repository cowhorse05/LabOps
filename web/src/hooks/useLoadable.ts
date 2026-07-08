import { useState, useEffect, useCallback, useRef } from 'react';

interface UseLoadableOptions {
  /** Auto-refresh interval in ms. 0 or undefined disables. */
  intervalMs?: number;
  /** Error message shown on failure via App.useApp().message. */
  errorMessage?: string;
  /** Callback invoked when an error occurs, e.g. to show a Toast. */
  onError?: (error: Error) => void;
}

/**
 * Generic hook for fetching data with loading state, error handling, and optional auto-refresh.
 *
 * Usage:
 *   const { data, loading, reload } = useLoadable(() => labopsApi.devices(), { intervalMs: 10000 });
 *
 * The fetcher is stored in a ref to avoid re-triggering the effect on every parent render.
 */
export function useLoadable<T>(
  fetcher: () => Promise<T>,
  options: UseLoadableOptions = {},
) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  // Store fetcher and options in refs so load() has stable identity.
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;
  const onErrorRef = useRef(options.onError);
  onErrorRef.current = options.onError;
  const errorMsgRef = useRef(options.errorMessage);
  errorMsgRef.current = options.errorMessage;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await fetcherRef.current();
      setData(result);
      setError(null);
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      setError(error);
      onErrorRef.current?.(error);
      if (errorMsgRef.current) {
        console.error(`${errorMsgRef.current}:`, err);
      }
    } finally {
      setLoading(false);
    }
  }, []); // stable identity — never changes

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

  const onErrorRef = useRef(options.onError);
  onErrorRef.current = options.onError;

  // Store fetchers in a ref so load() has stable identity across renders.
  // DashboardPage passes an inline array literal, which would otherwise cause
  // the useCallback to recreate load() on every render, triggering an infinite
  // effect cleanup/setup loop.
  const fetchersRef = useRef(fetchers);
  fetchersRef.current = fetchers;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const results = await Promise.all(
        fetchersRef.current.map((f) =>
          f().catch((err) => {
            console.error('useLoadableAll partial failure:', err);
            const error = err instanceof Error ? err : new Error(String(err));
            onErrorRef.current?.(error);
            return null;
          }),
        ),
      );
      setData(results as Result);
    } finally {
      setLoading(false);
    }
  }, []); // stable — uses ref for fetchers

  useEffect(() => {
    load();
    if (options.intervalMs && options.intervalMs > 0) {
      const timer = window.setInterval(load, options.intervalMs);
      return () => window.clearInterval(timer);
    }
  }, [load, options.intervalMs]);

  return { data, loading, reload: load };
}
