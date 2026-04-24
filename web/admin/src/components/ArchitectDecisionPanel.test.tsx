import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import ArchitectDecisionPanel from "./ArchitectDecisionPanel";
import type { ArchitectDecisionAuditResponse } from "../types";

const HAPPY_RESPONSE: ArchitectDecisionAuditResponse = {
  available: true,
  final_markdown: "**ship** the architect version",
  architect_baseline_markdown: "## baseline\n\n- keep existing api shape",
  baseline_reason: "captured before architect rewrite",
  timestamps: {
    architect_meta_at: "2026-04-24T10:00:00Z",
    baseline_created_at: "2026-04-24T09:30:00Z",
    decision_audit_created_at: "2026-04-24T10:05:00Z",
  },
  decision_audit: {
    schema_version: 1,
    plan_file: "audit-architect-decisions",
    project: "kasmos",
    created_at: "2026-04-24T10:05:00Z",
    baseline_source: "parallel_cache",
    summary: "architect narrowed the implementation boundary",
    planner_summary: "planner split api and ui separately",
    baseline_summary: "baseline used the task detail page contract",
    final_decision: "use a read-only hq api and lazy admin panel",
    differences: [
      {
        area: "admin ui",
        scope: "task detail",
        planner_proposal: "render markdown only",
        architect_baseline: "add a structured operator view",
        final_decision: "ship the structured panel",
        rationale: "operators need the comparison at a glance",
        related_files: ["web/admin/src/components/ArchitectDecisionPanel.tsx"],
        task_numbers: [4],
      },
    ],
  },
};

describe("ArchitectDecisionPanel", () => {
  it("renders the available audit with summaries, differences, markdown, and timestamps", () => {
    render(<ArchitectDecisionPanel response={HAPPY_RESPONSE} />);

    expect(
      screen.getByRole("heading", { name: "architect decisions" }),
    ).toBeTruthy();
    expect(screen.getByText("architect narrowed the implementation boundary")).toBeTruthy();
    expect(screen.getByText("planner split api and ui separately")).toBeTruthy();
    expect(screen.getByText("baseline used the task detail page contract")).toBeTruthy();
    expect(screen.getByText("use a read-only hq api and lazy admin panel")).toBeTruthy();
    expect(screen.getByText("ship")).toBeTruthy();
    expect(screen.getByRole("table")).toBeTruthy();
    expect(screen.getByText("admin ui")).toBeTruthy();
    expect(screen.getByText("operators need the comparison at a glance")).toBeTruthy();
    expect(screen.getByText("architect baseline", { selector: "summary" })).toBeTruthy();
    expect(screen.getByText("decision audit created")).toBeTruthy();
  });

  it("renders an unavailable state from a known reason", () => {
    render(
      <ArchitectDecisionPanel
        response={{ available: false, reason: "architect_not_run" }}
      />,
    );

    expect(screen.getByText("architect has not run for this task")).toBeTruthy();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("renders an inline error state", () => {
    render(
      <ArchitectDecisionPanel
        response={null}
        error={new Error("cache read failed")}
      />,
    );

    expect(
      screen.getByText("could not load architect decisions: cache read failed"),
    ).toBeTruthy();
  });

  it("omits the baseline details when baseline markdown is absent", () => {
    const response: ArchitectDecisionAuditResponse = {
      ...HAPPY_RESPONSE,
      architect_baseline_markdown: undefined,
    };

    render(<ArchitectDecisionPanel response={response} />);

    expect(screen.queryByText("architect baseline", { selector: "summary" })).toBeNull();
    expect(screen.getByText("ship the structured panel")).toBeTruthy();
  });
});
