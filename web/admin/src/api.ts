import type { Status, TaskEntry, SubtaskEntry, TopicEntry, AuditEvent } from "./types";

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

async function request(path: string, init?: RequestInit): Promise<Response> {
  const response = await fetch(path, init);
  if (!response.ok) {
    let message = `HTTP error ${response.status}`;
    try {
      const body = (await response.clone().json()) as { error?: string };
      if (body.error) {
        message = body.error;
      }
    } catch {
      // ignore parse errors — body may not be JSON
    }
    throw new Error(message);
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
