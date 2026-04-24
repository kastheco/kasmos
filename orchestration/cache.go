package orchestration

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	architectBaselineSchemaVersion      = 1
	architectDecisionAuditSchemaVersion = 1
)

func architectMetaFilename(planSlug string) string {
	return planSlug + "-architect.json"
}

func architectBaselineFilename(planSlug string) string {
	return planSlug + "-architect-baseline.json"
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
	return nil
}

// ArchitectMetaExists reports whether the architect metadata file for planSlug exists in cacheDir.
func ArchitectMetaExists(cacheDir, planSlug string) bool {
	filename := filepath.Join(cacheDir, architectMetaFilename(planSlug))
	_, err := os.Stat(filename)
	return err == nil
}

// ArchitectBaselineDescriptionHash returns the stable SHA-256 hash for planner prompt description text.
func ArchitectBaselineDescriptionHash(description string) string {
	sum := sha256.Sum256([]byte(description))
	return fmt.Sprintf("%x", sum[:])
}

// NewArchitectBaselineIdentity builds the expected identity for a baseline artifact.
func NewArchitectBaselineIdentity(planFile, project, description string) ArchitectBaselineIdentity {
	return ArchitectBaselineIdentity{
		PlanFile:        planFile,
		Project:         project,
		DescriptionHash: ArchitectBaselineDescriptionHash(description),
	}
}

// SaveArchitectBaseline serializes baseline to JSON and writes it to cacheDir/<planSlug>-architect-baseline.json,
// creating cacheDir with mode 0755 if it does not exist.
func SaveArchitectBaseline(cacheDir, planSlug string, baseline *ArchitectBaseline) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create architect baseline cache dir: %w", err)
	}

	encoded, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal architect baseline: %w", err)
	}

	filename := filepath.Join(cacheDir, architectBaselineFilename(planSlug))
	encoded = append(encoded, '\n')
	if err := os.WriteFile(filename, encoded, 0o644); err != nil {
		return fmt.Errorf("write architect baseline: %w", err)
	}
	return nil
}

// LoadArchitectBaseline reads and deserializes the architect baseline file for planSlug from cacheDir.
// Returns (nil, nil) when the file does not exist.
func LoadArchitectBaseline(cacheDir, planSlug string) (*ArchitectBaseline, error) {
	filename := filepath.Join(cacheDir, architectBaselineFilename(planSlug))
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("read architect baseline: %w", err)
	}

	var baseline ArchitectBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("read architect baseline: %w", err)
	}

	return &baseline, nil
}

// ValidateArchitectBaseline confirms baseline belongs to the expected planner input.
func ValidateArchitectBaseline(b *ArchitectBaseline, expected ArchitectBaselineIdentity) error {
	if b == nil {
		return fmt.Errorf("architect baseline is nil")
	}
	if b.SchemaVersion != architectBaselineSchemaVersion {
		return fmt.Errorf("unsupported architect baseline schema version: %d", b.SchemaVersion)
	}
	if b.BaselineMarkdown == "" {
		return fmt.Errorf("architect baseline markdown is empty")
	}
	if b.PlanFile != expected.PlanFile {
		return fmt.Errorf("architect baseline plan file mismatch: got %q, want %q", b.PlanFile, expected.PlanFile)
	}
	if b.Project != expected.Project {
		return fmt.Errorf("architect baseline project mismatch: got %q, want %q", b.Project, expected.Project)
	}
	if b.DescriptionHash != expected.DescriptionHash {
		return fmt.Errorf("architect baseline description hash mismatch: got %q, want %q", b.DescriptionHash, expected.DescriptionHash)
	}
	return nil
}

// ArchitectBaselineExists reports whether the architect baseline file for planSlug exists in cacheDir.
func ArchitectBaselineExists(cacheDir, planSlug string) bool {
	filename := filepath.Join(cacheDir, architectBaselineFilename(planSlug))
	_, err := os.Stat(filename)
	return err == nil
}

// ClearArchitectBaseline removes only the architect baseline artifact for planSlug.
func ClearArchitectBaseline(cacheDir, planSlug string) error {
	filename := filepath.Join(cacheDir, architectBaselineFilename(planSlug))
	if err := os.Remove(filename); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("clear architect baseline: %w", err)
	}
	return nil
}
