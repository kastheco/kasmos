import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { Mock } from "vitest";
import AgentPreview from "./AgentPreview";
import type { PresentationResponse, PresentationTurn, PresentationRow } from "../../types";
import { FILTER_STORAGE_KEY } from "./FilterToolbar";

// ---------------------------------------------------------------------------
// localStorage stub — jsdom does not expose a fully working localStorage in
// this test setup, so we provide an in-memory replacement.
// ---------------------------------------------------------------------------

const localStorageStore: Record<string, string> = {};
const localStorageMock = {
  getItem: (key: string) => localStorageStore[key] ?? null,
  setItem: (key: string, value: string) => { localStorageStore[key] = value; },
  removeItem: (key: string) => { delete localStorageStore[key]; },
  clear: () => { Object.keys(localStorageStore).forEach((k) => delete localStorageStore[k]); },
};
vi.stubGlobal("localStorage", localStorageMock);

// ---------------------------------------------------------------------------
// Mock the API module so no real fetch calls are made.
// ---------------------------------------------------------------------------

vi.mock("../../api", () => ({
  getInstancePresentation: vi.fn(),
  sendInstancePermission: vi.fn(),
}));

import * as api from "../../api";

// ---------------------------------------------------------------------------
// Test data helpers
// ---------------------------------------------------------------------------

function makeRow(overrides: Partial<PresentationRow> = {}): PresentationRow {
  return {
    kind: "prose",
    text: "default text",
    timestamp: null,
    tool_name: "",
    is_error: false,
    ...overrides,
  };
}

function makeTurn(overrides: Partial<PresentationTurn> = {}): PresentationTurn {
  return {
    id: "turn-1",
    number: 1,
    started_at: new Date("2026-01-01T10:00:00Z"),
    completed_at: new Date("2026-01-01T10:00:05Z"),
    interrupted: false,
    tool_count: 0,
    rows: [],
    ...overrides,
  };
}

