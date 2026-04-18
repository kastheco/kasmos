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
import {
  normalizeTaskEntry,
  normalizeTaskStatus,
  normalizeExecutionMode,
  listInstances,
  getInstanceCapture,
  getInstancePresentation,
  sendInstancePermission,
  pauseInstance,
  resumeInstance,
  restartInstance,
  killInstance,
  sendInstancePrompt,
  RequestError,
  TaskExistsError,
  createTask,
  createTopic,
} from "./api.ts";

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

// Capture what URL and method were used last.
let _lastFetchedUrl: string = "";
let _lastFetchedMethod: string = "GET";

// Stub global fetch for these tests.
(globalThis as Record<string, unknown>).fetch = async (
  input: string | URL | Request,
  _init?: RequestInit,
): Promise<Response> => {
  _lastFetchedUrl = String(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
  _lastFetchedMethod = _init?.method ?? "GET";
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

// getInstanceCapture: depth="120" maps to start=-120.
mockFetch(true, 200, "depth 120");
await getInstanceCapture("proj", "inst", { depth: "120" });
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/inst/capture?start=-120",
  "depth 120 maps to start=-120",
);

// getInstanceCapture: depth="1000" maps to start=-1000.
mockFetch(true, 200, "depth 1000");
await getInstanceCapture("proj", "inst", { depth: "1000" });
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/inst/capture?start=-1000",
  "depth 1000 maps to start=-1000",
);

// getInstanceCapture: depth="full" maps to start=- and end=-.
mockFetch(true, 200, "depth full");
await getInstanceCapture("proj", "inst", { depth: "full" });
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/inst/capture?start=-&end=-",
  "depth full maps to start=- and end=-",
);

// getInstanceCapture: depth wins over explicit start and end.
mockFetch(true, 200, "depth precedence");
await getInstanceCapture("proj", "inst", { depth: "120", start: "5", end: "10" });
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/inst/capture?start=-120",
  "depth wins over explicit start and end",
);

// sendInstancePrompt: happy path — sends POST with JSON body.
let _lastFetchedInit: RequestInit | undefined;
(globalThis as Record<string, unknown>).fetch = async (
  input: string | URL | Request,
  init?: RequestInit,
): Promise<Response> => {
  _lastFetchedUrl = String(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
  _lastFetchedInit = init;
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

mockFetch(true, 200, "");
await sendInstancePrompt("proj", "inst", "hello agent");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/inst/send",
  "sendInstancePrompt posts to /send",
);
assertEqual(
  (_lastFetchedInit as RequestInit).method,
  "POST",
  "sendInstancePrompt uses POST",
);
assertEqual(
  JSON.parse((_lastFetchedInit as RequestInit).body as string).prompt,
  "hello agent",
  "sendInstancePrompt sends prompt in body",
);

// sendInstancePrompt: server error surfaces message.
mockFetch(false, 500, JSON.stringify({ error: "agent not running" }));
let sendError: Error | null = null;
try {
  await sendInstancePrompt("proj", "inst", "ping");
} catch (e) {
  sendError = e as Error;
}
if (!sendError) throw new Error("sendInstancePrompt should throw on server error");
assertEqual(sendError.message, "agent not running", "sendInstancePrompt surfaces server error");

// sendInstancePrompt: network failure (fetch rejects) propagates.
// Save and restore the stub so later tests (RequestError / HTTP 500 below)
// still run against the mockFetch-backed fetch rather than this throwing one.
const savedFetch = (globalThis as Record<string, unknown>).fetch;
(globalThis as Record<string, unknown>).fetch = async () => {
  throw new Error("network down");
};
let netError: Error | null = null;
try {
  await sendInstancePrompt("proj", "inst", "ping");
} catch (e) {
  netError = e as Error;
}
(globalThis as Record<string, unknown>).fetch = savedFetch;
if (!netError) throw new Error("sendInstancePrompt should propagate network failure");
assertEqual(netError.message, "network down", "sendInstancePrompt propagates network error");

console.log("api.test.ts instance tests ok");

// ---- instance action helpers (pauseInstance / resumeInstance / restartInstance / killInstance) ---

// Reinstall the _lastFetchedMethod-tracking fetch stub. The sendInstancePrompt
// section above installs a custom stub that only tracks _lastFetchedInit but
// not _lastFetchedMethod. Restoring proper tracking here lets the action helper
// assertions below use _lastFetchedMethod directly.
(globalThis as Record<string, unknown>).fetch = async (
  input: string | URL | Request,
  init?: RequestInit,
): Promise<Response> => {
  _lastFetchedUrl = String(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
  _lastFetchedMethod = init?.method ?? "GET";
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

// pauseInstance: URL-encodes project and title, uses POST method.
mockFetch(true, 204, "");
await pauseInstance("my project", "inst/name");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/my%20project/instances/inst%2Fname/pause",
  "pauseInstance encodes project and title in URL",
);
assertEqual(_lastFetchedMethod, "POST", "pauseInstance uses POST method");

// resumeInstance: URL-encodes both segments.
mockFetch(true, 204, "");
await resumeInstance("proj", "my agent");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/my%20agent/resume",
  "resumeInstance encodes title in URL",
);
assertEqual(_lastFetchedMethod, "POST", "resumeInstance uses POST method");

// restartInstance: URL-encodes both segments.
mockFetch(true, 204, "");
await restartInstance("proj", "agent-1");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/agent-1/restart",
  "restartInstance forms correct URL",
);
assertEqual(_lastFetchedMethod, "POST", "restartInstance uses POST method");

