package loop

import (
	"fmt"
	"strings"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
)

func (p *Processor) taskEntry(planFile string) (taskstore.TaskEntry, bool) {
	if p.config.Store == nil {
		return taskstore.TaskEntry{}, false
	}
	entry, err := p.config.Store.Get(p.config.Project, planFile)
	if err != nil {
		return taskstore.TaskEntry{}, false
	}
	return entry, true
}

func executionPhaseForEntry(entry taskstore.TaskEntry) taskfsm.ExecutionPhase {
	return taskfsm.ExecutionPhase(strings.TrimSpace(entry.ExecutionState.Phase))
}

// preAppliedTargetStatus returns the FSM target status for a signal-bearing
// event whose originator (e.g. the HTTP admin handler) may have applied the
// transition before emitting the gateway row. Only signal-bearing lifecycle
// events are covered; every entry mirrors the transition table at
// config/taskfsm/fsm.go:96-129 and must stay in sync with it.
func preAppliedTargetStatus(event taskfsm.Event) (taskfsm.Status, bool) {
	switch event {
	case taskfsm.PlanStart:
		return taskfsm.StatusPlanning, true
	case taskfsm.PlannerFinished:
		return taskfsm.StatusReady, true
	case taskfsm.ImplementFinished:
		return taskfsm.StatusReviewing, true
	case taskfsm.ReviewApproved:
		return taskfsm.StatusVerifying, true
	case taskfsm.ReviewChangesRequested:
		return taskfsm.StatusImplementing, true
	case taskfsm.VerifyApproved:
		return taskfsm.StatusDone, true
	case taskfsm.VerifyFailed:
		return taskfsm.StatusImplementing, true
	}
	return "", false
}

func (p *Processor) setExecutionState(planFile string, state taskstore.ExecutionState) error {
	if p.config.Store == nil {
		return fmt.Errorf("nil task store")
	}
	writer, ok := p.config.Store.(taskstore.ExecutionStateWriter)
	if !ok {
		return fmt.Errorf("task store does not support execution state writes")
	}
	return writer.SetExecutionState(p.config.Project, planFile, state)
}

// ProcessorConfig holds the dependencies needed to construct a Processor.
type ProcessorConfig struct {
	// Store is the plan state persistence backend. Must be non-nil.
	Store taskstore.Store
	// Project is the project name used with the store.
	Project string
	// Dir is the legacy directory path used for file rename operations (may be empty).
	Dir string
	// AutoReviewFix enables automatic fixer spawning after review changes.
	AutoReviewFix bool
	// AutoAdvance enables automatic planning→implementing handoff.
	AutoAdvance bool
	// AutoReadinessReview enables the post-reviewer master-agent readiness gate.
	// When true, a reviewer approval (non-master origin) is intercepted and converted
	// into a SpawnMasterAction instead of flowing directly to ReviewApprovedAction.
	AutoReadinessReview bool
	// MaxReviewFixCycles is the maximum number of review-fix cycles allowed
	// before emitting ReviewCycleLimitAction instead of SpawnCoderAction.
	// Zero or negative means unlimited.
	MaxReviewFixCycles int
	// Hooks is an optional registry of FSM transition hooks. When non-nil and
	// non-empty it is attached to the FSM so hooks fire after every successful
	// state write.
	Hooks *taskfsm.HookRegistry
}

// Processor converts signal scan results into typed Action values without
// performing side effects. The caller is responsible for executing the returned
// actions (spawning agents, creating PRs, etc.).
type Processor struct {
	config            ProcessorConfig
	fsm               *taskfsm.TaskStateMachine
	waveOrchestrators map[string]*orchestration.WaveOrchestrator
	// activeWaveOrchs tracks plans whose wave orchestrator is active.
	// ImplementFinished signals are suppressed for plans in this set so that
	// individual wave-task agents don't prematurely trigger the reviewing state.
	activeWaveOrchs map[string]bool
}

// NewProcessor creates a Processor backed by the given store and project.
func NewProcessor(cfg ProcessorConfig) *Processor {
	fsm := taskfsm.New(cfg.Store, cfg.Project, cfg.Dir)
	if cfg.Hooks != nil && cfg.Hooks.Len() > 0 {
		fsm.SetHooks(cfg.Hooks)
	}
	return &Processor{
		config:            cfg,
		fsm:               fsm,
		waveOrchestrators: make(map[string]*orchestration.WaveOrchestrator),
		activeWaveOrchs:   make(map[string]bool),
	}
}

