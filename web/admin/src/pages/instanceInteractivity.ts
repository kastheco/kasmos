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
 *  2. terminal session-ended capture error disables with specific reason
 *  3. daemon-managed SDK rows (valid_actions present) in running/ready are enabled
 *     because /send is forwarded through the daemon
 *  4. standalone SDK rows (no valid_actions) are disabled — no tmux pane
 *  5. tmux running/ready are enabled
 */
export function composerStateForInstance(
  instance: InstanceEntry | null,
  captureError?: CaptureErrorInfo | string | null,
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
  // Terminal session-ended disables the composer.
  if (captureError !== undefined && captureError !== null) {
    const reason = captureErrorComposerReason(captureError);
    if (reason !== null) {
      return { disabled: true, reason };
    }
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
// Error labels
// ---------------------------------------------------------------------------

/**
 * Structured capture error — carries an optional HTTP status alongside the
 * message so classification helpers can make precise decisions without
 * parsing status codes from error strings.
 */
export interface CaptureErrorInfo {
  status?: number;
  message: string;
}

export type CaptureErrorKind =
  | "kas-serve"
  | "session-ended"
  | "paused"
  | "daemon-unavailable"
  | "tmux-stderr"
  | "generic";

/** Normalise the overloaded error argument to a CaptureErrorInfo or null. */
function normaliseCaptureError(
  error: CaptureErrorInfo | string | null,
): CaptureErrorInfo | null {
  if (error === null || error === undefined) return null;
  if (typeof error === "string") {
    if (!error) return null;
    return { message: error };
  }
  if (!error.message) return null;
  return error;
}

/**
 * Classifies a capture error into a semantic kind.
 * Returns null when there is no error.
 */
export function captureErrorKind(
  error: CaptureErrorInfo | string | null,
): CaptureErrorKind | null {
  const e = normaliseCaptureError(error);
  if (!e) return null;
  const msg = e.message;
  if (msg.includes("kas serve") || msg.includes("--repo")) {
    return "kas-serve";
  }
  if (e.status === 410 || msg.includes("tmux session not found")) {
    return "session-ended";
  }
  if (e.status === 409 && msg.includes("cannot capture pane from a paused instance")) {
    return "paused";
  }
  if (e.status === 502) {
    if (msg.trim() === "daemon unavailable") {
      return "daemon-unavailable";
    }
    if (msg.trim()) {
      return "tmux-stderr";
    }
  }
  return "generic";
}

/**
 * Maps a capture error to a user-friendly lowercase label.
 * Returns null when there is no error to show.
 */
export function captureErrorLabel(
  error: CaptureErrorInfo | string | null,
): string | null {
  const kind = captureErrorKind(error);
  if (kind === null) return null;
  const e = normaliseCaptureError(error)!;
  switch (kind) {
    case "kas-serve":
      return "pane capture unavailable — run kas serve --repo <path> to enable";
    case "session-ended":
      return "session ended";
    case "paused":
      return "instance is paused";
    case "daemon-unavailable":
      return "daemon unavailable — preview will resume when the daemon is back";
    case "tmux-stderr": {
      const firstLine = e.message.split("\n")[0].trim().slice(0, 160);
      return firstLine || "pane output is not available right now";
    }
    case "generic":
      return "pane output is not available right now";
  }
}

/**
 * Returns a composer-facing reason string when the capture error should
 * disable the composer, or null when it should not affect the composer.
 */
export function captureErrorComposerReason(
  error: CaptureErrorInfo | string | null,
): string | null {
  const kind = captureErrorKind(error);
  switch (kind) {
    case "session-ended":
      return "session ended — restart or kill the instance";
    case "paused":
      return "instance is paused";
    default:
      return null;
  }
}

/**
 * Returns true when the polling loop should be suspended due to this error.
 * Daemon-unavailable errors must keep polling so recovery is automatic.
 */
export function shouldSuspendTerminalPolling(
  error: CaptureErrorInfo | string | null,
): boolean {
  const kind = captureErrorKind(error);
  return kind === "session-ended" || kind === "paused";
}
