package orchestration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestSaveAndLoadArchitectBaseline(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	planSlug := "planner"
	identity := NewArchitectBaselineIdentity("planner", "kasmos", "goal text")
	original := &ArchitectBaseline{
		SchemaVersion:    architectBaselineSchemaVersion,
		PlanFile:         identity.PlanFile,
		Project:          identity.Project,
		DescriptionHash:  identity.DescriptionHash,
		CreatedAt:        time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
		BaselineMarkdown: "## baseline\n\n- task shape",
		Surfaces:         []string{"orchestration"},
		Risks:            []string{"stale draft"},
		Notes:            []string{"advisory only"},
	}

	require.NoError(t, SaveArchitectBaseline(cacheDir, planSlug, original))
	loaded, err := LoadArchitectBaseline(cacheDir, planSlug)
	require.NoError(t, err)

	assert.Equal(t, original, loaded)
	require.NoError(t, ValidateArchitectBaseline(loaded, identity))
	assert.True(t, ArchitectBaselineExists(cacheDir, planSlug))

	filename := filepath.Join(cacheDir, planSlug+"-architect-baseline.json")
	require.FileExists(t, filename)
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, byte('\n'), data[len(data)-1])
}

func TestArchitectBaselineDescriptionHash(t *testing.T) {
	assert.Equal(t, ArchitectBaselineDescriptionHash("goal text"), ArchitectBaselineDescriptionHash("goal text"))
	assert.NotEqual(t, ArchitectBaselineDescriptionHash("goal text"), ArchitectBaselineDescriptionHash("goal text "))
}

func TestLoadArchitectBaseline_Missing(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")

	baseline, err := LoadArchitectBaseline(cacheDir, "missing")
	require.NoError(t, err)
	assert.Nil(t, baseline)
	assert.False(t, ArchitectBaselineExists(cacheDir, "missing"))
}

func TestLoadArchitectBaseline_CorruptJSON(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, architectBaselineFilename("planner")), []byte("{bad json"), 0o644))

	baseline, err := LoadArchitectBaseline(cacheDir, "planner")
	assert.Nil(t, baseline)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read architect baseline")
}

func TestValidateArchitectBaseline(t *testing.T) {
	identity := NewArchitectBaselineIdentity("planner", "kasmos", "goal text")
	valid := func() *ArchitectBaseline {
		return &ArchitectBaseline{
			SchemaVersion:    architectBaselineSchemaVersion,
			PlanFile:         identity.PlanFile,
			Project:          identity.Project,
			DescriptionHash:  identity.DescriptionHash,
			BaselineMarkdown: "baseline",
		}
	}

	tests := []struct {
		name      string
		baseline  *ArchitectBaseline
		expected  ArchitectBaselineIdentity
		errSubstr string
	}{
		{
			name:      "nil baseline",
			baseline:  nil,
			expected:  identity,
			errSubstr: "nil",
		},
		{
			name: "empty baseline markdown",
			baseline: func() *ArchitectBaseline {
				b := valid()
				b.BaselineMarkdown = ""
				return b
			}(),
			expected:  identity,
			errSubstr: "markdown is empty",
		},
		{
			name: "mismatched plan file",
			baseline: func() *ArchitectBaseline {
				b := valid()
				b.PlanFile = "other"
				return b
			}(),
			expected:  identity,
			errSubstr: "plan file mismatch",
		},
		{
			name: "mismatched project",
			baseline: func() *ArchitectBaseline {
				b := valid()
				b.Project = "other"
				return b
			}(),
			expected:  identity,
			errSubstr: "project mismatch",
		},
		{
			name: "stale identity",
			baseline: func() *ArchitectBaseline {
				b := valid()
				b.DescriptionHash = ArchitectBaselineDescriptionHash("old goal")
				return b
			}(),
			expected:  identity,
			errSubstr: "description hash mismatch",
		},
		{
			name: "unsupported schema version",
			baseline: func() *ArchitectBaseline {
				b := valid()
				b.SchemaVersion = 999
				return b
			}(),
			expected:  identity,
			errSubstr: "unsupported",
		},
	}

	require.NoError(t, ValidateArchitectBaseline(valid(), identity))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArchitectBaseline(tt.baseline, tt.expected)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}

func TestClearArchitectBaseline(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	planSlug := "planner"

	require.NoError(t, SaveArchitectMeta(cacheDir, planSlug, &ArchitectMeta{}))
	require.NoError(t, SaveArchitectBaseline(cacheDir, planSlug, &ArchitectBaseline{
		SchemaVersion:    architectBaselineSchemaVersion,
		PlanFile:         "planner",
		Project:          "kasmos",
		DescriptionHash:  ArchitectBaselineDescriptionHash("goal"),
		BaselineMarkdown: "baseline",
	}))

	require.NoError(t, ClearArchitectBaseline(cacheDir, planSlug))
	assert.False(t, ArchitectBaselineExists(cacheDir, planSlug))
	assert.True(t, ArchitectMetaExists(cacheDir, planSlug))
	require.NoError(t, ClearArchitectBaseline(cacheDir, planSlug))
}
