---
work_package_id: WP05
title: Tests
lane: planned
dependencies: []
subtasks: [T024, T025, T026, T027, T028]
history:
- timestamp: '2026-02-20T12:00:00Z'
  lane: planned
  actor: planner
  action: created work package
---

# WP05: Tests

## Implementation Command

```bash
spec-kitty implement WP05 --base WP04
```

## Objective

Write comprehensive table-driven tests for all browser pure functions in a new file `internal/tui/browser_test.go`. Tests cover: filesystem scanning with temp directories, phase detection accuracy, action mapping correctness, directory name parsing edge cases, and filter matching behavior.

## Context

### Constitution Testing Requirements

From `.kittify/memory/constitution.md`:
- Use `go test ./...` for all testing
- Standard library `testing` package; table-driven tests for parsers and state machines
- No hard coverage target, but untested features are not considered complete

### Testable Functions (from WP01)

All functions in browser.go are pure (no Model receiver or side effects), making them straightforward to test:

1. `scanFeatures() ([]FeatureEntry, error)` - filesystem scan
2. `detectPhase(featureDir string) (FeaturePhase, int)` - phase classification
3. `actionsForPhase(phase FeaturePhase) []lifecycleAction` - action mapping
4. `parseFeatureDir(name string) (number, slug string)` - name parsing
5. `filterFeatures(entries []FeatureEntry, query string) []int` - filter matching

### Existing Test Patterns

The codebase uses standard Go test patterns. Example from `internal/task/`:
```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {name: "case one", input: "...", expected: "..."},
        {name: "case two", input: "...", expected: "..."},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test body
        })
    }
}
```

---

## Subtask T024: Test scanFeatures() with Temp Directory Structures

**Purpose**: Verify that scanFeatures correctly discovers features, classifies their phases, and sorts results. Uses temp directories to simulate real kitty-specs structures.

**Steps**:

1. Create a helper function to set up temp kitty-specs directories:

   ```go
   // setupTestFeatures creates a temp directory with a kitty-specs structure.
   // Returns the temp dir path (caller must chdir to it).
   func setupTestFeatures(t *testing.T, features map[string]struct {
       hasSpec bool
       hasPlan bool
       wpCount int
   }) string {
       t.Helper()
       dir := t.TempDir()

       kittyDir := filepath.Join(dir, "kitty-specs")
       if err := os.MkdirAll(kittyDir, 0o755); err != nil {
           t.Fatal(err)
       }

       for name, cfg := range features {
           featureDir := filepath.Join(kittyDir, name)
           if err := os.MkdirAll(featureDir, 0o755); err != nil {
               t.Fatal(err)
           }
           if cfg.hasSpec {
               os.WriteFile(filepath.Join(featureDir, "spec.md"), []byte("# spec"), 0o644)
           }
           if cfg.hasPlan {
               os.WriteFile(filepath.Join(featureDir, "plan.md"), []byte("# plan"), 0o644)
           }
           if cfg.wpCount > 0 {
               tasksDir := filepath.Join(featureDir, "tasks")
               os.MkdirAll(tasksDir, 0o755)
               for i := 1; i <= cfg.wpCount; i++ {
                   filename := fmt.Sprintf("WP%02d-task.md", i)
                   os.WriteFile(filepath.Join(tasksDir, filename), []byte("---\n---"), 0o644)
               }
           }
       }

       return dir
   }
   ```

