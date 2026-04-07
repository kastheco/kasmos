package taskparser

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePlan_MultiWave(t *testing.T) {
	input := `# Feature Plan

> **For Claude:** ...

**Goal:** Build a thing
**Architecture:** Some approach
**Tech Stack:** Go

**Waves:** 2 (T1,T2 parallel → T3 sequential)

---

## Wave 1
### Task 1: First Thing

**Files:**
- Create: ` + "`path/to/file.go`" + `

**Step 1: Do something**

Some instructions here.

### Task 2: Second Thing

**Files:**
- Modify: ` + "`other/file.go`" + `

**Step 1: Do other thing**

More instructions.

## Wave 2
### Task 3: Final Thing

**Files:**
- Modify: ` + "`path/to/file.go`" + `

**Step 1: Wrap up**

Final instructions.
`
	plan, err := Parse(input)
	require.NoError(t, err)

	assert.Equal(t, "Build a thing", plan.Goal)
	assert.Equal(t, "Some approach", plan.Architecture)
	assert.Equal(t, "Go", plan.TechStack)

	require.Len(t, plan.Waves, 2)

	// Wave 1: two tasks
	require.Len(t, plan.Waves[0].Tasks, 2)
	assert.Equal(t, 1, plan.Waves[0].Number)
	assert.Equal(t, 1, plan.Waves[0].Tasks[0].Number)
	assert.Equal(t, "First Thing", plan.Waves[0].Tasks[0].Title)
	assert.Contains(t, plan.Waves[0].Tasks[0].Body, "Do something")
	assert.Equal(t, 2, plan.Waves[0].Tasks[1].Number)
	assert.Equal(t, "Second Thing", plan.Waves[0].Tasks[1].Title)

	// Wave 2: one task
	require.Len(t, plan.Waves[1].Tasks, 1)
	assert.Equal(t, 2, plan.Waves[1].Number)
	assert.Equal(t, 3, plan.Waves[1].Tasks[0].Number)
}

func TestParsePlan_NoWaveHeaders(t *testing.T) {
	input := `# Old Plan

**Goal:** Legacy thing

---

### Task 1: Something
Step 1: do it

### Task 2: Another
Step 1: do it too
`
	_, err := Parse(input)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoWaveHeaders)
	assert.Contains(t, err.Error(), "no wave headers found")
}

func TestExtractMetadata_DoesNotRequireWaves(t *testing.T) {
	input := `# Notes

**Goal:** untangle ingest and parse
**Architecture:** separate save-time metadata from execution-time validation
**Tech Stack:** go, sqlite

## Step 1

Draft notes only.
`

	meta := ExtractMetadata(input)
	assert.Equal(t, Metadata{
		Goal:         "untangle ingest and parse",
		Architecture: "separate save-time metadata from execution-time validation",
		TechStack:    "go, sqlite",
	}, meta)
	assert.False(t, HasWaveHeaders(input))

	_, err := Parse(input)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoWaveHeaders))
}

func TestParsePlan_EmptyPlan(t *testing.T) {
	_, err := Parse("")
	require.Error(t, err)
}

func TestParsePlan_TaskHeaderSeparatorVariants(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		wantNum int
		wantTtl string
	}{
		{"colon", "### Task 1: Do thing", 1, "Do thing"},
		// Renumbering assigns 1..N in traversal order, so every single-task plan gets number 1
		// regardless of what the markdown says. The wantNum=1 values below are intentional;
		// wantTtl still verifies the regex correctly handles each separator variant.
		{"em-dash", "### Task 2 \u2014 Do thing", 1, "Do thing"},
		{"en-dash", "### Task 3 \u2013 Do thing", 1, "Do thing"},
		{"hyphen", "### Task 4 - Do thing", 1, "Do thing"},
		{"colon-no-space", "### Task 5:Do thing", 1, "Do thing"},
		{"backtick-title", "### Task 1 \u2014 `kas audit` subcommand", 1, "`kas audit` subcommand"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "## Wave 1\n\n" + tt.header + "\n\nBody text.\n"
			plan, err := Parse(input)
			require.NoError(t, err)
			require.Len(t, plan.Waves, 1)
			require.Len(t, plan.Waves[0].Tasks, 1, "task header must be parsed: %s", tt.header)
			assert.Equal(t, tt.wantNum, plan.Waves[0].Tasks[0].Number)
			assert.Equal(t, tt.wantTtl, plan.Waves[0].Tasks[0].Title)
		})
	}
}

