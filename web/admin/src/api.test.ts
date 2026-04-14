// Regression coverage for the legacy-status normalization boundary in api.ts.
// Mirrors config/taskactions/handler_test.go's legacy compatibility tests so
// the SPA reader path cannot drift from the web task-actions handler. Runs
// during `npm run build` via node --experimental-strip-types; no test runner
// is required because the normalization helpers are pure and do not touch
// DOM/network APIs.
//
// Before this regression test, tasks persisted with legacy statuses
// ("in_progress" / "completed") rendered as "unknown" in StatusBadge and
// dropped out of TasksPage/DashboardPage counts entirely.

import type { Status, TaskEntry } from "./types.ts";
import { normalizeTaskEntry, normalizeTaskStatus } from "./api.ts";

function assertEqual<T>(actual: T, expected: T, msg: string): void {
  if (actual !== expected) {
    throw new Error(
      `${msg}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
    );
  }
}

// normalizeTaskStatus maps legacy persisted statuses to canonical ones.
assertEqual(
  normalizeTaskStatus("in_progress"),
  "implementing" as Status,
  "legacy in_progress → implementing",
);
assertEqual(
  normalizeTaskStatus("completed"),
  "done" as Status,
  "legacy completed → done",
);

// Canonical statuses pass through unchanged so StatusBadge keeps rendering
// them with their dedicated label/class.
for (const canonical of [
  "ready",
  "planning",
  "implementing",
  "reviewing",
  "verifying",
  "done",
  "cancelled",
] as const) {
  assertEqual(
    normalizeTaskStatus(canonical),
    canonical,
    `canonical ${canonical} unchanged`,
  );
}

// normalizeTaskEntry rewrites only the status field and preserves every other
// TaskEntry property so downstream consumers (MetadataPanel, DashboardPage
// counts, TasksPage filters) see canonical values without losing metadata.
const legacyImplementing: TaskEntry = {
  filename: "legacy-impl",
  status: "in_progress" as Status,
  goal: "legacy goal",
  topic: "legacy topic",
  implementing_at: "2026-01-01T00:00:00Z",
};
const normalized = normalizeTaskEntry(legacyImplementing);
assertEqual(
  normalized.status,
  "implementing" as Status,
  "entry legacy status normalized",
);
assertEqual(normalized.goal, "legacy goal", "entry goal preserved");
assertEqual(normalized.topic, "legacy topic", "entry topic preserved");
assertEqual(
  normalized.implementing_at,
  "2026-01-01T00:00:00Z",
  "entry implementing_at preserved",
);
assertEqual(
  normalized.filename,
  "legacy-impl",
  "entry filename preserved",
);

// normalizeTaskEntry returns the same reference when no change is needed so
// callers do not pay a shallow-clone cost on every task in the list response.
const alreadyCanonical: TaskEntry = {
  filename: "canonical",
  status: "verifying",
};
if (normalizeTaskEntry(alreadyCanonical) !== alreadyCanonical) {
  throw new Error("canonical entry must be returned by reference");
}

// Covers config/taskactions/handler_test.go:192-281: a legacy "completed" row
// must normalize to "done" so the SPA shows it in the done card/filter and
// StatusBadge renders the done label/class rather than falling through to
// the unknown style bucket.
const legacyDone: TaskEntry = {
  filename: "legacy-done",
  status: "completed" as Status,
};
assertEqual(
  normalizeTaskEntry(legacyDone).status,
  "done" as Status,
  "legacy completed entry normalized to done",
);

console.log("api.test.ts ok");
