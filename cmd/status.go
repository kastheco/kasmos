package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/lineartrigger"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/spf13/cobra"
)

// statusTask is the JSON-serialisable representation of a task entry for the
// status command output.
type statusTask struct {
	Name              string `json:"name"`
	Status            string `json:"status"`
	Stage             string `json:"stage,omitempty"`
	Phase             string `json:"phase,omitempty"`
	ActiveAgentType   string `json:"active_agent_type,omitempty"`
	ActiveWave        int    `json:"active_wave,omitempty"`
	ReviewCycle       int    `json:"review_cycle,omitempty"`
	HasReviewFeedback bool   `json:"has_review_feedback,omitempty"`
	HasInstance       bool   `json:"has_instance"`
	StaleInstance     bool   `json:"stale_instance"`
	Branch            string `json:"branch"`
}

// statusInstance is the JSON-serialisable representation of an instance record
// for the status command output.
type statusInstance struct {
	Title   string `json:"title"`
	Status  string `json:"status"`
	Program string `json:"program"`
	Task    string `json:"task,omitempty"`
	Type    string `json:"type,omitempty"`
}

// statusOrphan is the JSON-serialisable representation of an orphan tmux session
// for the status command output.
type statusOrphan struct {
	Name string `json:"name"`
	Age  string `json:"age"`
}

type statusLinearTriggers struct {
	Enabled    bool   `json:"enabled"`
	LastPoll   string `json:"last_poll,omitempty"`
	Dispatched int    `json:"dispatched,omitempty"`
	Rejected   int    `json:"rejected,omitempty"`
	Errors     int    `json:"errors,omitempty"`
}

type statusLinearWebhooks struct {
	Enabled      bool   `json:"enabled"`
	LastDelivery string `json:"last_delivery,omitempty"`
	Accepted     int    `json:"accepted,omitempty"`
	Duplicate    int    `json:"duplicate,omitempty"`
	Ignored      int    `json:"ignored,omitempty"`
	Rejected     int    `json:"rejected,omitempty"`
	Errors       int    `json:"errors,omitempty"`
}

// statusData is the top-level JSON structure returned by executeStatus when
// format == "json".
type statusData struct {
	Tasks          []statusTask         `json:"tasks"`
	Instances      []statusInstance     `json:"instances"`
	OrphanSessions []statusOrphan       `json:"orphan_sessions"`
	LinearTriggers statusLinearTriggers `json:"linear_triggers"`
	LinearWebhooks statusLinearWebhooks `json:"linear_webhooks"`
}

func statusAgentLabel(agent string) string {
	switch strings.TrimSpace(agent) {
	case "elaborator":
		return "architect"
	default:
		return strings.TrimSpace(agent)
	}
}

func statusDisplayReviewRound(entry taskstore.TaskEntry) int {
	phase := strings.TrimSpace(entry.ExecutionState.Phase)
	if phase == string(taskfsm.ExecutionPhaseFixing) || phase == string(taskfsm.ExecutionPhaseReviewing) || entry.Status == taskstore.StatusReviewing {
		return entry.ReviewCycle + 1
	}
	return 0
}

func statusStageForTask(entry taskstore.TaskEntry) string {
	phase := strings.TrimSpace(entry.ExecutionState.Phase)
	round := statusDisplayReviewRound(entry)
	switch phase {
	case string(taskfsm.ExecutionPhasePlanned):
		return "planned"
	case string(taskfsm.ExecutionPhaseArchitecting):
		return "architecting"
	case string(taskfsm.ExecutionPhaseWaveRunning):
		if entry.ExecutionState.ActiveWave > 0 {
			return fmt.Sprintf("wave %d running", entry.ExecutionState.ActiveWave)
		}
		return "wave running"
	case string(taskfsm.ExecutionPhaseWaveWaiting):
		return "waiting for confirmation"
	case string(taskfsm.ExecutionPhaseSingleAgentImplementing):
		return "implementing"
	case string(taskfsm.ExecutionPhaseFixing):
		if round > 0 {
			return fmt.Sprintf("fixing round %d", round)
		}
		return "fixing"
	case string(taskfsm.ExecutionPhaseReviewing):
		if round > 0 {
			return fmt.Sprintf("reviewing round %d", round)
		}
		return "reviewing"
	}
	return string(entry.Status)
}