// killInstance: URL-encodes both segments.
mockFetch(true, 204, "");
await killInstance("my project", "planner-1");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/my%20project/instances/planner-1/kill",
  "killInstance encodes project in URL",
);
assertEqual(_lastFetchedMethod, "POST", "killInstance uses POST method");

// action helpers surface backend error string unchanged on non-2xx.
mockFetch(false, 409, JSON.stringify({ error: "instance not running" }));
let actionError: Error | null = null;
try {
  await pauseInstance("proj", "agent-1");
} catch (e) {
  actionError = e as Error;
}
if (!actionError) {
  throw new Error("pauseInstance should throw on HTTP error");
}
assertEqual(
  actionError.message,
  "instance not running",
  "pauseInstance surfaces backend error string unchanged",
);

console.log("api.test.ts action helper tests ok");

// ---- RequestError -----------------------------------------------------------

// RequestError carries status code
const reqErr = new RequestError("bad request", 400);
assertEqual(reqErr.status, 400, "RequestError.status");
assertEqual(reqErr.message, "bad request", "RequestError.message");
assertEqual(reqErr.name, "RequestError", "RequestError.name");
if (!(reqErr instanceof Error)) {
  throw new Error("RequestError should be an instance of Error");
}

// TaskExistsError is a RequestError with status 409
const existsErr = new TaskExistsError("already exists");
assertEqual(existsErr.status, 409, "TaskExistsError.status");
assertEqual(existsErr.name, "TaskExistsError", "TaskExistsError.name");
if (!(existsErr instanceof RequestError)) {
  throw new Error("TaskExistsError should be an instance of RequestError");
}
if (!(existsErr instanceof Error)) {
  throw new Error("TaskExistsError should be an instance of Error");
}

// HTTP error throws RequestError with status
mockFetch(false, 500, JSON.stringify({ error: "internal server error" }));
let requestErrCaught: unknown = null;
try {
  await listInstances("proj");
} catch (e) {
  requestErrCaught = e;
}
if (!(requestErrCaught instanceof RequestError)) {
  throw new Error("HTTP 500 should throw RequestError");
}
assertEqual((requestErrCaught as RequestError).status, 500, "HTTP 500 error has status 500");

// ---- createTask -------------------------------------------------------------

// createTask: success case returns normalized TaskEntry
const taskPayload: TaskEntry = {
  filename: "new-task",
  status: "ready",
  branch: "plan/new-task",
};
mockFetch(true, 201, JSON.stringify(taskPayload));
let _lastFetchInit: RequestInit | undefined;
const origFetch = (globalThis as Record<string, unknown>).fetch as (
  input: unknown,
  init?: RequestInit,
) => Promise<Response>;
(globalThis as Record<string, unknown>).fetch = async (
  input: unknown,
  init?: RequestInit,
): Promise<Response> => {
  _lastFetchInit = init;
  return origFetch(input, init);
};
const gotTask = await createTask("my-project", {
  filename: "new-task",
  description: "test task",
  branch: "plan/new-task",
  created_at: "2026-01-01T00:00:00Z",
});
assertEqual(gotTask.filename, "new-task", "createTask returns filename");
assertEqual(gotTask.status, "ready", "createTask returns status");
// Verify status: "ready" is in the posted body
const postedBody = JSON.parse(_lastFetchInit!.body as string) as { status: string };
assertEqual(postedBody.status, "ready", "createTask posts status: ready");

// createTask: 409 maps to TaskExistsError
mockFetch(false, 409, JSON.stringify({ error: "task already exists" }));
let createTaskErr: unknown = null;
try {
  await createTask("proj", {
    filename: "dup",
    description: "dup",
    branch: "plan/dup",
    created_at: "2026-01-01T00:00:00Z",
  });
} catch (e) {
  createTaskErr = e;
}
if (!(createTaskErr instanceof TaskExistsError)) {
  throw new Error("createTask 409 should throw TaskExistsError");
}

