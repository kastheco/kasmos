import { useCallback, useEffect, useRef, useState } from "react";

export type AutoRefreshState<T> = {
  data: T | null;
  loading: boolean;
  error: string | null;
  lastUpdatedAt: Date | null;
  isRefreshing: boolean;
  refresh: () => Promise<void>;
};

export function useAutoRefresh<T>(
  load: () => Promise<T>,
  deps: React.DependencyList,
  intervalMs = 10000,
): AutoRefreshState<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<Date | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const mountedRef = useRef(true);
  const hasDataRef = useRef(false);
  const inFlightRef = useRef(false);
  const generationRef = useRef(0);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const refresh = useCallback(async () => {
    if (!mountedRef.current) return;
    if (inFlightRef.current) return; // skip if previous refresh still running
    inFlightRef.current = true;

    const gen = generationRef.current;
    const hadData = hasDataRef.current;
    if (hadData) {
      setIsRefreshing(true);
    } else {
      setLoading(true);
    }

    try {
      const result = await load();
      if (!mountedRef.current || gen !== generationRef.current) return;
      setData(result);
      setError(null);
      setLastUpdatedAt(new Date());
      hasDataRef.current = true;
    } catch (err) {
      if (!mountedRef.current || gen !== generationRef.current) return;
      const msg = err instanceof Error ? err.message : "failed to load";
      if (hadData) {
        // keep previous data and lastUpdatedAt, only surface the error
        setError(msg);
      } else {
        setError(msg);
        setData(null);
      }
    } finally {
      if (gen === generationRef.current) {
        inFlightRef.current = false;
        if (mountedRef.current) {
          setLoading(false);
          setIsRefreshing(false);
        }
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  // Initial load and re-load when deps change.  Bump the generation so any
  // in-flight request from the previous deps is discarded on completion, and
  // reset the in-flight guard so the new fetch proceeds immediately.
  useEffect(() => {
    generationRef.current++;
    inFlightRef.current = false;
    hasDataRef.current = false;
    setLoading(true);
    setError(null);
    void refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refresh]);

  // Background polling
  useEffect(() => {
    if (intervalMs <= 0) return;

    const id = setInterval(() => {
      if (document.hidden) return;
      void refresh();
    }, intervalMs);

    return () => clearInterval(id);
  }, [refresh, intervalMs]);

  return { data, loading, error, lastUpdatedAt, isRefreshing, refresh };
}