func statusRecoveryHints(tasks []statusTask) []string {
	var hints []string
	seen := make(map[string]struct{})
	appendHint := func(hint string) {
		if hint == "" {
			return
		}
		if _, ok := seen[hint]; ok {
			return
		}
		seen[hint] = struct{}{}
		hints = append(hints, hint)
	}

	for _, t := range tasks {
		phase := strings.TrimSpace(t.Phase)
		switch {
		case phase == "dirty_worktree":
			appendHint("  kas task recover <task-name> --action implement-finished               # discard dirty worktree and resume review")
		case phase == string(taskfsm.ExecutionPhaseWaveWaiting):
			appendHint("  kas task recover <task-name> --action advance-wave                      # advance to next wave after decision")
			appendHint("  kas task recover <task-name> --action retry-wave                        # re-run failed tasks in the current wave")
		case t.Status == string(taskstore.StatusImplementing) && t.StaleInstance:
			appendHint("  kas instance restart <title>                                            # stale paused instance - restart to continue")
		case t.Status == string(taskstore.StatusPlanning):
			appendHint("  kas task recover <task-name> --action planner-finished                # finish planning safely")
		case phase == string(taskfsm.ExecutionPhaseArchitecting):
			appendHint("  kas task recover <task-name> --action architect-finished              # resume architect handoff")
		case phase == string(taskfsm.ExecutionPhaseFixing) || phase == string(taskfsm.ExecutionPhaseSingleAgentImplementing):
			appendHint("  kas task recover <task-name> --action implement-finished             # hand implementation to review")
		case t.Status == string(taskstore.StatusImplementing) && !t.HasInstance:
			appendHint("  kas task implement <task-name>                                          # spawn a coder (no implementing instance found)")
		case t.Status == string(taskstore.StatusVerifying):
			appendHint("  kas task recover <task-name> --action verify-approved                     # approve verification and mark done")
			appendHint("  kas task recover <task-name> --action verify-failed --feedback ...         # send back to implementation")
		case phase == string(taskfsm.ExecutionPhaseReviewing) || t.Status == string(taskstore.StatusReviewing):
			appendHint("  kas task recover <task-name> --action review-approved                # finish review")
			appendHint("  kas task recover <task-name> --action review-changes --feedback ...  # queue fixer recovery")
			appendHint("  kas task recover <task-name> --action advance-review-cycle --feedback ...  # persist next review round")
		}
		if t.Status == string(taskstore.StatusReady) {
			appendHint("  kas task implement <task-name>                                        # start implementing a ready task")
		}
	}

	return hints
}

var statusStaleInstanceThreshold = 24 * time.Hour

func isStatusRecordStale(r instanceRecord, now time.Time) bool {
	if r.Status != instancePaused || r.UpdatedAt.IsZero() {
		return false
	}
	return now.Sub(r.UpdatedAt) > statusStaleInstanceThreshold
}

func annotateStatusTasksWithInstances(tasks []statusTask, records []instanceRecord, now time.Time) {
	for ti := range tasks {
		for _, r := range records {
			if r.TaskFile != tasks[ti].Name {
				continue
			}
			tasks[ti].HasInstance = true
			if isStatusRecordStale(r, now) {
				tasks[ti].StaleInstance = true
			}
		}
	}
}

type linearTriggerStatsReader interface {
	LinearTriggerStats(project string, since time.Time) (taskstore.LinearTriggerStats, error)
}

type webhookStatsReader interface {
	LinearWebhookStats(project string, since time.Time) (taskstore.LinearWebhookStats, error)
}

func statusLinearTriggerSummary(store taskstore.Store, project string, cfg lineartrigger.Config, now time.Time) statusLinearTriggers {
	if !cfg.Enabled {
		return statusLinearTriggers{}
	}
	summary := statusLinearTriggers{Enabled: true}
	reader, ok := store.(linearTriggerStatsReader)
	if !ok || reader == nil {
		return summary
	}
	stats, err := reader.LinearTriggerStats(project, now.Add(-24*time.Hour))
	if err != nil {
		return summary
	}
	if !stats.LastSeenAt.IsZero() {
		summary.LastPoll = relativeAge(now, stats.LastSeenAt)
	}
	summary.Dispatched = stats.Dispatched
	summary.Rejected = stats.Rejected
	summary.Errors = stats.Failed
	return summary
}

