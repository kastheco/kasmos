---
work_package_id: "WP11"
title: "AI Helpers (Analyze Failure + Generate Prompt)"
lane: "planned"
dependencies:
  - "WP04"
  - "WP08"
subtasks:
  - "Analyze failure: spawn headless worker to analyze output"
  - "Generate prompt: spawn headless worker to generate prompt from task"
  - "Analysis viewport rendering (V9 mockup)"
  - "analyzeStartedMsg/analyzeCompletedMsg handlers"
  - "genPromptStartedMsg/genPromptCompletedMsg handlers"
  - "Restart with suggested prompt flow"
phase: "Wave 2 - Task Sources + Worker Management"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
history:
  - timestamp: "2026-02-17T00:00:00Z"
    lane: "planned"
    agent: "planner"
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP11 - AI Helpers (Analyze Failure + Generate Prompt)

## Mission

Implement on-demand AI helpers: failure analysis (`a` key) and prompt generation
(`g` key). These are NOT automatic -- the user explicitly triggers them. Each
helper spawns a short-lived headless worker that analyzes content and returns a
structured result. This delivers the FR-012 requirement (on-demand AI helpers).

## Scope

### Files to Create

```
internal/tui/helpers.go     # analyzeCmd, genPromptCmd implementations
```

### Files to Modify

```
internal/tui/update.go      # Analyze and gen-prompt message handlers
internal/tui/panels.go      # Analysis viewport rendering
internal/tui/keys.go        # Enable analyze/genPrompt keys
internal/tui/model.go       # Analysis state fields
```

### Technical References

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 2**: AI helper messages (lines 338-364)
- `design-artifacts/tui-mockups.md`:
  - **V9**: AI failure analysis view (lines 365-402)
- `design-artifacts/tui-keybinds.md`:
  - `a` analyze: enabled when selected worker is failed (line 29)
  - `g` gen prompt: enabled when task source loaded (line 28)
- `design-artifacts/tui-styles.md`:
  - Analysis view styles (lines 438-455)

## Implementation

### Failure Analysis (`a` key)

When user presses `a` on a failed worker:

1. Emit `analyzeStartedMsg{WorkerID}`
2. Show spinner in viewport: "Analyzing failure for w-NNN..."
3. Spawn a headless OpenCode worker with a structured analysis prompt:
   ```
   Analyze this failed agent output and identify the root cause.
   
   Worker: {id} ({role})
   Exit code: {code}
   Duration: {duration}
   
   Output (last 200 lines):
   {output_tail}
   
   Respond in this exact format:
   ROOT_CAUSE: <one paragraph explaining what went wrong>
   SUGGESTED_PROMPT: <a revised prompt that would fix the issue>
   ```
4. The analysis worker runs headless (not tracked in the worker table)
5. When analysis worker exits, parse its output for ROOT_CAUSE and SUGGESTED_PROMPT
6. Emit `analyzeCompletedMsg{WorkerID, RootCause, SuggestedPrompt, Err}`

**Analysis worker configuration**:
- Role: "reviewer" (read-only, analytical)
- The analysis worker itself is spawned via the same WorkerBackend
- It does NOT appear in the worker table (track separately in Model)
- Timeout: kill after 60 seconds if not done

### Analysis Viewport (panels.go)

When `analyzeCompletedMsg` is received, render the analysis view in the viewport
matching V9 mockup:

```
Analysis header (hot pink bold): "Analysis: w-004 coder"
Separator line
Root Cause label (orange bold): "Root Cause:"
Root cause text (normal)
Blank line
Suggested Fix label (green bold): "Suggested Fix:"
Suggested prompt text (normal)
Blank line
Hint (faint): "Press r to restart with suggested prompt"
```

Set the viewport title to "Analysis: {id} {role}" instead of "Output: ..."

Add to Model:
- `analysisMode bool` -- viewport shows analysis instead of output
- `analysisResult *AnalysisResult` -- parsed root cause + suggested prompt
- `analysisWorkerID string` -- which worker is being analyzed

Press `esc` to dismiss analysis and return to normal output view.

### Restart with Suggested Prompt

When analysis is showing and user presses `r`:
1. Open spawn dialog pre-filled with:
   - Same role as the failed worker
   - Prompt = the suggested prompt from analysis
2. This reuses the restart flow from WP07

### Prompt Generation (`g` key)

When user presses `g` with table focused and task source loaded:

1. Get the selected task from the task source
2. Emit `genPromptStartedMsg{TaskID}`
3. Show spinner in viewport: "Generating prompt for {taskID}..."
4. Spawn a headless OpenCode worker with a prompt generation request:
   ```
   Generate an implementation prompt for this task.
   
   Task: {id} - {title}
   Description: {description}
   Dependencies: {deps}
   Suggested role: {role}
   
   Context files in this project:
   {list relevant files if available}
   
   Generate a detailed, actionable prompt suitable for an AI coding agent.
   The prompt should be specific enough to implement without further clarification.
   ```
5. Parse the generated prompt from the helper's output
6. Emit `genPromptCompletedMsg{TaskID, Prompt, Err}`
7. Open spawn dialog pre-filled with the generated prompt

### Parsing Analysis/Prompt Output

Simple string parsing:
- Split output on "ROOT_CAUSE:" and "SUGGESTED_PROMPT:" markers
- Trim whitespace
- If markers not found, use the entire output as the result
- Handle partial results gracefully (root cause found but no suggestion)

### Key Activation

```go
m.keys.Analyze.SetEnabled(selected != nil && selected.State == StateFailed)
m.keys.GenPrompt.SetEnabled(m.hasTaskSource() && m.focused == panelTable)
```

Remember the `g` key conflict from WP06: GenPrompt is only active when table is
focused. When viewport is focused, `g` = GotoTop.

## What NOT to Do

- Do NOT auto-analyze on failure (user must press `a` explicitly)
- Do NOT auto-generate prompts (user must press `g` explicitly)
- Do NOT stream analysis output to the viewport (show spinner, then full result)
- Do NOT add analysis workers to the main worker table
- Do NOT implement retry logic for analysis (if it fails, show error)

## Acceptance Criteria

1. Select a failed worker, press `a` -- spinner shows, then analysis view appears
2. Analysis shows root cause + suggested prompt matching V9 layout
3. Press `r` in analysis mode -- spawn dialog opens with suggested prompt
4. Press `esc` -- returns to normal output view
5. Select a task in table, press `g` -- prompt is generated and spawn dialog opens
6. `a` disabled for non-failed workers, `g` disabled without task source
7. Analysis worker timeout (60s) works -- shows timeout error
8. `go test ./...` passes
