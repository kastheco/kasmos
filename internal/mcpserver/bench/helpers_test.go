package bench

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// ─── package-level resolved paths ────────────────────────────────────────────

var (
	// kasBinaryPath is where the compiled kas binary lives during bench tests.
	kasBinaryPath string
	// fixtureRoot is the directory that contains the synthetic fixture tree.
	fixtureRoot string
	// benchToolsMissing is set when rg/fd aren't available (e.g. CI).
	// Tests that need them call skipIfNoBenchTools.
	benchToolsMissing bool
)

// skipIfNoBenchTools skips a test that requires rg/fd when they aren't installed.
func skipIfNoBenchTools(tb testing.TB) {
	tb.Helper()
	if benchToolsMissing {
		tb.Skip("skipping: rg/fd not found in PATH")
	}
}

// ─── scenario model ──────────────────────────────────────────────────────────

// scenario describes a single benchmark workload used by all three arms
// (mcp, direct, bash).  Every field is optional; callers use only the fields
// relevant to their tool.
type scenario struct {
	key      string // stable identifier – report.go joins arms by this key
	tool     string // "read_file" | "grep" | "find_files"
	file     string // relative path inside fixtureRoot (read_file arm)
	path     string // directory to scope grep/find (relative to fixtureRoot)
	pattern  string // regex/glob pattern for grep or find_files
	glob     string // file-type filter for grep (e.g. "*.go")
	from     int    // start line (read_file)
	lines    int    // max lines to read (read_file)
	maxDepth int    // fd --max-depth for find_files
}

// ─── scenario catalog ────────────────────────────────────────────────────────
//
// Keys must remain stable because report.go joins arms by exact key string.

var scenarios = []scenario{
	// read_file ──────────────────────────────────────────────────────────────
	{key: "read_small", tool: "read_file", file: "small.go", from: 1, lines: 50},
	{key: "read_medium", tool: "read_file", file: "medium.go", from: 1, lines: 200},
	{key: "read_large", tool: "read_file", file: "large.go", from: 1, lines: 500},

	// grep ───────────────────────────────────────────────────────────────────
	{key: "grep_narrow", tool: "grep", path: ".", pattern: "BENCH_MARKER_UNIQUE"},
	{key: "grep_broad", tool: "grep", path: ".", pattern: "BENCH_BROAD_HIT"},
	{key: "grep_filtered", tool: "grep", path: ".", pattern: `fmt\.Errorf|errors\.New`, glob: "*.go"},

	// find_files ─────────────────────────────────────────────────────────────
	{key: "find_extension", tool: "find_files", path: ".", pattern: "*.go", maxDepth: 3},
	{key: "find_pattern", tool: "find_files", path: ".", pattern: "sub_*.go", maxDepth: 3},
}

// ─── TestMain ─────────────────────────────────────────────────────────────────

