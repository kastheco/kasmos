package loop

import (
	"fmt"
	"log"
	"strings"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
)

// plannerDraftAgg tracks aggregation state for one plan file in a multi-planner run.
type plannerDraftAgg struct {
	expectedProfiles map[string]bool // set of profiles that must report
	receivedProfiles map[string]bool // set of profiles that have reported
	done             bool            // true once synthesis has been triggered
}

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
	case taskfsm.ImplementStart:
		return taskfsm.StatusImplementing, true
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
	// HeadSHA resolves the live HEAD SHA of a task branch. When AutoReadinessReview
	// is true and this is nil, the gate fails closed.
	HeadSHA func(branch string) (string, error)
	// MergeBaseSHA resolves the merge-base of a task branch.
	MergeBaseSHA func(branch string) (string, error)
	// PlannerProfiles is the ordered list of agent profile names that should
	// be spawned in parallel when PlannerDraftMode is true. Each entry must
	// reference a configured [agents.<profile>] section. When empty, the
	// legacy single-planner path is used regardless of PlannerDraftMode.
	PlannerProfiles []string
	// PlannerDraftMode enables the multi-planner fan-out path. When true and
	// PlannerProfiles is non-empty, plan_start clears the draft cache, spawns
	// one planner per profile, and aggregates planner_draft_finished signals
	// into a single synthesized planner_finished transition.
	PlannerDraftMode bool
	// CacheDir is the directory where planner draft files are stored. When set
	// and PlannerDraftMode is enabled, ProcessPlannerDraftSignals seeds already-
	// received profiles from on-disk cache on first call for a given plan file.
	CacheDir string
	// MaxReviewFixCycles is the maximum number of review-fix cycles allowed
	// before emitting ReviewCycleLimitAction instead of SpawnCoderAction.
	// Zero or negative means unlimited.
	MaxReviewFixCycles int
	// ReadinessSelfFixMaxLines is the maximum number of net lines the master agent
	// may change in a self-fix attempt. Processor construction does not apply an
	// implicit default; callers must provide a positive value when they want a limit.
	ReadinessSelfFixMaxLines int
	// ReadinessMaxVerifyCycles is the maximum number of verify-round attempts before
	// the loop is force-promoted to approved. Zero or negative disables forced
	// promotion.
	ReadinessMaxVerifyCycles int
	// Hooks is an optional registry of FSM transition hooks. When non-nil and
	// non-empty it is attached to the FSM so hooks fire after every successful
	// state write.
	Hooks *taskfsm.HookRegistry
}

// bindVerification resolves live HEAD for the task branch and decides whether
// an approval may be admitted. by is empty when the approval must be rejected.
func (p *Processor) bindVerification(planFile, origin, reviewedSHA string) (rec RecordVerificationAction, stale StaleVerificationAction, ok bool) {
	rec.PlanFile = planFile
	stale = StaleVerificationAction{PlanFile: planFile, ReviewedSHA: reviewedSHA}
	entry, _ := p.taskEntry(planFile)
	if p.config.HeadSHA == nil {
		stale.Reason = "unbound_verification: head resolver unavailable"
		return rec, stale, false
	}
	head, err := p.config.HeadSHA(entry.Branch)
	if err != nil {
		stale.Reason = fmt.Sprintf("head_unresolvable: %v", err)
		return rec, stale, false
	}
	stale.CurrentSHA = head
	if origin == "operator" || origin == "force_promoted" || origin == "auto" {
		rec.SHA, rec.By = head, origin
	} else {
		if reviewedSHA == "" {
			stale.Reason = "unbound_master_approval: master approved without reviewed_sha"
			return rec, stale, false
		}
		if !strings.EqualFold(reviewedSHA, head) {
			stale.Reason = fmt.Sprintf("stale_master_approval: master reviewed %s but head is %s", gitpkg.ShortSHA(reviewedSHA), gitpkg.ShortSHA(head))
			return rec, stale, false
		}
		rec.SHA, rec.By = head, "master"
	}
	if p.config.MergeBaseSHA != nil {
		if base, err := p.config.MergeBaseSHA(entry.Branch); err == nil {
			rec.BaseSHA = base
		} else {
			log.Printf("resolve verification merge-base for %s: %v", planFile, err)
		}
	}
	return rec, stale, true
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
	// plannerDraftAggs tracks per-plan aggregation state for multi-planner runs.
	// Keyed by plan file.
	plannerDraftAggs      map[string]*plannerDraftAgg
	gatewaySignalOutcomes map[int64]GatewaySignalOutcome
}

