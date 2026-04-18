// Pure tests for instanceInteractivity.ts helpers.
// No DOM, no jsdom, no testing-library — runs with `tsx` directly.

import type { InstanceEntry } from "../types.ts";
import {
  composerStateForInstance,
  shouldSubmitComposerKey,
  isAtBottom,
  previewLineLimit,
  captureErrorLabel,
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

// kas serve specific message
{
  const label = captureErrorLabel("run kas serve --repo /path to enable capture");
  assertTrue(
    label !== null && label.includes("kas serve"),
    "kas serve error: label includes kas serve",
  );
}

// generic fallback
{
  const label = captureErrorLabel("connection refused");
  assertEqual(
    label,
    "pane output is not available right now",
    "generic error: fallback label",
  );
}

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

console.log("instanceInteractivity.test.ts ok");
