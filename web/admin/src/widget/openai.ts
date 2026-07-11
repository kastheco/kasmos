import { useCallback, useEffect, useRef, useState } from "react";
import type { DisplayMode, MonitorSnapshot } from "./types";

export interface OpenAiGlobals {
  toolInput?: { project?: string; task?: string };
  toolOutput?: MonitorSnapshot;
  widgetState?: { project?: string; task?: string };
  displayMode?: DisplayMode;
  maxHeight?: number;
  theme?: "light" | "dark";
  setWidgetState?: (state: { project?: string; task?: string }) => Promise<void> | void;
  callTool?: (name: string, args: { project?: string; task?: string }) => Promise<unknown>;
  sendFollowUpMessage?: (message: { prompt: string }) => Promise<void> | void;
  requestDisplayMode?: (request: { mode: DisplayMode }) => Promise<void> | void;
}

declare global { interface Window { openai?: OpenAiGlobals } }

function currentGlobals(): OpenAiGlobals { return window.openai ?? {}; }

export function useOpenAiGlobal(): OpenAiGlobals {
  const [globals, setGlobals] = useState<OpenAiGlobals>(currentGlobals);
  useEffect(() => {
    const update = (raw: Event) => {
      const detail = (raw as CustomEvent<{ globals?: Partial<OpenAiGlobals> }>).detail;
      const patch = detail?.globals ?? detail ?? {};
      setGlobals((previous) => ({ ...previous, ...patch }));
    };
    window.addEventListener("openai:set_globals", update);
    return () => window.removeEventListener("openai:set_globals", update);
  }, []);
  return globals;
}

function snapshotFrom(result: unknown): MonitorSnapshot | undefined {
  if (!result || typeof result !== "object") return undefined;
  const value = result as { structuredContent?: unknown; structured_content?: unknown };
  return (value.structuredContent ?? value.structured_content ?? result) as MonitorSnapshot;
}

export function useMonitorSnapshot(globals: OpenAiGlobals, project?: string, task?: string) {
  const [snapshot, setSnapshot] = useState<MonitorSnapshot | undefined>(() => globals.toolOutput);
  const [stale, setStale] = useState(false);
  const inFlight = useRef(false);
  const failures = useRef(0);
  const timer = useRef<number | undefined>(undefined);
  const mounted = useRef(true);
  const baseDelay = globals.displayMode === "pip" || globals.displayMode === "fullscreen" ? 2000 : 3000;

  useEffect(() => { if (globals.toolOutput) setSnapshot(globals.toolOutput); }, [globals.toolOutput]);
  const refresh = useCallback(async () => {
    if (inFlight.current || document.visibilityState !== "visible" || !window.openai?.callTool) return;
    inFlight.current = true;
    try {
      const next = snapshotFrom(await window.openai.callTool("open_monitor", { project, task }));
      if (next && mounted.current) setSnapshot(next);
      failures.current = 0;
      if (mounted.current) setStale(false);
    } catch {
      failures.current += 1;
      if (mounted.current) setStale(true);
    } finally { inFlight.current = false; }
  }, [project, task]);

  useEffect(() => {
    mounted.current = true;
    const schedule = () => {
      window.clearInterval(timer.current);
      const delay = Math.min(baseDelay * 2 ** failures.current, 30000);
      timer.current = window.setInterval(async () => { await refresh(); schedule(); }, delay);
    };
    const visibility = () => {
      if (document.visibilityState === "visible") { void refresh(); schedule(); }
      else window.clearInterval(timer.current);
    };
    schedule();
    document.addEventListener("visibilitychange", visibility);
    return () => { mounted.current = false; window.clearInterval(timer.current); document.removeEventListener("visibilitychange", visibility); };
  }, [baseDelay, refresh]);
  return { snapshot, stale, refresh };
}