// SetReviewFixConfig updates the runtime review-fix loop settings.
func (p *Processor) SetReviewFixConfig(enabled bool, maxCycles int) {
	p.config.AutoReviewFix = enabled
	p.config.MaxReviewFixCycles = maxCycles
}

// SetReadinessReviewConfig updates the runtime readiness-review gate setting.
func (p *Processor) SetReadinessReviewConfig(enabled bool) {
	p.config.AutoReadinessReview = enabled
}

// SetWaveOrchestratorActive marks or unmarks a plan as having an active wave
// orchestrator. When active, ImplementFinished signals for that plan are
// suppressed in ProcessFSMSignals.
func (p *Processor) SetWaveOrchestratorActive(planFile string, active bool) {
	if active {
		p.activeWaveOrchs[planFile] = true
	} else {
		delete(p.activeWaveOrchs, planFile)
	}
}

// SyncWaveOrchestrators replaces the processor's orchestrator registry with the
// caller's current orchestrator set.
func (p *Processor) SyncWaveOrchestrators(orchestrators map[string]*orchestration.WaveOrchestrator) {
	p.waveOrchestrators = make(map[string]*orchestration.WaveOrchestrator, len(orchestrators))
	for planFile, orch := range orchestrators {
		p.waveOrchestrators[planFile] = orch
	}
	for planFile := range p.activeWaveOrchs {
		if _, ok := orchestrators[planFile]; !ok {
			delete(p.activeWaveOrchs, planFile)
		}
	}
}

// RegisterOrchestrator creates a wave orchestrator for the given plan with the
// specified wave number and task numbers in the running state. Intended for
// tests and daemon restore operations.
func (p *Processor) RegisterOrchestrator(planFile string, waveNumber int, taskNumbers []int) {
	tasks := make([]taskparser.Task, len(taskNumbers))
	for i, n := range taskNumbers {
		tasks[i] = taskparser.Task{Number: n, Title: fmt.Sprintf("Task %d", n)}
	}
	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{{Number: waveNumber, Tasks: tasks}},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	if p.config.Store != nil {
		orch.SetStore(p.config.Store, p.config.Project)
	}
	orch.StartNextWave() // puts tasks into running state
	p.waveOrchestrators[planFile] = orch
}

// WaveOrchestrator returns the active WaveOrchestrator for the given plan file,
// or nil if none is registered.
func (p *Processor) WaveOrchestrator(planFile string) *orchestration.WaveOrchestrator {
	return p.waveOrchestrators[planFile]
}

// ClearWaveOrchestrator removes all orchestration state for the given plan.
// Use this once a wave flow is finished or when handoff should be recreated
// from a fresh implement-wave signal.
func (p *Processor) ClearWaveOrchestrator(planFile string) {
	delete(p.waveOrchestrators, planFile)
	delete(p.activeWaveOrchs, planFile)
}

