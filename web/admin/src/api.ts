import type { Status, TaskEntry, SubtaskEntry, TopicEntry, AuditEvent, InstanceEntry, InstanceAction, ScrollbackDepth } from "./types";

// Legacy persisted statuses that predate canonical normalization at ingest.
// Mirrors config/taskfsm/fsm.go:MapLegacyStatus so the SPA reader boundary
// stays consistent with the web task-actions handler's precheck path, which
// accepts "in_progress" / "completed" rows and converts them before applying
// FSM transitions. Without this, tasks imported before ingest-time
// normalization render as "unknown" in StatusBadge and drop out of
// status-based filters/counts.
const LEGACY_STATUS_MAP: Record<string, Status> = {
  in_progress: "implementing",
  completed: "done",
};

export function normalizeTaskStatus(raw: string): Status {
  return (LEGACY_STATUS_MAP[raw] ?? raw) as Status;
}

export function normalizeTaskEntry(entry: TaskEntry): TaskEntry {
  const canonical = normalizeTaskStatus(entry.status);
  if (canonical === entry.status) return entry;
  return { ...entry, status: canonical };
}

export class RequestError extends Error {
  constructor(message: string, readonly status: number, readonly code?: string) {
    super(message);
    this.name = "RequestError";
  }
}

export class TaskExistsError extends RequestError {
  constructor(message: string) {
    super(message, 409);
    this.name = "TaskExistsError";
  }
}

export class RepoNotRegisteredError extends RequestError {
  constructor(message: string) {
    super(message, 503, "repo_not_registered");
    this.name = "RepoNotRegisteredError";
  }
}

export interface ScaffoldSyncRequest {
  worktrees: boolean;
  trust: boolean;
}

export interface ScaffoldSyncResponse {
  ok: boolean;
  output: string;
  error?: string;
}

async function request(path: string, init?: RequestInit): Promise<Response> {
  const response = await fetch(path, init);
  if (!response.ok) {
    let message = `HTTP error ${response.status}`;
    let code: string | undefined;
    try {
      const body = (await response.clone().json()) as { error?: string; code?: string };
      if (body.error) message = body.error;
      code = body.code;
    } catch {
      // ignore parse errors — body may not be JSON
    }
    throw new RequestError(message, response.status, code);
  }
  return response;
}

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await request(path, init);
  return response.json() as Promise<T>;
}

async function requestText(path: string, init?: RequestInit): Promise<string> {
  const response = await request(path, init);
  return response.text();
}

type AuditEventResponse = {
  ID: number;
  Kind: string;
  Timestamp: string;
  Project: string;
  TaskFile: string;
  InstanceTitle: string;
  AgentType: string;
  WaveNumber: number;
  TaskNumber: number;
  Message: string;
  Detail: string;
  Level: string;
};

function normalizeAuditEvent(raw: AuditEventResponse): AuditEvent {
  return {
    id: raw.ID,
    kind: raw.Kind,
    timestamp: raw.Timestamp,
    project: raw.Project,
    task_file: raw.TaskFile,
    instance_title: raw.InstanceTitle,
    agent_type: raw.AgentType,
    wave_number: raw.WaveNumber,
    task_number: raw.TaskNumber,
    message: raw.Message,
    detail: raw.Detail,
    level: raw.Level,
  };
}

export function resolveProjectName(
  search: string,
  hostname: string = window.location.hostname,
): string {
  const params = new URLSearchParams(search);
  const project = params.get("project");
  if (project) return project;

  const local = ["localhost", "127.0.0.1", ""];
  if (!local.includes(hostname)) return hostname;

  return "kasmos";
}

export async function listProjects(): Promise<string[]> {
  return (await requestJSON<string[] | null>("/v1/projects")) ?? [];
}

export async function listTasks(project: string): Promise<TaskEntry[]> {
  const raw =
    (await requestJSON<TaskEntry[] | null>(
      `/v1/projects/${encodeURIComponent(project)}/tasks`,
    )) ?? [];
  return raw.map(normalizeTaskEntry);
}

export async function getTask(
  project: string,
  filename: string,
): Promise<TaskEntry> {
  const raw = await requestJSON<TaskEntry>(
    `/v1/projects/${encodeURIComponent(project)}/tasks/${encodeURIComponent(filename)}`,
  );
  return normalizeTaskEntry(raw);
}

