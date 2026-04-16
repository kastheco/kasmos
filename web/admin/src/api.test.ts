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
  listInstances,
  getInstanceCapture,
  pauseInstance,
  resumeInstance,
  restartInstance,
  killInstance,
  sendInstancePrompt,
  RequestError,
  TaskExistsError,
  RepoNotRegisteredError,
  createTask,
  createTopic,
  getProjectConfig,
  saveProjectConfig,
  runProjectScaffoldSync,
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

// ---- RequestError.code ------------------------------------------------------

// RequestError without code has code=undefined
const reqErrNoCode = new RequestError("bad request", 400);
if (reqErrNoCode.code !== undefined) {
  throw new Error("RequestError without code should have code=undefined");
}

// RequestError with code carries it
const reqErrWithCode = new RequestError("service unavailable", 503, "repo_not_registered");
assertEqual(reqErrWithCode.code, "repo_not_registered", "RequestError.code");

// RequestError.code is extracted from JSON body on HTTP error
mockFetch(false, 503, JSON.stringify({ error: "db only mode", code: "repo_not_registered" }));
let codeErr: unknown = null;
try {
  await listInstances("proj");
} catch (e) {
  codeErr = e;
}
if (!(codeErr instanceof RequestError)) {
  throw new Error("HTTP 503 should throw RequestError");
}
assertEqual(
  (codeErr as RequestError).code,
  "repo_not_registered",
  "RequestError.code extracted from JSON body",
);

// RepoNotRegisteredError is a RequestError with status 503 and code repo_not_registered
const repoErr = new RepoNotRegisteredError("db only mode");
assertEqual(repoErr.status, 503, "RepoNotRegisteredError.status");
assertEqual(repoErr.code, "repo_not_registered", "RepoNotRegisteredError.code");
assertEqual(repoErr.name, "RepoNotRegisteredError", "RepoNotRegisteredError.name");
if (!(repoErr instanceof RequestError)) {
  throw new Error("RepoNotRegisteredError should be an instance of RequestError");
}

console.log("api.test.ts RequestError.code tests ok");

// ---- getProjectConfig -------------------------------------------------------

// getProjectConfig: 200 returns text body
mockFetch(true, 200, "[settings]\nkey = \"value\"");
const gotConfig = await getProjectConfig("my-project");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/my-project/config",
  "getProjectConfig fetches correct URL",
);
assertEqual(
  gotConfig,
  "[settings]\nkey = \"value\"",
  "getProjectConfig returns text body",
);

// getProjectConfig: 404 maps to empty string
mockFetch(false, 404, JSON.stringify({ error: "not found" }));
const emptyConfig = await getProjectConfig("my-project");
assertEqual(emptyConfig, "", "getProjectConfig 404 maps to empty string");

// getProjectConfig: 503 + repo_not_registered maps to RepoNotRegisteredError
mockFetch(false, 503, JSON.stringify({ error: "db only mode", code: "repo_not_registered" }));
let repoNotRegErr: unknown = null;
try {
  await getProjectConfig("my-project");
} catch (e) {
  repoNotRegErr = e;
}
if (!(repoNotRegErr instanceof RepoNotRegisteredError)) {
  throw new Error("getProjectConfig 503/repo_not_registered should throw RepoNotRegisteredError");
}
assertEqual(
  (repoNotRegErr as RepoNotRegisteredError).message,
  "db only mode",
  "RepoNotRegisteredError preserves message",
);

// getProjectConfig: 503 without code rethrows as RequestError (not RepoNotRegisteredError)
mockFetch(false, 503, JSON.stringify({ error: "service unavailable" }));
let other503Err: unknown = null;
try {
  await getProjectConfig("my-project");
} catch (e) {
  other503Err = e;
}
if (!(other503Err instanceof RequestError)) {
  throw new Error("getProjectConfig 503 without code should throw RequestError");
}
if (other503Err instanceof RepoNotRegisteredError) {
  throw new Error("getProjectConfig 503 without code must NOT be RepoNotRegisteredError");
}