// ProcessFSMSignals converts FSM sentinel signals into Action values.
// It validates each signal against the plan state machine, suppresses
// ImplementFinished when a wave orchestrator is active, and emits typed
// actions for the caller to execute.
//
// Extracted from app.go metadataResultMsg handler (lines 921-1077).
func (p *Processor) ProcessFSMSignals(signals []taskfsm.Signal) []Action {
	var actions []Action
	for _, sig := range signals {
		// Guard: suppress ImplementFinished when a wave orchestrator is active.
		// Wave task agents write this sentinel after each task, but the wave
		// orchestrator owns the implementing→reviewing transition.
		if sig.Event == taskfsm.ImplementFinished {
			if entry, ok := p.taskEntry(sig.TaskFile); ok {
				phase := executionPhaseForEntry(entry)
				if phase == taskfsm.ExecutionPhaseWaveRunning || phase == taskfsm.ExecutionPhaseWaveWaiting {
					continue
				}
			}
			if p.activeWaveOrchs[sig.TaskFile] {
				continue
			}
			if _, hasOrch := p.waveOrchestrators[sig.TaskFile]; hasOrch {
				continue
			}
		}

		alreadyApplied := false
		if err := p.fsm.Transition(sig.TaskFile, sig.Event); err != nil {
			// The signal's originator (e.g. the HTTP admin handler) may have
			// applied the FSM transition itself before emitting the gateway row
			// and marked the payload with fsm_applied=true. If so, the daemon
			// must still emit the downstream actions — spawn reviewer, spawn
			// master, spawn fixer, create PR, etc. — because the HTTP handler
			// only writes state; it never drives the daemon side effects.
			//
			// The PreApplied gate matters: it keeps the existing drop behaviour
			// for stale / out-of-order signals from the filesystem bridge and
			// MCP signal_create where the daemon remains the sole FSM driver,
			// even when the task happens to land in a state that matches the
			// signal's target (see TestProcessor_ProcessFSMSignals_InvalidReviewChangesRequested_HasNoActions).
			if sig.PreApplied {
				if target, ok := preAppliedTargetStatus(sig.Event); ok {
					if entry, entryOK := p.taskEntry(sig.TaskFile); entryOK {
						if taskfsm.Status(entry.Status) == target {
							alreadyApplied = true
						}
					}
				}
			}
			if !alreadyApplied {
				continue
			}
		}

		switch sig.Event {
		case taskfsm.ImplementFinished:
			actions = append(actions, SpawnReviewerAction{PlanFile: sig.TaskFile})

		case taskfsm.ReviewApproved:
			// ReviewApproved transitions reviewing → verifying.
			// Always emit ReviewApprovedAction first: it carries reviewer side-effects
			// (audit log, toast, ClickUp progress, reviewer pause) that are independent
			// of whether the task waits for a master agent or completes immediately.
			actions = append(actions, ReviewApprovedAction{
				PlanFile:   sig.TaskFile,
				ReviewBody: sig.Body,
			})
			if p.config.AutoReadinessReview {
				// Readiness gate active: spawn master agent for holistic check.
				// VerifyApprovedAction side-effects fire when VerifyApproved arrives.
				actions = append(actions, SpawnMasterAction{PlanFile: sig.TaskFile})
				break
			}
			// No readiness gate: chain verify_approved immediately so the task moves
			// from verifying → done inside the processor (single FSM driver).
			if err := p.fsm.Transition(sig.TaskFile, taskfsm.VerifyApproved); err == nil {
				actions = append(actions, VerifyApprovedAction{
					PlanFile:   sig.TaskFile,
					ReviewBody: sig.Body,
				})
				if p.config.Store != nil {
					if entry, err := p.config.Store.Get(p.config.Project, sig.TaskFile); err == nil {
						if shouldCreatePR(entry) {
							actions = append(actions, CreatePRAction{
								PlanFile:   sig.TaskFile,
								ReviewBody: sig.Body,
							})
						}
					}
				}
			}

		case taskfsm.VerifyApproved:
			// VerifyApproved transitions verifying → done (emitted by master agent).
			actions = append(actions, VerifyApprovedAction{
				PlanFile:   sig.TaskFile,
				ReviewBody: sig.Body,
			})
			if p.config.Store != nil {
				if entry, err := p.config.Store.Get(p.config.Project, sig.TaskFile); err == nil {
					if shouldCreatePR(entry) {
						actions = append(actions, CreatePRAction{
							PlanFile:   sig.TaskFile,
							ReviewBody: sig.Body,
						})
					}
				}
			}

		case taskfsm.VerifyFailed:
			// VerifyFailed transitions verifying → implementing (emitted by master agent).
			actions = append(actions, VerifyFailedAction{
				PlanFile: sig.TaskFile,
				Feedback: sig.Body,
			})
			if !p.config.AutoReviewFix {
				break
			}
			if p.config.MaxReviewFixCycles > 0 && p.config.Store != nil {
				if entry, err := p.config.Store.Get(p.config.Project, sig.TaskFile); err == nil {
					if entry.ReviewCycle+1 > p.config.MaxReviewFixCycles {
						actions = append(actions, ReviewCycleLimitAction{
							PlanFile: sig.TaskFile,
							Cycle:    entry.ReviewCycle + 1,
							Limit:    p.config.MaxReviewFixCycles,
						})
						break
					}
				}
			}
			actions = append(actions, IncrementReviewCycleAction{PlanFile: sig.TaskFile})
			actions = append(actions, SpawnFixerAction{
				PlanFile: sig.TaskFile,
				Feedback: sig.Body,
			})

		case taskfsm.ReviewChangesRequested:
			actions = append(actions, ReviewChangesAction{
				PlanFile: sig.TaskFile,
				Feedback: sig.Body,
			})
			if !p.config.AutoReviewFix {
				break
			}
			if p.config.MaxReviewFixCycles > 0 && p.config.Store != nil {
				if entry, err := p.config.Store.Get(p.config.Project, sig.TaskFile); err == nil {
					if entry.ReviewCycle+1 > p.config.MaxReviewFixCycles {
						actions = append(actions, ReviewCycleLimitAction{
							PlanFile: sig.TaskFile,
							Cycle:    entry.ReviewCycle + 1,
							Limit:    p.config.MaxReviewFixCycles,
						})
						break // don't spawn fixer
					}
				}
			}
			actions = append(actions, IncrementReviewCycleAction{PlanFile: sig.TaskFile})
			actions = append(actions, SpawnFixerAction{
				PlanFile: sig.TaskFile,
				Feedback: sig.Body,
			})

		case taskfsm.PlannerFinished:
			actions = append(actions, PlannerCompleteAction{PlanFile: sig.TaskFile})
			if p.config.AutoAdvance {
				actions = append(actions, AutoImplementAction{PlanFile: sig.TaskFile})
			}

		case taskfsm.PlanStart:
			actions = append(actions, SpawnPlannerAction{PlanFile: sig.TaskFile})
		}
	}
	return actions
}

