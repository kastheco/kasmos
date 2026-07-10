// Package livestatus defines the canonical live orchestration status contract.
package livestatus

import (
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
)

const SchemaVersion = 1

const (
	defaultCap = 20
	hardMaxCap = 100
)

const (
	KindNeedsDecision  = "needs_decision"
	KindReviewFeedback = "review_feedback"
	KindStaleInstance  = "stale_instance"
)

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
}

type LifecycleCounts struct {
	Planning     int `json:"planning"`
	Ready        int `json:"ready"`
	Implementing int `json:"implementing"`
	Reviewing    int `json:"reviewing"`
	Verifying    int `json:"verifying"`
	Total        int `json:"total"`
}

type ActiveAgent struct {
	Task   string `json:"task"`
	Role   string `json:"role"`
	Wave   int    `json:"wave,omitempty"`
	Stage  string `json:"stage,omitempty"`
	Ready  bool   `json:"ready,omitempty"`
	Active bool   `json:"active,omitempty"`
}

type AttentionItem struct {
	Task   string `json:"task"`
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

type Truncation struct {
	ActiveAgents int `json:"active_agents,omitempty"`
	Attention    int `json:"attention,omitempty"`
}

type Input struct {
	Project string
	Now     time.Time
	Cap     int
	Daemon  DaemonHeartbeat
	Tasks   []TaskInput
	Agents  []AgentInput
}

type DaemonHeartbeat struct {
	Running   bool
	Uptime    string
	RepoCount int
}

type TaskInput struct {
	Filename       string
	Status         taskstore.Status
	Phase          string
	ReviewFeedback bool
}

type AgentInput struct {
	Task         string
	Role         string
	Wave         int
	Ready        bool
	Active       bool
	Loading      bool
	HealthReason string
}

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

	agents := make([]ActiveAgent, 0, len(in.Agents))
	for _, agent := range in.Agents {
		agents = append(agents, ActiveAgent{Task: agent.Task, Role: agent.Role, Wave: agent.Wave, Stage: stageFor(agent), Ready: agent.Ready, Active: agent.Active})
	}
	var truncated Truncation
	if len(agents) > cap {
		truncated.ActiveAgents = len(agents) - cap
		agents = agents[:cap]
	}

	attention := make([]AttentionItem, 0)
	for _, task := range in.Tasks {
		if strings.TrimSpace(task.Phase) == string(taskfsm.ExecutionPhaseWaveWaiting) {
			attention = append(attention, AttentionItem{Task: task.Filename, Kind: KindNeedsDecision})
		}
		if task.ReviewFeedback {
			attention = append(attention, AttentionItem{Task: task.Filename, Kind: KindReviewFeedback})
		}
	}
	for _, agent := range in.Agents {
		if strings.TrimSpace(agent.HealthReason) != "" {
			attention = append(attention, AttentionItem{Task: agent.Task, Kind: KindStaleInstance, Detail: agent.HealthReason})
		}
	}
	if len(attention) > cap {
		truncated.Attention = len(attention) - cap
		attention = attention[:cap]
	}

	return LiveStatus{SchemaVersion: SchemaVersion, GeneratedAt: now, Project: in.Project, DaemonRunning: in.Daemon.Running, Uptime: in.Daemon.Uptime, RepoCount: in.Daemon.RepoCount, Lifecycle: lifecycle, ActiveAgents: agents, Attention: attention, Truncated: truncated}
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
