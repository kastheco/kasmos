import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import Monitor from "../Monitor";
import type { MonitorSnapshot } from "../types";

const snapshot: MonitorSnapshot = {
  schema_version: 2, generated_at: "2026-07-10T12:00:00Z", project: "kasmos", daemon_running: true,
  lifecycle: { planning: 1, ready: 2, implementing: 3, reviewing: 1, verifying: 0, total: 7 },
  active_agents: [{ task: "monitor", role: "coder", active: true, wave: 2, task_number: 3, branch: "feature", worktree: "worktree" }],
  attention: [{ task: "monitor", kind: "needs_decision", detail: "review feedback" }], truncated: {}, projects: ["kasmos"],
  tasks: [{ filename: "monitor", status: "implementing", active_wave: 2, total_waves: 3, subtasks_done: 2, subtasks_total: 3 }],
  focus: { filename: "monitor", waves: [{ wave: 2, active: true, tasks: [{ number: 3, title: "widget", status: "running" }] }], readiness: { status: "reviewing", has_review_feedback: true } },
  events: [{ at: "2026-07-10T12:00:00Z", kind: "signal", message: "coder started" }],
};

function host(overrides: Partial<NonNullable<Window["openai"]>> = {}) {
  const value = { toolOutput: snapshot, displayMode: "inline" as const, theme: "dark" as const, callTool: vi.fn().mockResolvedValue({ structuredContent: snapshot }), setWidgetState: vi.fn(), sendFollowUpMessage: vi.fn(), requestDisplayMode: vi.fn(), ...overrides };
  window.openai = value;
  return value;
}

afterEach(() => { cleanup(); vi.useRealTimers(); delete window.openai; });
beforeEach(() => { Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" }); });

describe("kasmos monitor host contract", () => {
  it("first-paint invariant: renders tool output with zero bridge round trips", () => { const openai = host(); render(<Monitor />); expect(screen.getAllByRole("listitem").some((item) => item.textContent === "3 implementing")).toBe(true); expect(openai.callTool).not.toHaveBeenCalled(); });
  it("display-mode invariant: host globals decide layout", async () => {
    const openai = host({ displayMode: "fullscreen" }); render(<Monitor />); expect(screen.getByRole("heading", { name: "waves" })).toBeTruthy(); expect(screen.getByRole("heading", { name: "events" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: /pin/ })); expect(openai.requestDisplayMode).toHaveBeenCalledWith({ mode: "pip" }); expect(screen.getByRole("heading", { name: "events" })).toBeTruthy();
    act(() => window.dispatchEvent(new CustomEvent("openai:set_globals", { detail: { globals: { displayMode: "pip" } } }))); expect(screen.queryByRole("heading", { name: "events" })).toBeNull();
  });
  it("polling-hygiene invariant: polls, single-flights, pauses, and preserves last good state", async () => {
    vi.useFakeTimers(); let resolve!: (value: unknown) => void; const pending = new Promise((done) => { resolve = done; }); const openai = host({ callTool: vi.fn().mockReturnValue(pending) }); const view = render(<Monitor />);
    await act(async () => { await vi.advanceTimersByTimeAsync(6000); }); expect(openai.callTool).toHaveBeenCalledTimes(1); expect(screen.getAllByRole("listitem").some((item) => item.textContent === "3 implementing")).toBe(true);
    resolve({ structuredContent: snapshot }); await act(async () => { await pending; }); Object.defineProperty(document, "visibilityState", { configurable: true, value: "hidden" }); fireEvent(document, new Event("visibilitychange")); await act(async () => { await vi.advanceTimersByTimeAsync(9000); }); expect(openai.callTool).toHaveBeenCalledTimes(1); view.unmount();
  });
  it("authority-boundary invariant: actions send messages and only polling uses open_monitor", () => { const openai = host({ displayMode: "fullscreen" }); render(<Monitor />); screen.getAllByRole("button").filter((button) => /start wave|look at blocker|approve review/.test(button.textContent ?? "")).forEach(fireEvent.click); expect(openai.sendFollowUpMessage).toHaveBeenCalledTimes(3); expect(openai.callTool).not.toHaveBeenCalled(); });
  it("no-network invariant: a full poll cycle never fetches", async () => { vi.useFakeTimers(); const fetchSpy = vi.fn(() => { throw new Error("network forbidden"); }); vi.stubGlobal("fetch", fetchSpy); host(); render(<Monitor />); await act(async () => { await vi.advanceTimersByTimeAsync(3000); }); expect(fetchSpy).not.toHaveBeenCalled(); vi.unstubAllGlobals(); });
  it("degraded-host invariant: missing optional methods still renders", () => { host({ requestDisplayMode: undefined, sendFollowUpMessage: undefined }); render(<Monitor />); expect(screen.getByText("kasmos monitor")).toBeTruthy(); expect(screen.queryByRole("button", { name: /pin/ })).toBeNull(); });
  it("a11y invariant: stale status is live and actions are native buttons", () => { host({ displayMode: "fullscreen" }); render(<Monitor />); expect(document.querySelector('[aria-live="polite"]')).toBeTruthy(); expect(screen.getByRole("button", { name: "look at blocker" }).tabIndex).toBe(0); });
});