// createTask: 400 stays as RequestError (not TaskExistsError)
mockFetch(false, 400, JSON.stringify({ error: "bad request" }));
let createTask400Err: unknown = null;
try {
  await createTask("proj", {
    filename: "bad",
    description: "bad",
    branch: "plan/bad",
    created_at: "2026-01-01T00:00:00Z",
  });
} catch (e) {
  createTask400Err = e;
}
if (!(createTask400Err instanceof RequestError)) {
  throw new Error("createTask 400 should throw RequestError");
}
if (createTask400Err instanceof TaskExistsError) {
  throw new Error("createTask 400 must NOT be TaskExistsError");
}
assertEqual((createTask400Err as RequestError).status, 400, "createTask 400 error status");

// createTask: 500 stays as RequestError
mockFetch(false, 500, JSON.stringify({ error: "server error" }));
let createTask500Err: unknown = null;
try {
  await createTask("proj", {
    filename: "err",
    description: "err",
    branch: "plan/err",
    created_at: "2026-01-01T00:00:00Z",
  });
} catch (e) {
  createTask500Err = e;
}
if (!(createTask500Err instanceof RequestError)) {
  throw new Error("createTask 500 should throw RequestError");
}
assertEqual((createTask500Err as RequestError).status, 500, "createTask 500 error status");

// ---- createTopic ------------------------------------------------------------

// createTopic: success case
mockFetch(true, 201, "{}");
let createTopicErr: unknown = null;
try {
  await createTopic("my-project", "backend");
} catch (e) {
  createTopicErr = e;
}
if (createTopicErr !== null) {
  throw new Error(`createTopic success should not throw, got: ${String(createTopicErr)}`);
}

// createTopic: 409 is swallowed (duplicate = success)
mockFetch(false, 409, JSON.stringify({ error: "topic already exists" }));
let createTopicDupErr: unknown = null;
try {
  await createTopic("proj", "existing-topic");
} catch (e) {
  createTopicDupErr = e;
}
if (createTopicDupErr !== null) {
  throw new Error("createTopic 409 should not throw");
}

// createTopic: non-409 errors are rethrown
mockFetch(false, 500, JSON.stringify({ error: "server error" }));
let createTopicServerErr: unknown = null;
try {
  await createTopic("proj", "broken-topic");
} catch (e) {
  createTopicServerErr = e;
}
if (!(createTopicServerErr instanceof RequestError)) {
  throw new Error("createTopic 500 should throw RequestError");
}
assertEqual(
  (createTopicServerErr as RequestError).status,
  500,
  "createTopic 500 error status",
);

console.log("api.test.ts new helpers ok");

// ---- normalizeExecutionMode -------------------------------------------------

assertEqual(normalizeExecutionMode("tmux"), "tmux", "tmux passes through");
assertEqual(normalizeExecutionMode("sdk"), "sdk", "sdk passes through");
assertEqual(normalizeExecutionMode("headless"), "sdk", "headless normalizes to sdk");
assertEqual(normalizeExecutionMode(undefined), undefined, "undefined returns undefined");
assertEqual(normalizeExecutionMode("unknown"), undefined, "unknown returns undefined");
assertEqual(normalizeExecutionMode(""), undefined, "empty string returns undefined");

console.log("api.test.ts normalizeExecutionMode ok");

// ---- listInstances normalizes execution_mode --------------------------------

// headless instance in list response is normalized to sdk
const headlessInstanceList = [
  {
    title: "agent-1",
    status: "running",
    branch: "feat/x",
    program: "claude",
    execution_mode: "headless",
  },
  {
    title: "agent-2",
    status: "ready",
    branch: "feat/y",
    program: "codex",
    execution_mode: "tmux",
  },
  {
    title: "agent-3",
    status: "running",
    branch: "feat/z",
    program: "amp",
  },
];
mockFetch(true, 200, JSON.stringify(headlessInstanceList));
const normalizedInstances = await listInstances("proj");
assertEqual(
  normalizedInstances[0].execution_mode,
  "sdk",
  "listInstances normalizes headless to sdk",
);
assertEqual(
  normalizedInstances[1].execution_mode,
  "tmux",
  "listInstances keeps tmux unchanged",
);
assertEqual(
  normalizedInstances[2].execution_mode,
  undefined,
  "listInstances maps missing mode to undefined",
);

console.log("api.test.ts listInstances normalization ok");

// ---- getInstancePresentation ------------------------------------------------