2. Write test cases:

   ```go
   func TestScanFeatures(t *testing.T) {
       tests := []struct {
           name     string
           features map[string]struct{ hasSpec, hasPlan bool; wpCount int }
           wantLen  int
           wantFirst string // expected first entry's number (sorted desc)
       }{
           {
               name:     "mixed phases",
               features: map[string]struct{ hasSpec, hasPlan bool; wpCount int }{
                   "001-alpha": {true, false, 0},
                   "002-beta":  {true, true, 0},
                   "003-gamma": {true, true, 3},
               },
               wantLen:   3,
               wantFirst: "003", // highest number first
           },
           {
               name:     "excludes dirs without spec.md",
               features: map[string]struct{ hasSpec, hasPlan bool; wpCount int }{
                   "001-valid":   {true, false, 0},
                   "002-invalid": {false, true, 5},
               },
               wantLen:   1,
               wantFirst: "001",
           },
           {
               name:     "empty kitty-specs",
               features: map[string]struct{ hasSpec, hasPlan bool; wpCount int }{},
               wantLen:  0,
           },
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               dir := setupTestFeatures(t, tt.features)
               // chdir so scanFeatures finds kitty-specs/
               origDir, _ := os.Getwd()
               os.Chdir(dir)
               defer os.Chdir(origDir)

               entries, err := scanFeatures()
               if err != nil {
                   t.Fatalf("scanFeatures() error: %v", err)
               }
               if len(entries) != tt.wantLen {
                   t.Errorf("got %d entries, want %d", len(entries), tt.wantLen)
               }
               if tt.wantLen > 0 && entries[0].Number != tt.wantFirst {
                   t.Errorf("first entry number = %q, want %q", entries[0].Number, tt.wantFirst)
               }
           })
       }
   }
   ```

3. Important: Tests must `chdir` to the temp directory because `scanFeatures()` uses relative paths (`kitty-specs/*/spec.md`). Use `defer os.Chdir(origDir)` to restore.

**Files**: `internal/tui/browser_test.go` (new file)

**Validation**:
- [ ] Test passes for mixed phases, excluded dirs, and empty state
- [ ] Sorting verified (highest number first)
- [ ] chdir/restore pattern doesn't leak between tests

---

## Subtask T025: Test Phase Detection Accuracy

**Purpose**: Verify that detectPhase correctly classifies features into PhaseSpecOnly, PhasePlanReady, and PhaseTasksReady based on file existence.

**Steps**:

1. Write table-driven test:

   ```go
   func TestDetectPhase(t *testing.T) {
       tests := []struct {
           name      string
           hasPlan   bool
           wpCount   int
           wantPhase FeaturePhase
           wantWPs   int
       }{
           {name: "spec only", hasPlan: false, wpCount: 0, wantPhase: PhaseSpecOnly, wantWPs: 0},
           {name: "plan ready", hasPlan: true, wpCount: 0, wantPhase: PhasePlanReady, wantWPs: 0},
           {name: "tasks ready 1 WP", hasPlan: true, wpCount: 1, wantPhase: PhaseTasksReady, wantWPs: 1},
           {name: "tasks ready 5 WPs", hasPlan: true, wpCount: 5, wantPhase: PhaseTasksReady, wantWPs: 5},
           {name: "tasks ready no plan", hasPlan: false, wpCount: 3, wantPhase: PhaseTasksReady, wantWPs: 3},
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               dir := t.TempDir()
               os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# spec"), 0o644)
               if tt.hasPlan {
                   os.WriteFile(filepath.Join(dir, "plan.md"), []byte("# plan"), 0o644)
               }
               if tt.wpCount > 0 {
                   tasksDir := filepath.Join(dir, "tasks")
                   os.MkdirAll(tasksDir, 0o755)
                   for i := 1; i <= tt.wpCount; i++ {
                       filename := fmt.Sprintf("WP%02d-slug.md", i)
                       os.WriteFile(filepath.Join(tasksDir, filename), []byte("---"), 0o644)
                   }
               }

               phase, wpCount := detectPhase(dir)
               if phase != tt.wantPhase {
                   t.Errorf("phase = %v, want %v", phase, tt.wantPhase)
               }
               if wpCount != tt.wantWPs {
                   t.Errorf("wpCount = %d, want %d", wpCount, tt.wantWPs)
               }
           })
       }
   }
   ```

2. Key edge case: "tasks ready no plan" - a feature can have WPs without a plan.md (unusual but possible if plan was deleted). The phase should still be TasksReady because WP files exist.

**Files**: `internal/tui/browser_test.go`

