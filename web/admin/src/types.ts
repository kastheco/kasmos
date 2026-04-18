export type Status =
  | "ready"
  | "planning"
  | "implementing"
  | "reviewing"
  | "verifying"
  | "done"
  | "cancelled";

export interface ExecutionState {
  execution_phase: string;
  active_agent_type?: string;
  active_wave?: number;
}

export type SubtaskStatus =
  | "pending"
  | "running"
  | "complete"
  | "failed"
  | "closed"
  | "done"
  | "blocked"
  | "in_review";

export interface TaskEntry {
  filename: string;
  status: Status;
  description?: string;
  branch?: string;
  topic?: string;
  created_at?: string;
  implemented?: string;
  planning_at?: string;
  implementing_at?: string;
  reviewing_at?: string;
  verifying_at?: string;
  done_at?: string;
  goal?: string;
  content?: string;
  clickup_task_id?: string;
  review_cycle?: number;
  pr_url?: string;
  pr_review_decision?: string;
  pr_check_status?: string;
  execution_state?: ExecutionState;
  latest_review_feedback?: string;
}

export interface SubtaskEntry {
  task_number: number;
  title: string;
  status: SubtaskStatus;
}

export interface TopicEntry {
  name: string;
  created_at: string;
}

export type InstanceStatus = "running" | "ready" | "loading" | "paused";

export type InstanceAction = "pause" | "resume" | "restart" | "kill";
export type ScrollbackDepth = "120" | "1000" | "full";
export type ExecutionMode = "tmux" | "sdk";

export type PresentationRowKind =
  | "thinking"
  | "tool"
  | "result"
  | "system"
  | "permission"
  | "response"
  | "prose"
  | "status";

export interface PresentationRow {
  kind: PresentationRowKind;
  text: string;
  timestamp: Date | null;
  tool_name: string;
  is_error: boolean;
}

export interface PresentationTurn {
  id: string;
  number: number;
  started_at: Date | null;
  completed_at: Date | null;
  interrupted: boolean;
  tool_count: number;
  rows: PresentationRow[];
}

export interface PresentationResponse {
  supported: boolean;
  turns: PresentationTurn[] | null;
  captured_at: Date;
}

export type PermissionDecision = "allow_once" | "allow_always" | "reject";

export interface InstanceEntry {
  title: string;
  status: InstanceStatus;
  branch: string;
  program: string;
  task_file?: string;
  agent_type?: string;
  wave_number?: number;
  task_number?: number;
  created_at?: string;
  updated_at?: string;
  execution_mode?: ExecutionMode;
  valid_actions?: InstanceAction[];
}

export interface AuditEvent {
  id: number;
  kind: string;
  timestamp: string;
  project: string;
  task_file: string;
  instance_title: string;
  agent_type: string;
  wave_number: number;
  task_number: number;
  message: string;
  detail: string;
  level: string;
}
