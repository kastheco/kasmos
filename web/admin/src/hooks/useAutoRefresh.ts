import { useCallback, useEffect, useRef, useState } from "react";

export type AutoRefreshState<T> = {
  data: T | null;
  loading: boolean;
  error: string | null;
  lastUpdatedAt: Date | null;
  isRefreshing: boolean;
  refresh: () => Promise<void>;
};

// Exported for unit testing. Resets all internal hook state when input deps
// change so the next render shows null data instead of stale content captured
// against the previous deps. Without clearing data here, consumers like the
// admin instances page would briefly render a previous instance's pane under
// the newly selected row before the next poll resolves.
export function resetAutoRefreshStateForDepsChange<T>(
  setData: (v: T | null) => void,
  setLoading: (v: boolean) => void,
  setError: (v: string | null) => void,
  generationRef: { current: number },
  inFlightRef: { current: boolean },
  hasDataRef: { current: boolean },
): void {
  generationRef.current++;
  inFlightRef.current = false;
  hasDataRef.current = false;
  setData(null);
  setLoading(true);
  setError(null);
}

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

  // Initial load and re-load when deps change. Clears any data held against
  // the previous deps so the next render cannot show stale content under the
  // new selection, then bumps generation and triggers a fresh fetch.
  useEffect(() => {
    resetAutoRefreshStateForDepsChange<T>(
      setData,
      setLoading,
      setError,
      generationRef,
      inFlightRef,
      hasDataRef,
    );
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
