import { render, screen, fireEvent, waitFor, act, within } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ConfigPage from "./ConfigPage";

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
const mockShow = vi.fn();
vi.mock("../hooks/useToast", () => ({
  useToast: () => ({ show: mockShow }),
}));

// API functions — keep real class exports, stub only the three async functions
const mockGetProjectConfig = vi.fn();
const mockSaveProjectConfig = vi.fn();
const mockRunProjectScaffoldSync = vi.fn();
vi.mock("../api", async (importOriginal) => {
  const orig = await importOriginal<typeof import("../api")>();
  return {
    ...orig,
    getProjectConfig: (...args: Parameters<typeof orig.getProjectConfig>) =>
      mockGetProjectConfig(...args) as ReturnType<typeof orig.getProjectConfig>,
    saveProjectConfig: (...args: Parameters<typeof orig.saveProjectConfig>) =>
      mockSaveProjectConfig(...args) as ReturnType<typeof orig.saveProjectConfig>,
    runProjectScaffoldSync: (...args: Parameters<typeof orig.runProjectScaffoldSync>) =>
      mockRunProjectScaffoldSync(...args) as ReturnType<typeof orig.runProjectScaffoldSync>,
  };
});

// LastUpdated — render the timestamp value as text for easy assertions
vi.mock("../components/LastUpdated", () => ({
  default: ({ timestamp }: { timestamp: Date | null }) => (
    <span data-testid="last-updated">{timestamp ? "updated" : ""}</span>
  ),
}));