function makePresentation(
  overrides: Partial<PresentationResponse> = {},
): PresentationResponse {
  return {
    supported: true,
    turns: null,
    captured_at: new Date("2026-01-01T10:00:00Z"),
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("AgentPreview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorageMock.removeItem(FILTER_STORAGE_KEY);
  });

  afterEach(() => {
    localStorageMock.removeItem(FILTER_STORAGE_KEY);
  });

  it("renders completed turn with tool, result, response, and prose rows", async () => {
    const turn = makeTurn({
      tool_count: 1,
      rows: [
        makeRow({ kind: "tool", text: "reading file", tool_name: "Read" }),
        makeRow({ kind: "result", text: "file contents here" }),
        makeRow({ kind: "response", text: "" }),
        makeRow({ kind: "prose", text: "I found the content you needed." }),
      ],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("reading file")).toBeTruthy();
      expect(screen.getByText("file contents here")).toBeTruthy();
      // response row renders as a divider with the label "response"
      expect(screen.getByText("response")).toBeTruthy();
      expect(screen.getByText("I found the content you needed.")).toBeTruthy();
    });
  });

  it("renders a new response divider when prose resumes after tool rows", async () => {
    const turn = makeTurn({
      tool_count: 1,
      rows: [
        makeRow({ kind: "response", text: "" }),
        makeRow({ kind: "prose", text: "using architect first" }),
        makeRow({ kind: "tool", text: "reading file", tool_name: "Read" }),
        makeRow({ kind: "result", text: "file contents here" }),
        makeRow({ kind: "response", text: "" }),
        makeRow({ kind: "prose", text: "next i'm reading overlay code" }),
      ],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("using architect first")).toBeTruthy();
      expect(screen.getByText("reading file")).toBeTruthy();
      expect(screen.getByText("next i'm reading overlay code")).toBeTruthy();
    });

    const responseDividers = screen.getAllByText("response");
    expect(responseDividers).toHaveLength(2);
  });

  it("renders running turn with • running pill", async () => {
    const turn = makeTurn({
      completed_at: null,
      interrupted: false,
      rows: [],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("• running")).toBeTruthy();
    });
  });

  it("renders interrupted turn with gold status row (no extra badge)", async () => {
    const turn = makeTurn({
      completed_at: null,
      interrupted: true,
      rows: [
        makeRow({ kind: "status", text: "interrupted by user" }),
      ],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("interrupted by user")).toBeTruthy();
    });
    // The status row is rendered once — no duplicate interruption badge.
    const statusRows = screen
      .getAllByText("interrupted by user")
      .filter((el) => el.tagName !== "BODY");
    expect(statusRows.length).toBe(1);
  });

  it("shows waiting message when supported=true and turns is null", async () => {
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ supported: true, turns: null }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText(/waiting for agent output/i)).toBeTruthy();
    });
  });

  it("shows waiting message when supported=true and turns is empty", async () => {
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ supported: true, turns: [] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText(/waiting for agent output/i)).toBeTruthy();
    });
  });

  it("shows unsupported message when supported=false", async () => {
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ supported: false, turns: null }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(
        screen.getByText(/structured preview is not supported/i),
      ).toBeTruthy();
    });
  });

  it("calls onError with the error message when the API throws", async () => {
    const onError = vi.fn();
    (api.getInstancePresentation as Mock).mockRejectedValue(
      new Error("network error"),
    );

    render(
      <AgentPreview
        project="my-project"
        title="agent-1"
        onError={onError}
      />,
    );

    await waitFor(() => {
      expect(onError).toHaveBeenCalledWith("network error");
    });
  });

  it("calls onError with null when error clears after a successful fetch", async () => {
    const onError = vi.fn();
    const mock = api.getInstancePresentation as Mock;
    mock.mockRejectedValueOnce(new Error("transient error"));
    mock.mockResolvedValue(makePresentation({ turns: [] }));

    render(
      <AgentPreview
        project="my-project"
        title="agent-1"
        onError={onError}
      />,
    );

    // First call fails → onError("transient error")
    await waitFor(() => {
      expect(onError).toHaveBeenCalledWith("transient error");
    });
  });

  it("calls onFollowStateChange with true on initial mount", async () => {
    const onFollowStateChange = vi.fn();
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [] }),
    );

    render(
      <AgentPreview
        project="my-project"
        title="agent-1"
        onFollowStateChange={onFollowStateChange}
      />,
    );

    await waitFor(() => {
      expect(onFollowStateChange).toHaveBeenCalledWith(true);
    });
  });

  it("renders result error row (is_error=true) distinct from success result", async () => {
    const turn = makeTurn({
      rows: [
        makeRow({ kind: "result", text: "command failed", is_error: true }),
      ],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("command failed")).toBeTruthy();
    });
    // Error result has data-kind="result" and data-error="true"
    const el = screen.getByText("command failed").closest("[data-kind='result']");
    expect(el).toBeTruthy();
    expect(el?.getAttribute("data-error")).toBe("true");
  });

  it("does not show a loading spinner that persists once data arrives", async () => {
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ supported: true, turns: [] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText(/waiting for agent output/i)).toBeTruthy();
    });

    // Loading placeholder should no longer be present once data arrives.
    expect(screen.queryByText(/loading/i)).toBeNull();
  });

  // -------------------------------------------------------------------------
  // Filter toolbar
  // -------------------------------------------------------------------------

  it("renders the filter toolbar with three toggles", async () => {
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ supported: true, turns: [] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("hide thinking")).toBeTruthy();
      expect(screen.getByText("hide tools")).toBeTruthy();
      expect(screen.getByText("hide system")).toBeTruthy();
    });
  });

  it("persists filter state to localStorage when a toggle is clicked", async () => {
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ supported: true, turns: [] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("hide tools")).toBeTruthy();
    });

    fireEvent.click(screen.getByText("hide tools"));

    const stored = JSON.parse(localStorage.getItem(FILTER_STORAGE_KEY) ?? "{}");
    expect(stored.hideTools).toBe(true);
  });

  it("restores filter state from localStorage on mount", async () => {
    localStorage.setItem(
      FILTER_STORAGE_KEY,
      JSON.stringify({ hideTools: true, hideThinking: false, hideSystem: false }),
    );

    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ supported: true, turns: [] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      const btn = screen.getByText("hide tools");
      // aria-pressed reflects the active state
      expect(btn.getAttribute("aria-pressed")).toBe("true");
    });
  });

  it("hides tool rows when hideTools filter is active", async () => {
    localStorage.setItem(
      FILTER_STORAGE_KEY,
      JSON.stringify({ hideTools: true, hideThinking: false, hideSystem: false }),
    );

    const turn = makeTurn({
      tool_count: 1,
      rows: [
        makeRow({ kind: "tool", text: "tool-call-output", tool_name: "Bash" }),
        makeRow({ kind: "prose", text: "prose output" }),
      ],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("prose output")).toBeTruthy();
    });

    // Tool row must be hidden
    expect(screen.queryByText("tool-call-output")).toBeNull();
  });

  it("never hides permission rows even when hideTools is active", async () => {
    localStorage.setItem(
      FILTER_STORAGE_KEY,
      JSON.stringify({ hideTools: true, hideThinking: true, hideSystem: true }),
    );

    const turn = makeTurn({
      rows: [
        makeRow({ kind: "permission", text: "allow file access?" }),
      ],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    // sendInstancePermission must be in the mock so PermissionCard renders
    (api.sendInstancePermission as Mock).mockResolvedValue(undefined);

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("allow file access?")).toBeTruthy();
    });
  });

  // -------------------------------------------------------------------------
  // Collapse
  // -------------------------------------------------------------------------

  it("collapse button hides tool rows but keeps prose and response visible", async () => {
    const turn = makeTurn({
      tool_count: 1,
      rows: [
        makeRow({ kind: "tool", text: "tool-text", tool_name: "Bash" }),
        makeRow({ kind: "response", text: "" }),
        makeRow({ kind: "prose", text: "prose-text" }),
      ],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("tool-text")).toBeTruthy();
    });

    fireEvent.click(screen.getByRole("button", { name: /collapse turn/i }));

    await waitFor(() => {
      // Tool row hidden
      expect(screen.queryByText("tool-text")).toBeNull();
      // Prose and response divider still visible
      expect(screen.getByText("prose-text")).toBeTruthy();
      expect(screen.getByText("response")).toBeTruthy();
    });
  });

  it("collapse keeps permission rows visible", async () => {
    const turn = makeTurn({
      rows: [
        makeRow({ kind: "thinking", text: "thinking-text" }),
        makeRow({ kind: "permission", text: "allow?" }),
      ],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    (api.sendInstancePermission as Mock).mockResolvedValue(undefined);

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("thinking-text")).toBeTruthy();
    });

    fireEvent.click(screen.getByRole("button", { name: /collapse turn/i }));

    await waitFor(() => {
      expect(screen.queryByText("thinking-text")).toBeNull();
      expect(screen.getByText("allow?")).toBeTruthy();
    });
  });

  it("expand button restores hidden rows", async () => {
    const turn = makeTurn({
      tool_count: 1,
      rows: [
        makeRow({ kind: "tool", text: "tool-text", tool_name: "Bash" }),
        makeRow({ kind: "prose", text: "prose-text" }),
      ],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("tool-text")).toBeTruthy();
    });

    fireEvent.click(screen.getByRole("button", { name: /collapse turn/i }));
    await waitFor(() => {
      expect(screen.queryByText("tool-text")).toBeNull();
    });

    fireEvent.click(screen.getByRole("button", { name: /expand turn/i }));
    await waitFor(() => {
      expect(screen.getByText("tool-text")).toBeTruthy();
    });
  });

  // -------------------------------------------------------------------------
  // Copy fallback
  // -------------------------------------------------------------------------

  it("shows copy fallback textarea when clipboard is unavailable", async () => {
    // Remove clipboard API to simulate unavailable context
    const originalClipboard = Object.getOwnPropertyDescriptor(
      navigator,
      "clipboard",
    );
    Object.defineProperty(navigator, "clipboard", {
      value: undefined,
      configurable: true,
    });

    const turn = makeTurn({
      rows: [makeRow({ kind: "prose", text: "some output" })],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText("some output")).toBeTruthy();
    });

    fireEvent.click(screen.getByRole("button", { name: /copy turn/i }));

    await waitFor(() => {
      expect(screen.getByText(/copy manually/i)).toBeTruthy();
    });

    // Restore clipboard
    if (originalClipboard) {
      Object.defineProperty(navigator, "clipboard", originalClipboard);
    }
  });

  // -------------------------------------------------------------------------
  // Markdown rendering
  // -------------------------------------------------------------------------

  it("renders prose rows with markdown bold", async () => {
    const turn = makeTurn({
      rows: [makeRow({ kind: "prose", text: "This is **bold** text." })],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      const strong = document.querySelector("strong");
      expect(strong).toBeTruthy();
      expect(strong?.textContent).toBe("bold");
    });
  });

  it("does not render raw HTML in prose rows", async () => {
    const turn = makeTurn({
      rows: [
        makeRow({ kind: "prose", text: "<script>alert('xss')</script> safe" }),
      ],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText(/safe/)).toBeTruthy();
    });
    expect(document.querySelector("script")).toBeNull();
  });

  // -------------------------------------------------------------------------
  // Permission card integration
  // -------------------------------------------------------------------------

  it("renders interactive permission card for first unresolved permission row", async () => {
    const turn = makeTurn({
      rows: [makeRow({ kind: "permission", text: "allow write?" })],
    });
    (api.getInstancePresentation as Mock).mockResolvedValue(
      makePresentation({ turns: [turn] }),
    );
    (api.sendInstancePermission as Mock).mockResolvedValue(undefined);

    render(<AgentPreview project="my-project" title="agent-1" />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "allow" })).toBeTruthy();
      expect(screen.getByRole("button", { name: "deny" })).toBeTruthy();
      expect(screen.getByRole("button", { name: "always" })).toBeTruthy();
    });
  });
});
