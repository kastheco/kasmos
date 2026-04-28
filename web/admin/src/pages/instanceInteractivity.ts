/**
 * Pure helpers for the InstancesPage composer and follow-mode UX.
 * No DOM, no React — importable from both the page and plain Node tests.
 */

import type { InstanceEntry, ScrollbackDepth } from "../types";

// ---------------------------------------------------------------------------
// Preview routing helpers
// ---------------------------------------------------------------------------

/**
 * Returns true when the instance has a daemon-backed web path — either because
 * the list API explicitly set managed_by_daemon, or because valid_actions is
 * non-empty (the legacy proxy for daemon ownership from older daemons).
 */
export function hasDaemonBackedWebPath(instance: InstanceEntry | null): boolean {
  if (!instance) return false;
  return instance.managed_by_daemon === true || (instance.valid_actions?.length ?? 0) > 0;
}

/**
 * Returns true when the instance is a daemon-managed SDK row that exposes the
 * structured presentation endpoint. Requires canonical "sdk" mode and daemon
 * ownership — either via the explicit managed_by_daemon flag or the legacy
 * valid_actions fallback.
 */
export function supportsStructuredPreview(instance: InstanceEntry | null): boolean {
  return instance?.execution_mode === "sdk" && hasDaemonBackedWebPath(instance);
}

/**
 * Returns true when the instance should be rendered with the terminal
 * (tmux pane capture) preview path. SDK rows never use terminal capture
 * regardless of daemon ownership.
 */
export function usesTerminalPreview(instance: InstanceEntry | null): boolean {
  if (!instance) return false;
  return instance.execution_mode !== "sdk";
}

// ---------------------------------------------------------------------------
// Composer state
// ---------------------------------------------------------------------------

export interface ComposerState {
  disabled: boolean;
  reason: string | null;
}

/**
 * Returns whether the composer is enabled for a given instance and, if not,
 * a lowercase reason string to display in the UI.
 *
 * Priority order:
 *  1. loading / paused always disable (regardless of mode)
 *  2. daemon-managed SDK rows (valid_actions present) in running/ready are enabled
 *     because /send is forwarded through the daemon
 *  3. standalone SDK rows (no valid_actions) are disabled — no tmux pane
 *  4. tmux running/ready are enabled
 */
export function composerStateForInstance(
  instance: InstanceEntry | null,
): ComposerState {
  if (!instance) {
    return { disabled: true, reason: "no instance selected" };
  }
  if (instance.status === "loading") {
    return { disabled: true, reason: "instance is loading" };
  }
  if (instance.status === "paused") {
    return { disabled: true, reason: "instance is paused" };
  }
  // Standalone SDK rows have no daemon-forwarded /send path.
  if (instance.execution_mode === "sdk" && !supportsStructuredPreview(instance)) {
    return { disabled: true, reason: "standalone sdk instance" };
  }
  if (instance.status === "running" || instance.status === "ready") {
    return { disabled: false, reason: null };
  }
  return { disabled: true, reason: "instance not available" };
}

// ---------------------------------------------------------------------------
// Key classification
// ---------------------------------------------------------------------------

export interface KeyEvent {
  key: string;
  shiftKey?: boolean;
  ctrlKey?: boolean;
  metaKey?: boolean;
}

/**
 * Returns true when the keyboard event should submit the composer (send).
 * - Enter (without Shift) sends.
 * - Ctrl+Enter and Cmd+Enter send.
 * - Shift+Enter inserts a newline (returns false).
 */
export function shouldSubmitComposerKey(event: KeyEvent): boolean {
  if (event.key !== "Enter") return false;
  if (event.ctrlKey || event.metaKey) return true;
  if (event.shiftKey) return false;
  return true;
}

// ---------------------------------------------------------------------------
// Follow-mode scroll detection
// ---------------------------------------------------------------------------

/**
 * Returns true when the scroll position is at (or within tolerancePx of) the
 * bottom of a scrollable container.
 */
export function isAtBottom(
  scrollTop: number,
  clientHeight: number,
  scrollHeight: number,
  tolerancePx = 8,
): boolean {
  return scrollTop + clientHeight >= scrollHeight - tolerancePx;
}

// ---------------------------------------------------------------------------
// Depth / line limits
// ---------------------------------------------------------------------------

/**
 * Returns the maximum number of lines to render for a given depth preset.
 * Returns 0 for "full", meaning unbounded (TerminalPreview treats maxLines<=0
 * as unbounded).
 */
export function previewLineLimit(depth: ScrollbackDepth): number {
  switch (depth) {
    case "120":
      return 120;
    case "1000":
      return 1000;
    case "full":
      return 0;
  }
}

// ---------------------------------------------------------------------------
// Error labels
// ---------------------------------------------------------------------------

/**
 * Maps a raw capture error message to a user-friendly lowercase label.
 * Returns null when there is no error to show.
 */
export function captureErrorLabel(message: string | null): string | null {
  if (!message) return null;
  // Specific message for missing repo argument to kas serve.
  if (message.includes("kas serve") || message.includes("--repo")) {
    return "pane capture unavailable — run kas serve --repo <path> to enable";
  }
  return "pane output is not available right now";
}
