export type DisplayMode = "pip" | "inline" | "fullscreen" | "sidebar";

export interface LifecycleCounts { planning: number; ready: number; implementing: number; reviewing: number; verifying: number; total: number }
export interface ActiveAgent { task: string; role: string; wave?: number; stage?: string; ready?: boolean; active?: boolean; worktree?: string; branch?: string; task_number?: number; last_activity?: string; paused?: boolean }
export interface AttentionItem { task: string; kind: string; detail?: string }
export interface TaskSummary { filename: string; status: string; phase?: string; topic?: string; branch?: string; active_wave?: number; total_waves?: number; subtasks_done: number; subtasks_total: number; review_cycle?: number; pr_url?: string; pr_check_status?: string; pr_review_decision?: string; blocked?: boolean }
export interface EventItem { at: string; kind: string; task?: string; agent?: string; wave?: number; task_number?: number; message: string; level?: string }
export interface WaveTask { number: number; title: string; status: string }
export interface WaveProgress { wave: number; active?: boolean; tasks: WaveTask[] }
export interface Readiness { status: string; review_cycle?: number; has_review_feedback?: boolean; pr_check_status?: string; pr_review_decision?: string; last_verify_outcome?: string }
export interface TaskFocus { filename: string; goal?: string; waves: WaveProgress[]; readiness: Readiness }
export interface MonitorSnapshot { schema_version: number; generated_at: string; project: string; daemon_running: boolean; uptime?: string; repo_count?: number; lifecycle: LifecycleCounts; active_agents: ActiveAgent[]; attention: AttentionItem[]; truncated: { active_agents?: number; attention?: number; tasks?: number; events?: number }; projects?: string[]; tasks?: TaskSummary[]; events?: EventItem[]; focus?: TaskFocus }
