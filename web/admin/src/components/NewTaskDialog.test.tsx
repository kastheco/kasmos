import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import NewTaskDialog from "./NewTaskDialog";
import type { NewTaskDialogResult } from "./NewTaskDialog";
import type { TopicEntry, TaskEntry } from "../types";

// ---------------------------------------------------------------------------
// Mock the API module so no real fetch calls are made.
// ---------------------------------------------------------------------------

vi.mock("../api", () => {
  class TaskExistsError extends Error {
    status = 409;
    constructor(message: string) {
      super(message);
      this.name = "TaskExistsError";
    }
  }

  return {
    createTask: vi.fn(),
    createTopic: vi.fn(),
    updateTaskContent: vi.fn(),
    applyTaskTransition: vi.fn(),
    TaskExistsError,
  };
});

import * as api from "../api";

// ---------------------------------------------------------------------------
// Test data
// ---------------------------------------------------------------------------

function makeTopics(names: string[]): TopicEntry[] {
  return names.map((name) => ({ name, created_at: "2026-01-01T00:00:00Z" }));
}

const DEFAULT_TOPICS = makeTopics(["backend", "frontend", "infra"]);

const STUB_TASK: TaskEntry = {
  filename: "add-dark-mode",
  status: "ready",
  description: "add a dark mode toggle",
  branch: "plan/add-dark-mode",
  created_at: "2026-04-15T00:00:00Z",
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderDialog(
  props: Partial<{
    open: boolean;
    project: string;
    topics: TopicEntry[];
    onClose: () => void;
    onCreated: (r: NewTaskDialogResult) => void;
  }> = {},
) {
  const onClose = props.onClose ?? vi.fn();
  const onCreated = props.onCreated ?? vi.fn().mockResolvedValue(undefined);
  render(
    <NewTaskDialog
      open={props.open ?? true}
      project={props.project ?? "kasmos"}
      topics={props.topics ?? DEFAULT_TOPICS}
      onClose={onClose}
      onCreated={onCreated}
    />,
  );
  return { onClose, onCreated };
}

function getDescriptionTextarea() {
  return screen.getByLabelText("description") as HTMLTextAreaElement;
}

function getFilenameInput() {
  return screen.getByLabelText("filename") as HTMLInputElement;
}

function getSubmitButton() {
  return screen.getByRole("button", { name: /create task/i }) as HTMLButtonElement;
}

// ---------------------------------------------------------------------------
// Setup
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.createTask).mockResolvedValue(STUB_TASK);
  vi.mocked(api.createTopic).mockResolvedValue(undefined);
  vi.mocked(api.updateTaskContent).mockResolvedValue(undefined);
  vi.mocked(api.applyTaskTransition).mockResolvedValue(STUB_TASK);
});

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

describe("rendering", () => {
  it("renders nothing when open is false", () => {
    renderDialog({ open: false });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("renders the dialog when open is true", () => {
    renderDialog();
    expect(screen.getByRole("dialog")).toBeTruthy();
  });

  it("has the correct aria attributes", () => {
    renderDialog();
    const dialog = screen.getByRole("dialog");
    expect(dialog.getAttribute("aria-modal")).toBe("true");
    expect(dialog.getAttribute("aria-labelledby")).toBe("new-task-title");
  });
});

// ---------------------------------------------------------------------------
// Filename auto-fill
// ---------------------------------------------------------------------------

describe("filename auto-fill", () => {
  it("derives filename from description when untouched", () => {
    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add a dark mode toggle" },
    });
    // deriveFilenameFromDescription("add a dark mode toggle")
    // → deriveTaskTitle → "add a dark mode toggle" (≤6 words)
    // → slugifyTaskName → "add-a-dark-mode-toggle"
    expect(getFilenameInput().value).toBe("add-a-dark-mode-toggle");
  });

  it("keeps filename blank when description is empty", () => {
    renderDialog();
    // textarea starts empty — filename should be blank too
    expect(getFilenameInput().value).toBe("");
  });

  it("submit button is disabled when filename is blank", () => {
    renderDialog();
    expect(getSubmitButton().disabled).toBe(true);
  });

  it("updates filename as description changes", () => {
    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "please fix the login bug" },
    });
    // filler "please " stripped → "fix the login bug" (4 words, slug = "fix-the-login-bug")
    expect(getFilenameInput().value).toBe("fix-the-login-bug");

    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add oauth support" },
    });
    expect(getFilenameInput().value).toBe("add-oauth-support");
  });
});

// ---------------------------------------------------------------------------
// filenameTouched
// ---------------------------------------------------------------------------

describe("filenameTouched", () => {
  it("persists a manually set filename when description changes afterwards", () => {
    renderDialog();
    // First seed description so there is something to change from
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    // Manually edit the filename field
    fireEvent.change(getFilenameInput(), { target: { value: "my-custom-slug" } });
    expect(getFilenameInput().value).toBe("my-custom-slug");

    // Now change description — filename must NOT change
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "something completely different" },
    });
    expect(getFilenameInput().value).toBe("my-custom-slug");
  });

  it("sanitizes manual filename input", () => {
    renderDialog();
    fireEvent.change(getDescriptionTextarea(), { target: { value: "anything" } });
    // Type something with uppercase and spaces (sanitizeFilenameInput converts them)
    fireEvent.change(getFilenameInput(), { target: { value: "Auth Refactor" } });
    expect(getFilenameInput().value).toBe("auth-refactor");
  });
});

