package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mustWriteFile creates all parent directories and writes a minimal file.
func mustWriteFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

// setupTestFeatures creates a temp directory with a kitty-specs structure.
// Returns the temp dir path (caller must chdir to it before calling scanFeatures).
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
			os.WriteFile(filepath.Join(featureDir, "spec.md"), []byte("# spec"), 0o644) //nolint:errcheck
		}
		if cfg.hasPlan {
			os.WriteFile(filepath.Join(featureDir, "plan.md"), []byte("# plan"), 0o644) //nolint:errcheck
		}
		if cfg.wpCount > 0 {
			tasksDir := filepath.Join(featureDir, "tasks")
			os.MkdirAll(tasksDir, 0o755) //nolint:errcheck
			for i := 1; i <= cfg.wpCount; i++ {
				filename := fmt.Sprintf("WP%02d-task.md", i)
				os.WriteFile(filepath.Join(tasksDir, filename), []byte("---\n---"), 0o644) //nolint:errcheck
			}
		}
	}

	return dir
}

// ---------------------------------------------------------------------------
// T024: scanFeatures() - table-driven with temp directory structures
// ---------------------------------------------------------------------------

// TestScanFeaturesTableDriven verifies that scanFeatures correctly discovers
// features, classifies their phases, and sorts results descending by number.
//
// NOTE: os.Chdir is process-global; do NOT use t.Parallel() in this test.
func TestScanFeaturesTableDriven(t *testing.T) {
	type featureCfg struct {
		hasSpec bool
		hasPlan bool
		wpCount int
	}

	tests := []struct {
		name      string
		features  map[string]featureCfg
		wantLen   int
		wantFirst string // expected first entry's Number (sorted desc)
	}{
		{
			name: "mixed phases",
			features: map[string]featureCfg{
				"001-alpha": {hasSpec: true, hasPlan: false, wpCount: 0},
				"002-beta":  {hasSpec: true, hasPlan: true, wpCount: 0},
				"003-gamma": {hasSpec: true, hasPlan: true, wpCount: 3},
			},
			wantLen:   3,
			wantFirst: "003", // highest number first
		},
		{
			name: "excludes dirs without spec.md",
			features: map[string]featureCfg{
				"001-valid":   {hasSpec: true, hasPlan: false, wpCount: 0},
				"002-invalid": {hasSpec: false, hasPlan: true, wpCount: 5},
			},
			wantLen:   1,
			wantFirst: "001",
		},
		{
			name:     "empty kitty-specs",
			features: map[string]featureCfg{},
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to the anonymous struct type expected by setupTestFeatures.
			cfgs := make(map[string]struct {
				hasSpec bool
				hasPlan bool
				wpCount int
			}, len(tt.features))
			for k, v := range tt.features {
				cfgs[k] = struct {
					hasSpec bool
					hasPlan bool
					wpCount int
				}{v.hasSpec, v.hasPlan, v.wpCount}
			}

			dir := setupTestFeatures(t, cfgs)

			// chdir so scanFeatures finds kitty-specs/ via relative path.
			origDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("getwd: %v", err)
			}
			if err := os.Chdir(dir); err != nil {
				t.Fatalf("chdir: %v", err)
			}
			defer os.Chdir(origDir) //nolint:errcheck

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

// TestScanFeatures verifies the full scan: sort order, phase classification,
// exclusion of dirs without spec.md, and relative Dir field.
//
// NOTE: os.Chdir is process-global; do NOT use t.Parallel() in this test.
func TestScanFeatures(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	mustWriteFile(t, filepath.Join(root, "kitty-specs", "021-old", "spec.md"))
	mustWriteFile(t, filepath.Join(root, "kitty-specs", "021-old", "plan.md"))
	mustWriteFile(t, filepath.Join(root, "kitty-specs", "022-current", "spec.md"))
	mustWriteFile(t, filepath.Join(root, "kitty-specs", "022-current", "tasks", "WP01-a.md"))
	mustWriteFile(t, filepath.Join(root, "kitty-specs", "022-current", "tasks", "WP02-b.md"))
	mustWriteFile(t, filepath.Join(root, "kitty-specs", "020-spec-only", "spec.md"))
	mustWriteFile(t, filepath.Join(root, "kitty-specs", "023-no-spec", "plan.md"))

	entries, err := scanFeatures()
	if err != nil {
		t.Fatalf("scanFeatures() error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("scanFeatures() length mismatch: got=%d want=3", len(entries))
	}

	if entries[0].Number != "022" || entries[0].Phase != PhaseTasksReady || entries[0].WPCount != 2 {
		t.Fatalf("first entry mismatch: got=%+v", entries[0])
	}
	if entries[1].Number != "021" || entries[1].Phase != PhasePlanReady {
		t.Fatalf("second entry mismatch: got=%+v", entries[1])
	}
	if entries[2].Number != "020" || entries[2].Phase != PhaseSpecOnly {
		t.Fatalf("third entry mismatch: got=%+v", entries[2])
	}

	if entries[0].Dir != filepath.Join("kitty-specs", "022-current") {
		t.Fatalf("relative dir mismatch: got=%q want=%q", entries[0].Dir, filepath.Join("kitty-specs", "022-current"))
	}
}

// TestScanFeaturesEmptyOrMissingKittySpecs verifies that scanFeatures returns
// an empty slice (not an error) when kitty-specs/ is absent or empty.
//
// NOTE: os.Chdir is process-global; do NOT use t.Parallel() in this test.
func TestScanFeaturesEmptyOrMissingKittySpecs(t *testing.T) {
	t.Run("missing kitty-specs", func(t *testing.T) {
		root := t.TempDir()
		t.Chdir(root)

		entries, err := scanFeatures()
		if err != nil {
			t.Fatalf("scanFeatures() error: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("entries length mismatch: got=%d want=0", len(entries))
		}
	})

	t.Run("empty kitty-specs", func(t *testing.T) {
		root := t.TempDir()
		t.Chdir(root)

		if err := os.MkdirAll(filepath.Join(root, "kitty-specs"), 0o755); err != nil {
			t.Fatalf("create kitty-specs: %v", err)
		}

		entries, err := scanFeatures()
		if err != nil {
			t.Fatalf("scanFeatures() error: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("entries length mismatch: got=%d want=0", len(entries))
		}
	})
}

// ---------------------------------------------------------------------------
// T025: detectPhase() - accuracy across all phase combinations
// ---------------------------------------------------------------------------

// TestDetectPhase verifies that detectPhase correctly classifies features into
// PhaseSpecOnly, PhasePlanReady, and PhaseTasksReady based on file existence.
// Uses absolute temp dir paths so t.Parallel() is safe here.
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
		// EC-2: WPs without plan.md still detected as TasksReady (file-existence-based).
		{name: "tasks ready no plan", hasPlan: false, wpCount: 3, wantPhase: PhaseTasksReady, wantWPs: 3},
		// EC-2 variant: completed work (plan + WPs) still classifies as TasksReady.
		{name: "tasks ready all done lane files", hasPlan: true, wpCount: 2, wantPhase: PhaseTasksReady, wantWPs: 2},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# spec"), 0o644) //nolint:errcheck
			if tt.hasPlan {
				os.WriteFile(filepath.Join(dir, "plan.md"), []byte("# plan"), 0o644) //nolint:errcheck
			}
			if tt.wpCount > 0 {
				tasksDir := filepath.Join(dir, "tasks")
				os.MkdirAll(tasksDir, 0o755) //nolint:errcheck
				for i := 1; i <= tt.wpCount; i++ {
					filename := fmt.Sprintf("WP%02d-slug.md", i)
					os.WriteFile(filepath.Join(tasksDir, filename), []byte("---"), 0o644) //nolint:errcheck
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

// TestDetectPhaseAbsolutePaths verifies detectPhase with the three canonical
// phase directories created in a single shared temp root.
func TestDetectPhaseAbsolutePaths(t *testing.T) {
	root := t.TempDir()

	specOnlyDir := filepath.Join(root, "kitty-specs", "001-spec-only")
	planReadyDir := filepath.Join(root, "kitty-specs", "002-plan-ready")
	tasksReadyDir := filepath.Join(root, "kitty-specs", "003-tasks-ready")

	mustWriteFile(t, filepath.Join(specOnlyDir, "spec.md"))
	mustWriteFile(t, filepath.Join(planReadyDir, "spec.md"))
	mustWriteFile(t, filepath.Join(planReadyDir, "plan.md"))
	mustWriteFile(t, filepath.Join(tasksReadyDir, "spec.md"))
	mustWriteFile(t, filepath.Join(tasksReadyDir, "plan.md"))
	mustWriteFile(t, filepath.Join(tasksReadyDir, "tasks", "WP01-one.md"))
	mustWriteFile(t, filepath.Join(tasksReadyDir, "tasks", "WP02-two.md"))

	tests := []struct {
		name      string
		feature   string
		wantPhase FeaturePhase
		wantCount int
	}{
		{name: "spec only", feature: specOnlyDir, wantPhase: PhaseSpecOnly, wantCount: 0},
		{name: "plan ready", feature: planReadyDir, wantPhase: PhasePlanReady, wantCount: 0},
		{name: "tasks ready", feature: tasksReadyDir, wantPhase: PhaseTasksReady, wantCount: 2},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			phase, count := detectPhase(tt.feature)
			if phase != tt.wantPhase || count != tt.wantCount {
				t.Fatalf("detectPhase() mismatch: got=(%v,%d) want=(%v,%d)", phase, count, tt.wantPhase, tt.wantCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// T026: actionsForPhase() - mapping correctness and prompt format
// ---------------------------------------------------------------------------

// TestActionsForPhaseTableDriven verifies that each phase maps to the correct
// lifecycle actions with correct labels, roles, and counts.
func TestActionsForPhaseTableDriven(t *testing.T) {
	tests := []struct {
		phase      FeaturePhase
		wantCount  int
		wantLabels []string
		wantRoles  []string
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
		tt := tt
		t.Run(tt.phase.String(), func(t *testing.T) {
			t.Parallel()
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

// TestActionsForPhase verifies the legacy non-table-driven assertions for
// completeness and backward compatibility with existing test coverage.
func TestActionsForPhase(t *testing.T) {
	specOnly := actionsForPhase(PhaseSpecOnly)
	if len(specOnly) != 2 {
		t.Fatalf("PhaseSpecOnly actions length: got=%d want=2", len(specOnly))
	}
	if specOnly[0].label != "clarify" || specOnly[1].label != "plan" {
		t.Fatalf("PhaseSpecOnly actions order mismatch: got=%q,%q", specOnly[0].label, specOnly[1].label)
	}
	for _, action := range specOnly {
		if action.role != "planner" {
			t.Fatalf("spec action role mismatch: got=%q want=%q", action.role, "planner")
		}
		if strings.Count(action.promptFmt, "%s") != 1 {
			t.Fatalf("spec action promptFmt placeholders: got=%q", action.promptFmt)
		}
	}

	planReady := actionsForPhase(PhasePlanReady)
	if len(planReady) != 1 {
		t.Fatalf("PhasePlanReady actions length: got=%d want=1", len(planReady))
	}
	if planReady[0].label != "tasks" {
		t.Fatalf("PhasePlanReady action label mismatch: got=%q want=%q", planReady[0].label, "tasks")
	}
	if planReady[0].role != "planner" {
		t.Fatalf("PhasePlanReady action role mismatch: got=%q want=%q", planReady[0].role, "planner")
	}
	if strings.Count(planReady[0].promptFmt, "%s") != 1 {
		t.Fatalf("PhasePlanReady promptFmt placeholders: got=%q", planReady[0].promptFmt)
	}

	if got := actionsForPhase(PhaseTasksReady); got != nil {
		t.Fatalf("PhaseTasksReady actions mismatch: got=%v want=nil", got)
	}
}

// TestActionsForPhasePromptFormat verifies that promptFmt correctly interpolates
// the feature directory path for all non-nil phases.
func TestActionsForPhasePromptFormat(t *testing.T) {
	for _, phase := range []FeaturePhase{PhaseSpecOnly, PhasePlanReady} {
		phase := phase
		t.Run(phase.String(), func(t *testing.T) {
			t.Parallel()
			actions := actionsForPhase(phase)
			for i, action := range actions {
				result := fmt.Sprintf(action.promptFmt, "kitty-specs/test-feature")
				if !strings.Contains(result, "kitty-specs/test-feature") {
					t.Errorf("action[%d] promptFmt %q did not interpolate feature dir", i, action.promptFmt)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// T027: parseFeatureDir() - edge cases
// ---------------------------------------------------------------------------

// TestParseFeatureDir verifies directory name parsing handles standard and
// edge-case names correctly, including empty input and degenerate separators.
func TestParseFeatureDir(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNumber string
		wantSlug   string
	}{
		// Standard cases
		{name: "standard 3-digit prefix", input: "022-spec-kitty-feature-browser", wantNumber: "022", wantSlug: "spec-kitty-feature-browser"},
		{name: "simple", input: "001-simple", wantNumber: "001", wantSlug: "simple"},
		{name: "multiple hyphens", input: "010-hub-tui-navigator", wantNumber: "010", wantSlug: "hub-tui-navigator"},
		{name: "non numeric prefix", input: "no-number-prefix", wantNumber: "no", wantSlug: "number-prefix"},
		{name: "standalone", input: "standalone", wantNumber: "standalone", wantSlug: ""},
		// Edge cases
		{name: "no hyphen", input: "standalone", wantNumber: "standalone", wantSlug: ""},
		{name: "leading hyphen", input: "-weird", wantNumber: "", wantSlug: "weird"},
		{name: "empty", input: "", wantNumber: "", wantSlug: ""},
		{name: "only hyphen", input: "-", wantNumber: "", wantSlug: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

// ---------------------------------------------------------------------------
// T028: filterFeatures() - case-insensitive matching and edge cases
// ---------------------------------------------------------------------------

// testEntries is the shared dataset for filterFeatures tests.
var testEntries = []FeatureEntry{
	{Number: "022", Slug: "spec-kitty-feature-browser"},
	{Number: "018", Slug: "blocked-task-visual-feedback"},
	{Number: "016", Slug: "kasmos-agent-orchestrator"},
	{Number: "010", Slug: "hub-tui-navigator"},
}

// TestFilterFeatures verifies the filter function handles case-insensitive
// matching, empty queries, partial matches, and no-match scenarios.
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
		// "a" appears in spec-kitty-feature-browser, blocked-task-visual-feedback,
		// kasmos-agent-orchestrator, hub-tui-navigator -> all 4
		{name: "multiple matches", query: "a", wantIndices: []int{0, 1, 2, 3}},
		{name: "no matches", query: "zzz", wantIndices: []int{}},
		{name: "substring in middle", query: "task", wantIndices: []int{1}},
		{name: "mixed case partial", query: "Kit", wantIndices: []int{0}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterFeatures(testEntries, tt.query)
			// Normalize nil vs empty slice for comparison.
			if len(got) == 0 && len(tt.wantIndices) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.wantIndices) {
				t.Fatalf("filterFeatures(%q) = %v, want %v", tt.query, got, tt.wantIndices)
			}
		})
	}
}

// TestFilterFeaturesLegacy preserves the original test coverage with the
// Browser-helpers entry that exercises mixed-case slug matching.
func TestFilterFeaturesLegacy(t *testing.T) {
	entries := []FeatureEntry{
		{Number: "022", Slug: "spec-kitty-feature-browser"},
		{Number: "021", Slug: "session-history"},
		{Number: "020", Slug: "Browser-helpers"},
	}

	tests := []struct {
		name  string
		query string
		want  []int
	}{
		{name: "empty query", query: "", want: []int{0, 1, 2}},
		{name: "case insensitive", query: "browser", want: []int{0, 2}},
		{name: "partial", query: "feat", want: []int{0}},
		{name: "no match", query: "missing", want: []int{}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterFeatures(entries, tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("filterFeatures() mismatch: got=%v want=%v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// T026 (additional): FeaturePhase.String() and phaseBadge()
// ---------------------------------------------------------------------------

func TestFeaturePhaseString(t *testing.T) {
	tests := []struct {
		name  string
		phase FeaturePhase
		want  string
	}{
		{name: "spec only", phase: PhaseSpecOnly, want: "spec only"},
		{name: "plan ready", phase: PhasePlanReady, want: "plan ready"},
		{name: "tasks ready", phase: PhaseTasksReady, want: "tasks ready"},
		{name: "unknown", phase: FeaturePhase(99), want: "unknown"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.phase.String(); got != tt.want {
				t.Fatalf("String() mismatch: got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestPhaseBadge(t *testing.T) {
	tests := []struct {
		name    string
		phase   FeaturePhase
		wpCount int
		want    string
	}{
		{name: "spec only", phase: PhaseSpecOnly, wpCount: 0, want: "spec only"},
		{name: "plan ready", phase: PhasePlanReady, wpCount: 0, want: "plan ready"},
		{name: "tasks ready", phase: PhaseTasksReady, wpCount: 3, want: "tasks ready (3 WPs)"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := phaseBadge(tt.phase, tt.wpCount)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("phaseBadge() missing label: got=%q want-contains=%q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// T029: BenchmarkScanFeatures - NFR-001 (under 200ms for 50 features)
// ---------------------------------------------------------------------------

// BenchmarkScanFeatures validates scan performance against NFR-001.
// Expected result: comfortably below 200ms/op on local filesystems with 50 features.
//
// Dataset: 20 spec-only + 15 plan-ready + 15 tasks-ready = 50 features.
//
// NOTE: os.Chdir is process-global; do NOT use b.RunParallel() in this benchmark.
func BenchmarkScanFeatures(b *testing.B) {
	root := b.TempDir()
	kittyDir := filepath.Join(root, "kitty-specs")
	if err := os.MkdirAll(kittyDir, 0o755); err != nil {
		b.Fatalf("mkdir kitty-specs: %v", err)
	}

	// 20 spec-only features.
	for i := 1; i <= 20; i++ {
		dir := filepath.Join(kittyDir, fmt.Sprintf("%03d-spec-only-%d", i, i))
		os.MkdirAll(dir, 0o755)                                         //nolint:errcheck
		os.WriteFile(filepath.Join(dir, "spec.md"), []byte("x"), 0o644) //nolint:errcheck
	}

	// 15 plan-ready features.
	for i := 21; i <= 35; i++ {
		dir := filepath.Join(kittyDir, fmt.Sprintf("%03d-plan-ready-%d", i, i))
		os.MkdirAll(dir, 0o755)                                         //nolint:errcheck
		os.WriteFile(filepath.Join(dir, "spec.md"), []byte("x"), 0o644) //nolint:errcheck
		os.WriteFile(filepath.Join(dir, "plan.md"), []byte("x"), 0o644) //nolint:errcheck
	}

	// 15 tasks-ready features (3 WPs each).
	for i := 36; i <= 50; i++ {
		dir := filepath.Join(kittyDir, fmt.Sprintf("%03d-tasks-ready-%d", i, i))
		tasksDir := filepath.Join(dir, "tasks")
		os.MkdirAll(tasksDir, 0o755)                                    //nolint:errcheck
		os.WriteFile(filepath.Join(dir, "spec.md"), []byte("x"), 0o644) //nolint:errcheck
		os.WriteFile(filepath.Join(dir, "plan.md"), []byte("x"), 0o644) //nolint:errcheck
		for wp := 1; wp <= 3; wp++ {
			name := fmt.Sprintf("WP%02d-task.md", wp)
			os.WriteFile(filepath.Join(tasksDir, name), []byte("---"), 0o644) //nolint:errcheck
		}
	}

	origDir, err := os.Getwd()
	if err != nil {
		b.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		b.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entries, err := scanFeatures()
		if err != nil {
			b.Fatalf("scanFeatures() error: %v", err)
		}
		if len(entries) != 50 {
			b.Fatalf("expected 50 entries, got %d", len(entries))
		}
	}
}