func statusLinearWebhookSummary(store taskstore.Store, project string, cfg lineartrigger.Config, now time.Time) statusLinearWebhooks {
	if !cfg.Webhook.Enabled {
		return statusLinearWebhooks{}
	}
	summary := statusLinearWebhooks{Enabled: true}
	reader, ok := store.(webhookStatsReader)
	if !ok || reader == nil {
		return summary
	}
	stats, err := reader.LinearWebhookStats(project, now.Add(-24*time.Hour))
	if err != nil {
		return summary
	}
	if !stats.LastDeliveryAt.IsZero() {
		summary.LastDelivery = relativeAge(now, stats.LastDeliveryAt)
	}
	summary.Accepted = stats.Accepted
	summary.Duplicate = stats.Duplicate
	summary.Ignored = stats.Ignored
	summary.Rejected = stats.Rejected
	summary.Errors = stats.Failed
	return summary
}

func renderStatusLinearTriggers(summary statusLinearTriggers) string {
	if !summary.Enabled {
		return "linear triggers: disabled\n"
	}
	lastPoll := "never"
	if summary.LastPoll != "" {
		lastPoll = summary.LastPoll + " ago"
	}
	return fmt.Sprintf("linear triggers: enabled (last poll %s, %d dispatched, %d rejected, %d errors)\n",
		lastPoll, summary.Dispatched, summary.Rejected, summary.Errors)
}

func renderStatusLinearWebhooks(summary statusLinearWebhooks) string {
	if !summary.Enabled {
		return "linear webhooks: disabled\n"
	}
	lastDelivery := "never"
	if summary.LastDelivery != "" {
		lastDelivery = summary.LastDelivery + " ago"
	}
	return fmt.Sprintf("linear webhooks: enabled (last delivery %s, %d accepted, %d duplicate, %d ignored, %d rejected, %d errors)\n",
		lastDelivery, summary.Accepted, summary.Duplicate, summary.Ignored, summary.Rejected, summary.Errors)
}

func currentRepoLinearTriggerStatusLine(now time.Time) (string, error) {
	_, project, err := resolveRepoInfo()
	if err != nil {
		return "", err
	}
	tomlResult, err := config.LoadTOMLConfig()
	if err != nil {
		return "", err
	}
	var triggerCfg lineartrigger.Config
	if tomlResult != nil {
		triggerCfg = tomlResult.LinearTriggers
	}
	store, err := taskstore.OpenAuthoritativeStore(project)
	if err != nil {
		return "", err
	}
	defer store.Close()
	return renderStatusLinearTriggers(statusLinearTriggerSummary(store, project, triggerCfg, now)), nil
}

// executeStatus assembles a unified overview of active tasks, agent instances,
// and orphan tmux sessions. It is the testable core of NewStatusCmd.
//
// Parameters:
//   - state: instance state manager (required)
//   - store: task store; may be nil (tasks section shows "no active tasks")
//   - project: project name used for store queries
//   - ex: executor for tmux discovery
//   - format: "text" or "json"
func executeStatus(state config.StateManager, store taskstore.Store, project string, ex Executor, format string) string {
	tomlResult, _ := config.LoadTOMLConfig()
	var triggerCfg lineartrigger.Config
	if tomlResult != nil {
		triggerCfg = tomlResult.LinearTriggers
	}
	return executeStatusWithLinearTriggers(state, store, project, ex, format, triggerCfg, time.Now())
}