// ConfirmDialog — minimal stub that renders two buttons when open
vi.mock("../components/ConfirmDialog", () => ({
  default: (props: {
    open: boolean;
    title: string;
    confirmLabel?: string;
    cancelLabel?: string;
    onConfirm: () => void;
    onCancel: () => void;
  }) =>
    props.open ? (
      <div data-testid="confirm-dialog">
        <button data-testid="confirm-dialog-confirm" onClick={props.onConfirm}>
          {props.confirmLabel ?? "confirm"}
        </button>
        <button data-testid="confirm-dialog-cancel" onClick={props.onCancel}>
          {props.cancelLabel ?? "cancel"}
        </button>
      </div>
    ) : null,
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const DEFAULT_CONFIG = "[settings]\nkey = \"value\"";

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("ConfigPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseProject.mockReturnValue({
      project: "my-project",
      projectSearch: "?project=my-project",
    });
    mockGetProjectConfig.mockResolvedValue(DEFAULT_CONFIG);
    mockSaveProjectConfig.mockResolvedValue(undefined);
    mockRunProjectScaffoldSync.mockResolvedValue({ ok: true, output: "synced ok" });
  });

  // ---- initial load --------------------------------------------------------

  it("renders loading state when project is empty string", () => {
    mockUseProject.mockReturnValue({ project: "", projectSearch: "" });
    render(<ConfigPage />);
    expect(screen.getByText(/loading/i)).toBeTruthy();
    expect(mockGetProjectConfig).not.toHaveBeenCalled();
  });

  it("loads config on mount", async () => {
    render(<ConfigPage />);
    await waitFor(() => {
      expect(mockGetProjectConfig).toHaveBeenCalledWith("my-project");
    });
    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    expect(textarea.value).toBe(DEFAULT_CONFIG);
  });

  it("records lastUpdatedAt after successful load", async () => {
    render(<ConfigPage />);
    await waitFor(() => {
      expect(screen.getByTestId("last-updated").textContent).toBe("updated");
    });
  });

  it("records lastUpdatedAt after 404 (empty config)", async () => {
    mockGetProjectConfig.mockResolvedValueOnce("");
    render(<ConfigPage />);
    await waitFor(() => {
      expect(screen.getByTestId("last-updated").textContent).toBe("updated");
    });
    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    expect(textarea.value).toBe("");
  });

  it("reloads config when project changes", async () => {
    const { rerender } = render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledTimes(1));

    mockUseProject.mockReturnValue({
      project: "other-project",
      projectSearch: "?project=other-project",
    });
    await act(async () => {
      rerender(<ConfigPage />);
    });
    await waitFor(() =>
      expect(mockGetProjectConfig).toHaveBeenCalledWith("other-project"),
    );
  });

  // ---- save ----------------------------------------------------------------

  it("save button is disabled when draft equals saved value", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    const saveBtn = screen.getByRole("button", { name: "save" }) as HTMLButtonElement;
    expect(saveBtn.disabled).toBe(true);
  });

  it("save button is enabled after editing the textarea", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "new toml" } });
    const saveBtn = screen.getByRole("button", { name: "save" }) as HTMLButtonElement;
    expect(saveBtn.disabled).toBe(false);
  });

  it("save success: calls saveProjectConfig, shows toast, reloads from server", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledTimes(1));
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "new toml" } });
    const saveBtn = screen.getByRole("button", { name: "save" });
    await act(async () => {
      fireEvent.click(saveBtn);
    });
    expect(mockSaveProjectConfig).toHaveBeenCalledWith("my-project", "new toml");
    expect(mockShow).toHaveBeenCalledWith(
      "config saved - restart daemon and tui to apply",
    );
    // Reload triggered after save
    expect(mockGetProjectConfig).toHaveBeenCalledTimes(2);
  });

  it("save failure: shows inline error, preserves draft, no toast", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledTimes(1));
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "bad toml" } });
    mockSaveProjectConfig.mockRejectedValueOnce(new Error("invalid toml syntax"));
    const saveBtn = screen.getByRole("button", { name: "save" });
    await act(async () => {
      fireEvent.click(saveBtn);
    });
    // Inline error banner appears
    const alert = screen.getByRole("alert");
    expect(alert.textContent).toContain("invalid toml syntax");
    // Draft is preserved
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("bad toml");
    // No toast on failure
    expect(mockShow).not.toHaveBeenCalled();
    // No reload after failed save
    expect(mockGetProjectConfig).toHaveBeenCalledTimes(1);
  });

  // ---- reload button -------------------------------------------------------

  it("reload button fetches fresh config from server", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledTimes(1));
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "unsaved" } });
    mockGetProjectConfig.mockResolvedValueOnce("server version");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "reload" }));
    });
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledTimes(2));
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("server version");
  });

  // ---- scaffold sync -------------------------------------------------------

  it("scaffold sync sends worktrees=false trust=false by default", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "run sync" }));
    });
    expect(mockRunProjectScaffoldSync).toHaveBeenCalledWith("my-project", {
      worktrees: false,
      trust: false,
    });
  });

  it("scaffold sync with worktrees checkbox checked", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    fireEvent.click(screen.getByLabelText(/include worktrees/i));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "run sync" }));
    });
    expect(mockRunProjectScaffoldSync).toHaveBeenCalledWith("my-project", {
      worktrees: true,
      trust: false,
    });
  });

  it("scaffold sync success: renders output in pre panel", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "run sync" }));
    });
    await waitFor(() => expect(screen.getByText("synced ok")).toBeTruthy());
  });

  it("scaffold sync failure: renders error text and output", async () => {
    mockRunProjectScaffoldSync.mockResolvedValueOnce({
      ok: false,
      output: "partial output",
      error: "sync failed",
    });
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "run sync" }));
    });
    await waitFor(() => {
      expect(screen.getByText("sync failed")).toBeTruthy();
      expect(screen.getByText("partial output")).toBeTruthy();
    });
  });

  it("scaffold sync rejected promise: renders the error message", async () => {
    mockRunProjectScaffoldSync.mockRejectedValueOnce(new Error("network down"));
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "run sync" }));
    });
    await waitFor(() => {
      expect(screen.getByText("network down")).toBeTruthy();
    });
  });

  // ---- trust confirm flow --------------------------------------------------

  it("trust checked: opens confirm dialog before sending request", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    fireEvent.click(screen.getByLabelText(/trust project for codex/i));
    // click run sync — should open dialog, NOT call API
    fireEvent.click(screen.getByRole("button", { name: "run sync" }));
    expect(screen.getByTestId("confirm-dialog")).toBeTruthy();
    expect(mockRunProjectScaffoldSync).not.toHaveBeenCalled();
  });

  it("trust confirm: clicking confirm sends the request with trust=true", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    fireEvent.click(screen.getByLabelText(/trust project for codex/i));
    fireEvent.click(screen.getByRole("button", { name: "run sync" }));
    // Click the confirm button inside the dialog
    await act(async () => {
      fireEvent.click(screen.getByTestId("confirm-dialog-confirm"));
    });
    await waitFor(() =>
      expect(mockRunProjectScaffoldSync).toHaveBeenCalledWith("my-project", {
        worktrees: false,
        trust: true,
      }),
    );
  });

  it("trust confirm: clicking cancel does not send the request", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    fireEvent.click(screen.getByLabelText(/trust project for codex/i));
    fireEvent.click(screen.getByRole("button", { name: "run sync" }));
    fireEvent.click(screen.getByTestId("confirm-dialog-cancel"));
    expect(screen.queryByTestId("confirm-dialog")).toBeNull();
    expect(mockRunProjectScaffoldSync).not.toHaveBeenCalled();
  });

  // ---- repo not registered -------------------------------------------------

  it("repo not registered: shows empty-state card with correct copy", async () => {
    // Import RepoNotRegisteredError from the mocked module (which spreads the
    // real class via ...orig, so instanceof checks work).
    const { RepoNotRegisteredError } = await import("../api");
    mockGetProjectConfig.mockRejectedValueOnce(
      new RepoNotRegisteredError("db only mode"),
    );
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    const msg = await screen.findByText(
      /config editing requires kas serve --repo/i,
    );
    expect(msg).toBeTruthy();
    expect(msg.textContent).toContain("bare-db mode");
    // Editor and sync controls are suppressed
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("button", { name: "run sync" })).toBeNull();
  });

  // ---- notice text ---------------------------------------------------------

  it("renders the apply-changes notice", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    expect(
      screen.getByText(/restart daemon and tui to apply saved changes/i),
    ).toBeTruthy();
  });

  // ---- no background polling -----------------------------------------------

  it("does not call getProjectConfig more than once on initial mount (no auto-refresh)", async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(mockGetProjectConfig).toHaveBeenCalledOnce());
    // Wait extra time to confirm no background polling
    await new Promise((r) => setTimeout(r, 50));
    expect(mockGetProjectConfig).toHaveBeenCalledTimes(1);
  });

  // Suppress the unused 'within' import warning — used above for dialog checks
  void within;
});
