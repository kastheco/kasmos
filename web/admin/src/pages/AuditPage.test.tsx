import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AuditPage from "./AuditPage";
import type { AuditEvent } from "../types";

const mockUseAutoRefresh = vi.fn();
const mockUseProject = vi.fn();

vi.mock("../hooks/useAutoRefresh", () => ({
  useAutoRefresh: (...args: unknown[]) => mockUseAutoRefresh(...args),
}));

vi.mock("../hooks/useProject", () => ({
  useProject: () => mockUseProject(),
}));

vi.mock("../components/LastUpdated", () => ({
  default: () => <div data-testid="last-updated" />,
}));

vi.mock("../components/Skeleton", () => ({
  default: () => <div data-testid="skeleton" />,
}));

function makeAuditEvent(overrides: Partial<AuditEvent>): AuditEvent {
  return {
    id: 1,
    kind: "agent_killed",
    timestamp: "2026-04-24T12:00:00Z",
    project: "kasmos",
    task_file: "plan.md",
    instance_title: "coder-1",
    agent_type: "coder",
    wave_number: 1,
    task_number: 1,
    message: "killed instance",
    detail: "",
    level: "info",
    ...overrides,
  };
}

describe("AuditPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseProject.mockReturnValue({
      project: "kasmos",
      projectSearch: "?project=kasmos",
    });
  });

  it("coalesces adjacent kill rows and keeps raw grouped details expandable", () => {
    const preserve = makeAuditEvent({
      id: 1,
      message: "agent stopped (branch preserved)",
      detail: JSON.stringify({
        action: "kill_instance",
        cleanup: false,
        branch_preserved: true,
        group_key: "agent_killed:coder-1",
      }),
    });
    const cleanup = makeAuditEvent({
      id: 2,
      message: "killed and removed instance",
      detail: JSON.stringify({
        action: "kill_and_remove_instance",
        cleanup: true,
        branch_preserved: false,
        group_key: "agent_killed:coder-1",
      }),
    });
    mockUseAutoRefresh.mockReturnValue({
      data: [preserve, cleanup],
      loading: false,
      error: null,
      lastUpdatedAt: null,
      isRefreshing: false,
    });

    render(
      <MemoryRouter>
        <AuditPage />
      </MemoryRouter>,
    );

    expect(screen.queryByText("agent stopped (branch preserved)")).toBeNull();
    fireEvent.click(screen.getByText("killed and removed instance"));
    expect(screen.getByText(/event 1/)).toBeTruthy();
    expect(screen.getByText(/event 2/)).toBeTruthy();
    expect(screen.getByText(/kill_and_remove_instance/)).toBeTruthy();
    expect(screen.getByText(/kill_instance/)).toBeTruthy();
  });
});
