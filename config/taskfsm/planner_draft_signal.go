package taskfsm

// PlannerDraftSignal represents a parsed planner-draft-finished gateway signal.
// It carries the plan file and the ID of the planner agent that produced the draft.
// This signal type is intentionally not an FSM event — it is aggregated by the
// orchestration loop to synthesize a single planner_finished transition once all
// configured planners have emitted their drafts.
type PlannerDraftSignal struct {
	TaskFile  string
	PlannerID string
}
