package orchestration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- planner draft cache helpers ---

func TestPlannerDraftCacheFilename(t *testing.T) {
	name, err := PlannerDraftCacheFilename("my-plan", "claude")
	require.NoError(t, err)
	assert.Equal(t, "my-plan-planner-claude.md", name)

	_, err = PlannerDraftCacheFilename("", "claude")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan slug")

	_, err = PlannerDraftCacheFilename("plan", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "profile")

	_, err = PlannerDraftCacheFilename("plan/bad", "claude")
	require.Error(t, err)

	_, err = PlannerDraftCacheFilename("plan", "pro/file")
	require.Error(t, err)
}

func TestSaveAndLoadPlannerDraft(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	markdown := "## draft\n\n- wave 1\n- wave 2\n"

	require.NoError(t, SavePlannerDraft(cacheDir, "my-plan", "claude", markdown))

	entry, err := LoadPlannerDraft(cacheDir, "my-plan", "claude")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "claude", entry.Profile)
	assert.Equal(t, markdown, entry.Markdown)
	assert.True(t, entry.ModTime.IsZero() == false)
	assert.NotEmpty(t, entry.Path)

	// cacheDir created
	info, err := os.Stat(cacheDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestLoadPlannerDraft_Missing(t *testing.T) {
	tmp := t.TempDir()
	entry, err := LoadPlannerDraft(filepath.Join(tmp, "cache"), "plan", "codex")
	require.NoError(t, err)
	assert.Nil(t, entry)
}

func TestListPlannerDraftCaches(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")

	require.NoError(t, SavePlannerDraft(cacheDir, "plan", "zephyr", "zephyr draft"))
	require.NoError(t, SavePlannerDraft(cacheDir, "plan", "alpha", "alpha draft"))
	require.NoError(t, SavePlannerDraft(cacheDir, "plan", "bravo", "bravo draft"))
	// unrelated file – should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "plan-architect.json"), []byte("{}"), 0o644))

	entries, err := ListPlannerDraftCaches(cacheDir, "plan")
	require.NoError(t, err)
	require.Len(t, entries, 3)
	// sorted by profile
	assert.Equal(t, "alpha", entries[0].Profile)
	assert.Equal(t, "bravo", entries[1].Profile)
	assert.Equal(t, "zephyr", entries[2].Profile)
	assert.Equal(t, "alpha draft", entries[0].Markdown)
}

func TestListPlannerDraftCaches_MissingDir(t *testing.T) {
	tmp := t.TempDir()
	entries, err := ListPlannerDraftCaches(filepath.Join(tmp, "no-such-dir"), "plan")
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestClearPlannerDraftCaches(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	planSlug := "plan"

	require.NoError(t, SavePlannerDraft(cacheDir, planSlug, "alpha", "alpha draft"))
	require.NoError(t, SavePlannerDraft(cacheDir, planSlug, "bravo", "bravo draft"))
	// legacy baseline file
	legacyPath := filepath.Join(cacheDir, planSlug+"-architect-baseline.json")
	require.NoError(t, os.WriteFile(legacyPath, []byte(`{}`), 0o644))
	// unrelated file – must survive
	survivorPath := filepath.Join(cacheDir, planSlug+"-architect.json")
	require.NoError(t, os.WriteFile(survivorPath, []byte(`{}`), 0o644))

	require.NoError(t, ClearPlannerDraftCaches(cacheDir, planSlug))

	entries, err := ListPlannerDraftCaches(cacheDir, planSlug)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.NoFileExists(t, legacyPath)
	assert.FileExists(t, survivorPath)

	// idempotent – missing files are not errors
	require.NoError(t, ClearPlannerDraftCaches(cacheDir, planSlug))
}

func TestClearPlannerDraftCaches_MissingDir(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, ClearPlannerDraftCaches(filepath.Join(tmp, "no-such-dir"), "plan"))
}

func TestSaveAndLoadArchitectMeta(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	planSlug := "planner"

	original := &ArchitectMeta{
		PlanID:          "plan-123",
		SchemaVersion:   1,
		ArchitectModel:  "model-alpha",
		ArchitectEffort: "high",
		CacheVersion:    3,
		Waves: []WaveMeta{{
			Wave:             1,
			Parallel:         true,
			ConflictAnalysis: "contentious file conflict",
			Tasks: []TaskMeta{
				{
					TaskNumber:        1,
					Title:             "Implement task one",
					PreferredModel:    "model-small",
					FallbackModel:     "model-medium",
					EscalationPolicy:  "manual",
					EstimatedTokens:   1200,
					FilesToModify:     []string{"file1.go", "file2.go"},
					DependencyNumbers: []int{2},
					VerifyChecks:      []string{"go test ./..."},
					ContextRefs:       []string{"ref://task-1"},
				},
			},
		}},
		CachedSnippets: map[string]string{"snippet:one": "code"},
		DecisionAudit: &ArchitectDecisionAudit{
			SchemaVersion:   architectDecisionAuditSchemaVersion,
			PlanFile:        "planner",
			Project:         "kasmos",
			CreatedAt:       time.Date(2026, 4, 24, 12, 30, 0, 0, time.UTC),
			BaselineSource:  "parallel_cache",
			Summary:         "architect accepted the shape with one routing adjustment",
			PlannerSummary:  "planner split metadata and hq API tasks",
			BaselineSummary: "baseline kept metadata scoped to orchestration cache",
			FinalDecision:   "store the decision audit inside architect metadata",
			Differences: []ArchitectDecisionDifference{{
				Area:              "metadata cache",
				Scope:             "orchestration",
				PlannerProposal:   "add an external planner snapshot",
				ArchitectBaseline: "reuse existing architect cache",
				FinalDecision:     "add an optional decision_audit object",
				Rationale:         "keeps cache ownership in one artifact",
				RelatedFiles:      []string{"orchestration/meta.go", "orchestration/cache.go"},
				TaskNumbers:       []int{1, 2},
			}},
			PlannerDrafts: []ArchitectPlannerDraftDecision{
				{
					Profile:   "claude",
					CachePath: ".kasmos/cache/planner-planner-claude.md",
					Summary:   "claude proposed two waves",
					Decision:  "accept",
					Rationale: "well-structured",
				},
			},
		},
	}

	require.NoError(t, SaveArchitectMeta(cacheDir, planSlug, original))
	loaded, err := LoadArchitectMeta(cacheDir, planSlug)
	require.NoError(t, err)

	assert.Equal(t, original, loaded)

	filename := filepath.Join(cacheDir, planSlug+"-architect.json")
	require.FileExists(t, filename)
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, byte('\n'), data[len(data)-1])

	require.Len(t, loaded.Waves, 1)
	assert.Equal(t, 1, loaded.Waves[0].Tasks[0].TaskNumber)
}

