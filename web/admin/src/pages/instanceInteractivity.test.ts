// Pure tests for instanceInteractivity.ts helpers.
// No DOM, no jsdom, no testing-library — runs with `tsx` directly.

import type { InstanceEntry } from "../types.ts";
import {
  composerStateForInstance,
  shouldSubmitComposerKey,
  isAtBottom,
  previewLineLimit,
  captureErrorLabel,
  captureErrorComposerReason,
  shouldSuspendTerminalPolling,
  hasDaemonBackedWebPath,
  supportsStructuredPreview,
  usesTerminalPreview,
} from "./instanceInteractivity.ts";

function assertEqual<T>(actual: T, expected: T, msg: string): void {
  if (actual !== expected) {
    throw new Error(
      `${msg}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
    );
  }
}

function assertTrue(cond: boolean, msg: string): void {
  if (!cond) throw new Error(msg);
}

function assertFalse(cond: boolean, msg: string): void {
  if (cond) throw new Error(`expected false: ${msg}`);
}

// ---------------------------------------------------------------------------
// composerStateForInstance
// ---------------------------------------------------------------------------

const base: InstanceEntry = {
  title: "my-inst",
  status: "running",
  branch: "main",
  program: "claude",
};

// ready instance is enabled
{
  const s = composerStateForInstance({ ...base, status: "ready" });
  assertFalse(s.disabled, "ready instance: composer enabled");
  assertEqual(s.reason, null, "ready instance: no reason");
}

// running instance is enabled
{
  const s = composerStateForInstance({ ...base, status: "running" });
  assertFalse(s.disabled, "running instance: composer enabled");
  assertEqual(s.reason, null, "running instance: no reason");
}

// standalone SDK instance (no valid_actions) is disabled
{
  const s = composerStateForInstance({ ...base, execution_mode: "sdk" });
  assertTrue(s.disabled, "standalone sdk instance: composer disabled");
  assertEqual(s.reason, "standalone sdk instance", "standalone sdk instance: reason");
}

// daemon-managed SDK instance (valid_actions present) running is enabled
{
  const s = composerStateForInstance({
    ...base,
    execution_mode: "sdk",
    valid_actions: ["pause", "resume"],
  });
  assertFalse(s.disabled, "daemon-managed sdk running: composer enabled");
  assertEqual(s.reason, null, "daemon-managed sdk running: no reason");
}

// daemon-managed SDK instance ready is enabled
{
  const s = composerStateForInstance({
    ...base,
    status: "ready",
    execution_mode: "sdk",
    valid_actions: ["pause"],
  });
  assertFalse(s.disabled, "daemon-managed sdk ready: composer enabled");
  assertEqual(s.reason, null, "daemon-managed sdk ready: no reason");
}

// loading instance is disabled
{
  const s = composerStateForInstance({ ...base, status: "loading" });
  assertTrue(s.disabled, "loading instance: composer disabled");
  assertEqual(s.reason, "instance is loading", "loading instance: reason");
}

// paused instance is disabled
{
  const s = composerStateForInstance({ ...base, status: "paused" });
  assertTrue(s.disabled, "paused instance: composer disabled");
  assertEqual(s.reason, "instance is paused", "paused instance: reason");
}

// null instance is disabled
{
  const s = composerStateForInstance(null);
  assertTrue(s.disabled, "null instance: composer disabled");
  assertTrue(s.reason !== null, "null instance: reason present");
}

// ---------------------------------------------------------------------------
// shouldSubmitComposerKey
// ---------------------------------------------------------------------------

// plain Enter submits
assertTrue(shouldSubmitComposerKey({ key: "Enter" }), "plain Enter submits");

// Ctrl+Enter submits
assertTrue(
  shouldSubmitComposerKey({ key: "Enter", ctrlKey: true }),
  "Ctrl+Enter submits",
);

// Cmd+Enter submits
assertTrue(
  shouldSubmitComposerKey({ key: "Enter", metaKey: true }),
  "Cmd+Enter submits",
);

// Shift+Enter does NOT submit
assertFalse(
  shouldSubmitComposerKey({ key: "Enter", shiftKey: true }),
  "Shift+Enter does not submit",
);

// Other keys do not submit
assertFalse(
  shouldSubmitComposerKey({ key: "a" }),
  "letter key does not submit",
);
assertFalse(
  shouldSubmitComposerKey({ key: "Tab" }),
  "Tab does not submit",
);

// ---------------------------------------------------------------------------
// isAtBottom
// ---------------------------------------------------------------------------

// exactly at bottom
assertTrue(
  isAtBottom(900, 100, 1000),
  "exactly at bottom: isAtBottom true",
);

// within tolerance (scrollHeight=1000, scrollTop=895, clientHeight=100 → diff=5)
assertTrue(
  isAtBottom(895, 100, 1000, 8),
  "within 8px tolerance: isAtBottom true",
);

// outside tolerance (diff=15 > 8)
assertFalse(
  isAtBottom(885, 100, 1000, 8),
  "outside tolerance: isAtBottom false",
);

// at top of short content
assertFalse(
  isAtBottom(0, 100, 500, 8),
  "at top: isAtBottom false when content is taller",
);

// ---------------------------------------------------------------------------
// previewLineLimit
// ---------------------------------------------------------------------------

assertEqual(previewLineLimit("120"), 120, 'previewLineLimit("120") = 120');
assertEqual(previewLineLimit("1000"), 1000, 'previewLineLimit("1000") = 1000');
assertEqual(previewLineLimit("full"), 0, 'previewLineLimit("full") = 0 (unbounded)');

// ---------------------------------------------------------------------------
// captureErrorLabel
// ---------------------------------------------------------------------------

assertEqual(captureErrorLabel(null), null, "null → null");
assertEqual(captureErrorLabel(""), null, "empty string → null");
assertEqual(captureErrorLabel({ message: "" }), null, "empty CaptureErrorInfo → null");

// kas serve specific message — plain string
{
  const label = captureErrorLabel("run kas serve --repo /path to enable capture");
  assertTrue(
    label !== null && label.includes("kas serve"),
    "kas serve error (string): label includes kas serve",
  );
}

// kas serve via CaptureErrorInfo
{
  const label = captureErrorLabel({ message: "run kas serve --repo /path to enable capture" });
  assertTrue(
    label !== null && label.includes("kas serve"),
    "kas serve error (CaptureErrorInfo): label includes kas serve",
  );
}

// 410 / session-ended
{
  const label = captureErrorLabel({ status: 410, message: "tmux session not found" });
  assertEqual(label, "session ended", "410 status: label is 'session ended'");
}

// session-ended via message (no status)
{
  const label = captureErrorLabel({ message: "tmux session not found" });
  assertEqual(label, "session ended", "tmux session not found message: label is 'session ended'");
}

// 409 / paused
{
  const label = captureErrorLabel({ status: 409, message: "cannot capture pane from a paused instance" });
  assertEqual(label, "instance is paused", "409 paused: label is 'instance is paused'");
}

// 502 / daemon-unavailable (exact trimmed message)
{
  const label = captureErrorLabel({ status: 502, message: "daemon unavailable" });
  assertTrue(
    label !== null && label.includes("daemon unavailable") && label.includes("preview will resume"),
    "502 daemon unavailable: label describes daemon down",
  );
}

// 502 / tmux-stderr — first line, capped
{
  const longMsg = "a".repeat(200);
  const multiLine = `first line\nsecond line`;
  const labelLong = captureErrorLabel({ status: 502, message: longMsg });
  assertTrue(
    labelLong !== null && labelLong.length <= 160,
    "502 tmux-stderr long message: capped to 160 chars",
  );
  const labelMulti = captureErrorLabel({ status: 502, message: multiLine });
  assertEqual(labelMulti, "first line", "502 tmux-stderr: only first line returned");
}

// generic fallback — plain string
{
  const label = captureErrorLabel("connection refused");
  assertEqual(
    label,
    "pane output is not available right now",
    "generic error (string): fallback label",
  );
}

// generic fallback — CaptureErrorInfo with no matching status
{
  const label = captureErrorLabel({ status: 500, message: "internal error" });
  assertEqual(
    label,
    "pane output is not available right now",
    "generic error (CaptureErrorInfo 500): fallback label",
  );
}

// ---------------------------------------------------------------------------
// captureErrorComposerReason
// ---------------------------------------------------------------------------

assertEqual(captureErrorComposerReason(null), null, "null → no composer reason");
assertEqual(captureErrorComposerReason(""), null, "empty string → no composer reason");

{
  const reason = captureErrorComposerReason({ status: 410, message: "tmux session not found" });
  assertTrue(
    reason !== null && reason.includes("session ended"),
    "session-ended: composer reason mentions session ended",
  );
}

{
  const reason = captureErrorComposerReason({ status: 409, message: "cannot capture pane from a paused instance" });
  assertEqual(reason, "instance is paused", "paused: composer reason is 'instance is paused'");
}

{
  const reason = captureErrorComposerReason({ status: 502, message: "daemon unavailable" });
  assertEqual(reason, null, "daemon-unavailable: no composer reason (polling continues)");
}

{
  const reason = captureErrorComposerReason("connection refused");
  assertEqual(reason, null, "generic: no composer reason");
}

// ---------------------------------------------------------------------------
// shouldSuspendTerminalPolling
// ---------------------------------------------------------------------------

assertFalse(shouldSuspendTerminalPolling(null), "null → do not suspend");
assertFalse(shouldSuspendTerminalPolling(""), "empty string → do not suspend");

assertTrue(
  shouldSuspendTerminalPolling({ status: 410, message: "tmux session not found" }),
  "session-ended: suspend polling",
);

assertTrue(
  shouldSuspendTerminalPolling({ message: "tmux session not found" }),
  "session-ended (no status): suspend polling",
);

assertTrue(
  shouldSuspendTerminalPolling({ status: 409, message: "cannot capture pane from a paused instance" }),
  "paused: suspend polling",
);

assertFalse(
  shouldSuspendTerminalPolling({ status: 502, message: "daemon unavailable" }),
  "daemon-unavailable: do not suspend polling",
);

assertFalse(
  shouldSuspendTerminalPolling({ status: 502, message: "some tmux stderr" }),
  "tmux-stderr: do not suspend polling",
);

assertFalse(
  shouldSuspendTerminalPolling("connection refused"),
  "generic: do not suspend polling",
);

// ---------------------------------------------------------------------------
// supportsStructuredPreview
// ---------------------------------------------------------------------------

// null → false
assertFalse(supportsStructuredPreview(null), "supportsStructuredPreview(null) = false");

// daemon-managed SDK: sdk + valid_actions → true
assertTrue(
  supportsStructuredPreview({
    ...base,
    execution_mode: "sdk",
    valid_actions: ["pause", "resume"],
  }),
  "daemon-managed sdk with valid_actions: supportsStructuredPreview true",
);

// standalone SDK: sdk but no valid_actions → false
assertFalse(
  supportsStructuredPreview({ ...base, execution_mode: "sdk" }),
  "standalone sdk (no valid_actions): supportsStructuredPreview false",
);

// standalone SDK: sdk with empty valid_actions → false
assertFalse(
  supportsStructuredPreview({ ...base, execution_mode: "sdk", valid_actions: [] }),
  "sdk with empty valid_actions: supportsStructuredPreview false",
);

// tmux row → false (tmux never uses structured preview)
assertFalse(
  supportsStructuredPreview({ ...base, execution_mode: "tmux" }),
  "tmux instance: supportsStructuredPreview false",
);

// no execution_mode → false
assertFalse(
  supportsStructuredPreview({ ...base }),
  "no execution_mode: supportsStructuredPreview false",
);

// ---------------------------------------------------------------------------
// usesTerminalPreview
// ---------------------------------------------------------------------------

// null → false
assertFalse(usesTerminalPreview(null), "usesTerminalPreview(null) = false");

// tmux → true
assertTrue(
  usesTerminalPreview({ ...base, execution_mode: "tmux" }),
  "tmux instance: usesTerminalPreview true",
);

// no execution_mode → true (conservative fallback)
assertTrue(
  usesTerminalPreview({ ...base }),
  "no execution_mode: usesTerminalPreview true (fallback)",
);

// sdk → false
assertFalse(
  usesTerminalPreview({ ...base, execution_mode: "sdk" }),
  "sdk instance: usesTerminalPreview false",
);

// sdk with valid_actions → still false (daemon-managed sdk also uses structured preview)
assertFalse(
  usesTerminalPreview({ ...base, execution_mode: "sdk", valid_actions: ["pause"] }),
  "daemon-managed sdk: usesTerminalPreview false",
);

// sdk with managed_by_daemon → still false
assertFalse(
  usesTerminalPreview({ ...base, execution_mode: "sdk", managed_by_daemon: true }),
  "managed_by_daemon sdk: usesTerminalPreview false",
);

// ---------------------------------------------------------------------------
// hasDaemonBackedWebPath
// ---------------------------------------------------------------------------

assertFalse(hasDaemonBackedWebPath(null), "hasDaemonBackedWebPath(null) = false");

// managed_by_daemon true → true (even without valid_actions)
assertTrue(
  hasDaemonBackedWebPath({ ...base, managed_by_daemon: true }),
  "managed_by_daemon: hasDaemonBackedWebPath true",
);

// managed_by_daemon false, no valid_actions → false
assertFalse(
  hasDaemonBackedWebPath({ ...base, managed_by_daemon: false }),
  "managed_by_daemon false, no valid_actions: hasDaemonBackedWebPath false",
);

// valid_actions non-empty (legacy fallback) → true
assertTrue(
  hasDaemonBackedWebPath({ ...base, valid_actions: ["pause"] }),
  "valid_actions non-empty: hasDaemonBackedWebPath true",
);

// valid_actions empty → false
assertFalse(
  hasDaemonBackedWebPath({ ...base, valid_actions: [] }),
  "valid_actions empty: hasDaemonBackedWebPath false",
);

// ---------------------------------------------------------------------------
// supportsStructuredPreview — managed_by_daemon cases
// ---------------------------------------------------------------------------

// sdk + managed_by_daemon true, no valid_actions → true
assertTrue(
  supportsStructuredPreview({ ...base, execution_mode: "sdk", managed_by_daemon: true }),
  "sdk + managed_by_daemon (no valid_actions): supportsStructuredPreview true",
);

// sdk + managed_by_daemon true, empty valid_actions → true
assertTrue(
  supportsStructuredPreview({
    ...base,
    execution_mode: "sdk",
    managed_by_daemon: true,
    valid_actions: [],
  }),
  "sdk + managed_by_daemon (empty valid_actions): supportsStructuredPreview true",
);

// sdk + valid_actions fallback (no managed_by_daemon) → true
assertTrue(
  supportsStructuredPreview({ ...base, execution_mode: "sdk", valid_actions: ["pause", "resume"] }),
  "sdk + valid_actions fallback: supportsStructuredPreview true",
);

// standalone sdk (no managed_by_daemon, no valid_actions) → false
assertFalse(
  supportsStructuredPreview({ ...base, execution_mode: "sdk" }),
  "standalone sdk: supportsStructuredPreview false",
);

// tmux → false
assertFalse(
  supportsStructuredPreview({ ...base, execution_mode: "tmux", managed_by_daemon: true }),
  "tmux: supportsStructuredPreview false",
);

// ---------------------------------------------------------------------------
// composerStateForInstance — managed_by_daemon cases
// ---------------------------------------------------------------------------

// running daemon-managed SDK (managed_by_daemon, no valid_actions) → enabled
{
  const s = composerStateForInstance({
    ...base,
    execution_mode: "sdk",
    managed_by_daemon: true,
  });
  assertFalse(s.disabled, "running daemon-managed sdk (managed_by_daemon): composer enabled");
  assertEqual(s.reason, null, "running daemon-managed sdk (managed_by_daemon): no reason");
}

// ready daemon-managed SDK (managed_by_daemon, no valid_actions) → enabled
{
  const s = composerStateForInstance({
    ...base,
    status: "ready",
    execution_mode: "sdk",
    managed_by_daemon: true,
  });
  assertFalse(s.disabled, "ready daemon-managed sdk (managed_by_daemon): composer enabled");
  assertEqual(s.reason, null, "ready daemon-managed sdk (managed_by_daemon): no reason");
}

// standalone SDK still disabled with expected reason
{
  const s = composerStateForInstance({ ...base, execution_mode: "sdk" });
  assertTrue(s.disabled, "standalone sdk: composer disabled");
  assertEqual(s.reason, "standalone sdk instance", "standalone sdk: reason unchanged");
}

// running terminal row + 410 capture error disables composer with session-ended reason
{
  const s = composerStateForInstance(
    { ...base, status: "running" },
    { status: 410, message: "tmux session not found" },
  );
  assertTrue(s.disabled, "running terminal + 410 captureError: composer disabled");
  assertTrue(
    s.reason !== null && s.reason.includes("session ended"),
    "running terminal + 410 captureError: reason mentions session ended",
  );
}

// running terminal row + generic capture error does NOT disable via capture error
{
  const s = composerStateForInstance(
    { ...base, status: "running" },
    { status: 500, message: "internal error" },
  );
  assertFalse(s.disabled, "running terminal + generic captureError: composer still enabled");
}

// running terminal row + daemon-unavailable does NOT disable composer
{
  const s = composerStateForInstance(
    { ...base, status: "running" },
    { status: 502, message: "daemon unavailable" },
  );
  assertFalse(s.disabled, "running terminal + daemon-unavailable: composer still enabled");
}

// paused terminal row (status-driven) is disabled regardless of captureError
{
  const s = composerStateForInstance(
    { ...base, status: "paused" },
    { status: 200, message: "" },
  );
  assertTrue(s.disabled, "paused terminal row: composer disabled");
  assertEqual(s.reason, "instance is paused", "paused terminal row: reason is 'instance is paused'");
}

console.log("instanceInteractivity.test.ts ok");