func TestParsePlan_H3WaveHeaders(t *testing.T) {
	input := `# Plan

**Goal:** Sync config

### Wave 1

#### Task 1: Patch opencode.jsonc

Body of task 1.

#### Task 2: Patch main branch

Body of task 2.

### Wave 2

#### Task 3: Tests

Body of task 3.
`
	plan, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, plan.Waves, 2)
	require.Len(t, plan.Waves[0].Tasks, 2)
	assert.Equal(t, "Patch opencode.jsonc", plan.Waves[0].Tasks[0].Title)
	assert.Equal(t, "Patch main branch", plan.Waves[0].Tasks[1].Title)
	require.Len(t, plan.Waves[1].Tasks, 1)
	assert.Equal(t, "Tests", plan.Waves[1].Tasks[0].Title)
}

func TestParsePlan_PerWaveTaskNumbersRenumberedGlobally(t *testing.T) {
	// Plans authored with per-wave numbering (Task 1, Task 2 in every wave)
	// must produce globally unique task numbers after parsing.
	input := `# Plan

**Goal:** fix duplicate subtask keys

## Wave 1

### Task 1: alpha

Body of alpha.

### Task 2: beta

Body of beta.

## Wave 2

### Task 1: gamma

Body of gamma.

### Task 2: delta

Body of delta.
`
	plan, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, plan.Waves, 2)
	require.Len(t, plan.Waves[0].Tasks, 2)
	require.Len(t, plan.Waves[1].Tasks, 2)

	assert.Equal(t, 1, plan.Waves[0].Tasks[0].Number)
	assert.Equal(t, "alpha", plan.Waves[0].Tasks[0].Title)
	assert.Equal(t, 2, plan.Waves[0].Tasks[1].Number)
	assert.Equal(t, "beta", plan.Waves[0].Tasks[1].Title)
	assert.Equal(t, 3, plan.Waves[1].Tasks[0].Number)
	assert.Equal(t, "gamma", plan.Waves[1].Tasks[0].Title)
	assert.Equal(t, 4, plan.Waves[1].Tasks[1].Number)
	assert.Equal(t, "delta", plan.Waves[1].Tasks[1].Title)
}

func TestParsePlan_AlreadyGlobalTaskNumbersRemainStable(t *testing.T) {
	// Plans already using globally-unique numbers (1, 2, 3, 4 across waves)
	// must not be changed by the renumbering pass.
	input := `# Plan

**Goal:** keep stable global numbers

## Wave 1

### Task 1: first

Body 1.

### Task 2: second

Body 2.

## Wave 2

### Task 3: third

Body 3.

### Task 4: fourth

Body 4.
`
	plan, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, plan.Waves, 2)
	require.Len(t, plan.Waves[0].Tasks, 2)
	require.Len(t, plan.Waves[1].Tasks, 2)

	assert.Equal(t, 1, plan.Waves[0].Tasks[0].Number)
	assert.Equal(t, 2, plan.Waves[0].Tasks[1].Number)
	assert.Equal(t, 3, plan.Waves[1].Tasks[0].Number)
	assert.Equal(t, 4, plan.Waves[1].Tasks[1].Number)
}

func TestParsePlan_HeaderExtraction(t *testing.T) {
	input := `# Plan

**Goal:** My goal here
**Architecture:** My arch here
**Tech Stack:** Go, bubbletea

## Wave 1
### Task 1: Only Task

Do the thing.
`
	plan, err := Parse(input)
	require.NoError(t, err)
	assert.Equal(t, "My goal here", plan.Goal)
	assert.Equal(t, "My arch here", plan.Architecture)
	assert.Equal(t, "Go, bubbletea", plan.TechStack)
}
