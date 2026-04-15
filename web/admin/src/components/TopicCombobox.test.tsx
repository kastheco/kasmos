import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import TopicCombobox, { type TopicSelection } from "./TopicCombobox";
import type { TopicEntry } from "../types";

function makeTopics(names: string[]): TopicEntry[] {
  return names.map((name) => ({ name, created_at: "2026-01-01T00:00:00Z" }));
}

const DEFAULT_TOPICS = makeTopics(["backend", "frontend", "docs", "infra"]);

function renderCombobox(
  topics: TopicEntry[] = DEFAULT_TOPICS,
  value = "",
  onChange = vi.fn(),
) {
  const result = render(
    <TopicCombobox
      topics={topics}
      value={value}
      onChange={onChange}
      id="topic-combo"
      placeholder="pick a topic"
    />,
  );
  const input = screen.getByRole("combobox");
  return { input, onChange, result };
}

// ---- filtering ---------------------------------------------------------------

describe("filtering", () => {
  it("shows all topics when input is empty and combobox is focused", () => {
    const { input } = renderCombobox();
    fireEvent.focus(input);
    const options = screen.getAllByRole("option");
    // All 4 default topics should be visible (no create row since input is empty)
    expect(options).toHaveLength(4);
  });

  it("filters existing topics by case-insensitive substring", () => {
    const { input } = renderCombobox();
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "end" } });
    // "frontend" matches "end", "backend" matches "end"
    const options = screen.getAllByRole("option");
    const names = options.map((o) => o.textContent ?? "");
    expect(names.some((n) => n.includes("frontend"))).toBe(true);
    expect(names.some((n) => n.includes("backend"))).toBe(true);
    expect(names.some((n) => n.includes("docs"))).toBe(false);
  });

  it("is case-insensitive when filtering", () => {
    const { input } = renderCombobox();
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "FRONT" } });
    const options = screen.getAllByRole("option");
    const names = options.map((o) => o.textContent ?? "");
    expect(names.some((n) => n.includes("frontend"))).toBe(true);
  });
});

// ---- create row semantics ----------------------------------------------------

describe("create row", () => {
  it("shows create row when typed value has no exact match even if partial matches exist", () => {
    const { input } = renderCombobox();
    fireEvent.focus(input);
    // "front" is NOT an exact match even though "frontend" exists
    fireEvent.change(input, { target: { value: "front" } });
    const options = screen.getAllByRole("option");
    const createOption = options.find((o) =>
      o.textContent?.includes('create'),
    );
    expect(createOption).toBeTruthy();
  });

  it("does not show create row when typed value exactly matches an existing topic (case-insensitive)", () => {
    const { input } = renderCombobox();
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "Frontend" } });
    const options = screen.getAllByRole("option");
    const createOption = options.find((o) =>
      o.textContent?.includes('create'),
    );
    expect(createOption).toBeUndefined();
  });

  it("does not show create row when input is empty", () => {
    const { input } = renderCombobox();
    fireEvent.focus(input);
    // no change — input stays empty
    const options = screen.getAllByRole("option");
    const createOption = options.find((o) =>
      o.textContent?.includes('create'),
    );
    expect(createOption).toBeUndefined();
  });

  it("shows create row for fully novel topic name", () => {
    const { input } = renderCombobox();
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "foo" } });
    const options = screen.getAllByRole("option");
    const createOption = options.find((o) =>
      o.textContent?.includes('"foo"') || o.textContent?.includes('\u201cfoo\u201d'),
    );
    expect(createOption).toBeTruthy();
  });
});

// ---- keyboard navigation: Enter ---------------------------------------------

describe("Enter key", () => {
  it("selects highlighted existing row on Enter", () => {
    const onChange = vi.fn();
    const { input } = renderCombobox(DEFAULT_TOPICS, "", onChange);
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "doc" } });
    // ArrowDown to highlight "docs"
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ value: "docs", isNew: false }),
    );
  });

  it("selects the create row on Enter when it is highlighted", () => {
    const onChange = vi.fn();
    const { input } = renderCombobox(DEFAULT_TOPICS, "", onChange);
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "newfeature" } });
    // filtered list will be empty (no substring match); create row is at idx 0
    // ArrowDown once to highlight create row
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "Enter" });
    const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0] as TopicSelection;
    expect(lastCall.isNew).toBe(true);
    expect(lastCall.value).toBe("newfeature");
  });
});

// ---- keyboard navigation: Escape --------------------------------------------

