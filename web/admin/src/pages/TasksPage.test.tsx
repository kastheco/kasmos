import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import TasksPage from "./TasksPage";
import type { NewTaskDialogResult } from "../components/NewTaskDialog";
import type { TaskEntry } from "../types";

// ---------------------------------------------------------------------------
// Module mocks
// ---------------------------------------------------------------------------

// useProject
const mockUseProject = vi.fn(() => ({
  project: "my-project",
  projectSearch: "?project=my-project",
}));
vi.mock("../hooks/useProject", () => ({
  useProject: () => mockUseProject(),
}));

// useToast
const mockShowToast = vi.fn();
vi.mock("../hooks/useToast", () => ({
  useToast: () => ({ show: mockShowToast }),
}));

// useAutoRefresh — two calls per render (tasks, topics); alternate on call count
const mockRefreshTasks = vi.fn().mockResolvedValue(undefined);
const mockRefreshTopics = vi.fn().mockResolvedValue(undefined);
let autoRefreshCallCount = 0;
vi.mock("../hooks/useAutoRefresh", () => ({
  useAutoRefresh: () => {
    autoRefreshCallCount++;
    const isTopics = autoRefreshCallCount % 2 === 0;
    return {
      data: [],
      loading: false,
      error: null,
      lastUpdatedAt: null,
      isRefreshing: false,
      refresh: isTopics ? mockRefreshTopics : mockRefreshTasks,
    };
  },
}));

// LastUpdated
vi.mock("../components/LastUpdated", () => ({
  default: () => <span data-testid="last-updated" />,
}));

// TaskActionsMenu
vi.mock("../components/TaskActionsMenu", () => ({
  default: () => <button data-testid="task-actions-menu">actions</button>,
}));

// NewTaskDialog — capture callbacks for testing
let capturedOnClose: (() => void) | null = null;
let capturedOnCreated: ((result: NewTaskDialogResult) => Promise<void> | void) | null =
  null;
let capturedOpen = false;
vi.mock("../components/NewTaskDialog", () => ({
  default: (props: {
    open: boolean;
    project: string;
    topics: unknown[];
    onClose(): void;
    onCreated(result: NewTaskDialogResult): Promise<void> | void;
  }) => {
    capturedOpen = props.open;
    capturedOnClose = props.onClose;
    capturedOnCreated = props.onCreated;
    return props.open ? (
      <div data-testid="new-task-dialog">dialog</div>
    ) : null;
  },
}));

