// Package livestatus defines the canonical live orchestration status contract.
package livestatus

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
)

// SchemaVersion identifies the current live-status wire contract.
const SchemaVersion = 2

const (
	defaultCap = 20
	hardMaxCap = 100
)

const (
	// KindNeedsDecision identifies a task waiting for operator input.
	KindNeedsDecision = "needs_decision"
	// KindReviewFeedback identifies a task with unresolved review feedback.
	KindReviewFeedback = "review_feedback"
	// KindStaleInstance identifies an agent with a reported health problem.
	KindStaleInstance = "stale_instance"
)

// LiveStatus is the canonical compact orchestration snapshot.
type LiveStatus struct {
	SchemaVersion int             `json:"schema_version"`
	GeneratedAt   time.Time       `json:"generated_at"`
	Project       string          `json:"project"`
	DaemonRunning bool            `json:"daemon_running"`
	Uptime        string          `json:"uptime,omitempty"`
	RepoCount     int             `json:"repo_count,omitempty"`
	Lifecycle     LifecycleCounts `json:"lifecycle"`
	ActiveAgents  []ActiveAgent   `json:"active_agents"`
	Attention     []AttentionItem `json:"attention"`
	Truncated     Truncation      `json:"truncated"`
	Projects      []string        `json:"projects,omitempty"`
	Tasks         []TaskSummary   `json:"tasks,omitempty"`
	Events        []EventItem     `json:"events,omitempty"`
	Focus         *TaskFocus      `json:"focus,omitempty"`
}

// LifecycleCounts summarizes active tasks by lifecycle status.
type LifecycleCounts struct {
	Planning     int `json:"planning"`
	Ready        int `json:"ready"`
	Implementing int `json:"implementing"`
	Reviewing    int `json:"reviewing"`
	Verifying    int `json:"verifying"`
	Total        int `json:"total"`
}

// ActiveAgent describes an agent currently associated with a task.
type ActiveAgent struct {
	Task         string     `json:"task"`
	Role         string     `json:"role"`
	Wave         int        `json:"wave,omitempty"`
	Stage        string     `json:"stage,omitempty"`
	Ready        bool       `json:"ready,omitempty"`
	Active       bool       `json:"active,omitempty"`
	Worktree     string     `json:"worktree,omitempty"`
	Branch       string     `json:"branch,omitempty"`
	TaskNumber   int        `json:"task_number,omitempty"`
	LastActivity *time.Time `json:"last_activity,omitempty"`
	Paused       bool       `json:"paused,omitempty"`
}

type TaskSummary struct {
	Filename         string `json:"filename"`
	Status           string `json:"status"`
	Phase            string `json:"phase,omitempty"`
	Topic            string `json:"topic,omitempty"`
	Branch           string `json:"branch,omitempty"`
	ActiveWave       int    `json:"active_wave,omitempty"`
	TotalWaves       int    `json:"total_waves,omitempty"`
	SubtasksDone     int    `json:"subtasks_done"`
	SubtasksTotal    int    `json:"subtasks_total"`
	ReviewCycle      int    `json:"review_cycle,omitempty"`
	PRURL            string `json:"pr_url,omitempty"`
	PRCheckStatus    string `json:"pr_check_status,omitempty"`
	PRReviewDecision string `json:"pr_review_decision,omitempty"`
	Blocked          bool   `json:"blocked,omitempty"`
}
type EventItem struct {
	At         time.Time `json:"at"`
	Kind       string    `json:"kind"`
	Task       string    `json:"task,omitempty"`
	Agent      string    `json:"agent,omitempty"`
	Wave       int       `json:"wave,omitempty"`
	TaskNumber int       `json:"task_number,omitempty"`
	Message    string    `json:"message"`
	Level      string    `json:"level,omitempty"`
}
type TaskFocus struct {
	Filename  string         `json:"filename"`
	Goal      string         `json:"goal,omitempty"`
	Waves     []WaveProgress `json:"waves"`
	Readiness Readiness      `json:"readiness"`
}
type WaveProgress struct {
	Wave   int        `json:"wave"`
	Active bool       `json:"active,omitempty"`
	Tasks  []WaveTask `json:"tasks"`
}
type WaveTask struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Status string `json:"status"`
}
type Readiness struct {
	Status            string `json:"status"`
	ReviewCycle       int    `json:"review_cycle,omitempty"`
	HasReviewFeedback bool   `json:"has_review_feedback,omitempty"`
	PRCheckStatus     string `json:"pr_check_status,omitempty"`
	PRReviewDecision  string `json:"pr_review_decision,omitempty"`
	LastVerifyOutcome string `json:"last_verify_outcome,omitempty"`
}