// ---------------------------------------------------------------------------
// Duplicate filename error
// ---------------------------------------------------------------------------

describe("duplicate filename error", () => {
  it("shows inline filenameError and keeps dialog open on TaskExistsError", async () => {
    const { TaskExistsError } = api as unknown as {
      TaskExistsError: new (m: string) => Error;
    };
    vi.mocked(api.createTask).mockRejectedValueOnce(
      new TaskExistsError("already exists"),
    );

    const { onCreated } = renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeTruthy();
    });

    expect(screen.getByRole("alert").textContent).toContain(
      "filename already exists",
    );
    // Dialog stays open
    expect(screen.getByRole("dialog")).toBeTruthy();
    // onCreated never called
    expect(onCreated).not.toHaveBeenCalled();
  });

  it("clears filenameError on the next submission attempt", async () => {
    const { TaskExistsError } = api as unknown as {
      TaskExistsError: new (m: string) => Error;
    };
    vi.mocked(api.createTask)
      .mockRejectedValueOnce(new TaskExistsError("conflict"))
      .mockResolvedValueOnce(STUB_TASK);

    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });

    // First submit — triggers duplicate error
    fireEvent.click(getSubmitButton());
    await waitFor(() => {
      expect(screen.getByRole("alert").textContent).toContain("filename already exists");
    });

    // Second submit — should clear the error
    fireEvent.click(getSubmitButton());
    await waitFor(() => {
      expect(screen.queryAllByRole("alert")).toHaveLength(0);
    });
  });
});

// ---------------------------------------------------------------------------
// Create-topic path ordering
// ---------------------------------------------------------------------------

describe("create-topic ordering", () => {
  it("calls createTopic before createTask when topic is new", async () => {
    const callOrder: string[] = [];
    vi.mocked(api.createTopic).mockImplementation(async () => {
      callOrder.push("createTopic");
    });
    vi.mocked(api.createTask).mockImplementation(async () => {
      callOrder.push("createTask");
      return STUB_TASK;
    });

    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });

    // Type a new topic into the combobox
    const combobox = screen.getByRole("combobox");
    fireEvent.focus(combobox);
    fireEvent.change(combobox, { target: { value: "new-topic" } });

    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(callOrder).toContain("createTopic");
      expect(callOrder).toContain("createTask");
    });

    expect(callOrder.indexOf("createTopic")).toBeLessThan(
      callOrder.indexOf("createTask"),
    );
  });

  it("does not call createTopic when topic is empty", async () => {
    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(vi.mocked(api.createTask)).toHaveBeenCalled();
    });

    expect(vi.mocked(api.createTopic)).not.toHaveBeenCalled();
  });

  it("does not call createTopic when topic matches existing entry (isNew=false)", async () => {
    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });

    // Type an existing topic name into the combobox
    const combobox = screen.getByRole("combobox");
    fireEvent.focus(combobox);
    fireEvent.change(combobox, { target: { value: "backend" } });

    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(vi.mocked(api.createTask)).toHaveBeenCalled();
    });

    expect(vi.mocked(api.createTopic)).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Content-write warning path
// ---------------------------------------------------------------------------

describe("content-write warning", () => {
  it("calls onCreated with content warning and closes when updateTaskContent fails", async () => {
    const contentError = new Error("storage full");
    vi.mocked(api.updateTaskContent).mockRejectedValueOnce(contentError);

    const { onCreated, onClose } = renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(onCreated).toHaveBeenCalledWith(
        expect.objectContaining({
          task: STUB_TASK,
          plannerRequested: false,
          warning: expect.objectContaining({ stage: "content" }),
        }),
      );
    });

    expect(onClose).toHaveBeenCalled();
  });

  it("does not call applyTaskTransition when content write fails", async () => {
    vi.mocked(api.updateTaskContent).mockRejectedValueOnce(new Error("err"));

    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    // Enable planner toggle — transition should still not run if content failed
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(vi.mocked(api.updateTaskContent)).toHaveBeenCalled();
    });

    expect(vi.mocked(api.applyTaskTransition)).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Planner toggle
// ---------------------------------------------------------------------------

describe("planner toggle — success", () => {
  it("calls applyTaskTransition with plan_start when kickOffPlanner is checked", async () => {
    const { onCreated } = renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(vi.mocked(api.applyTaskTransition)).toHaveBeenCalledWith(
        "kasmos",
        "add-dark-mode",
        "plan_start",
      );
    });

    expect(onCreated).toHaveBeenCalledWith(
      expect.objectContaining({ task: STUB_TASK, plannerRequested: true }),
    );
  });

  it("does NOT call applyTaskTransition when kickOffPlanner is unchecked", async () => {
    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    // Do NOT check the planner checkbox
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(vi.mocked(api.updateTaskContent)).toHaveBeenCalled();
    });

    expect(vi.mocked(api.applyTaskTransition)).not.toHaveBeenCalled();
  });

  it("calls onCreated with plannerRequested: false when checkbox is unchecked", async () => {
    const { onCreated } = renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(onCreated).toHaveBeenCalledWith(
        expect.objectContaining({ plannerRequested: false }),
      );
    });
  });
});

