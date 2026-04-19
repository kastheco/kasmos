import { describe, it, expect } from "vitest";
import type { PresentationTurn, PresentationRow } from "../../types";
import { buildDisplayTurns } from "./TurnTimeline";

function row(kind: PresentationRow["kind"], text: string): PresentationRow {
  return {
    kind,
    text,
    timestamp: null,
    tool_name: kind === "tool" ? "bash" : "",
    is_error: false,
  };
}

function turn(rows: PresentationRow[]): PresentationTurn {
  return {
    id: "turn-1",
    number: 1,
    started_at: new Date("2026-01-01T00:00:00Z"),
    completed_at: new Date("2026-01-01T00:00:10Z"),
    interrupted: false,
    tool_count: rows.filter((r) => r.kind === "tool").length,
    rows,
  };
}

describe("buildDisplayTurns", () => {
  it("preserves original turns for non-codex programs", () => {
    const turns = [turn([row("tool", "check file"), row("prose", "done")])];
    expect(buildDisplayTurns(turns, "claude")).toBe(turns);
    expect(buildDisplayTurns(turns, undefined)).toBe(turns);
  });

  it("splits codex turns on new prose bursts and reattaches preamble activity below the prose", () => {
    const turns = [
      turn([
        row("tool", "preflight tool"),
        row("result", "preflight result"),
        row("response", ""),
        row("prose", "first response"),
        row("tool", "tool after first response"),
        row("result", "result after first response"),
        row("prose", "second response"),
        row("tool", "tool after second response"),
      ]),
    ];

    const display = buildDisplayTurns(turns, "codex");

    expect(display).toHaveLength(2);
    expect(display[0].rows.map((r) => r.text)).toEqual([
      "first response",
      "preflight tool",
      "preflight result",
      "tool after first response",
      "result after first response",
    ]);
    expect(display[0].tool_count).toBe(2);
    expect(display[1].rows.map((r) => r.text)).toEqual([
      "second response",
      "tool after second response",
    ]);
    expect(display[1].tool_count).toBe(1);
  });
});
