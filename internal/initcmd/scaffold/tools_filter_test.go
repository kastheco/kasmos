package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// miniDoc is a minimal tools-reference for testing with 2 categories, 3 tools, and a table.
const miniDoc = "## Available CLI Tools\n\nThese tools are available in this environment. Prefer them over lower-level alternatives when they apply.\n\n### Code Search & Refactoring\n\n- **ast-grep** (" + "`" + "sg" + "`" + "): Structural code search and replace using AST patterns.\n  - Find all calls: " + "`" + "sg --pattern 'fmt.Errorf($$$)' --lang go" + "`" + "\n  - Structural replace: " + "`" + "sg --pattern 'errors.New($MSG)' --rewrite 'fmt.Errorf($MSG)' --lang go" + "`" + "\n- **comby** (" + "`" + "comby" + "`" + "): Language-aware structural search/replace with hole syntax.\n  - " + "`" + "comby 'if err != nil { return :[rest] }' 'if err != nil { return fmt.Errorf(\":[context]: %w\", err) }' .go" + "`" + "\n\n### Diff & Change Analysis\n\n- **difftastic** (" + "`" + "difft" + "`" + "): Structural diff that understands syntax.\n  - Compare files: " + "`" + "difft old.go new.go" + "`" + "\n\n### When to Use What\n\n| Task | Preferred Tool | Fallback |\n|------|---------------|----------|\n| Rename symbol across files | " + "`" + "sg" + "`" + " (ast-grep) | " + "`" + "sd" + "`" + " for simple strings |\n| Structural code rewrite | " + "`" + "sg" + "`" + " or " + "`" + "comby" + "`" + " | manual edit |\n| Review code changes | " + "`" + "difft" + "`" + " | " + "`" + "git diff" + "`" + " |\n"

func TestFilterToolsReference(t *testing.T) {
	t.Run("all selected returns full document", func(t *testing.T) {
		result := FilterToolsReference(miniDoc, []string{"sg", "comby", "difft"})
		assert.Contains(t, result, "ast-grep")
		assert.Contains(t, result, "comby")
		assert.Contains(t, result, "difftastic")
		assert.Contains(t, result, "### Code Search & Refactoring")
		assert.Contains(t, result, "### Diff & Change Analysis")
		assert.Contains(t, result, "### When to Use What")
	})

	t.Run("none selected strips all tools and headers", func(t *testing.T) {
		result := FilterToolsReference(miniDoc, []string{})
		assert.Contains(t, result, "## Available CLI Tools")
		assert.NotContains(t, result, "### Code Search")
		assert.NotContains(t, result, "### Diff")
		assert.NotContains(t, result, "ast-grep")
		assert.NotContains(t, result, "comby")
		assert.NotContains(t, result, "difft")
		assert.NotContains(t, result, "### When to Use What")
	})

	t.Run("partial category keeps header and selected tool", func(t *testing.T) {
		result := FilterToolsReference(miniDoc, []string{"sg"})
		assert.Contains(t, result, "### Code Search & Refactoring")
		assert.Contains(t, result, "ast-grep")
		assert.NotContains(t, result, "comby")
	})

	t.Run("empty category header stripped", func(t *testing.T) {
		result := FilterToolsReference(miniDoc, []string{"sg"})
		assert.NotContains(t, result, "### Diff & Change Analysis")
		assert.NotContains(t, result, "difftastic")
	})

	t.Run("table rows filtered by binary presence", func(t *testing.T) {
		result := FilterToolsReference(miniDoc, []string{"difft"})
		assert.Contains(t, result, "Review code changes")
		assert.NotContains(t, result, "Rename symbol")
		assert.NotContains(t, result, "Structural code rewrite")
	})

	t.Run("table suppressed when all data rows filtered", func(t *testing.T) {
		result := FilterToolsReference(miniDoc, []string{"comby"})
		assert.NotContains(t, result, "### When to Use What")
		assert.NotContains(t, result, "| Task |")
	})

	t.Run("multi-line sub-bullets removed atomically", func(t *testing.T) {
		result := FilterToolsReference(miniDoc, []string{"comby", "difft"})
		assert.NotContains(t, result, "sg --pattern")
		assert.NotContains(t, result, "ast-grep")
		assert.Contains(t, result, "comby")
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		result := FilterToolsReference("", []string{"sg"})
		assert.Empty(t, result)
	})

	t.Run("nil selected treated as empty", func(t *testing.T) {
		result := FilterToolsReference(miniDoc, nil)
		assert.NotContains(t, result, "ast-grep")
	})
}

func TestFilterToolsReferenceWithRealTemplate(t *testing.T) {
	content, err := templates.ReadFile("templates/shared/tools-reference.md")
	if err != nil {
		t.Skip("embedded templates not available in test context")
	}
	src := string(content)

	t.Run("all 14 tools selected preserves full content", func(t *testing.T) {
		all := []string{"sg", "comby", "difft", "sd", "yq", "mlr", "glow", "typos", "scc", "tokei", "watchexec", "hyperfine", "procs", "mprocs"}
		result := FilterToolsReference(src, all)
		assert.Contains(t, result, "### Code Search & Refactoring")
		assert.Contains(t, result, "### Diff & Change Analysis")
		assert.Contains(t, result, "### Text Processing")
		assert.Contains(t, result, "### Code Quality")
		assert.Contains(t, result, "### When to Use What")
		assert.False(t, strings.HasSuffix(result, "\n\n\n"))
	})
}
