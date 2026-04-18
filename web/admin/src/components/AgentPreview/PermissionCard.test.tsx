import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Mock } from "vitest";
import { PermissionCard } from "./PermissionCard";

vi.mock("../../api", () => ({
  sendInstancePermission: vi.fn(),
}));

import * as api from "../../api";

describe("PermissionCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders allow, deny, and always buttons when interactive", () => {
    render(
      <PermissionCard
        text="allow file write?"
        project="proj"
        title="agent-1"
        interactive={true}
      />,
    );
    expect(screen.getByRole("button", { name: "allow" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "always" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "deny" })).toBeTruthy();
  });

  it("shows the permission text", () => {
    render(
      <PermissionCard
        text="allow file write?"
        project="proj"
        title="agent-1"
        interactive={true}
      />,
    );
    expect(screen.getByText("allow file write?")).toBeTruthy();
  });

  it("disables all buttons while a choice is pending", async () => {
    let resolve: () => void;
    (api.sendInstancePermission as Mock).mockReturnValue(
      new Promise<void>((r) => {
        resolve = r;
      }),
    );

    render(
      <PermissionCard
        text="allow?"
        project="proj"
        title="agent-1"
        interactive={true}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "allow" }));

    await waitFor(() => {
      expect(
        (screen.getByRole("button", { name: "allow" }) as HTMLButtonElement).disabled,
      ).toBe(true);
      expect(
        (screen.getByRole("button", { name: "always" }) as HTMLButtonElement).disabled,
      ).toBe(true);
      expect(
        (screen.getByRole("button", { name: "deny" }) as HTMLButtonElement).disabled,
      ).toBe(true);
    });

    resolve!();
  });

  it("hides the card on success", async () => {
    (api.sendInstancePermission as Mock).mockResolvedValue(undefined);

    render(
      <PermissionCard
        text="allow read?"
        project="proj"
        title="agent-1"
        interactive={true}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "allow" }));

    await waitFor(() => {
      expect(screen.queryByText("allow read?")).toBeNull();
    });
  });

  it("restores buttons and shows inline error on failure", async () => {
    (api.sendInstancePermission as Mock).mockRejectedValue(
      new Error("Server error"),
    );

    render(
      <PermissionCard
        text="allow exec?"
        project="proj"
        title="agent-1"
        interactive={true}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "deny" }));

    await waitFor(() => {
      // Buttons should be re-enabled
      expect(
        (screen.getByRole("button", { name: "allow" }) as HTMLButtonElement).disabled,
      ).toBe(false);
      // Inline error shown (lowercase)
      expect(screen.getByText(/server error/i)).toBeTruthy();
    });
  });

  it("calls sendInstancePermission with allow_once when allow is clicked", async () => {
    (api.sendInstancePermission as Mock).mockResolvedValue(undefined);

    render(
      <PermissionCard
        text="allow?"
        project="my-proj"
        title="my-agent"
        interactive={true}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "allow" }));

    await waitFor(() => {
      expect(api.sendInstancePermission).toHaveBeenCalledWith(
        "my-proj",
        "my-agent",
        "allow_once",
      );
    });
  });

  it("calls sendInstancePermission with allow_always when always is clicked", async () => {
    (api.sendInstancePermission as Mock).mockResolvedValue(undefined);

    render(
      <PermissionCard
        text="allow?"
        project="my-proj"
        title="my-agent"
        interactive={true}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "always" }));

    await waitFor(() => {
      expect(api.sendInstancePermission).toHaveBeenCalledWith(
        "my-proj",
        "my-agent",
        "allow_always",
      );
    });
  });

  it("calls sendInstancePermission with reject when deny is clicked", async () => {
    (api.sendInstancePermission as Mock).mockResolvedValue(undefined);

    render(
      <PermissionCard
        text="allow?"
        project="my-proj"
        title="my-agent"
        interactive={true}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "deny" }));

    await waitFor(() => {
      expect(api.sendInstancePermission).toHaveBeenCalledWith(
        "my-proj",
        "my-agent",
        "reject",
      );
    });
  });

  it("renders read-only state when not interactive", () => {
    render(
      <PermissionCard
        text="allow write?"
        project="proj"
        title="agent-1"
        interactive={false}
      />,
    );
    expect(screen.queryByRole("button", { name: "allow" })).toBeNull();
    expect(screen.queryByRole("button", { name: "deny" })).toBeNull();
    expect(screen.queryByRole("button", { name: "always" })).toBeNull();
    expect(screen.getByText("allow write?")).toBeTruthy();
  });
});
