# Reliable Planner Completion Signal

**Goal:** Ensure the planner agent explicitly signals completion after confirming with the user that the plan is finalized, rather than relying on the agent remembering to touch a sentinel after an open-ended Q&A conversation.

**Problem:** The planner's sentinel instruction (`touch docs/plans/.signals/planner-finished-<plan>.md`) is mentioned in `planner.md` and the `writing-plans` skill, but after a long interactive Q&A session the agent forgets to execute it. The TUI then shows a stale "planning" instance with no auto-finish trigger. Currently auto-finish requires BOTH:
1. Sentinel file written (agent responsibility — unreliable)
2. Tmux pane death (only happens if agent process exits)

Neither fires reliably for interactive planning sessions.

**Architecture:** Two-pronged fix — make `buildPlanPrompt` explicitly instruct the confirm-then-signal workflow, and reinforce it in the planner agent prompt so it's impossible to miss.

**Key constraint:** Planning is a Q&A process. The planner MUST NOT signal completion just because it's idle/waiting for input. It must explicitly ask the user if they're happy with the plan, get confirmation, THEN write the sentinel.

**Tech Stack:** Go (prompt strings), markdown (agent prompt)

**Size:** Small (1 wave, 2 tasks, ~30 min)

---

## Wave 1

### Task 1: Update buildPlanPrompt to include confirm-then-signal instruction

**Files:**
- Modify: `app/app_state.go` — `buildPlanPrompt()` function (line ~1249)
- Modify: `contracts/planner_prompt_contract_test.go` — add contract assertion

Update `buildPlanPrompt` to append explicit instructions:

```go
func buildPlanPrompt(planName, description string) string {
	return fmt.Sprintf(
		"Plan %s. Goal: %s. "+
			"Use the `writing-plans` superpowers skill. "+
			"The plan MUST include ## Wave N sections (at minimum ## Wave 1) "+
			"grouping all tasks — kasmos requires Wave headers to orchestrate implementation.\n\n"+
			"IMPORTANT — when the plan is complete:\n"+
			"1. Ask the user to confirm they are happy with the plan\n"+
			"2. Only after explicit user confirmation, signal completion: "+
			"touch docs/plans/.signals/planner-finished-%s\n"+
			"Do NOT signal completion until the user confirms the plan is finalized.",
		planName, description, "%s") // %s is the plan filename — needs to be filled at call site
}
```

Note: `buildPlanPrompt` doesn't currently receive the plan filename. The call site in `app_actions.go:681` passes `planstate.DisplayName(planFile)` and `entry.Description`. Either:
- Add `planFile` as a third parameter to `buildPlanPrompt`, OR
- Construct the signal path at the call site and pass it in

The simpler approach: add `planFile` parameter since the sentinel filename must match exactly.

Update the contract test to assert the new text is present:
```go
"confirm they are happy with the plan",
"planner-finished-",
```

### Task 2: Reinforce confirm-then-signal in planner agent prompt

**Files:**
- Modify: `.opencode/agents/planner.md` — Plan State section

Replace the current one-liner about signaling:

```
When running under `KASMOS_MANAGED=1`, planner completion must be signaled with
`docs/plans/.signals/planner-finished-<date>-<name>.md`.
```

With an explicit workflow:

```
### Completion (KASMOS_MANAGED=1 only)

When running under kasmos orchestration, planning is interactive — you and the user
refine the plan together. When you believe the plan is ready:

1. **Ask the user** — "are you happy with this plan?" or similar
2. **Wait for explicit confirmation** — do NOT assume silence or idle means done
3. **Signal completion** — `touch docs/plans/.signals/planner-finished-<date>-<name>.md`

The sentinel file triggers kasmos to prompt the user for implementation. Never write it
before the user confirms the plan is finalized.
```

Update contract test to assert new required text (e.g. `"Ask the user"`).