// Happy path: supported=true with turns and dates.
const presentationPayload = {
  supported: true,
  captured_at: "2026-04-17T10:00:00Z",
  turns: [
    {
      id: "turn-1",
      number: 1,
      started_at: "2026-04-17T10:00:01Z",
      completed_at: "2026-04-17T10:00:05Z",
      interrupted: false,
      tool_count: 2,
      rows: [
        {
          kind: "prose",
          text: "hello",
          timestamp: "2026-04-17T10:00:02Z",
          tool_name: "",
          is_error: false,
        },
        {
          kind: "tool",
          text: "run bash",
          timestamp: null,
          tool_name: "bash",
          is_error: false,
        },
      ],
    },
  ],
};
mockFetch(true, 200, JSON.stringify(presentationPayload));
const gotPresentation = await getInstancePresentation("proj", "agent-1");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/agent-1/presentation",
  "getInstancePresentation calls correct URL",
);
assertEqual(gotPresentation.supported, true, "getInstancePresentation: supported");
if (!(gotPresentation.captured_at instanceof Date)) {
  throw new Error("getInstancePresentation: captured_at must be a Date");
}
assertEqual(
  gotPresentation.captured_at.toISOString(),
  "2026-04-17T10:00:00.000Z",
  "getInstancePresentation: captured_at parsed",
);
if (!gotPresentation.turns || gotPresentation.turns.length !== 1) {
  throw new Error("getInstancePresentation: expected 1 turn");
}
const turn = gotPresentation.turns[0];
if (!(turn.started_at instanceof Date)) {
  throw new Error("getInstancePresentation: turn.started_at must be a Date");
}
assertEqual(turn.id, "turn-1", "turn id");
assertEqual(turn.number, 1, "turn number");
assertEqual(turn.tool_count, 2, "turn tool_count");
assertEqual(turn.rows.length, 2, "turn row count");
assertEqual(turn.rows[0].kind, "prose", "row[0] kind");
if (!(turn.rows[0].timestamp instanceof Date)) {
  throw new Error("row[0] timestamp must be a Date");
}
assertEqual(turn.rows[1].tool_name, "bash", "row[1] tool_name");
assertEqual(turn.rows[1].timestamp, null, "row[1] null timestamp");

// supported=false: turns is null — must not throw.
const unsupportedPayload = {
  supported: false,
  captured_at: "2026-04-17T10:00:00Z",
  turns: null,
};
mockFetch(true, 200, JSON.stringify(unsupportedPayload));
const unsupportedPresentation = await getInstancePresentation("proj", "agent-2");
assertEqual(unsupportedPresentation.supported, false, "unsupported: supported=false");
assertEqual(unsupportedPresentation.turns, null, "unsupported: turns=null");

// 404 propagates as RequestError.
mockFetch(false, 404, JSON.stringify({ error: "instance not found" }));
let presentationErr: unknown = null;
try {
  await getInstancePresentation("proj", "missing");
} catch (e) {
  presentationErr = e;
}
if (!(presentationErr instanceof RequestError)) {
  throw new Error("getInstancePresentation 404 should throw RequestError");
}
assertEqual((presentationErr as RequestError).status, 404, "getInstancePresentation 404 status");

console.log("api.test.ts getInstancePresentation ok");

// ---- sendInstancePermission -------------------------------------------------

// Reinstall the full-tracking stub so _lastFetchedInit is captured.
(globalThis as Record<string, unknown>).fetch = async (
  input: string | URL | Request,
  init?: RequestInit,
): Promise<Response> => {
  _lastFetchedUrl = String(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
  _lastFetchedMethod = init?.method ?? "GET";
  _lastFetchedInit = init;
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

mockFetch(true, 200, "");
await sendInstancePermission("proj", "agent-1", "allow_once");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/proj/instances/agent-1/permission",
  "sendInstancePermission: correct URL",
);
assertEqual(
  (_lastFetchedInit as RequestInit).method,
  "POST",
  "sendInstancePermission: POST method",
);
assertEqual(
  JSON.parse((_lastFetchedInit as RequestInit).body as string).choice,
  0,
  "sendInstancePermission: allow_once maps to 0",
);

mockFetch(true, 200, "");
await sendInstancePermission("proj", "agent-1", "allow_always");
assertEqual(
  JSON.parse((_lastFetchedInit as RequestInit).body as string).choice,
  1,
  "sendInstancePermission: allow_always maps to 1",
);

mockFetch(true, 200, "");
await sendInstancePermission("proj", "agent-1", "reject");
assertEqual(
  JSON.parse((_lastFetchedInit as RequestInit).body as string).choice,
  2,
  "sendInstancePermission: reject maps to 2",
);

// 409 propagates as RequestError.
mockFetch(false, 409, JSON.stringify({ error: "conflict" }));
let permErr: unknown = null;
try {
  await sendInstancePermission("proj", "agent-1", "allow_once");
} catch (e) {
  permErr = e;
}
if (!(permErr instanceof RequestError)) {
  throw new Error("sendInstancePermission 409 should throw RequestError");
}
assertEqual((permErr as RequestError).status, 409, "sendInstancePermission 409 status");

console.log("api.test.ts sendInstancePermission ok");
