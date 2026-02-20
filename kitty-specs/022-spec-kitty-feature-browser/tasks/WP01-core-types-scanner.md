---
work_package_id: WP01
title: Core Types, Scanner, and Phase Detection
lane: "done"
dependencies: []
base_branch: main
base_commit: ee4dc161d4104dee012377433baca9a0eb672b99
created_at: '2026-02-20T07:00:17.530661+00:00'
subtasks: [T001, T002, T003, T004, T005, T006]
shell_pid: "3948330"
reviewed_by: "kas"
review_status: "approved"
history:
- timestamp: '2026-02-20T12:00:00Z'
  lane: planned
  actor: planner
  action: created work package
---

# WP01: Core Types, Scanner, and Phase Detection

## Implementation Command

```bash
spec-kitty implement WP01
```

## Objective

Create the foundation types and pure functions for the feature browser in a new file `internal/tui/browser.go`. This WP defines the type system (FeaturePhase, FeatureEntry, lifecycleAction), the filesystem scanner that discovers features in `kitty-specs/`, phase classification logic, directory name parsing, and a filter function. All functions in this WP are pure (no Model receiver) for testability.

## Context

The feature browser needs to scan `kitty-specs/` to discover features and classify their lifecycle phase. The existing scanner `listSpecKittyFeatureDirs()` in `internal/tui/newdialog.go` (lines 544-562) only finds features with `tasks/WP*.md` files. The browser needs a broader scan that finds ALL features with at least a `spec.md`.

The existing `autoDetectSpecKittySource()` in `internal/task/source.go` (lines 125-176) demonstrates a grouping pattern (collect candidates by feature dir, sort by recency) but also only scans for WP files.

### Existing Color Palette (from `internal/tui/styles.go` lines 21-34)

```go
colorPurple    = lipgloss.Color("#7D56F4")
colorGreen     = lipgloss.Color("#73F59F")
colorLightBlue = lipgloss.Color("#82CFFF")
colorMidGray   = lipgloss.Color("#5C5C5C")
colorCream     = lipgloss.Color("#FFFDF5")
colorDarkGray  = lipgloss.Color("#383838")
colorLightGray = lipgloss.Color("#9B9B9B")
```

### Constitution Requirements

- Go 1.24+
- Follow existing kasmos code style (camelCase unexported, PascalCase exported)
- File placement: `internal/tui/browser.go` per the plan

---

## Subtask T001: Define FeaturePhase Enum with String() and phaseBadge()

**Purpose**: Create the phase enumeration that classifies features by their lifecycle stage. The String() method provides display text; phaseBadge() provides styled rendering using the kasmos palette.

**Steps**:

1. Define the `FeaturePhase` type as `int` with three constants:
   ```go
   type FeaturePhase int

   const (
       PhaseSpecOnly   FeaturePhase = iota // has spec.md only
       PhasePlanReady                      // has spec.md + plan.md, no WPs
       PhaseTasksReady                     // has tasks/WP*.md files
   )
   ```

2. Implement `String()` method returning display labels:
   - `PhaseSpecOnly` -> `"spec only"`
   - `PhasePlanReady` -> `"plan ready"`
   - `PhaseTasksReady` -> `"tasks ready"`

3. Implement `phaseBadge(phase FeaturePhase, wpCount int) string`:
   - `PhaseSpecOnly` -> styled with `colorMidGray`, text "spec only"
   - `PhasePlanReady` -> styled with `colorLightBlue`, text "plan ready"
   - `PhaseTasksReady` -> styled with `colorGreen`, text "tasks ready (N WPs)" where N is wpCount

**Pattern reference**: `taskStatusBadge()` in `internal/tui/styles.go` (lines 286-301) uses the same per-state styling pattern.

**Files**: `internal/tui/browser.go` (new file, add to top)

**Validation**:
- [ ] Three phase constants defined
- [ ] String() returns correct labels
- [ ] phaseBadge renders with correct colors from existing palette

---

## Subtask T002: Define FeatureEntry Struct

**Purpose**: Represent a discovered spec-kitty feature for the browser listing.

**Steps**:

1. Define the struct:
   ```go
   type FeatureEntry struct {
       Number  string       // e.g., "022"
       Slug    string       // e.g., "spec-kitty-feature-browser"
       Dir     string       // e.g., "kitty-specs/022-spec-kitty-feature-browser"
       Phase   FeaturePhase
       WPCount int          // number of WP files (meaningful when Phase == PhaseTasksReady)
   }
   ```

2. The struct uses relative paths (from CWD) for `Dir`, matching the convention used by `listSpecKittyFeatureDirs()` and `DetectSourceType()`.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Struct fields match data-model.md specification
- [ ] Dir uses relative paths consistent with existing code

---

## Subtask T003: Define lifecycleAction Struct and actionsForPhase()

**Purpose**: Map each feature phase to the applicable spec-kitty lifecycle actions. These actions populate the inline sub-menu when a non-tasks-ready feature is selected.

**Steps**:

1. Define the struct (unexported, used only within the browser):
   ```go
   type lifecycleAction struct {
       label       string // display label, e.g., "clarify"
       description string // display description, e.g., "run /spec-kitty.clarify"
       role        string // agent role for spawn dialog
       promptFmt   string // prompt format string with %s for feature dir
   }
   ```

2. Implement `actionsForPhase(phase FeaturePhase) []lifecycleAction`:
   - `PhaseSpecOnly` returns:
     - `{label: "clarify", description: "run /spec-kitty.clarify", role: "planner", promptFmt: "Run /spec-kitty.clarify for feature %s"}`
     - `{label: "plan", description: "run /spec-kitty.plan", role: "planner", promptFmt: "Run /spec-kitty.plan for feature %s"}`
   - `PhasePlanReady` returns:
     - `{label: "tasks", description: "run /spec-kitty.tasks", role: "planner", promptFmt: "Run /spec-kitty.tasks for feature %s"}`
   - `PhaseTasksReady` returns nil (direct dashboard load, no sub-menu)

**Pattern reference**: The existing `openSpawnDialogWithPrefill(role, prompt, files)` in `internal/tui/overlays.go` (lines 71-79) accepts a role and prompt string. The `promptFmt` field here will be formatted with `fmt.Sprintf(action.promptFmt, entry.Dir)` before passing to that function.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] PhaseSpecOnly returns exactly 2 actions (clarify, plan)
- [ ] PhasePlanReady returns exactly 1 action (tasks)
- [ ] PhaseTasksReady returns nil
- [ ] All roles are "planner" (planning actions, not implementation)
- [ ] promptFmt strings contain exactly one %s placeholder

---

## Subtask T004: Implement parseFeatureDir()

**Purpose**: Extract the feature number and slug from a directory name like `"022-spec-kitty-feature-browser"`. Used by scanFeatures to populate FeatureEntry fields.

**Steps**:

1. Implement `parseFeatureDir(name string) (number, slug string)`:
   - Split on first `-` character
   - Before the first `-` is the number (e.g., "022")
   - After the first `-` is the slug (e.g., "spec-kitty-feature-browser")
   - If no `-` found, number is the full name, slug is empty
   - The `name` parameter is just the directory basename, not a full path

2. Use `strings.SplitN(name, "-", 2)` for the split.

**Edge cases**:
- `"001-simple"` -> ("001", "simple")
- `"022-spec-kitty-feature-browser"` -> ("022", "spec-kitty-feature-browser")
- `"no-number-prefix"` -> ("no", "number-prefix")
- `"standalone"` -> ("standalone", "")

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Normal directory names parse correctly
- [ ] Single-segment names handled (slug = "")
- [ ] Multiple hyphens: only splits on the first one

---

## Subtask T005: Implement scanFeatures()

**Purpose**: Discover all valid spec-kitty features in `kitty-specs/` and classify their lifecycle phase. This is the primary data source for the browser.

**Steps**:

1. Implement `scanFeatures() ([]FeatureEntry, error)`:
   ```go
   func scanFeatures() ([]FeatureEntry, error) {
       // 1. Glob kitty-specs/*/spec.md to find all valid feature directories
       specFiles, err := filepath.Glob(filepath.Join("kitty-specs", "*", "spec.md"))
       if err != nil {
           return nil, fmt.Errorf("scan features: %w", err)
       }

       entries := make([]FeatureEntry, 0, len(specFiles))
       for _, specFile := range specFiles {
           featureDir := filepath.Dir(specFile)
           dirName := filepath.Base(featureDir)
           number, slug := parseFeatureDir(dirName)

           phase, wpCount := detectPhase(featureDir)

           entries = append(entries, FeatureEntry{
               Number:  number,
               Slug:    slug,
               Dir:     featureDir,
               Phase:   phase,
               WPCount: wpCount,
           })
       }

       // 2. Sort by number descending (most recent features first)
       sort.Slice(entries, func(i, j int) bool {
           return entries[i].Number > entries[j].Number
       })

       return entries, nil
   }
   ```