// ProcessTaskSignals converts wave-task completion sentinel signals into
// TaskCompleteAction values.
//
// Extracted from app.go metadataResultMsg handler (lines 1080-1107).
func (p *Processor) ProcessTaskSignals(signals []taskfsm.TaskSignal) []Action {
	var actions []Action
	for _, ts := range signals {
		orch, exists := p.waveOrchestrators[ts.TaskFile]
		if !exists {
			orch = p.restoreOrchestratorForTaskSignal(ts.TaskFile, ts.WaveNumber)
			if orch == nil {
				continue
			}
		}
		if ts.WaveNumber != orch.CurrentWaveNumber() {
			continue
		}
		if !orch.IsTaskRunning(ts.TaskNumber) {
			continue
		}
		orch.MarkTaskComplete(ts.TaskNumber)
		actions = append(actions, TaskCompleteAction{
			PlanFile:   ts.TaskFile,
			TaskNumber: ts.TaskNumber,
			WaveNumber: ts.WaveNumber,
		})
	}
	return actions
}

func (p *Processor) restoreOrchestratorForTaskSignal(planFile string, waveNumber int) *orchestration.WaveOrchestrator {
	if p.config.Store == nil || waveNumber < 1 {
		return nil
	}

	content, err := p.config.Store.GetContent(p.config.Project, planFile)
	if err != nil {
		return nil
	}
	plan, err := taskparser.Parse(content)
	if err != nil {
		return nil
	}
	if waveNumber > len(plan.Waves) {
		return nil
	}

	subtasks, err := p.config.Store.GetSubtasks(p.config.Project, planFile)
	if err != nil || len(subtasks) == 0 {
		return nil
	}

	taskToWave := make(map[int]int)
	for _, wave := range plan.Waves {
		for _, task := range wave.Tasks {
			taskToWave[task.Number] = wave.Number
		}
	}

	completed := make([]int, 0)
	failed := make([]int, 0)
	hasRunning := false
	waveTaskCount := 0
	for _, subtask := range subtasks {
		if taskToWave[subtask.TaskNumber] != waveNumber {
			continue
		}
		waveTaskCount++

		switch subtask.Status {
		case taskstore.SubtaskStatusRunning:
			hasRunning = true
		case taskstore.SubtaskStatusComplete, taskstore.SubtaskStatusDone, taskstore.SubtaskStatusClosed:
			completed = append(completed, subtask.TaskNumber)
		case taskstore.SubtaskStatusFailed:
			failed = append(failed, subtask.TaskNumber)
		}
	}
	// Don't restore if there are no tasks in this wave or if all tasks are
	// already resolved (complete/done/failed). Without a running task the
	// signal is stale and restoring would flicker subtask statuses.
	if waveTaskCount == 0 || (!hasRunning && len(completed)+len(failed) == waveTaskCount) {
		return nil
	}

	orch := orchestration.NewWaveOrchestrator(planFile, plan)
	orch.SetStore(p.config.Store, p.config.Project)
	orch.RestoreToWave(waveNumber, completed)
	for _, taskNumber := range failed {
		orch.MarkTaskFailed(taskNumber)
	}

	p.waveOrchestrators[planFile] = orch
	p.activeWaveOrchs[planFile] = true
	return orch
}