export async function getTaskContent(
  project: string,
  filename: string,
): Promise<string> {
  return requestText(
    `/v1/projects/${encodeURIComponent(project)}/tasks/${encodeURIComponent(filename)}/content`,
  );
}

export async function getSubtasks(
  project: string,
  filename: string,
): Promise<SubtaskEntry[]> {
  return (await requestJSON<SubtaskEntry[] | null>(
    `/v1/projects/${encodeURIComponent(project)}/tasks/${encodeURIComponent(filename)}/subtasks`,
  )) ?? [];
}

export async function listTopics(project: string): Promise<TopicEntry[]> {
  return (await requestJSON<TopicEntry[] | null>(
    `/v1/projects/${encodeURIComponent(project)}/topics`,
  )) ?? [];
}

export async function listAuditEvents(project: string): Promise<AuditEvent[]> {
  const raw =
    (await requestJSON<AuditEventResponse[] | null>(
      `/v1/projects/${encodeURIComponent(project)}/audit-events`,
    )) ?? [];
  return raw.map(normalizeAuditEvent);
}

export type AuditEventFilter = {
  kind?: string;
  task?: string;
  limit?: number;
};

export async function fetchAuditEvents(
  project: string,
  filter?: AuditEventFilter,
): Promise<AuditEvent[]> {
  const params = new URLSearchParams();
  if (filter?.kind) params.append("kind", filter.kind);
  if (filter?.task) params.set("task", filter.task);
  if (filter?.limit != null) params.set("limit", String(filter.limit));
  const qs = params.toString();
  const url = `/v1/projects/${encodeURIComponent(project)}/audit-events${qs ? `?${qs}` : ""}`;
  const raw = (await requestJSON<AuditEventResponse[] | null>(url)) ?? [];
  return raw.map(normalizeAuditEvent);
}

// ---- task action types -------------------------------------------------------

export type TransitionAction = { event: string; label: string };
export type OverrideAction = { target: string; label: string };
export type AvailableActionsResponse = {
  transitions: TransitionAction[];
  overrides: OverrideAction[];
};

// ---- task action API helpers -------------------------------------------------

function taskBase(project: string, filename: string): string {
  return `/v1/projects/${encodeURIComponent(project)}/tasks/${encodeURIComponent(filename)}`;
}

export async function getAvailableActions(
  project: string,
  filename: string,
): Promise<AvailableActionsResponse> {
  return requestJSON<AvailableActionsResponse>(
    `${taskBase(project, filename)}/available-actions`,
  );
}

export async function applyTaskTransition(
  project: string,
  filename: string,
  event: string,
): Promise<TaskEntry> {
  const raw = await requestJSON<TaskEntry>(
    `${taskBase(project, filename)}/transition`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ event }),
    },
  );
  return normalizeTaskEntry(raw);
}

export async function overrideTaskStatus(
  project: string,
  filename: string,
  target: string,
): Promise<TaskEntry> {
  const raw = await requestJSON<TaskEntry>(
    `${taskBase(project, filename)}/status`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ target }),
    },
  );
  return normalizeTaskEntry(raw);
}

export async function renameTask(
  project: string,
  filename: string,
  newFilename: string,
): Promise<TaskEntry> {
  const raw = await requestJSON<TaskEntry>(
    `${taskBase(project, filename)}/rename`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ new_filename: newFilename }),
    },
  );
  return normalizeTaskEntry(raw);
}

export async function updateTaskTopic(
  project: string,
  filename: string,
  topic: string,
): Promise<TaskEntry> {
  const raw = await requestJSON<TaskEntry>(
    `${taskBase(project, filename)}/topic`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ topic }),
    },
  );
  return normalizeTaskEntry(raw);
}

export async function updateTaskGoal(
  project: string,
  filename: string,
  goal: string,
): Promise<void> {
  await request(`${taskBase(project, filename)}/goal`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ goal }),
  });
}

export async function updateTaskContent(
  project: string,
  filename: string,
  content: string,
): Promise<void> {
  await request(`${taskBase(project, filename)}/content`, {
    method: "PUT",
    headers: { "Content-Type": "text/plain" },
    body: content,
  });
}

export async function deleteTask(
  project: string,
  filename: string,
): Promise<void> {
  await request(taskBase(project, filename), { method: "DELETE" });
}

// ---- instance API helpers ---------------------------------------------------

