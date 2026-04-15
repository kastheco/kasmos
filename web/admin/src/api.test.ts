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

import type { Status, TaskEntry, InstanceEntry } from "./types.ts";
import { normalizeTaskEntry, normalizeTaskStatus, listInstances, getInstanceCapture } from "./api.ts";

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

// ---- listInstances and getInstanceCapture -----------------------------------

// Minimal fetch mock: returns a configurable response for the next call.
let _mockResponse: { ok: boolean; status: number; body: string } | null = null;

function mockFetch(ok: boolean, status: number, body: string): void {
  _mockResponse = { ok, status, body };
}

// Capture what URL was fetched last.
let _lastFetchedUrl: string = "";

// Stub global fetch for these tests.
(globalThis as Record<string, unknown>).fetch = async (
  input: string | URL | Request,
  _init?: RequestInit,
): Promise<Response> => {
  _lastFetchedUrl = String(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
  const m = _mockResponse!;
  _mockResponse = null;
  const body = m.body;
  return {
    ok: m.ok,
    status: m.status,
    json: async () => JSON.parse(body) as unknown,
    text: async () => body,
    clone: () => ({
      json: async () => JSON.parse(body) as unknown,
      text: async () => body,
    }),
  } as unknown as Response;
};

// listInstances: URL-encodes project and returns parsed array.
const instanceList: InstanceEntry[] = [
  { title: "my-inst", status: "running", branch: "feat/x", program: "claude" },
];
mockFetch(true, 200, JSON.stringify(instanceList));
const gotInstances = await listInstances("my project");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/my%20project/instances",
  "listInstances encodes project in URL",
);
assertEqual(gotInstances.length, 1, "listInstances returns parsed array");
assertEqual(gotInstances[0].title, "my-inst", "listInstances preserves title");

// listInstances returns [] for a null body.
mockFetch(true, 200, "null");
const emptyInstances = await listInstances("proj");
assertEqual(emptyInstances.length, 0, "listInstances handles null body");

// getInstanceCapture: URL-encodes both project and title.
mockFetch(true, 200, "pane content");
const gotCapture = await getInstanceCapture("my project", "inst/title");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/my%20project/instances/inst%2Ftitle/capture",
  "getInstanceCapture encodes project and title",
);
assertEqual(gotCapture, "pane content", "getInstanceCapture returns text");

// getInstanceCapture: appends start/end query params when provided.
mockFetch(true, 200, "pane with range");
await getInstanceCapture("proj", "inst", { start: "10", end: "20" });
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/inst/capture?start=10&end=20",
  "getInstanceCapture appends start and end params",
);

// getInstanceCapture: appends only start when end is omitted.
mockFetch(true, 200, "pane start only");
await getInstanceCapture("proj", "inst", { start: "5" });
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/inst/capture?start=5",
  "getInstanceCapture appends only start param",
);

// getInstanceCapture: throws on HTTP error.
mockFetch(false, 404, JSON.stringify({ error: "not found" }));
let caughtError: Error | null = null;
try {
  await getInstanceCapture("proj", "missing");
} catch (e) {
  caughtError = e as Error;
}
if (!caughtError) {
  throw new Error("getInstanceCapture should throw on HTTP error");
}
assertEqual(caughtError.message, "not found", "getInstanceCapture surfaces error body");

console.log("api.test.ts instance tests ok");