**Validation**:
- [ ] All five phase combinations tested
- [ ] WP count accurate for tasks-ready features
- [ ] Edge case: WPs without plan.md still detected as TasksReady

---

## Subtask T026: Test actionsForPhase() Mapping

**Purpose**: Verify that each phase maps to the correct lifecycle actions with correct roles and prompt formats.

**Steps**:

1. Write table-driven test:

   ```go
   func TestActionsForPhase(t *testing.T) {
       tests := []struct {
           phase       FeaturePhase
           wantCount   int
           wantLabels  []string
           wantRoles   []string
       }{
           {
               phase:      PhaseSpecOnly,
               wantCount:  2,
               wantLabels: []string{"clarify", "plan"},
               wantRoles:  []string{"planner", "planner"},
           },
           {
               phase:      PhasePlanReady,
               wantCount:  1,
               wantLabels: []string{"tasks"},
               wantRoles:  []string{"planner"},
           },
           {
               phase:     PhaseTasksReady,
               wantCount: 0,
           },
       }

       for _, tt := range tests {
           t.Run(tt.phase.String(), func(t *testing.T) {
               actions := actionsForPhase(tt.phase)
               if len(actions) != tt.wantCount {
                   t.Fatalf("got %d actions, want %d", len(actions), tt.wantCount)
               }
               for i, action := range actions {
                   if action.label != tt.wantLabels[i] {
                       t.Errorf("action[%d].label = %q, want %q", i, action.label, tt.wantLabels[i])
                   }
                   if action.role != tt.wantRoles[i] {
                       t.Errorf("action[%d].role = %q, want %q", i, action.role, tt.wantRoles[i])
                   }
               }
           })
       }
   }
   ```

2. Also verify promptFmt contains exactly one `%s`:

   ```go
   func TestActionsForPhasePromptFormat(t *testing.T) {
       for _, phase := range []FeaturePhase{PhaseSpecOnly, PhasePlanReady} {
           actions := actionsForPhase(phase)
           for _, action := range actions {
               result := fmt.Sprintf(action.promptFmt, "kitty-specs/test-feature")
               if !strings.Contains(result, "kitty-specs/test-feature") {
                   t.Errorf("promptFmt %q did not interpolate feature dir", action.promptFmt)
               }
           }
       }
   }
   ```

**Files**: `internal/tui/browser_test.go`

**Validation**:
- [ ] PhaseSpecOnly returns clarify + plan actions (FR-007)
- [ ] PhasePlanReady returns tasks action (FR-008)
- [ ] PhaseTasksReady returns nil
- [ ] All roles are "planner"
- [ ] promptFmt correctly interpolates feature directory

---

## Subtask T027: Test parseFeatureDir() Edge Cases

**Purpose**: Verify directory name parsing handles standard and edge-case names correctly.

**Steps**:

1. Write table-driven test:

   ```go
   func TestParseFeatureDir(t *testing.T) {
       tests := []struct {
           name       string
           input      string
           wantNumber string
           wantSlug   string
       }{
           {name: "standard", input: "022-spec-kitty-feature-browser", wantNumber: "022", wantSlug: "spec-kitty-feature-browser"},
           {name: "simple", input: "001-simple", wantNumber: "001", wantSlug: "simple"},
           {name: "no hyphen", input: "standalone", wantNumber: "standalone", wantSlug: ""},
           {name: "leading hyphen", input: "-weird", wantNumber: "", wantSlug: "weird"},
           {name: "empty", input: "", wantNumber: "", wantSlug: ""},
           {name: "only hyphen", input: "-", wantNumber: "", wantSlug: ""},
           {name: "multiple hyphens", input: "010-hub-tui-navigator", wantNumber: "010", wantSlug: "hub-tui-navigator"},
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               num, slug := parseFeatureDir(tt.input)
               if num != tt.wantNumber {
                   t.Errorf("number = %q, want %q", num, tt.wantNumber)
               }
               if slug != tt.wantSlug {
                   t.Errorf("slug = %q, want %q", slug, tt.wantSlug)
               }
           })
       }
   }
   ```