export async function listInstances(project: string): Promise<InstanceEntry[]> {
  return (
    (await requestJSON<InstanceEntry[] | null>(
      `/v1/projects/${encodeURIComponent(project)}/instances`,
    )) ?? []
  );
}

export async function createTask(
  project: string,
  entry: {
    filename: string;
    description: string;
    topic?: string;
    branch: string;
    created_at: string;
  },
): Promise<TaskEntry> {
  let raw: TaskEntry;
  try {
    raw = await requestJSON<TaskEntry>(
      `/v1/projects/${encodeURIComponent(project)}/tasks`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...entry, status: "ready" }),
      },
    );
  } catch (err) {
    if (err instanceof RequestError && err.status === 409) {
      throw new TaskExistsError(err.message);
    }
    throw err;
  }
  return normalizeTaskEntry(raw);
}

export async function createTopic(
  project: string,
  name: string,
): Promise<void> {
  try {
    await request(`/v1/projects/${encodeURIComponent(project)}/topics`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, created_at: new Date().toISOString() }),
    });
  } catch (err) {
    if (err instanceof RequestError && err.status === 409) {
      // duplicate topic creation is treated as success
      return;
    }
    throw err;
  }
}

export async function getInstanceCapture(
  project: string,
  title: string,
  opts?: { start?: string; end?: string; depth?: ScrollbackDepth },
): Promise<string> {
  const params = new URLSearchParams();
  if (opts?.depth != null) {
    // depth wins over explicit start/end
    switch (opts.depth) {
      case "120":
        params.set("start", "-120");
        break;
      case "1000":
        params.set("start", "-1000");
        break;
      case "full":
        params.set("start", "-");
        params.set("end", "-");
        break;
    }
  } else {
    if (opts?.start != null) params.set("start", opts.start);
    if (opts?.end != null) params.set("end", opts.end);
  }
  const qs = params.toString();
  const url = `/v1/projects/${encodeURIComponent(project)}/instances/${encodeURIComponent(title)}/capture${qs ? `?${qs}` : ""}`;
  return requestText(url);
}

// ---- instance action helpers -------------------------------------------------

async function postInstanceAction(
  project: string,
  title: string,
  action: InstanceAction,
): Promise<void> {
  await request(
    `/v1/projects/${encodeURIComponent(project)}/instances/${encodeURIComponent(title)}/${action}`,
    { method: "POST" },
  );
}

export async function pauseInstance(project: string, title: string): Promise<void> {
  return postInstanceAction(project, title, "pause");
}

export async function resumeInstance(project: string, title: string): Promise<void> {
  return postInstanceAction(project, title, "resume");
}

export async function restartInstance(project: string, title: string): Promise<void> {
  return postInstanceAction(project, title, "restart");
}

export async function killInstance(project: string, title: string): Promise<void> {
  return postInstanceAction(project, title, "kill");
}

export async function sendInstancePrompt(
  project: string,
  title: string,
  prompt: string,
): Promise<void> {
  await request(
    `/v1/projects/${encodeURIComponent(project)}/instances/${encodeURIComponent(title)}/send`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ prompt }),
    },
  );
}

// ---- project config API helpers ----------------------------------------------

export async function getProjectConfig(project: string): Promise<string> {
  try {
    return await requestText(`/v1/projects/${encodeURIComponent(project)}/config`);
  } catch (err) {
    if (err instanceof RequestError && err.status === 404) return "";
    if (
      err instanceof RequestError &&
      err.status === 503 &&
      err.code === "repo_not_registered"
    ) {
      throw new RepoNotRegisteredError(err.message);
    }
    throw err;
  }
}

export async function saveProjectConfig(
  project: string,
  toml: string,
): Promise<void> {
  await request(`/v1/projects/${encodeURIComponent(project)}/config`, {
    method: "PUT",
    headers: { "Content-Type": "text/plain" },
    body: toml,
  });
}

export async function runProjectScaffoldSync(
  project: string,
  req: ScaffoldSyncRequest,
): Promise<ScaffoldSyncResponse> {
  // Always return the parsed body — the server encodes runner failures as
  // ok:false, so we do not throw on non-2xx status codes here.
  const response = await fetch(
    `/v1/projects/${encodeURIComponent(project)}/scaffold-sync`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
    },
  );
  return response.json() as Promise<ScaffoldSyncResponse>;
}