// NewProcessor creates a Processor backed by the given store and project.
func NewProcessor(cfg ProcessorConfig) *Processor {
	fsm := taskfsm.New(cfg.Store, cfg.Project, cfg.Dir)
	if cfg.Hooks != nil && cfg.Hooks.Len() > 0 {
		fsm.SetHooks(cfg.Hooks)
	}
	return &Processor{
		config:                cfg,
		fsm:                   fsm,
		waveOrchestrators:     make(map[string]*orchestration.WaveOrchestrator),
		activeWaveOrchs:       make(map[string]bool),
		plannerDraftAggs:      make(map[string]*plannerDraftAgg),
		gatewaySignalOutcomes: make(map[int64]GatewaySignalOutcome),
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

// shouldForcePromoteVerify returns true when the next VerifyFailed signal for
// planFile should be promoted to VerifyApproved instead, because the readiness
// verify-loop has reached its configured cap. The decision must be made before
// applying the FSM transition: once verifying→implementing has fired,
// VerifyApproved is no longer a legal transition from the current state.
//
// The attempt-count semantics: ReviewCycle is the number of completed
// fixer iterations. On the next verify_failed the attempt count is
// ReviewCycle+1, and we promote when that reaches (>=) ReadinessMaxVerifyCycles.
func (p *Processor) shouldForcePromoteVerify(planFile string) bool {
	if !p.config.AutoReviewFix || !p.config.AutoReadinessReview {
		return false
	}
	if p.config.ReadinessMaxVerifyCycles <= 0 || p.config.Store == nil {
		return false
	}
	entry, err := p.config.Store.Get(p.config.Project, planFile)
	if err != nil {
		return false
	}
	return entry.ReviewCycle+1 >= p.config.ReadinessMaxVerifyCycles
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

		// Readiness verify-loop cap: when a VerifyFailed signal arrives on the
		// terminal attempt, promote it to VerifyApproved *before* applying the
		// FSM transition. Otherwise the task would transition verifying→
		// implementing first and the downstream VerifyApproved transition
		// (which is only valid from verifying) would be rejected, leaving the
		// task permanently stuck in fixer loops.
		//
		// Skip force-promotion when the signal is PreApplied: the FSM transition
		// has already been applied by the signal's originator (e.g. the HTTP
		// admin handler) for the original event, so rewriting the event here
		// would emit side effects inconsistent with the actual persisted state.
		var pendingRecord *RecordVerificationAction
		if sig.Event == taskfsm.VerifyApproved && p.config.AutoReadinessReview {
			entry, entryOK := p.taskEntry(sig.TaskFile)
			eligible := entryOK && entry.Status == taskstore.StatusVerifying
			if sig.PreApplied {
				eligible = entryOK && entry.Status == taskstore.StatusDone
			}
			if !eligible {
				continue
			}
			origin := sig.Origin
			// Gateway payload is agent-controlled. Only in-process callers may
			// assert operator/internal provenance without a reviewed SHA.
			if sig.GatewayEntryID != 0 && origin != "master" {
				origin = ""
			}
			if sig.PreApplied && origin == "" && sig.ReviewedSHA == "" {
				origin = "operator"
			}
			rec, stale, ok := p.bindVerification(sig.TaskFile, origin, sig.ReviewedSHA)
			if !ok {
				p.setGatewaySignalOutcome(sig.GatewayEntryID, taskstore.SignalFailed, stale.Reason)
				actions = append(actions, stale,
					PausePlanAgentAction{PlanFile: sig.TaskFile, AgentType: session.AgentTypeMaster},
					SpawnMasterAction{PlanFile: sig.TaskFile})
				continue
			}
			pendingRecord = &rec
		}

		eventToApply := sig.Event
		forcePromotedVerify := false
		if sig.Event == taskfsm.VerifyFailed && !sig.PreApplied && p.shouldForcePromoteVerify(sig.TaskFile) {
			eventToApply = taskfsm.VerifyApproved
			forcePromotedVerify = true
			rec, stale, ok := p.bindVerification(sig.TaskFile, "force_promoted", "")
			if !ok {
				p.setGatewaySignalOutcome(sig.GatewayEntryID, taskstore.SignalFailed, stale.Reason)
				continue
			}
			pendingRecord = &rec
		}

		alreadyApplied := false
		if err := p.fsm.Transition(sig.TaskFile, eventToApply); err != nil {
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

		switch eventToApply {
		case taskfsm.ImplementStart:
			actions = append(actions, StartImplementationAction{PlanFile: sig.TaskFile})

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
			rec, stale, ok := p.bindVerification(sig.TaskFile, "auto", "")
			if !ok {
				p.setGatewaySignalOutcome(sig.GatewayEntryID, taskstore.SignalFailed, stale.Reason)
				actions = append(actions, stale,
					PausePlanAgentAction{PlanFile: sig.TaskFile, AgentType: session.AgentTypeMaster},
					SpawnMasterAction{PlanFile: sig.TaskFile})
				break
			}
			if err := p.fsm.Transition(sig.TaskFile, taskfsm.VerifyApproved); err == nil {
				actions = append(actions, rec)
				actions = append(actions, VerifyApprovedAction{
					PlanFile:   sig.TaskFile,
					ReviewBody: sig.Body,
				})
				if p.config.Store != nil {
					if entry, err := p.config.Store.Get(p.config.Project, sig.TaskFile); err == nil {
						_ = entry
						actions = append(actions, CreatePRAction{PlanFile: sig.TaskFile, ReviewBody: sig.Body})
					}
				}
			}

		case taskfsm.VerifyApproved:
			// VerifyApproved transitions verifying → done (emitted by master agent),
			// or synthesized locally when the readiness verify-loop cap is reached.
			if pendingRecord != nil {
				actions = append(actions, *pendingRecord)
			}
			actions = append(actions, VerifyApprovedAction{
				PlanFile:      sig.TaskFile,
				ReviewBody:    sig.Body,
				ForcePromoted: forcePromotedVerify,
			})
			if p.config.Store != nil {
				if entry, err := p.config.Store.Get(p.config.Project, sig.TaskFile); err == nil {
					_ = entry
					actions = append(actions, CreatePRAction{PlanFile: sig.TaskFile, ReviewBody: sig.Body})
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
			if p.config.PlannerDraftMode && len(p.config.PlannerProfiles) > 0 {
				// Drop any aggregation state from a prior run so signals from this
				// new fan-out aren't ignored by a stale agg.done=true.
				p.ResetPlannerDraftAgg(sig.TaskFile)
				// Multi-planner draft mode: clear stale caches, then spawn one
				// planner per configured profile. Only the first profile is primary.
				actions = append(actions, ClearPlannerDraftsAction{PlanFile: sig.TaskFile})
				for i, profile := range p.config.PlannerProfiles {
					actions = append(actions, SpawnPlannerAction{
						PlanFile:       sig.TaskFile,
						PlannerProfile: profile,
						Primary:        i == 0,
						DraftMode:      true,
					})
				}
				break
			}
			// Legacy single-planner path (or draft mode with no profiles configured).
			actions = append(actions, SpawnPlannerAction{
				PlanFile:       sig.TaskFile,
				PlannerProfile: "planner",
				Primary:        true,
				DraftMode:      false,
			})
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
			PlanFile:        ts.TaskFile,
			TaskNumber:      ts.TaskNumber,
			WaveNumber:      ts.WaveNumber,
			RetryGeneration: orch.RetryGeneration(),
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

// ProcessRetryWaveSignals reconstructs the active wave from persisted state
// and emits a retry action that replaces stale agents. Completed tasks remain
// complete; every unresolved task is returned to running state.
func (p *Processor) ProcessRetryWaveSignals(signals []taskfsm.WaveSignal) []Action {
	var actions []Action
	for _, sig := range signals {
		entry, ok := p.taskEntry(sig.TaskFile)
		if !ok || entry.Status != taskstore.StatusImplementing || entry.ExecutionState.ActiveWave < 1 {
			continue
		}
		waveNumber := entry.ExecutionState.ActiveWave
		content, err := p.config.Store.GetContent(p.config.Project, sig.TaskFile)
		if err != nil {
			continue
		}
		plan, err := taskparser.Parse(content)
		if err != nil || waveNumber > len(plan.Waves) {
			continue
		}
		subtasks, err := p.config.Store.GetSubtasks(p.config.Project, sig.TaskFile)
		if err != nil {
			continue
		}
		completed := make([]int, 0)
		for _, subtask := range subtasks {
			switch subtask.Status {
			case taskstore.SubtaskStatusComplete, taskstore.SubtaskStatusDone, taskstore.SubtaskStatusClosed:
				completed = append(completed, subtask.TaskNumber)
			}
		}
		orch := orchestration.NewWaveOrchestrator(sig.TaskFile, plan)
		orch.SetStore(p.config.Store, p.config.Project)
		orch.RestoreToWave(waveNumber, completed)
		if orch.ActiveTaskCount() == 0 {
			continue
		}
		p.waveOrchestrators[sig.TaskFile] = orch
		p.activeWaveOrchs[sig.TaskFile] = true
		actions = append(actions, RetryWaveAction{PlanFile: sig.TaskFile, Wave: waveNumber})
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

// ProcessPlannerDraftSignals collects planner-draft-finished signals and
// aggregates them toward a synthesized planner_finished transition once all
// configured parallel planners have reported their drafts.
//
// When all expected profiles have reported, this calls ProcessFSMSignals with a
// synthesized PlannerFinished signal so that the existing downstream behavior
// (PlannerCompleteAction, AutoImplementAction) fires in the same tick.
//
// Signals with unknown profiles, duplicate profiles, or signals for already-
// completed plans produce no actions. In legacy single-planner mode
// (PlannerDraftMode=false or PlannerProfiles empty), all draft signals are
// ignored — PlannerFinished arrives directly via ProcessFSMSignals.
func (p *Processor) ProcessPlannerDraftSignals(signals []taskfsm.PlannerDraftSignal) []Action {
	if !p.config.PlannerDraftMode || len(p.config.PlannerProfiles) == 0 {
		// Not in multi-planner mode; planner_finished comes directly via FSM.
		for _, sig := range signals {
			p.setGatewaySignalOutcome(sig.GatewayEntryID, taskstore.SignalFailed, "planner draft signal rejected outside draft mode")
		}
		return nil
	}

	var actions []Action
	for _, sig := range signals {
		agg := p.getOrInitDraftAgg(sig.TaskFile)
		if agg.done {
			// Already synthesized for this plan — ignore further signals.
			p.setGatewaySignalOutcome(sig.GatewayEntryID, taskstore.SignalFailed, "planner draft signal ignored after aggregation completed")
			continue
		}
		// Seed from on-disk cache before evaluating the current signal so that
		// a daemon/TUI restart can recover already-written drafts.
		if p.config.CacheDir != "" {
			p.seedDraftAggFromCache(agg, sig.TaskFile)
		}
		if len(agg.receivedProfiles) >= len(agg.expectedProfiles) {
			sigActions := p.synthesizePlannerFinished(sig.TaskFile, agg)
			if len(sigActions) == 0 {
				p.setGatewaySignalOutcome(sig.GatewayEntryID, taskstore.SignalFailed, "planner_finished synthesis rejected by processor")
				continue
			}
			actions = append(actions, sigActions...)
			continue
		}
		// Unknown profile — ignore.
		if !agg.expectedProfiles[sig.PlannerID] {
			p.setGatewaySignalOutcome(sig.GatewayEntryID, taskstore.SignalFailed, fmt.Sprintf("unknown planner draft profile %q", sig.PlannerID))
			continue
		}
		// Duplicate — ignore.
		if agg.receivedProfiles[sig.PlannerID] {
			p.setGatewaySignalOutcome(sig.GatewayEntryID, taskstore.SignalFailed, fmt.Sprintf("duplicate planner draft profile %q", sig.PlannerID))
			continue
		}
		agg.receivedProfiles[sig.PlannerID] = true

		if len(agg.receivedProfiles) < len(agg.expectedProfiles) {
			// Still waiting for more profiles.
			p.setGatewaySignalOutcome(sig.GatewayEntryID, taskstore.SignalDone, "planner draft recorded or waiting for peers")
			continue
		}

		sigActions := p.synthesizePlannerFinished(sig.TaskFile, agg)
		if len(sigActions) == 0 {
			p.setGatewaySignalOutcome(sig.GatewayEntryID, taskstore.SignalFailed, "planner_finished synthesis rejected by processor")
			continue
		}
		actions = append(actions, sigActions...)
	}
	return actions
}

func (p *Processor) setGatewaySignalOutcome(entryID int64, status taskstore.SignalStatus, result string) {
	if entryID == 0 {
		return
	}
	p.gatewaySignalOutcomes[entryID] = GatewaySignalOutcome{Status: status, Result: result}
}

// GatewayNoopOutcome returns the processor-specific acknowledgement outcome for
// a claimed gateway row that produced no actions. Planner draft rows need this
// context because a no-op can mean either "accepted and waiting for peers" or
// "rejected as unusable"; other signal types use the shared stateless fallback.
func (p *Processor) GatewayNoopOutcome(entry *taskstore.SignalEntry) (taskstore.SignalStatus, string) {
	if entry != nil && entry.ID != 0 {
		if outcome, ok := p.gatewaySignalOutcomes[entry.ID]; ok {
			delete(p.gatewaySignalOutcomes, entry.ID)
			return outcome.Status, outcome.Result
		}
	}
	return GatewayNoopOutcome(entry)
}

func (p *Processor) synthesizePlannerFinished(planFile string, agg *plannerDraftAgg) []Action {
	agg.done = true
	return p.ProcessFSMSignals([]taskfsm.Signal{{
		TaskFile: planFile,
		Event:    taskfsm.PlannerFinished,
	}})
}

// ResetPlannerDraftAgg drops any in-memory draft aggregation for planFile so a
// fresh fan-out's signals are not ignored by a stale agg.done from a prior run.
// Callers that respawn planners outside of ProcessFSMSignals (e.g. UI replan
// flows) must invoke this before launching the new planners.
func (p *Processor) ResetPlannerDraftAgg(planFile string) {
	delete(p.plannerDraftAggs, planFile)
}

// getOrInitDraftAgg returns the existing aggregation state for a plan file, or
// creates and registers a fresh one based on the configured PlannerProfiles.
func (p *Processor) getOrInitDraftAgg(planFile string) *plannerDraftAgg {
	if agg, ok := p.plannerDraftAggs[planFile]; ok {
		return agg
	}
	expected := make(map[string]bool, len(p.config.PlannerProfiles))
	for _, profile := range p.config.PlannerProfiles {
		expected[profile] = true
	}
	agg := &plannerDraftAgg{
		expectedProfiles: expected,
		receivedProfiles: make(map[string]bool),
	}
	p.plannerDraftAggs[planFile] = agg
	return agg
}

// seedDraftAggFromCache reads on-disk planner draft cache entries and marks
// any profiles found there as already received in agg. This allows a restarted
// processor to recover aggregation progress without reprocessing gateway rows.
func (p *Processor) seedDraftAggFromCache(agg *plannerDraftAgg, planFile string) {
	if agg.done {
		return
	}
	entries, err := orchestration.ListPlannerDraftCaches(p.config.CacheDir, planFile)
	if err != nil || len(entries) == 0 {
		return
	}
	for _, entry := range entries {
		if agg.expectedProfiles[entry.Profile] {
			agg.receivedProfiles[entry.Profile] = true
		}
	}
}
