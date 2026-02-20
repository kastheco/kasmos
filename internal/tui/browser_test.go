package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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
		t.Run(tt.name, func(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			got := phaseBadge(tt.phase, tt.wpCount)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("phaseBadge() missing label: got=%q want-contains=%q", got, tt.want)
			}
		})
	}
}

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

func TestParseFeatureDir(t *testing.T) {
	tests := []struct {
		name       string
		dirName    string
		wantNumber string
		wantSlug   string
	}{
		{name: "simple", dirName: "001-simple", wantNumber: "001", wantSlug: "simple"},
		{name: "multi hyphen", dirName: "022-spec-kitty-feature-browser", wantNumber: "022", wantSlug: "spec-kitty-feature-browser"},
		{name: "non numeric prefix", dirName: "no-number-prefix", wantNumber: "no", wantSlug: "number-prefix"},
		{name: "standalone", dirName: "standalone", wantNumber: "standalone", wantSlug: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			number, slug := parseFeatureDir(tt.dirName)
			if number != tt.wantNumber || slug != tt.wantSlug {
				t.Fatalf("parseFeatureDir() mismatch: got=(%q,%q) want=(%q,%q)", number, slug, tt.wantNumber, tt.wantSlug)
			}
		})
	}
}

func TestFilterFeatures(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			got := filterFeatures(entries, tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("filterFeatures() mismatch: got=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestDetectPhase(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			phase, count := detectPhase(tt.feature)
			if phase != tt.wantPhase || count != tt.wantCount {
				t.Fatalf("detectPhase() mismatch: got=(%v,%d) want=(%v,%d)", phase, count, tt.wantPhase, tt.wantCount)
			}
		})
	}
}

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

func mustWriteFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