func executeStatusWithLinearTriggers(state config.StateManager, store taskstore.Store, project string, ex Executor, format string, triggerCfg lineartrigger.Config, now time.Time) string {
	// 1. Tasks section — filter to non-done, non-cancelled entries.
	tasks := make([]statusTask, 0)
	if store != nil {
		entries, err := store.List(project)
		if err == nil {
			for _, e := range entries {
				if e.Status == taskstore.StatusCancelled || e.Status == taskstore.StatusDone {
					continue
				}
				tasks = append(tasks, statusTask{
					Name:              e.Filename,
					Status:            string(e.Status),
					Stage:             statusStageForTask(e),
					Phase:             strings.TrimSpace(e.ExecutionState.Phase),
					ActiveAgentType:   statusAgentLabel(e.ExecutionState.ActiveAgentType),
					ActiveWave:        e.ExecutionState.ActiveWave,
					ReviewCycle:       statusDisplayReviewRound(e),
					HasReviewFeedback: strings.TrimSpace(e.LatestReviewFeedback) != "",
					Branch:            e.Branch,
				})
			}
		}
	}

	// 2. Instances section.
	// Load records once; reuse them for orphan detection below to avoid a
	// second deserialisation inside buildKnownNames.
	instances := make([]statusInstance, 0)
	records, recordsErr := loadInstanceRecords(state)
	if recordsErr == nil {
		annotateStatusTasksWithInstances(tasks, records, now)
		for _, r := range records {
			agentType := r.AgentType
			if agentType == "" && (r.SoloAgent || r.TaskFile == "") {
				agentType = "solo"
			}
			instances = append(instances, statusInstance{
				Title:   r.Title,
				Status:  statusLabel(r.Status),
				Program: r.Program,
				Task:    r.TaskFile,
				Type:    agentType,
			})
		}
	}

	// 3. Orphan sessions section.
	// Build known names from the already-loaded records instead of calling
	// buildKnownNames (which would deserialise the state a second time).
	orphans := make([]statusOrphan, 0)
	if recordsErr == nil {
		known := make(map[string]struct{}, len(records))
		for _, r := range records {
			known[kasTmuxName(r.Title)] = struct{}{}
		}
		rows, discErr := discoverKasSessions(ex, known)
		if discErr == nil {
			for _, row := range rows {
				if !row.Managed {
					orphans = append(orphans, statusOrphan{
						Name: row.Name,
						Age:  relativeAge(now, row.Created),
					})
				}
			}
		}
	}

	data := statusData{
		Tasks:          tasks,
		Instances:      instances,
		OrphanSessions: orphans,
		LinearTriggers: statusLinearTriggerSummary(store, project, triggerCfg, now),
		LinearWebhooks: statusLinearWebhookSummary(store, project, triggerCfg, now),
	}

	// 4. JSON format.
	if format == "json" {
		b, err := json.Marshal(data)
		if err != nil {
			return fmt.Sprintf(`{"error": %q}`, err.Error())
		}
		return string(b)
	}

	// 5. Text format.
	var sb strings.Builder

	// Tasks section.
	sb.WriteString("tasks:\n")
	if len(tasks) == 0 {
		sb.WriteString("  no active tasks\n")
	} else {
		w := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  STATUS\tLIFECYCLE\tAGENT\tFEEDBACK\tNAME\tBRANCH")
		for _, t := range tasks {
			feedback := "-"
			if t.HasReviewFeedback {
				feedback = "yes"
			}
			agent := t.ActiveAgentType
			if agent == "" {
				agent = "-"
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\n", t.Status, t.Stage, agent, feedback, t.Name, t.Branch)
		}
		w.Flush()
	}

	sb.WriteString("\n")

	// Instances section.
	sb.WriteString("instances:\n")
	if len(instances) == 0 {
		sb.WriteString("  no instances\n")
	} else {
		w := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  TITLE\tSTATUS\tPROGRAM\tTASK\tTYPE")
		for _, i := range instances {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", i.Title, i.Status, i.Program, i.Task, i.Type)
		}
		w.Flush()
	}

	sb.WriteString("\n")

	// Orphan sessions section.
	sb.WriteString("orphan tmux sessions:\n")
	if len(orphans) == 0 {
		sb.WriteString("  no orphan tmux sessions\n")
	} else {
		w := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tAGE")
		for _, o := range orphans {
			fmt.Fprintf(w, "  %s\t%s\n", o.Name, o.Age)
		}
		w.Flush()
	}

	sb.WriteString("\n")
	sb.WriteString(renderStatusLinearTriggers(data.LinearTriggers))
	sb.WriteString(renderStatusLinearWebhooks(data.LinearWebhooks))

	// Hints section — only shown when at least one condition applies.
	hints := statusRecoveryHints(tasks)
	for _, i := range instances {
		if i.Status == "paused" {
			hints = append(hints, "  kas instance resume <title>       # resume a paused instance")
			break
		}
	}
	if len(orphans) > 0 {
		hints = append(hints, "  kas tmux adopt <session> <title>  # adopt an orphan tmux session")
		hints = append(hints, "  kas tmux kill <session>            # kill an orphan tmux session")
	}
	if len(hints) > 0 {
		sb.WriteString("\nhints:\n")
		for _, h := range hints {
			sb.WriteString(h + "\n")
		}
	}

	return sb.String()
}

// NewStatusCmd builds the `kas status` cobra command.
func NewStatusCmd() *cobra.Command {
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"st"},
		Short:   "show overview of tasks, instances, and orphan tmux sessions",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			state := config.LoadState()
			store, err := taskstore.OpenAuthoritativeStore(project)
			if err != nil {
				return err
			}
			defer store.Close()
			format := "text"
			if jsonFlag {
				format = "json"
			}
			fmt.Print(executeStatus(state, store, project, MakeExecutor(), format))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	return cmd
}
