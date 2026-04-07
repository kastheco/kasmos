package tmux_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNoRealTmuxInTests walks all _test.go files in the repository and fails if
// any file outside session/tmux/ invokes real-tmux constructors or exec.Command("tmux").
//
// Banned patterns (outside session/tmux/):
//   - tmux.NewTmuxSession(...)
//   - tmux.MakePtyFactory()
//   - cmd.MakeExecutor()
//   - exec.Command("tmux", ...)
//   - exec.CommandContext(ctx, "tmux", ...)
//
// The check uses go/ast so comments and string literals do not trigger false positives.
// Import aliases are resolved per-file so renamed imports still match.
func TestNoRealTmuxInTests(t *testing.T) {
	t.Helper()

	// Resolve repo root: two levels up from session/tmux (the package under test).
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Join(cwd, "..", "..")

	// Canonical rel-path prefix for the allowed package (forward-slash, no leading /).
	const allowedPrefix = "session/tmux"

	var violations []string

	err = filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden dirs (vendor, .git, etc.) and non-Go files.
		if info.IsDir() {
			base := info.Name()
			if strings.HasPrefix(base, ".") || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Compute relative path from repo root (always forward slashes for matching).
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		relFwd := filepath.ToSlash(rel)

		// Files under session/tmux/ are intentionally allowed to use real constructors.
		if strings.HasPrefix(relFwd, allowedPrefix+"/") || relFwd == allowedPrefix {
			return nil
		}

		vs := checkFile(path, relFwd)
		violations = append(violations, vs...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}

	if len(violations) == 0 {
		return
	}

	sort.Strings(violations)
	t.Errorf("real-tmux usage detected in test files outside session/tmux/:\n\n%s\n\nFix: replace with mocks (cmd_test.NewMockExecutor, tmux test fakes) so tests do not reach the live tmux server.",
		strings.Join(violations, "\n"))
}

// checkFile parses one _test.go file and returns violation strings of the form "rel/path.go:NN: reason".
func checkFile(absPath, relPath string) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		// Unparseable files are not our concern here.
		return nil
	}

	// Build import-alias → canonical-name map for relevant packages.
	// We care about:
	//   "os/exec"                                       → exec
	//   "github.com/kastheco/kasmos/session/tmux"       → tmux
	//   "github.com/kastheco/kasmos/cmd"                → cmd
	type pkgInfo struct {
		alias    string // local name used in code
		canonKey string // which category: "exec", "tmux", "cmd"
	}

	aliasMap := map[string]string{} // local name → category key

	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		var canonKey string
		switch importPath {
		case "os/exec":
			canonKey = "exec"
		case "github.com/kastheco/kasmos/session/tmux":
			canonKey = "tmux"
		case "github.com/kastheco/kasmos/cmd":
			canonKey = "cmd"
		default:
			continue
		}

		var localName string
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" && imp.Name.Name != "." {
			localName = imp.Name.Name
		} else {
			// Default: last path segment.
			parts := strings.Split(importPath, "/")
			localName = parts[len(parts)-1]
		}
		aliasMap[localName] = canonKey
	}

	if len(aliasMap) == 0 {
		return nil // no relevant imports → nothing to check
	}

	var violations []string

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		category, known := aliasMap[pkgIdent.Name]
		if !known {
			return true
		}

		pos := fset.Position(call.Pos())
		loc := relPath + ":" + itoa(pos.Line)
		fn := sel.Sel.Name

		switch category {
		case "tmux":
			switch fn {
			case "NewTmuxSession":
				violations = append(violations, loc+": calls tmux.NewTmuxSession — use a mock/fake instead")
			case "MakePtyFactory":
				violations = append(violations, loc+": calls tmux.MakePtyFactory — use a test PtyFactory stub instead")
			}
		case "cmd":
			if fn == "MakeExecutor" {
				violations = append(violations, loc+": calls cmd.MakeExecutor — use cmd_test.NewMockExecutor instead")
			}
		case "exec":
			if fn == "Command" || fn == "CommandContext" {
				// Only flag if the first string-literal argument is "tmux".
				firstArgIdx := 0
				if fn == "CommandContext" {
					firstArgIdx = 1 // skip ctx
				}
				if len(call.Args) > firstArgIdx {
					if lit, ok := call.Args[firstArgIdx].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						val := strings.Trim(lit.Value, `"`)
						if val == "tmux" {
							violations = append(violations, loc+": calls exec."+fn+`("tmux",...) — inject cmd.Executor instead`)
						}
					}
				}
			}
		}

		return true
	})

	return violations
}

// itoa converts an int to its decimal string without importing strconv at package scope.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