func TestLoadArchitectMeta_BackwardsCompatibleWithoutDecisionAudit(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	planSlug := "planner"
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, architectMetaFilename(planSlug)), []byte(`{
  "plan_id": "plan-123",
  "schema_version": 1,
  "waves": [],
  "cache_version": 3
}
`), 0o644))

	loaded, err := LoadArchitectMeta(cacheDir, planSlug)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Nil(t, loaded.DecisionAudit)
	assert.Equal(t, "plan-123", loaded.PlanID)
}

func TestLoadArchitectMeta_Missing(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")

	meta, err := LoadArchitectMeta(cacheDir, "missing")
	require.NoError(t, err)
	assert.Nil(t, meta)
	assert.False(t, ArchitectMetaExists(cacheDir, "missing"))
}

func TestArchitectMetaExists(t *testing.T) {
	tmp := t.TempDir()
	planSlug := "planner"
	cacheDir := filepath.Join(tmp, "cache")

	meta := &ArchitectMeta{}
	require.NoError(t, SaveArchitectMeta(cacheDir, planSlug, meta))

	assert.True(t, ArchitectMetaExists(cacheDir, planSlug))
	assert.False(t, ArchitectMetaExists(cacheDir, "other"))
}

func TestSaveArchitectMeta_Creates(t *testing.T) {
	tmp := t.TempDir()
	nestedDir := filepath.Join(tmp, "level1", "level2")
	planSlug := "nested"

	require.NoError(t, SaveArchitectMeta(nestedDir, planSlug, &ArchitectMeta{}))

	assert.True(t, ArchitectMetaExists(nestedDir, planSlug))
	info, err := os.Stat(nestedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestTaskMeta_LookupByNumber(t *testing.T) {
	meta := &ArchitectMeta{Waves: []WaveMeta{{
		Tasks: []TaskMeta{{TaskNumber: 1}, {TaskNumber: 3}},
	}, {
		Tasks: []TaskMeta{{TaskNumber: 7}},
	}}}

	found := meta.TaskByNumber(3)
	require.NotNil(t, found)
	assert.Equal(t, 3, found.TaskNumber)

	assert.Nil(t, meta.TaskByNumber(99))

	var nilMeta *ArchitectMeta
	assert.Nil(t, nilMeta.TaskByNumber(1))
}

func TestValidateArchitectDecisionAudit(t *testing.T) {
	const planFile = "planner"
	const project = "kasmos"
	valid := func() *ArchitectDecisionAudit {
		return &ArchitectDecisionAudit{
			SchemaVersion:  architectDecisionAuditSchemaVersion,
			PlanFile:       planFile,
			Project:        project,
			BaselineSource: "parallel_cache",
			FinalDecision:  "accept planner draft with metadata audit",
			Summary:        "no task shape changes",
			Differences: []ArchitectDecisionDifference{{
				Area:          "orchestration cache",
				FinalDecision: "add optional audit field",
			}},
		}
	}

	require.NoError(t, ValidateArchitectDecisionAudit(valid(), planFile, project))
	require.NoError(t, ValidateArchitectDecisionAudit(&ArchitectDecisionAudit{
		SchemaVersion:  architectDecisionAuditSchemaVersion,
		PlanFile:       planFile,
		Project:        project,
		BaselineSource: "inline",
		FinalDecision:  "accepted unchanged",
		Summary:        "planner draft accepted unchanged",
	}, planFile, project))
	// valid planner drafts
	withDrafts := valid()
	withDrafts.PlannerDrafts = []ArchitectPlannerDraftDecision{
		{Profile: "claude", Decision: "accept"},
		{Profile: "codex", Decision: "reject", Rationale: "too broad"},
	}
	require.NoError(t, ValidateArchitectDecisionAudit(withDrafts, planFile, project))

	tests := []struct {
		name      string
		audit     *ArchitectDecisionAudit
		errSubstr string
	}{
		{
			name:      "nil audit",
			audit:     nil,
			errSubstr: "nil",
		},
		{
			name: "schema mismatch",
			audit: func() *ArchitectDecisionAudit {
				a := valid()
				a.SchemaVersion = 999
				return a
			}(),
			errSubstr: "unsupported",
		},
		{
			name: "wrong plan",
			audit: func() *ArchitectDecisionAudit {
				a := valid()
				a.PlanFile = "other"
				return a
			}(),
			errSubstr: "plan file mismatch",
		},
		{
			name: "wrong project",
			audit: func() *ArchitectDecisionAudit {
				a := valid()
				a.Project = "other"
				return a
			}(),
			errSubstr: "project mismatch",
		},
		{
			name: "empty baseline source",
			audit: func() *ArchitectDecisionAudit {
				a := valid()
				a.BaselineSource = "  "
				return a
			}(),
			errSubstr: "baseline source is empty",
		},
		{
			name: "unsupported baseline source",
			audit: func() *ArchitectDecisionAudit {
				a := valid()
				a.BaselineSource = "planner draft"
				return a
			}(),
			errSubstr: "unsupported",
		},
		{
			name: "empty final decision",
			audit: func() *ArchitectDecisionAudit {
				a := valid()
				a.FinalDecision = "  "
				return a
			}(),
			errSubstr: "final decision is empty",
		},
		{
			name: "empty difference area",
			audit: func() *ArchitectDecisionAudit {
				a := valid()
				a.Differences[0].Area = " "
				return a
			}(),
			errSubstr: "area is empty",
		},
		{
			name: "empty difference final decision",
			audit: func() *ArchitectDecisionAudit {
				a := valid()
				a.Differences[0].FinalDecision = " "
				return a
			}(),
			errSubstr: "difference 0 final decision is empty",
		},
		{
			name: "planner draft missing profile",
			audit: func() *ArchitectDecisionAudit {
				a := valid()
				a.PlannerDrafts = []ArchitectPlannerDraftDecision{{Profile: "", Decision: "accept"}}
				return a
			}(),
			errSubstr: "planner draft 0 profile is empty",
		},
		{
			name: "planner draft missing decision",
			audit: func() *ArchitectDecisionAudit {
				a := valid()
				a.PlannerDrafts = []ArchitectPlannerDraftDecision{{Profile: "claude", Decision: "  "}}
				return a
			}(),
			errSubstr: "planner draft 0 decision is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArchitectDecisionAudit(tt.audit, planFile, project)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}

