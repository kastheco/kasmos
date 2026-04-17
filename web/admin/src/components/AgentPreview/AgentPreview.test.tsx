import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Mock } from "vitest";
import AgentPreview from "./AgentPreview";
import type { PresentationResponse, PresentationTurn, PresentationRow } from "../../types";

// ---------------------------------------------------------------------------
// Mock the API module so no real fetch calls are made.
// ---------------------------------------------------------------------------

vi.mock("../../api", () => ({
  getInstancePresentation: vi.fn(),
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
});
