import { useCallback, useEffect, useRef, useState } from "react";
import { LIVE_STATUS_SCHEMA_VERSION, MONITOR_CONTRACT_VERSION, type KasmosMonitorHost } from "./host";
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
  useEffect(() => { const update = (raw: Event) => { const detail = (raw as CustomEvent<{ globals?: Partial<OpenAiGlobals> }>).detail; const patch = detail?.globals ?? detail ?? {}; setGlobals((previous) => ({ ...previous, ...patch })); }; window.addEventListener("openai:set_globals", update); return () => window.removeEventListener("openai:set_globals", update); }, []);
  return globals;
}
export function snapshotFrom(result: unknown): MonitorSnapshot | undefined {
  if (!result || typeof result !== "object") return undefined;
  const value = result as { structuredContent?: unknown; structured_content?: unknown };
  const candidate = value.structuredContent ?? value.structured_content ?? result;
  if (!candidate || typeof candidate !== "object") return undefined;
  const snapshot = candidate as Partial<MonitorSnapshot>;
  if (snapshot.schema_version !== 2 || typeof snapshot.project !== "string" || typeof snapshot.daemon_running !== "boolean" || !snapshot.lifecycle || typeof snapshot.lifecycle !== "object" || !Array.isArray(snapshot.active_agents) || !Array.isArray(snapshot.attention) || !snapshot.truncated || typeof snapshot.truncated !== "object") return undefined;
  return snapshot as MonitorSnapshot;
}
export function snapshotRejection(result: unknown): "invalid" | "incompatible-schema" {
  const value = result as { structuredContent?: unknown; structured_content?: unknown };
  const candidate = (value?.structuredContent ?? value?.structured_content ?? result) as Partial<MonitorSnapshot> | undefined;
  const version = candidate && typeof candidate === "object" ? candidate.schema_version : undefined;
  return typeof version === "number" && version !== LIVE_STATUS_SCHEMA_VERSION ? "incompatible-schema" : "invalid";
}
export function openAiHost(globals: OpenAiGlobals): KasmosMonitorHost {
  const host: KasmosMonitorHost = {
    contractVersion: MONITOR_CONTRACT_VERSION, snapshot: snapshotFrom(globals.toolOutput), input: globals.toolInput, state: globals.widgetState,
    displayMode: globals.displayMode ?? "inline", visibility: document.visibilityState === "visible" ? "expanded" : "hidden", theme: globals.theme ?? "dark", maxHeight: globals.maxHeight,
    refresh: async (scope) => { const result = await window.openai?.callTool?.("refresh_monitor", { project: scope.project, task: scope.task }); return result as MonitorSnapshot; },
    subscribe: (listener) => { window.addEventListener("openai:set_globals", listener); return () => window.removeEventListener("openai:set_globals", listener); },
  };
  if (globals.setWidgetState) host.saveState = (scope) => window.openai?.setWidgetState?.(scope);
  if (globals.requestDisplayMode) host.requestDisplayMode = (mode) => window.openai?.requestDisplayMode?.({ mode });
  if (globals.sendFollowUpMessage) host.sendPrompt = (prompt) => window.openai?.sendFollowUpMessage?.({ prompt });
  return host;
}

export type MonitorPhase = "loading" | "ready" | "offline" | "incompatible";
export function useMonitorSnapshot(host: KasmosMonitorHost, project?: string, task?: string) {
  const [snapshot, setSnapshot] = useState<MonitorSnapshot | undefined>(host.snapshot);
  const [phase, setPhase] = useState<MonitorPhase>(host.contractVersion === MONITOR_CONTRACT_VERSION ? (host.snapshot ? "ready" : "loading") : "incompatible");
  const [stale, setStale] = useState(false);
  const snapshotRef = useRef(snapshot); snapshotRef.current = snapshot;
  const phaseRef = useRef(phase); phaseRef.current = phase;
  const scopeKey = `${project ?? ""}\u0000${task ?? ""}`; const latestScope = useRef(scopeKey); latestScope.current = scopeKey;
  const inFlight = useRef(false); const queued = useRef(false); const refreshRef = useRef<() => Promise<void>>(async () => {}); const coldStarted = useRef(false); const failures = useRef(0); const timer = useRef<number | undefined>(undefined); const mounted = useRef(true);
  const effectiveVisibility = document.visibilityState !== "visible" ? "hidden" : host.visibility;
  const previousEffectiveVisibility = useRef(effectiveVisibility);
  const baseDelay = effectiveVisibility === "collapsed" ? 15000 : host.displayMode === "inline" ? 3000 : 2000;
  useEffect(() => { if (host.snapshot) { setSnapshot(host.snapshot); setPhase("ready"); } }, [host.snapshot]);
  useEffect(() => () => { mounted.current = false; }, []);
  const refresh = useCallback(async () => {
    const requestScope = scopeKey; if (inFlight.current) { queued.current = true; return; } if (document.visibilityState !== "visible" || host.visibility === "hidden" || phaseRef.current === "incompatible") return;
    inFlight.current = true;
    try {
      const raw = await host.refresh({ project, task }); const next = snapshotFrom(raw);
      if (!next) { if (!snapshotRef.current && snapshotRejection(raw) === "incompatible-schema") { setPhase("incompatible"); return; } throw new Error("invalid snapshot"); }
      if (mounted.current && latestScope.current === requestScope) { setSnapshot(next); failures.current = 0; setStale(false); setPhase("ready"); }
    } catch { if (mounted.current && latestScope.current === requestScope) { failures.current += 1; setStale(true); if (!snapshotRef.current) setPhase("offline"); } }
    finally { inFlight.current = false; if (queued.current) { queued.current = false; void refreshRef.current(); } }
  }, [host, project, scopeKey, task]);
  refreshRef.current = refresh;
  useEffect(() => { if (!coldStarted.current && !host.snapshot && phase === "loading") { coldStarted.current = true; void refresh(); } }, [host.snapshot, phase, refresh]);
  useEffect(() => {
    let cancelled = false;
    const schedule = () => { window.clearTimeout(timer.current); if (cancelled || effectiveVisibility === "hidden" || phase === "incompatible") return; const delay = Math.min(baseDelay * 2 ** failures.current, 30000); timer.current = window.setTimeout(async () => { await refresh(); schedule(); }, delay); };
    const visibility = () => { if (document.visibilityState === "visible" && host.visibility !== "hidden") void refresh(); schedule(); };
    const becameExpanded = previousEffectiveVisibility.current !== "expanded" && effectiveVisibility === "expanded";
    previousEffectiveVisibility.current = effectiveVisibility;
    if (becameExpanded) void refresh();
    schedule(); document.addEventListener("visibilitychange", visibility);
    return () => { cancelled = true; window.clearTimeout(timer.current); document.removeEventListener("visibilitychange", visibility); };
  }, [baseDelay, effectiveVisibility, host.visibility, phase, refresh]);
  const visibleSnapshot = !project || snapshot?.project === project ? snapshot : undefined;
  return { snapshot: visibleSnapshot, stale, phase, refresh };
}