func TestMain(m *testing.M) {
	packageDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bench: os.Getwd: %v\n", err)
		os.Exit(1)
	}

	repoRoot := filepath.Clean(filepath.Join(packageDir, "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		fmt.Fprintf(os.Stderr, "bench: cannot locate go.mod at %s: %v\n", repoRoot, err)
		os.Exit(1)
	}

	// Build the real kas binary once.
	kasBinaryPath = filepath.Join(packageDir, "testdata", "kas_bench")
	if err := os.MkdirAll(filepath.Dir(kasBinaryPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "bench: mkdir testdata: %v\n", err)
		os.Exit(1)
	}
	buildCmd := exec.Command("go", "build", "-o", kasBinaryPath, ".")
	buildCmd.Dir = repoRoot
	buildCmd.Stdout = os.Stderr
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "bench: go build kas: %v\n", err)
		os.Exit(1)
	}

	// Check for required CLI tools. Bench/smoke tests that need rg/fd call
	// skipIfNoBenchTools(t) individually; report_test.go runs regardless.
	for _, tool := range []string{"rg", "fd"} {
		if _, err := exec.LookPath(tool); err != nil {
			fmt.Fprintf(os.Stderr, "bench: tool %q not found in PATH — bench/smoke tests will be skipped\n", tool)
			benchToolsMissing = true
			break
		}
	}

	// Create deterministic fixture tree inside the package directory so the
	// kas mcp subprocess can access files through its sandbox rules.
	fixtureRoot = filepath.Join(packageDir, "testdata", "fixtures")
	if err := createFixtures(fixtureRoot); err != nil {
		fmt.Fprintf(os.Stderr, "bench: createFixtures: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// Write JSON report only when the env var is set; otherwise print a summary.
	if reportPath := os.Getenv("KAS_MCP_BENCH_REPORT"); reportPath != "" {
		// Report writing is the responsibility of the benchmark functions
		// themselves (Task 2/3).  Here we just note where it should go.
		_ = reportPath
	}

	os.Exit(code)
}

// ─── fixture generation ───────────────────────────────────────────────────────

// createFixtures builds a deterministic directory tree under root suitable for
// all benchmark scenarios.  Re-running always produces the same file contents
// and line counts so grep hit-counts are stable across invocations.
func createFixtures(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	// small.go – 60 lines
	if err := writeFixture(root, "small.go", generateGoFile("small", 60, false)); err != nil {
		return err
	}
	// medium.go – 250 lines
	if err := writeFixture(root, "medium.go", generateGoFile("medium", 250, false)); err != nil {
		return err
	}
	// large.go – 600 lines
	if err := writeFixture(root, "large.go", generateGoFile("large", 600, false)); err != nil {
		return err
	}

	// Nested sub-package: sub/sub_alpha.go and sub/sub_beta.go
	subDir := filepath.Join(root, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		return err
	}
	if err := writeFixture(subDir, "sub_alpha.go", generateGoFile("sub_alpha", 80, true)); err != nil {
		return err
	}
	if err := writeFixture(subDir, "sub_beta.go", generateGoFile("sub_beta", 80, false)); err != nil {
		return err
	}

	return nil
}

// generateGoFile returns a deterministic Go file body for the given name,
// line count, and whether to include a unique marker.
//
// Every file contains:
//   - one BENCH_MARKER_UNIQUE occurrence only when includeUniqueMarker is true
//   - many BENCH_BROAD_HIT strings (one per line after line 10)
//   - several fmt.Errorf and errors.New calls (for grep_filtered)
func generateGoFile(name string, lineCount int, includeUniqueMarker bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, "// Code generated for benchmark fixtures — name=%s\n", name)
	if includeUniqueMarker {
		fmt.Fprintf(&b, "// BENCH_MARKER_UNIQUE\n")
	}
	fmt.Fprintf(&b, "package fixtures\n\n")
	fmt.Fprintf(&b, "import (\n\t\"errors\"\n\t\"fmt\"\n)\n\n")

	// Add a few error-construction lines (grep_filtered target).
	fmt.Fprintf(&b, "var errBench = fmt.Errorf(\"bench error in %s\")\n", name)
	fmt.Fprintf(&b, "var errBench2 = errors.New(\"bench static error\")\n\n")

	fmt.Fprintf(&b, "func bench_%s(n int) string {\n", name)
	fmt.Fprintf(&b, "\tif n < 0 {\n")
	fmt.Fprintf(&b, "\t\t_ = fmt.Errorf(\"negative: %%d\", n)\n")
	fmt.Fprintf(&b, "\t\treturn errors.New(\"negative\").Error()\n")
	fmt.Fprintf(&b, "\t}\n")

	// Fill remaining lines with BENCH_BROAD_HIT content.
	currentLines := strings.Count(b.String(), "\n")
	for i := currentLines; i < lineCount-3; i++ {
		fmt.Fprintf(&b, "\t// BENCH_BROAD_HIT line %04d\n", i)
	}

	fmt.Fprintf(&b, "\treturn fmt.Sprintf(\"result-%%d\", n)\n")
	fmt.Fprintf(&b, "}\n")
	fmt.Fprintf(&b, "\nvar _ = errBench\nvar _ = errBench2\n")

	return b.String()
}

func writeFixture(dir, name, content string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

// ─── MCP stdio client helper ──────────────────────────────────────────────────

// newMCPStdioClient creates a fully initialized MCP client connected to a kas
// mcp subprocess via stdio transport.  The subprocess working directory is set
// to fixtureRoot so the sandbox allows access to the fixture files.
//
// Set nocache=true to pass KAS_MCP_NOCACHE=1 to the subprocess, bypassing the
// application-level ristretto cache so raw latency is measured.
//
// The client is registered for cleanup via tb.Cleanup.
func newMCPStdioClient(tb testing.TB, nocache bool) *client.Client {
	tb.Helper()

	env := []string{}
	if nocache {
		env = append(env, "KAS_MCP_NOCACHE=1")
	}

	stdioTransport := transport.NewStdioWithOptions(
		kasBinaryPath,
		env,
		[]string{"mcp"},
		transport.WithCommandFunc(func(ctx context.Context, command string, cmdEnv []string, args []string) (*exec.Cmd, error) {
			cmd := exec.CommandContext(ctx, command, args...)
			cmd.Env = append(os.Environ(), cmdEnv...)
			cmd.Dir = fixtureRoot
			return cmd, nil
		}),
	)

	c := client.NewClient(stdioTransport)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	tb.Cleanup(cancel)

	if err := c.Start(ctx); err != nil {
		tb.Fatalf("newMCPStdioClient: Start: %v", err)
	}

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "bench-client",
				Version: "0.0.1",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		_ = c.Close()
		tb.Fatalf("newMCPStdioClient: Initialize: %v", err)
	}

	tb.Cleanup(func() { _ = c.Close() })
	return c
}