**Files**: `internal/tui/browser_test.go`

**Validation**:
- [ ] Standard 3-digit-prefix names parse correctly
- [ ] No-hyphen names: number = full name, slug = ""
- [ ] Multiple hyphens: only splits on first
- [ ] Edge cases don't panic

---

## Subtask T028: Test filterFeatures()

**Purpose**: Verify the filter function handles case-insensitive matching, empty queries, partial matches, and no-match scenarios.

**Steps**:

1. Set up test entries:

   ```go
   var testEntries = []FeatureEntry{
       {Number: "022", Slug: "spec-kitty-feature-browser"},
       {Number: "018", Slug: "blocked-task-visual-feedback"},
       {Number: "016", Slug: "kasmos-agent-orchestrator"},
       {Number: "010", Slug: "hub-tui-navigator"},
   }
   ```

2. Write table-driven test:

   ```go
   func TestFilterFeatures(t *testing.T) {
       tests := []struct {
           name        string
           query       string
           wantIndices []int
       }{
           {name: "empty query returns all", query: "", wantIndices: []int{0, 1, 2, 3}},
           {name: "exact match", query: "hub-tui-navigator", wantIndices: []int{3}},
           {name: "partial match", query: "kasmos", wantIndices: []int{2}},
           {name: "case insensitive", query: "BROWSER", wantIndices: []int{0}},
           {name: "multiple matches", query: "a", wantIndices: []int{0, 1, 2, 3}},
           {name: "no matches", query: "zzz", wantIndices: []int{}},
           {name: "substring in middle", query: "task", wantIndices: []int{1}},
           {name: "mixed case partial", query: "Kit", wantIndices: []int{0}},
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               got := filterFeatures(testEntries, tt.query)
               if len(got) != len(tt.wantIndices) {
                   t.Fatalf("got %d indices, want %d: got %v", len(got), len(tt.wantIndices), got)
               }
               for i, idx := range got {
                   if idx != tt.wantIndices[i] {
                       t.Errorf("index[%d] = %d, want %d", i, idx, tt.wantIndices[i])
                   }
               }
           })
       }
   }
   ```

**Files**: `internal/tui/browser_test.go`

**Validation**:
- [ ] Empty query returns all indices in order
- [ ] Case-insensitive matching works
- [ ] Partial substring matching works
- [ ] No matches returns empty slice (not nil - check with len())
- [ ] All indices point to correct entries

---

## Definition of Done

- [ ] All 5 test subtasks implemented
- [ ] `go test ./internal/tui/ -run TestScan` passes
- [ ] `go test ./internal/tui/ -run TestDetectPhase` passes
- [ ] `go test ./internal/tui/ -run TestActionsForPhase` passes
- [ ] `go test ./internal/tui/ -run TestParseFeatureDir` passes
- [ ] `go test ./internal/tui/ -run TestFilterFeatures` passes
- [ ] `go test ./internal/tui/` passes (all browser tests)
- [ ] No test pollution (temp directories cleaned up, CWD restored)

## Risks

- **os.Chdir in parallel tests**: `os.Chdir` is process-global and NOT safe for `t.Parallel()`. Do NOT use `t.Parallel()` for scanFeatures tests. The detectPhase and other pure function tests that use absolute paths CAN use `t.Parallel()`.
- **Glob behavior on empty directories**: `filepath.Glob` returns `nil, nil` (not error) for no matches. Tests should verify this works correctly.
- **Test isolation**: Each test creates its own temp directory. No shared state between tests.

## Reviewer Guidance

- Verify no `t.Parallel()` on tests that use `os.Chdir`
- Verify temp directories use `t.TempDir()` (auto-cleanup)
- Verify edge cases are covered (empty input, no matches, bounds)
- Verify test names are descriptive (for `go test -v` output)
- Run `go test ./internal/tui/ -v -run "Browser|Feature|Phase|Filter|Parse"` to see all browser tests