// react-router Link stub
vi.mock("react-router", () => ({
  Link: ({ children, to }: { children: React.ReactNode; to: unknown }) => (
    <a href={String(to)}>{children}</a>
  ),
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeTask(overrides: Partial<TaskEntry> = {}): TaskEntry {
  return {
    filename: "my-task",
    status: "ready",
    goal: "do something",
    topic: "infra",
    branch: "feat/my-task",
    created_at: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("TasksPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    autoRefreshCallCount = 0;
    capturedOnClose = null;
    capturedOnCreated = null;
    capturedOpen = false;
    mockUseProject.mockReturnValue({
      project: "my-project",
      projectSearch: "?project=my-project",
    });
  });

  it("renders the new task button", () => {
    render(<TasksPage />);
    expect(
      screen.getByRole("button", { name: "new task" }),
    ).toBeTruthy();
  });

  it("opens the dialog when new task button is clicked", () => {
    render(<TasksPage />);
    const btn = screen.getByRole("button", { name: "new task" });
    fireEvent.click(btn);
    expect(screen.getByTestId("new-task-dialog")).toBeTruthy();
  });

  it("disables the button when project is empty", () => {
    mockUseProject.mockReturnValue({ project: "", projectSearch: "" });
    render(<TasksPage />);
    const btn = screen.getByRole("button", { name: "new task" }) as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
  });

  it("handleCreated refreshes both tasks and topics queries", async () => {
    render(<TasksPage />);
    fireEvent.click(screen.getByRole("button", { name: "new task" }));

    const result: NewTaskDialogResult = {
      task: makeTask({ filename: "new-task" }),
      plannerRequested: false,
    };

    await act(async () => {
      await capturedOnCreated!(result);
    });

    expect(mockRefreshTasks).toHaveBeenCalled();
    expect(mockRefreshTopics).toHaveBeenCalled();
  });

  it("shows success toast: planner queued when plannerRequested and no warning", async () => {
    render(<TasksPage />);
    fireEvent.click(screen.getByRole("button", { name: "new task" }));

    const result: NewTaskDialogResult = {
      task: makeTask({ filename: "queued-task" }),
      plannerRequested: true,
    };

    await act(async () => {
      await capturedOnCreated!(result);
    });

    expect(mockShowToast).toHaveBeenCalledWith(
      "task 'queued-task' created, planner queued",
    );
  });

  it("shows success toast: ready when !plannerRequested and no warning", async () => {
    render(<TasksPage />);
    fireEvent.click(screen.getByRole("button", { name: "new task" }));

    const result: NewTaskDialogResult = {
      task: makeTask({ filename: "ready-task" }),
      plannerRequested: false,
    };

    await act(async () => {
      await capturedOnCreated!(result);
    });

    expect(mockShowToast).toHaveBeenCalledWith(
      "task 'ready-task' created (ready)",
    );
  });

  it("shows error toast when content save failed", async () => {
    render(<TasksPage />);
    fireEvent.click(screen.getByRole("button", { name: "new task" }));

    const result: NewTaskDialogResult = {
      task: makeTask({ filename: "partial-task" }),
      plannerRequested: false,
      warning: { stage: "content", error: new Error("disk full") },
    };

    await act(async () => {
      await capturedOnCreated!(result);
    });

    expect(mockShowToast).toHaveBeenCalledWith(
      "task 'partial-task' created; content save failed: disk full",
      { kind: "error" },
    );
  });

  it("shows error toast when planner queue failed", async () => {
    render(<TasksPage />);
    fireEvent.click(screen.getByRole("button", { name: "new task" }));

    const result: NewTaskDialogResult = {
      task: makeTask({ filename: "plan-fail-task" }),
      plannerRequested: true,
      warning: { stage: "plan_start", error: new Error("gateway timeout") },
    };

    await act(async () => {
      await capturedOnCreated!(result);
    });

    expect(mockShowToast).toHaveBeenCalledWith(
      "task 'plan-fail-task' created; planner queue failed: gateway timeout",
      { kind: "error" },
    );
  });

  it("dialog closes after handleCreated", async () => {
    render(<TasksPage />);
    fireEvent.click(screen.getByRole("button", { name: "new task" }));
    expect(screen.getByTestId("new-task-dialog")).toBeTruthy();

    const result: NewTaskDialogResult = {
      task: makeTask({ filename: "done-task" }),
      plannerRequested: false,
    };

    await act(async () => {
      await capturedOnCreated!(result);
    });

    expect(screen.queryByTestId("new-task-dialog")).toBeNull();
  });

  it("dialog closes via onClose and focus path is triggered", () => {
    render(<TasksPage />);
    const btn = screen.getByRole("button", { name: "new task" });
    fireEvent.click(btn);
    expect(screen.getByTestId("new-task-dialog")).toBeTruthy();

    act(() => {
      capturedOnClose!();
    });

    expect(screen.queryByTestId("new-task-dialog")).toBeNull();
  });

  it("closes the dialog when project changes", async () => {
    const { rerender } = render(<TasksPage />);
    fireEvent.click(screen.getByRole("button", { name: "new task" }));
    expect(screen.getByTestId("new-task-dialog")).toBeTruthy();

    mockUseProject.mockReturnValue({
      project: "other-project",
      projectSearch: "?project=other-project",
    });

    await act(async () => {
      rerender(<TasksPage />);
    });

    await waitFor(() => {
      expect(screen.queryByTestId("new-task-dialog")).toBeNull();
    });
  });
});
