package wizard

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