describe("planner toggle — transition failure", () => {
  it("calls onCreated with plan_start warning when applyTaskTransition fails", async () => {
    const transitionError = new Error("daemon not running");
    vi.mocked(api.applyTaskTransition).mockRejectedValueOnce(transitionError);

    const { onCreated, onClose } = renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(onCreated).toHaveBeenCalledWith(
        expect.objectContaining({
          task: STUB_TASK,
          plannerRequested: true,
          warning: expect.objectContaining({ stage: "plan_start" }),
        }),
      );
    });

    expect(onClose).toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Backdrop / Escape close
// ---------------------------------------------------------------------------

describe("close behaviour", () => {
  it("calls onClose when backdrop is clicked", () => {
    const { onClose } = renderDialog();
    // The backdrop is the direct parent of the dialog; click it (but not the
    // dialog itself) by targeting the element that has the backdrop class.
    const backdrop = screen.getByRole("dialog").parentElement!;
    fireEvent.click(backdrop);
    expect(onClose).toHaveBeenCalled();
  });

  it("calls onClose on Escape when not busy", () => {
    const { onClose } = renderDialog();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });

  it("calls onClose when cancel button is clicked", () => {
    const { onClose } = renderDialog();
    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// createTask receives correct payload
// ---------------------------------------------------------------------------

describe("createTask payload", () => {
  it("passes topic to createTask when topic is selected", async () => {
    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });

    const combobox = screen.getByRole("combobox");
    fireEvent.focus(combobox);
    fireEvent.change(combobox, { target: { value: "frontend" } });

    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(vi.mocked(api.createTask)).toHaveBeenCalledWith(
        "kasmos",
        expect.objectContaining({ topic: "frontend" }),
      );
    });
  });

  it("passes correct filename and branch to createTask", async () => {
    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(vi.mocked(api.createTask)).toHaveBeenCalledWith(
        "kasmos",
        expect.objectContaining({
          filename: "add-dark-mode",
          branch: "plan/add-dark-mode",
        }),
      );
    });
  });

  it("uses manually entered filename for createTask when filenameTouched", async () => {
    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    fireEvent.change(getFilenameInput(), { target: { value: "custom-slug" } });
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(vi.mocked(api.createTask)).toHaveBeenCalledWith(
        "kasmos",
        expect.objectContaining({
          filename: "custom-slug",
          branch: "plan/custom-slug",
        }),
      );
    });
  });
});

// ---------------------------------------------------------------------------
// General error path
// ---------------------------------------------------------------------------

describe("general error", () => {
  it("shows generalError and keeps dialog open on non-duplicate createTask failure", async () => {
    vi.mocked(api.createTask).mockRejectedValueOnce(new Error("network error"));

    const { onCreated } = renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      const alerts = screen.getAllByRole("alert");
      expect(alerts.some((el) => el.textContent?.includes("network error"))).toBe(true);
    });

    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(onCreated).not.toHaveBeenCalled();
  });

  it("shows generalError when createTopic fails", async () => {
    vi.mocked(api.createTopic).mockRejectedValueOnce(new Error("topic error"));

    renderDialog();
    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "add dark mode" },
    });

    const combobox = screen.getByRole("combobox");
    fireEvent.focus(combobox);
    fireEvent.change(combobox, { target: { value: "brand-new-topic" } });

    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      const alerts = screen.getAllByRole("alert");
      expect(alerts.some((el) => el.textContent?.includes("topic error"))).toBe(true);
    });

    // createTask should NOT have been called
    expect(vi.mocked(api.createTask)).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Dialog resets state on re-open
// ---------------------------------------------------------------------------

describe("state reset on open", () => {
  it("resets all fields when dialog is closed and reopened", () => {
    const { rerender } = render(
      <NewTaskDialog
        open={true}
        project="kasmos"
        topics={DEFAULT_TOPICS}
        onClose={vi.fn()}
        onCreated={vi.fn()}
      />,
    );

    fireEvent.change(getDescriptionTextarea(), {
      target: { value: "old description" },
    });
    expect(getFilenameInput().value).not.toBe("");

    // Close the dialog
    rerender(
      <NewTaskDialog
        open={false}
        project="kasmos"
        topics={DEFAULT_TOPICS}
        onClose={vi.fn()}
        onCreated={vi.fn()}
      />,
    );

    // Reopen
    rerender(
      <NewTaskDialog
        open={true}
        project="kasmos"
        topics={DEFAULT_TOPICS}
        onClose={vi.fn()}
        onCreated={vi.fn()}
      />,
    );

    expect(getDescriptionTextarea().value).toBe("");
    expect(getFilenameInput().value).toBe("");
  });
});

// Suppress act() warnings from async state updates in tests that don't need them.
void act;