2. Implement the helper `detectPhase(featureDir string) (FeaturePhase, int)`:
   - Check if `plan.md` exists in `featureDir` (using `os.Stat`)
   - Glob `tasks/WP*.md` files in `featureDir`
   - If WP files exist: `PhaseTasksReady` with count
   - Else if plan.md exists: `PhasePlanReady` with 0
   - Else: `PhaseSpecOnly` with 0

3. Required imports: `"fmt"`, `"os"`, `"path/filepath"`, `"sort"`, `"strings"`

**Pattern reference**: The existing `listSpecKittyFeatureDirs()` in `newdialog.go` (lines 544-562) globs `kitty-specs/*/tasks/WP*.md`. Our scanner globs `kitty-specs/*/spec.md` for broader coverage, then checks WP files per-feature.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Only directories containing `spec.md` are included (FR-002)
- [ ] Phase detection classifies correctly based on file existence (FR-004)
- [ ] Results sorted by number descending
- [ ] Empty `kitty-specs/` returns empty slice (not error)
- [ ] Missing `kitty-specs/` directory returns empty slice (not error)

---

## Subtask T006: Implement filterFeatures()

**Purpose**: Filter the feature list by a case-insensitive substring match against the slug. Returns indices into the original entries slice (avoids copying FeatureEntry structs). Used by the filter mode when the user presses `/`.

**Steps**:

1. Implement `filterFeatures(entries []FeatureEntry, query string) []int`:
   ```go
   func filterFeatures(entries []FeatureEntry, query string) []int {
       if query == "" {
           indices := make([]int, len(entries))
           for i := range entries {
               indices[i] = i
           }
           return indices
       }

       lower := strings.ToLower(query)
       indices := make([]int, 0, len(entries))
       for i, entry := range entries {
           if strings.Contains(strings.ToLower(entry.Slug), lower) {
               indices = append(indices, i)
           }
       }
       return indices
   }
   ```

2. Empty query returns all indices (full list, no filtering).
3. No matches returns empty slice.
4. Match is against `Slug` only (not Number or Dir), per AD-005 in plan.md.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Empty query returns all indices in order
- [ ] Case-insensitive matching ("Browser" matches "browser")
- [ ] Partial substring matching ("feat" matches "feature-browser")
- [ ] No matches returns empty slice

---

## Definition of Done

- [ ] `internal/tui/browser.go` exists with all types and functions
- [ ] File compiles: `go build ./internal/tui/`
- [ ] All types match data-model.md specification
- [ ] scanFeatures() correctly discovers features at all three phases
- [ ] filterFeatures() handles empty, partial, and no-match queries
- [ ] phaseBadge() uses colors from existing palette (no new color literals)

## Risks

- **Glob pattern edge cases**: `filepath.Glob` returns nil (not error) for no matches. Handle nil returns as empty slices.
- **Relative paths**: scanFeatures operates on paths relative to CWD. If kasmos is launched from a subdirectory, `kitty-specs/` won't be found. This matches existing behavior (listSpecKittyFeatureDirs has the same constraint).
- **Sort stability**: Go's sort.Slice is not stable. If two features have the same number (unlikely), their order is undefined. This is acceptable.

## Reviewer Guidance

- Verify all types match data-model.md exactly
- Verify phaseBadge uses existing colorMidGray/colorLightBlue/colorGreen (no new colors)
- Verify actionsForPhase mapping matches spec FR-007 and FR-008
- Verify scanFeatures excludes directories without spec.md (FR-002)
- Verify filterFeatures is a pure function with no Model dependency

## Activity Log

- 2026-02-20T07:06:00Z – unknown – shell_pid=3948330 – lane=for_review – Core types and scanner implemented; builds and tests pass
- 2026-02-20T08:34:54Z – unknown – shell_pid=3948330 – lane=done – Previously approved by user.