// ProcessWaveSignals converts implement-wave sentinel signals into
// AdvanceWaveAction values. It reads the plan from the store, creates a
// WaveOrchestrator, fast-forwards to the requested wave, and emits the action.
//
// Extracted from app.go metadataResultMsg handler (lines 1142-1191).
func (p *Processor) ProcessWaveSignals(signals []taskfsm.WaveSignal) []Action {
	var actions []Action
	for _, ws := range signals {
		if entry, ok := p.taskEntry(ws.TaskFile); ok {
			phase := executionPhaseForEntry(entry)
			if phase == taskfsm.ExecutionPhaseWaveRunning && entry.ExecutionState.ActiveWave == ws.WaveNumber {
				continue
			}
		}

		// Reject if an orchestrator is already running for this plan.
		if _, exists := p.waveOrchestrators[ws.TaskFile]; exists {
			continue
		}

		// Read and parse the plan from the store.
		content, err := p.config.Store.GetContent(p.config.Project, ws.TaskFile)
		if err != nil {
			continue
		}
		plan, err := taskparser.Parse(content)
		if err != nil {
			continue
		}
		if ws.WaveNumber > len(plan.Waves) {
			continue
		}

		orch := orchestration.NewWaveOrchestrator(ws.TaskFile, plan)
		if p.config.Store != nil {
			orch.SetStore(p.config.Store, p.config.Project)
		}
		p.waveOrchestrators[ws.TaskFile] = orch
		p.activeWaveOrchs[ws.TaskFile] = true

		// Fast-forward through earlier waves (all tasks auto-completed).
		for i := 1; i < ws.WaveNumber; i++ {
			tasks := orch.StartNextWave()
			for _, t := range tasks {
				orch.MarkTaskComplete(t.Number)
			}
		}

		// Start the requested wave.
		orch.StartNextWave()

		actions = append(actions, AdvanceWaveAction{
			PlanFile: ws.TaskFile,
			Wave:     ws.WaveNumber,
		})
	}
	return actions
}

// ProcessElaborationSignals converts architect-pass completion signals carried
// over the retained elaborator_finished contract into AdvanceWaveAction values.
// It re-reads the architect-enriched plan from the store, updates the
// orchestrator, persists the wave-running execution phase, and emits the action
// that starts wave 1.
//
// Extracted from app.go metadataResultMsg handler (lines 1198-1241).
func (p *Processor) ProcessElaborationSignals(signals []taskfsm.ElaborationSignal) []Action {
	var actions []Action
	for _, es := range signals {
		orch, exists := p.waveOrchestrators[es.TaskFile]
		if entry, ok := p.taskEntry(es.TaskFile); !ok || executionPhaseForEntry(entry) != taskfsm.ExecutionPhaseArchitecting {
			continue
		}
		if exists && orch.State() != orchestration.WaveStateElaborating {
			continue
		}

		// Re-read the architect-enriched plan from the store.
		content, err := p.config.Store.GetContent(p.config.Project, es.TaskFile)
		if err != nil {
			continue
		}
		plan, err := taskparser.Parse(content)
		if err != nil {
			continue
		}

		if !exists {
			orch = orchestration.NewWaveOrchestrator(es.TaskFile, plan)
			if p.config.Store != nil {
				orch.SetStore(p.config.Store, p.config.Project)
			}
			orch.SetElaborating()
			p.waveOrchestrators[es.TaskFile] = orch
		}

		// Replace the plan with the architect-enriched version and hand off the
		// actual wave start to the shared AdvanceWaveAction execution path.
		orch.UpdatePlan(plan)

		if err := p.setExecutionState(es.TaskFile, taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
			ActiveAgentType: session.AgentTypeCoder,
			ActiveWave:      1,
		}); err != nil {
			continue
		}
		p.activeWaveOrchs[es.TaskFile] = true

		actions = append(actions, AdvanceWaveAction{
			PlanFile: es.TaskFile,
			Wave:     1,
		})
	}
	return actions
}

// shouldCreatePR returns true when a plan entry is eligible for automatic PR
// creation: the review has been approved, the plan is on a branch, and no PR
// has been opened yet.
func shouldCreatePR(entry taskstore.TaskEntry) bool {
	return entry.Status == taskstore.StatusDone && entry.Branch != "" && entry.PRURL == ""
}
