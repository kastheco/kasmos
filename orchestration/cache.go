package orchestration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	architectDecisionAuditSchemaVersion = 1
)

func architectMetaFilename(planSlug string) string {
	return planSlug + "-architect.json"
}

// SaveArchitectMeta serializes meta to JSON and writes it to cacheDir/<planSlug>-architect.json,
// creating cacheDir with mode 0755 if it does not exist.
func SaveArchitectMeta(cacheDir, planSlug string, meta *ArchitectMeta) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	filename := filepath.Join(cacheDir, architectMetaFilename(planSlug))
	encoded = append(encoded, '\n')
	return os.WriteFile(filename, encoded, 0o644)
}

// LoadArchitectMeta reads and deserializes the architect metadata file for planSlug from cacheDir.
// Returns (nil, nil) when the file does not exist.
func LoadArchitectMeta(cacheDir, planSlug string) (*ArchitectMeta, error) {
	filename := filepath.Join(cacheDir, architectMetaFilename(planSlug))
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("read architect meta: %w", err)
	}

	var meta ArchitectMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("read architect meta: %w", err)
	}

	return &meta, nil
}

// ValidateArchitectDecisionAudit confirms audit belongs to the requested task and
// contains the minimum decision fields needed by hq.
func ValidateArchitectDecisionAudit(a *ArchitectDecisionAudit, planFile, project string) error {
	if a == nil {
		return fmt.Errorf("architect decision audit is nil")
	}
	if a.SchemaVersion != architectDecisionAuditSchemaVersion {
		return fmt.Errorf("unsupported architect decision audit schema version: %d", a.SchemaVersion)
	}
	if a.PlanFile != planFile {
		return fmt.Errorf("architect decision audit plan file mismatch: got %q, want %q", a.PlanFile, planFile)
	}
	if a.Project != project {
		return fmt.Errorf("architect decision audit project mismatch: got %q, want %q", a.Project, project)
	}
	switch strings.TrimSpace(a.BaselineSource) {
	case "parallel_cache", "inline", "absent", "stale":
	case "":
		return fmt.Errorf("architect decision audit baseline source is empty")
	default:
		return fmt.Errorf("unsupported architect decision audit baseline source: %q", a.BaselineSource)
	}
	if strings.TrimSpace(a.FinalDecision) == "" {
		return fmt.Errorf("architect decision audit final decision is empty")
	}
	for i, diff := range a.Differences {
		if strings.TrimSpace(diff.Area) == "" {
			return fmt.Errorf("architect decision audit difference %d area is empty", i)
		}
		if strings.TrimSpace(diff.FinalDecision) == "" {
			return fmt.Errorf("architect decision audit difference %d final decision is empty", i)
		}
	}
	for i, draft := range a.PlannerDrafts {
		if strings.TrimSpace(draft.Profile) == "" {
			return fmt.Errorf("architect decision audit planner draft %d profile is empty", i)
		}
		if strings.TrimSpace(draft.Decision) == "" {
			return fmt.Errorf("architect decision audit planner draft %d decision is empty", i)
		}
	}
	return nil
}

// ArchitectMetaExists reports whether the architect metadata file for planSlug exists in cacheDir.
func ArchitectMetaExists(cacheDir, planSlug string) bool {
	filename := filepath.Join(cacheDir, architectMetaFilename(planSlug))
	_, err := os.Stat(filename)
	return err == nil
}

// PlannerDraftCacheEntry holds the content and metadata of a single planner draft file.
type PlannerDraftCacheEntry struct {
	Profile  string
	Path     string
	Markdown string
	ModTime  time.Time
}

// PlannerDraftCacheFilename returns the filename for a planner draft cache file.
// Returns an error if planSlug or profile are empty or contain path-unsafe characters.
func PlannerDraftCacheFilename(planSlug, profile string) (string, error) {
	if planSlug == "" {
		return "", fmt.Errorf("plan slug must not be empty")
	}
	if profile == "" {
		return "", fmt.Errorf("profile must not be empty")
	}
	if strings.ContainsAny(planSlug, `/\`) || strings.Contains(planSlug, "..") {
		return "", fmt.Errorf("plan slug contains invalid characters")
	}
	if strings.ContainsAny(profile, `/\`) || strings.Contains(profile, "..") {
		return "", fmt.Errorf("profile contains invalid characters")
	}
	return planSlug + "-planner-" + profile + ".md", nil
}

// SavePlannerDraft writes markdown to cacheDir/<planSlug>-planner-<profile>.md,
// creating cacheDir with mode 0755 if it does not exist.
func SavePlannerDraft(cacheDir, planSlug, profile, markdown string) error {
	filename, err := PlannerDraftCacheFilename(planSlug, profile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create planner draft cache dir: %w", err)
	}
	return os.WriteFile(filepath.Join(cacheDir, filename), []byte(markdown), 0o644)
}

// LoadPlannerDraft reads a single planner draft from cacheDir for the given planSlug and profile.
// Returns (nil, nil) when the file does not exist.
func LoadPlannerDraft(cacheDir, planSlug, profile string) (*PlannerDraftCacheEntry, error) {
	filename, err := PlannerDraftCacheFilename(planSlug, profile)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(cacheDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read planner draft %s: %w", filename, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat planner draft %s: %w", filename, err)
	}
	return &PlannerDraftCacheEntry{
		Profile:  profile,
		Path:     path,
		Markdown: string(data),
		ModTime:  info.ModTime(),
	}, nil
}

// ListPlannerDraftCaches returns all planner draft entries for planSlug found in cacheDir.
// Results are sorted by profile name for deterministic ordering.
// Returns nil (not an error) when cacheDir does not exist or contains no matching files.
func ListPlannerDraftCaches(cacheDir, planSlug string) ([]PlannerDraftCacheEntry, error) {
	prefix := planSlug + "-planner-"
	suffix := ".md"

	dirEntries, err := os.ReadDir(cacheDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("list planner draft caches: %w", err)
	}

	var results []PlannerDraftCacheEntry
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		profile := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
		if profile == "" {
			continue
		}
		path := filepath.Join(cacheDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read planner draft %s: %w", name, err)
		}
		info, err := de.Info()
		if err != nil {
			return nil, fmt.Errorf("stat planner draft %s: %w", name, err)
		}
		results = append(results, PlannerDraftCacheEntry{
			Profile:  profile,
			Path:     path,
			Markdown: string(data),
			ModTime:  info.ModTime(),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Profile < results[j].Profile
	})
	return results, nil
}

// ClearPlannerDraftCaches removes all planner draft cache files for planSlug from cacheDir.
// It also silently removes the legacy <planSlug>-architect-baseline.json file if present
// so older runs do not leave stale artifacts behind.
// Missing files and a missing cacheDir are not errors.
func ClearPlannerDraftCaches(cacheDir, planSlug string) error {
	prefix := planSlug + "-planner-"
	suffix := ".md"

	dirEntries, err := os.ReadDir(cacheDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("clear planner draft caches: %w", err)
	}

	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			if err := os.Remove(filepath.Join(cacheDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove planner draft %s: %w", name, err)
			}
		}
	}

	// Silently remove the legacy <planSlug>-architect-baseline.json file if present.
	legacy := filepath.Join(cacheDir, planSlug+"-architect-baseline.json")
	if err := os.Remove(legacy); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove legacy architect baseline: %w", err)
	}
	return nil
}