describe("Escape key", () => {
  it("closes the list on Escape", () => {
    const { input } = renderCombobox();
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "doc" } });
    expect(screen.queryAllByRole("option").length).toBeGreaterThan(0);
    fireEvent.keyDown(input, { key: "Escape" });
    expect(screen.queryAllByRole("option")).toHaveLength(0);
  });

  it("preserves the last typed value after Escape", () => {
    const { input } = renderCombobox();
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "doc" } });
    fireEvent.keyDown(input, { key: "Escape" });
    expect((input as HTMLInputElement).value).toBe("doc");
  });

  it("does not bubble Escape to document listeners when the listbox is open", () => {
    const documentHandler = vi.fn();
    document.addEventListener("keydown", documentHandler);
    try {
      const { input } = renderCombobox();
      fireEvent.focus(input);
      fireEvent.change(input, { target: { value: "doc" } });
      expect(screen.queryAllByRole("option").length).toBeGreaterThan(0);
      documentHandler.mockClear();

      fireEvent.keyDown(input, { key: "Escape" });

      // Listbox closed AND no document-level Escape fired — this is what
      // keeps a containing dialog open when the user only meant to dismiss
      // the combobox popover.
      expect(screen.queryAllByRole("option")).toHaveLength(0);
      const sawEscape = documentHandler.mock.calls.some(
        ([evt]) => (evt as KeyboardEvent).key === "Escape",
      );
      expect(sawEscape).toBe(false);
    } finally {
      document.removeEventListener("keydown", documentHandler);
    }
  });

  it("lets Escape bubble to document listeners when the listbox is already closed", () => {
    const documentHandler = vi.fn();
    document.addEventListener("keydown", documentHandler);
    try {
      const { input } = renderCombobox();
      fireEvent.focus(input);
      fireEvent.change(input, { target: { value: "doc" } });
      // First Escape closes the listbox and is swallowed
      fireEvent.keyDown(input, { key: "Escape" });
      expect(screen.queryAllByRole("option")).toHaveLength(0);
      documentHandler.mockClear();

      // Second Escape should bubble so the containing dialog can close.
      fireEvent.keyDown(input, { key: "Escape" });

      const sawEscape = documentHandler.mock.calls.some(
        ([evt]) => (evt as KeyboardEvent).key === "Escape",
      );
      expect(sawEscape).toBe(true);
    } finally {
      document.removeEventListener("keydown", documentHandler);
    }
  });
});

// ---- outside click ----------------------------------------------------------

describe("outside click", () => {
  it("closes the list when clicking outside the combobox", () => {
    const { input } = renderCombobox();
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "doc" } });
    expect(screen.queryAllByRole("option").length).toBeGreaterThan(0);

    // Click outside
    fireEvent.mouseDown(document.body);

    expect(screen.queryAllByRole("option")).toHaveLength(0);
  });
});

// ---- empty input ------------------------------------------------------------

describe("empty input", () => {
  it("emits { value: '', isNew: false } when the input is cleared", () => {
    const onChange = vi.fn();
    const { input } = renderCombobox(DEFAULT_TOPICS, "", onChange);
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "doc" } });
    onChange.mockClear();
    fireEvent.change(input, { target: { value: "" } });
    expect(onChange).toHaveBeenCalledWith({ value: "", isNew: false });
  });
});

// ---- typing triggers onChange -----------------------------------------------

describe("onChange while typing", () => {
  it("calls onChange on every keystroke with correct isNew flag", () => {
    const onChange = vi.fn();
    const { input } = renderCombobox(DEFAULT_TOPICS, "", onChange);
    fireEvent.focus(input);
    // "infra" exists → isNew should be false
    fireEvent.change(input, { target: { value: "infra" } });
    const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0] as TopicSelection;
    expect(lastCall.isNew).toBe(false);
    expect(lastCall.value).toBe("infra");
  });

  it("calls onChange with isNew=true when no exact match exists", () => {
    const onChange = vi.fn();
    const { input } = renderCombobox(DEFAULT_TOPICS, "", onChange);
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "mobile" } });
    const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0] as TopicSelection;
    expect(lastCall.isNew).toBe(true);
  });

  it("canonicalizes case-insensitive exact matches to the stored topic name", () => {
    const onChange = vi.fn();
    const { input } = renderCombobox(DEFAULT_TOPICS, "", onChange);
    fireEvent.focus(input);
    // Existing topic is "frontend"; typing "Frontend" must emit "frontend"
    // so the dialog submits the canonical key rather than forking topic
    // identity in the backend.
    fireEvent.change(input, { target: { value: "Frontend" } });
    const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0] as TopicSelection;
    expect(lastCall.isNew).toBe(false);
    expect(lastCall.value).toBe("frontend");
  });
});
