import { useEffect, useMemo, useReducer, useRef } from "react";
import { openAiHost, useOpenAiGlobal } from "./openai";
import type { DisplayMode, MonitorSnapshot } from "./types";

export const MONITOR_CONTRACT_VERSION = 1;
export const LIVE_STATUS_SCHEMA_VERSION = 2;
export const REQUIRED_HOST_CAPABILITIES = ["contractVersion", "displayMode", "visibility", "theme", "refresh", "subscribe"] as const;
export const OPTIONAL_HOST_CAPABILITIES = ["saveState", "setBadge", "requestDisplayMode", "sendPrompt"] as const;

export type PaneVisibility = "expanded" | "collapsed" | "hidden";
export interface MonitorScope { project?: string; task?: string }
export interface MonitorBadge {
  level: "idle" | "running" | "attention" | "offline";
  running_agents: number; blocked: number; implementing: number; reviewing: number;
  project?: string; task?: string;
}
export interface KasmosMonitorHost {
  contractVersion: number;
  snapshot?: MonitorSnapshot;
  input?: MonitorScope;
  state?: MonitorScope;
  displayMode: DisplayMode;
  visibility: PaneVisibility;
  theme: "light" | "dark";
  maxHeight?: number;
  refresh(scope: MonitorScope): Promise<MonitorSnapshot>;
  subscribe(listener: () => void): () => void;
  saveState?(scope: MonitorScope): void | Promise<void>;
  setBadge?(badge: MonitorBadge): void;
  requestDisplayMode?(mode: DisplayMode): void | Promise<void>;
  sendPrompt?(prompt: string): void | Promise<void>;
}
declare global { interface Window { kasmosMonitorHost?: KasmosMonitorHost } }

export function embeddedHost(host: KasmosMonitorHost): KasmosMonitorHost {
  if (host.contractVersion !== MONITOR_CONTRACT_VERSION) return incompatibleHost();
  return host;
}

function incompatibleHost(): KasmosMonitorHost {
  return { contractVersion: -1, displayMode: "inline", visibility: "expanded", theme: "dark", refresh: async () => { throw new Error("monitor host contract version mismatch"); }, subscribe: () => () => {} };
}

export function inertHost(): KasmosMonitorHost {
  return { contractVersion: MONITOR_CONTRACT_VERSION, displayMode: "inline", visibility: "expanded", theme: "dark", refresh: async () => { throw new Error("monitor host unavailable"); }, subscribe: () => () => {} };
}

export function useMonitorHost(): KasmosMonitorHost {
  const embedded = useRef<KasmosMonitorHost | null>(null);
  if (embedded.current === null) embedded.current = window.kasmosMonitorHost ?? incompatibleHost();
  const globals = useOpenAiGlobal();
  const [, bump] = useReducer((n: number) => n + 1, 0);
  const injected = window.kasmosMonitorHost;
  useEffect(() => {
    if (!injected?.subscribe) return;
    return injected.subscribe(() => bump());
  }, [injected]);
  return useMemo(() => injected ? embeddedHost(injected) : window.openai ? openAiHost(globals) : inertHost(), [injected, globals]);
}
