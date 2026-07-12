import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import Monitor from "../Monitor";
import { deriveBadge } from "../badge";
import { MONITOR_CONTRACT_VERSION, type KasmosMonitorHost } from "../host";
import type { MonitorSnapshot } from "../types";

const snapshot: MonitorSnapshot = { schema_version: 2, generated_at: "now", project: "kasmos", daemon_running: true, lifecycle: { planning: 0, ready: 0, implementing: 1, reviewing: 0, verifying: 0, total: 1 }, active_agents: [], attention: [], truncated: {}, tasks: [] };
function embedded(overrides: Partial<KasmosMonitorHost> = {}): KasmosMonitorHost {
  return { contractVersion: MONITOR_CONTRACT_VERSION, displayMode: "sidebar", visibility: "expanded", theme: "dark", refresh: vi.fn().mockResolvedValue(snapshot), subscribe: () => () => {}, ...overrides };
}
beforeEach(() => { delete window.openai; Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" }); });
afterEach(() => { cleanup(); vi.useRealTimers(); delete window.kasmosMonitorHost; });

describe("monitor host adapter", () => {
  it("host-agnostic invariant", async () => { const setBadge = vi.fn(); const host = embedded({ setBadge }); window.kasmosMonitorHost = host; render(<Monitor />); await act(async () => {}); expect(host.refresh).toHaveBeenCalledTimes(1); expect(screen.getByText("kasmos monitor")).toBeTruthy(); expect(setBadge).toHaveBeenCalledTimes(1); });
  it("cold-mount invariant", async () => { const refresh = vi.fn().mockRejectedValue(new Error("offline")); window.kasmosMonitorHost = embedded({ refresh }); render(<Monitor />); await act(async () => {}); expect(screen.getByRole("button", { name: "retry" })).toBeTruthy(); screen.getByRole("button", { name: "retry" }).click(); await act(async () => {}); expect(refresh).toHaveBeenCalledTimes(2); });
  it("schema-fail-closed invariant", async () => { vi.useFakeTimers(); const refresh = vi.fn().mockResolvedValue({ schema_version: 3 }); window.kasmosMonitorHost = embedded({ refresh }); render(<Monitor />); await act(async () => {}); expect(screen.getByText(/version mismatch/)).toBeTruthy(); await act(async () => vi.advanceTimersByTimeAsync(60000)); expect(refresh).toHaveBeenCalledTimes(1); });
  it("badge invariant", () => { expect(deriveBadge(undefined, {}).level).toBe("offline"); expect(deriveBadge(snapshot, {}).level).toBe("idle"); expect(deriveBadge({ ...snapshot, attention: [{ task: "x", kind: "blocked" }] }, {}).level).toBe("attention"); expect(deriveBadge({ ...snapshot, active_agents: [{ task: "x", role: "coder", active: true }] }, {}).level).toBe("running"); });
});
