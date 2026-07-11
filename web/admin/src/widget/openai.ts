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
  const candidate = value.structuredContent ?? value.structured_content ?? result;
  if (!candidate || typeof candidate !== "object") return undefined;
  const snapshot = candidate as Partial<MonitorSnapshot>;
  if (snapshot.schema_version !== 2 || typeof snapshot.project !== "string" || typeof snapshot.daemon_running !== "boolean" || !snapshot.lifecycle || typeof snapshot.lifecycle !== "object" || !Array.isArray(snapshot.active_agents) || !Array.isArray(snapshot.attention) || !snapshot.truncated || typeof snapshot.truncated !== "object") return undefined;
  return snapshot as MonitorSnapshot;
}

export function useMonitorSnapshot(globals: OpenAiGlobals, project?: string, task?: string) {
  const [snapshot, setSnapshot] = useState<MonitorSnapshot | undefined>(() => snapshotFrom(globals.toolOutput));
  const [stale, setStale] = useState(false);
  const scopeKey = `${project ?? ""}\u0000${task ?? ""}`;
  const latestScope = useRef(scopeKey);
  latestScope.current = scopeKey;
  const inFlight = useRef<string | undefined>(undefined);
  const failures = useRef(0);
  const timer = useRef<number | undefined>(undefined);
  const mounted = useRef(true);
  const baseDelay = globals.displayMode === "pip" || globals.displayMode === "fullscreen" ? 2000 : 3000;

  useEffect(() => { const next = snapshotFrom(globals.toolOutput); if (next) setSnapshot(next); }, [globals.toolOutput]);
  useEffect(() => () => { mounted.current = false; }, []);
  const refresh = useCallback(async () => {
    const requestScope = scopeKey;
    if (inFlight.current === requestScope || document.visibilityState !== "visible" || !window.openai?.callTool) return;
    inFlight.current = requestScope;
    try {
      const next = snapshotFrom(await window.openai.callTool("open_monitor", { project, task }));
      if (!next) throw new Error("open_monitor returned an invalid snapshot");
      if (mounted.current && latestScope.current === requestScope) {
        setSnapshot(next);
        failures.current = 0;
        setStale(false);
      }
    } catch {
      if (mounted.current && latestScope.current === requestScope) {
        failures.current += 1;
        setStale(true);
      }
    } finally { if (inFlight.current === requestScope) inFlight.current = undefined; }
  }, [project, scopeKey, task]);

  useEffect(() => {
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
    return () => { window.clearInterval(timer.current); document.removeEventListener("visibilitychange", visibility); };
  }, [baseDelay, refresh]);
  const visibleSnapshot = !project || snapshot?.project === project ? snapshot : undefined;
  return { snapshot: visibleSnapshot, stale, refresh };
}