// getProjectConfig: 500 rethrows as RequestError
mockFetch(false, 500, JSON.stringify({ error: "internal error" }));
let configServerErr: unknown = null;
try {
  await getProjectConfig("my-project");
} catch (e) {
  configServerErr = e;
}
if (!(configServerErr instanceof RequestError)) {
  throw new Error("getProjectConfig 500 should throw RequestError");
}

console.log("api.test.ts getProjectConfig tests ok");

// ---- saveProjectConfig ------------------------------------------------------

// Reinstall a tracking stub so _lastFetchedInit is updated by every call.
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

// saveProjectConfig: uses PUT with text/plain content-type
mockFetch(true, 204, "");
await saveProjectConfig("my-project", "[settings]\nkey = \"value\"");
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/my-project/config",
  "saveProjectConfig fetches correct URL",
);
assertEqual(_lastFetchedMethod, "PUT", "saveProjectConfig uses PUT method");
assertEqual(
  ((_lastFetchedInit as RequestInit).headers as Record<string, string>)["Content-Type"],
  "text/plain",
  "saveProjectConfig sends Content-Type: text/plain",
);
assertEqual(
  (_lastFetchedInit as RequestInit).body,
  "[settings]\nkey = \"value\"",
  "saveProjectConfig sends TOML body",
);

// saveProjectConfig: throws on error
mockFetch(false, 422, JSON.stringify({ error: "invalid toml" }));
let saveConfigErr: unknown = null;
try {
  await saveProjectConfig("my-project", "bad = [[[");
} catch (e) {
  saveConfigErr = e;
}
if (!(saveConfigErr instanceof RequestError)) {
  throw new Error("saveProjectConfig error should throw RequestError");
}
assertEqual(
  (saveConfigErr as RequestError).message,
  "invalid toml",
  "saveProjectConfig surfaces server error message",
);

console.log("api.test.ts saveProjectConfig tests ok");

// ---- runProjectScaffoldSync -------------------------------------------------

// runProjectScaffoldSync: posts JSON and returns parsed body on success
mockFetch(true, 200, JSON.stringify({ ok: true, output: "synced" }));
const syncResult = await runProjectScaffoldSync("my-project", { worktrees: false, trust: false });
assertEqual(
  _lastFetchedUrl,
  "/v1/projects/my-project/scaffold-sync",
  "runProjectScaffoldSync fetches correct URL",
);
assertEqual(_lastFetchedMethod, "POST", "runProjectScaffoldSync uses POST method");
assertEqual(syncResult.ok, true, "runProjectScaffoldSync returns ok");
assertEqual(syncResult.output, "synced", "runProjectScaffoldSync returns output");

// runProjectScaffoldSync: always returns body, does not throw on ok:false
mockFetch(false, 200, JSON.stringify({ ok: false, output: "some output", error: "sync failed" }));
const failedSync = await runProjectScaffoldSync("my-project", { worktrees: true, trust: true });
assertEqual(failedSync.ok, false, "runProjectScaffoldSync ok:false does not throw");
assertEqual(failedSync.error, "sync failed", "runProjectScaffoldSync returns error field");
assertEqual(
  (_lastFetchedInit as RequestInit).body,
  JSON.stringify({ worktrees: true, trust: true }),
  "runProjectScaffoldSync sends correct request body",
);

// runProjectScaffoldSync: returns parsed body even on non-2xx HTTP status
mockFetch(false, 500, JSON.stringify({ ok: false, output: "", error: "server crash" }));
const serverCrashSync = await runProjectScaffoldSync("my-project", { worktrees: false, trust: false });
assertEqual(serverCrashSync.ok, false, "runProjectScaffoldSync 500 returns parsed body without throwing");

console.log("api.test.ts runProjectScaffoldSync tests ok");
