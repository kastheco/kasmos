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
  | "user"
  | "thinking"
  | "tool"
  | "tool_diff"
  | "result"
  | "tool_preview"
  | "code_block"
  | "system"
  | "permission"
  | "response"
  | "prose"
  | "status";

export type ToolDiffLineKind = "context" | "added" | "removed";

export interface ToolDiffLine {
  kind: ToolDiffLineKind;
  old_number?: number;
  new_number?: number;
  old_text?: string;
  new_text?: string;
}

export interface ToolDiffPayload {
  path?: string;
  lines?: ToolDiffLine[];
  truncated?: boolean;
  hidden_line_count?: number;
}

export interface ToolPreviewPayload {
  lines?: string[];
  truncated?: boolean;
  hidden_line_count?: number;
}

export interface TurnActivity {
  kind: string;
  label?: string;
  started_at: Date | null;
}

export interface PresentationRow {
  kind: PresentationRowKind;
  text: string;
  timestamp: Date | null;
  tool_name: string;
  is_error: boolean;
  tool_diff?: ToolDiffPayload;
  tool_preview?: ToolPreviewPayload;
}

export interface PresentationTurn {
  id: string;
  number: number;
  started_at: Date | null;
  completed_at: Date | null;
  interrupted: boolean;
  tool_count: number;
  rows: PresentationRow[];
  activity?: TurnActivity;
}

// RendererStats is a point-in-time snapshot of SDK renderer byte and eviction accounting.
// Mirrors session/sdk.RendererStats JSON wire shape. All fields are integers.
export interface RendererStats {
  bytes: number;
  lines: number;
  turns: number;
  max_bytes: number;
  max_turns: number;
  evicted_turns: number;
  evicted_lines: number;
  evicted_bytes: number;
  truncated_rows: number;
}

export interface PresentationResponse {
  supported: boolean;
  turns: PresentationTurn[] | null;
  captured_at: Date;
  // stats is the renderer retention snapshot at capture time.
  // Absent for tmux instances and daemon versions that pre-date this field.
  stats?: RendererStats;
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
  last_activity?: string;
  health_reason?: string;
  execution_mode?: ExecutionMode;
  valid_actions?: InstanceAction[];
  /** resource_profile is the active resource-control profile name
   *  ("interactive", "custom", …). Absent or empty means normal/no-op. */
  resource_profile?: string;
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

export interface AuditEventKillDetail {
  action: "kill_instance" | "kill_and_remove_instance";
  cleanup: boolean;
  branch_preserved: boolean;
  group_key?: string;
}

export type ArchitectDecisionUnavailableReason =
  | "architect_not_run"
  | "decision_audit_missing"
  | "repo_not_registered";

export interface ArchitectDecisionDifference {
  area: string;
  scope?: string;
  planner_proposal?: string;
  architect_baseline?: string;
  final_decision: string;
  rationale?: string;
  related_files?: string[];
  task_numbers?: number[];
}

export interface ArchitectDecisionAudit {
  schema_version: number;
  plan_file: string;
  project: string;
  created_at: string;
  baseline_source?: string;
  summary?: string;
  planner_summary?: string;
  baseline_summary?: string;
  final_decision?: string;
  differences?: ArchitectDecisionDifference[];
}

export interface ArchitectDecisionAuditResponse {
  available: boolean;
  reason?: ArchitectDecisionUnavailableReason | string;
  final_markdown?: string;
  decision_audit?: ArchitectDecisionAudit;
  architect_baseline_markdown?: string;
  baseline_reason?: string;
  timestamps?: {
    architect_meta_at?: string;
    baseline_created_at?: string;
    decision_audit_created_at?: string;
  };
}
