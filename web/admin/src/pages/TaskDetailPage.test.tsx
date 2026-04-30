import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import TaskDetailPage from "./TaskDetailPage";
import type { SubtaskEntry, TaskEntry } from "../types";

const mockRefresh = vi.fn().mockResolvedValue(undefined);

vi.mock("react-router", () => ({
  useParams: () => ({ filename: "audit-architect-decisions" }),
  useNavigate: () => vi.fn(),
}));

vi.mock("../hooks/useProject", () => ({
  useProject: () => ({ project: "kasmos", projectSearch: "?project=kasmos" }),
}));

vi.mock("../hooks/useToast", () => ({
  useToast: () => ({ show: vi.fn() }),
}));

vi.mock("../components/LastUpdated", () => ({
  default: () => <span data-testid="last-updated" />,
}));

vi.mock("../components/TaskActionsMenu", () => ({
  default: () => <button>actions</button>,
}));

vi.mock("../components/PlanEditor", () => ({
  default: () => <div data-testid="plan-editor">editor</div>,
}));

const baseTask: TaskEntry = {
  filename: "audit-architect-decisions",
  status: "implementing",
  goal: "show architect decisions",
  topic: "admin",
};

let task: TaskEntry = { ...baseTask };

const subtasks: SubtaskEntry[] = [
  { task_number: 5, title: "wire task detail tab", status: "running" },
];

vi.mock("../hooks/useAutoRefresh", () => ({
  useAutoRefresh: (_load: unknown, deps: unknown[]) => {
    const isArchitectHook = deps.length === 3;
    if (isArchitectHook) {
      return {
        data: {
          available: true,
          decision_audit: {
            schema_version: 1,
            plan_file: "audit-architect-decisions",
            project: "kasmos",
            created_at: "2026-04-24T10:00:00Z",
            summary: "architect compared the planner draft with the baseline",
            planner_summary: "planner changed the admin page",
            baseline_summary: "baseline kept the detail view read-only",
            baseline_source: "parallel_cache",
            final_decision: "show a read-only architect decisions tab",
            differences: [
              {
                area: "task detail",
                planner_proposal: "replace the plan view",
                baseline_proposal: "preserve the plan view",
                final_decision: "add a second tab",
                rationale: "editing should stay scoped to the plan",
                related_files: ["web/admin/src/pages/TaskDetailPage.tsx"],
                task_numbers: [5],
              },
            ],
          },
        },
        loading: false,
        error: null,
        lastUpdatedAt: null,
        isRefreshing: false,
        refresh: mockRefresh,
      };
    }
    return {
      data: { task, content: "# plan\n\ncurrent task markdown", subtasks },
      loading: false,
      error: null,
      lastUpdatedAt: null,
      isRefreshing: false,
      refresh: mockRefresh,
    };
  },
}));

describe("TaskDetailPage", () => {
  beforeEach(() => {
    mockRefresh.mockClear();
    task = { ...baseTask };
  });

  it("defaults to the plan tab and preserves plan editing", async () => {
    render(<TaskDetailPage />);

    expect(screen.getByRole("button", { name: "plan" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByText("current task markdown")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "edit" }));

    expect(await screen.findByTestId("plan-editor")).toBeTruthy();
    expect(screen.getByRole("button", { name: "cancel edit" })).toBeTruthy();
  });

  it("shows architect decisions in a read-only tab and exits edit mode", async () => {
    render(<TaskDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "edit" }));
    expect(await screen.findByTestId("plan-editor")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "architect decisions" }));

    expect(screen.queryByTestId("plan-editor")).toBeNull();
    expect(screen.queryByRole("button", { name: "cancel edit" })).toBeNull();
    expect(screen.getByText("architect compared the planner draft with the baseline")).toBeTruthy();
    expect(screen.getByText("show a read-only architect decisions tab")).toBeTruthy();
    expect(screen.getByText("web/admin/src/pages/TaskDetailPage.tsx", { exact: false })).toBeTruthy();
  });

  it("renders linear identifier as external link when set", () => {
    task = {
      ...baseTask,
      linear_identifier: "KAS-42",
      linear_url: "https://linear.app/kasmos/issue/KAS-42/link-web-admin",
    };

    render(<TaskDetailPage />);

    expect(screen.getByText("linear")).toBeTruthy();
    const link = screen.getByRole("link", { name: "KAS-42" });
    expect(link.getAttribute("href")).toBe(
      "https://linear.app/kasmos/issue/KAS-42/link-web-admin",
    );
    expect(link.getAttribute("target")).toBe("_blank");
    expect(link.getAttribute("rel")).toBe("noreferrer");
  });

  it("omits linear row when empty", () => {
    render(<TaskDetailPage />);

    expect(screen.queryByText("linear")).toBeNull();
    expect(screen.queryByRole("link", { name: /KAS-/ })).toBeNull();
  });
});