// AttentionItem describes state that may require operator attention.
type AttentionItem struct {
	Task   string `json:"task"`
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

// Truncation reports omitted items from bounded lists.
type Truncation struct {
	ActiveAgents int `json:"active_agents,omitempty"`
	Attention    int `json:"attention,omitempty"`
	Tasks        int `json:"tasks,omitempty"`
	Events       int `json:"events,omitempty"`
}
type Include struct {
	Projects bool
	Tasks    bool
	Events   bool
	Focus    string
}

// Input contains the surface-neutral state used to assemble a snapshot.
type Input struct {
	Project   string
	Now       time.Time
	Cap       int
	Daemon    DaemonHeartbeat
	Tasks     []TaskInput
	Agents    []AgentInput
	Include   Include
	Projects  []string
	Events    []EventItem
	FocusTask *FocusInput
}
type FocusInput struct {
	Filename   string
	Goal       string
	Content    string
	Subtasks   []taskstore.SubtaskEntry
	ActiveWave int
	Readiness  Readiness
}

// DaemonHeartbeat contains best-effort daemon health context.
type DaemonHeartbeat struct {
	Running   bool
	Uptime    string
	RepoCount int
}

// TaskInput contains the task fields needed by the assembler.
type TaskInput struct {
	Filename                                                         string
	Status                                                           taskstore.Status
	Phase                                                            string
	ReviewFeedback                                                   bool
	Topic, Branch                                                    string
	ActiveWave, TotalWaves, SubtasksDone, SubtasksTotal, ReviewCycle int
	PRURL, PRCheckStatus, PRReviewDecision                           string
	// BlockedReason is non-empty when the task is stopped awaiting a human
	// answer. It is the whole point of the block: it must reach attention[]
	// verbatim, because a supervisor that only learns a task is blocked cannot
	// tell anyone what to decide.
	BlockedReason string
}

// AgentInput contains the agent fields needed by the assembler.
type AgentInput struct {
	Task             string
	Role             string
	Wave             int
	Ready            bool
	Active           bool
	Loading          bool
	HealthReason     string
	Worktree, Branch string
	TaskNumber       int
	LastActivity     *time.Time
	Paused           bool
}

// Assemble builds a deterministic live-status snapshot from the supplied state.
func Assemble(in Input) LiveStatus {
	now := in.Now.UTC()
	cap := in.Cap
	if cap <= 0 {
		cap = defaultCap
	}
	if cap > hardMaxCap {
		cap = hardMaxCap
	}

	var lifecycle LifecycleCounts
	for _, task := range in.Tasks {
		switch task.Status {
		case taskstore.StatusPlanning:
			lifecycle.Planning++
		case taskstore.StatusReady:
			lifecycle.Ready++
		case taskstore.StatusImplementing:
			lifecycle.Implementing++
		case taskstore.StatusReviewing:
			lifecycle.Reviewing++
		case taskstore.StatusVerifying:
			lifecycle.Verifying++
		default:
			continue
		}
		lifecycle.Total++
	}

	agentInputs := append([]AgentInput(nil), in.Agents...)
	sort.Slice(agentInputs, func(i, j int) bool {
		left, right := agentInputs[i], agentInputs[j]
		if left.Task != right.Task {
			return left.Task < right.Task
		}
		if left.Role != right.Role {
			return left.Role < right.Role
		}
		return left.Wave < right.Wave
	})

	agents := make([]ActiveAgent, 0, len(agentInputs))
	for _, agent := range agentInputs {
		worktree := ""
		if trimmed := strings.TrimSpace(agent.Worktree); trimmed != "" {
			worktree = filepath.Base(trimmed)
		}
		agents = append(agents, ActiveAgent{Task: agent.Task, Role: agent.Role, Wave: agent.Wave, Stage: stageFor(agent), Ready: agent.Ready, Active: agent.Active, Worktree: worktree, Branch: agent.Branch, TaskNumber: agent.TaskNumber, LastActivity: agent.LastActivity, Paused: agent.Paused})
	}
	var truncated Truncation
	if len(agents) > cap {
		truncated.ActiveAgents = len(agents) - cap
		agents = agents[:cap]
	}

	attention := make([]AttentionItem, 0)
	for _, task := range in.Tasks {
		if reason := strings.TrimSpace(task.BlockedReason); reason != "" {
			attention = append(attention, AttentionItem{Task: task.Filename, Kind: KindNeedsDecision, Detail: reason})
		} else if strings.TrimSpace(task.Phase) == string(taskfsm.ExecutionPhaseWaveWaiting) {
			attention = append(attention, AttentionItem{Task: task.Filename, Kind: KindNeedsDecision})
		}
		if task.ReviewFeedback && task.Status == taskstore.StatusImplementing {
			attention = append(attention, AttentionItem{Task: task.Filename, Kind: KindReviewFeedback})
		}
	}
	for _, agent := range agentInputs {
		if strings.TrimSpace(agent.HealthReason) != "" {
			attention = append(attention, AttentionItem{Task: agent.Task, Kind: KindStaleInstance, Detail: agent.HealthReason})
		}
	}
	blocked := make(map[string]bool, len(attention))
	for _, item := range attention {
		blocked[item.Task] = true
	}
	if len(attention) > cap {
		truncated.Attention = len(attention) - cap
		attention = attention[:cap]
	}
	result := LiveStatus{SchemaVersion: SchemaVersion, GeneratedAt: now, Project: in.Project, DaemonRunning: in.Daemon.Running, Uptime: in.Daemon.Uptime, RepoCount: in.Daemon.RepoCount, Lifecycle: lifecycle, ActiveAgents: agents, Attention: attention}
	if in.Include.Projects {
		result.Projects = append([]string(nil), in.Projects...)
		sort.Strings(result.Projects)
	}
	if in.Include.Tasks {
		for _, task := range in.Tasks {
			if !activeStatus(task.Status) {
				continue
			}
			result.Tasks = append(result.Tasks, TaskSummary{Filename: task.Filename, Status: string(task.Status), Phase: task.Phase, Topic: task.Topic, Branch: task.Branch, ActiveWave: task.ActiveWave, TotalWaves: task.TotalWaves, SubtasksDone: task.SubtasksDone, SubtasksTotal: task.SubtasksTotal, ReviewCycle: task.ReviewCycle, PRURL: task.PRURL, PRCheckStatus: task.PRCheckStatus, PRReviewDecision: task.PRReviewDecision, Blocked: blocked[task.Filename]})
		}
		sort.Slice(result.Tasks, func(i, j int) bool { return result.Tasks[i].Filename < result.Tasks[j].Filename })
		if len(result.Tasks) > cap {
			truncated.Tasks = len(result.Tasks) - cap
			result.Tasks = result.Tasks[:cap]
		}
	}
	if in.Include.Events {
		result.Events = append([]EventItem(nil), in.Events...)
		if len(result.Events) > cap {
			truncated.Events = len(result.Events) - cap
			result.Events = result.Events[:cap]
		}
	}
	if in.Include.Focus != "" && in.FocusTask != nil {
		f := in.FocusTask
		result.Focus = &TaskFocus{Filename: f.Filename, Goal: f.Goal, Waves: DeriveWaves(f.Content, f.Subtasks, f.ActiveWave), Readiness: f.Readiness}
	}
	result.Truncated = truncated
	return result
}

func activeStatus(status taskstore.Status) bool {
	switch status {
	case taskstore.StatusPlanning, taskstore.StatusReady, taskstore.StatusImplementing, taskstore.StatusReviewing, taskstore.StatusVerifying:
		return true
	}
	return false
}

func stageFor(agent AgentInput) string {
	switch {
	case agent.Loading:
		return "loading"
	case agent.Ready:
		return "ready"
	case agent.Active:
		return "running"
	default:
		return ""
	}
}
