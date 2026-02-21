package wizard

import (
	"fmt"
	"os/exec"

	"github.com/charmbracelet/huh"
)

// toolDef defines a CLI tool the wizard can detect.
type toolDef struct {
	Binary string // executable name to look up in PATH
	Name   string // human-friendly display name
}

// toolCatalog is the static list of tools from tools-reference.md.
var toolCatalog = []toolDef{
	{"sg", "ast-grep"},
	{"comby", "comby"},
	{"difft", "difftastic"},
	{"sd", "sd"},
	{"yq", "yq"},
	{"mlr", "miller"},
	{"glow", "glow"},
	{"typos", "typos"},
	{"scc", "scc"},
	{"tokei", "tokei"},
	{"watchexec", "watchexec"},
	{"hyperfine", "hyperfine"},
	{"procs", "procs"},
	{"mprocs", "mprocs"},
}

// toolDetectResult holds the result of looking up a single tool.
type toolDetectResult struct {
	Binary string
	Name   string
	Path   string
	Found  bool
}

// lookupFunc abstracts exec.LookPath for testing.
type lookupFunc func(binary string) (string, error)

// detectTools probes PATH for each tool in the catalog.
func detectTools(catalog []toolDef, lookup lookupFunc) []toolDetectResult {
	results := make([]toolDetectResult, 0, len(catalog))
	for _, t := range catalog {
		path, err := lookup(t.Binary)
		results = append(results, toolDetectResult{
			Binary: t.Binary,
			Name:   t.Name,
			Path:   path,
			Found:  err == nil,
		})
	}
	return results
}

// runToolsStage detects CLI tools on PATH and lets the user confirm the selection.
func runToolsStage(state *State) error {
	results := detectTools(toolCatalog, exec.LookPath)

	var options []huh.Option[string]
	var preSelected []string

	for _, r := range results {
		label := fmt.Sprintf("%s  (%s)", r.Name, r.Binary)
		if r.Found {
			label = fmt.Sprintf("%s  (%s)  detected: %s", r.Name, r.Binary, r.Path)
			preSelected = append(preSelected, r.Binary)
		} else {
			label = fmt.Sprintf("%s  (%s)  not found", r.Name, r.Binary)
		}
		options = append(options, huh.NewOption(label, r.Binary))
	}

	state.SelectedTools = preSelected

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which CLI tools should agents reference?").
				Description("Pre-selected tools were detected on your PATH. You can add tools you plan to install.").
				Options(options...).
				Value(&state.SelectedTools),
		),
	)

	if err := form.Run(); err != nil {
		return fmt.Errorf("tool discovery: %w", err)
	}

	return nil
}
