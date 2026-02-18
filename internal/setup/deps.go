package setup

import "os/exec"

type DepResult struct {
	Name        string
	Found       bool
	Path        string
	Required    bool
	InstallHint string
}

func CheckDependencies() []DepResult {
	checks := []struct {
		name     string
		required bool
		hint     string
	}{
		{"opencode", true, "go install github.com/anomalyco/opencode@latest"},
		{"git", true, "install via system package manager"},
	}

	results := make([]DepResult, 0, len(checks))
	for _, c := range checks {
		path, err := exec.LookPath(c.name)
		results = append(results, DepResult{
			Name:        c.name,
			Found:       err == nil,
			Path:        path,
			Required:    c.required,
			InstallHint: c.hint,
		})
	}

	return results
}
