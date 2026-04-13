package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/kastheco/kasmos/internal/binpath"
	"github.com/kastheco/kasmos/internal/check"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/internal/mcpclient"
	"github.com/kastheco/kasmos/internal/platform"
	"github.com/spf13/cobra"
)

// probeSharedMCPFunc is the seam for probing the shared HTTP MCP endpoint. Tests
// replace it to avoid real network calls.
var probeSharedMCPFunc = func(ctx context.Context) error {
	return mcpclient.ProbeHTTP(ctx, mcpclient.SharedEndpointURL)
}

// SetProbeSharedMCPForTest swaps the probe seam and returns the previous value
// so callers can restore it in t.Cleanup. For tests only.
func SetProbeSharedMCPForTest(fn func(context.Context) error) func(context.Context) error {
	prev := probeSharedMCPFunc
	probeSharedMCPFunc = fn
	return prev
}

// errUnhealthy is returned when health < 100% to signal exit code 1 without printing a message.
var errUnhealthy = errors.New("unhealthy")

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Audit skills sync health across all harnesses",
		Long: `Audits skill layers and reports completeness per harness:

  1. Global skills  (~/.agents/skills/ → harness global dirs)
  2. Project skills (.agents/skills/ → harness project dirs)

Exit code 0 if 100% healthy, exit code 1 otherwise.`,
		RunE: runCheck,
		// Suppress usage on error — health failures are not usage errors.
		SilenceUsage: true,
		// Suppress cobra's "Error: ..." line for the unhealthy sentinel.
		SilenceErrors: true,
	}
	cmd.Flags().BoolP("verbose", "v", false, "show per-skill detail for each harness")
	return cmd
}

func runCheck(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working dir: %w", err)
	}

	registry := harness.NewRegistry()
	result := check.Audit(home, cwd, registry)

	renderGlobal(cmd, result.Global, verbose)
	if result.InProject {
		renderProject(cmd, result.Project, verbose)
	}
	if result.BinaryPath != nil {
		renderBinaryPath(cmd, result.BinaryPath)
	}

	// Probe the shared HTTP MCP endpoint. A reachable endpoint is required for
	// managed harness launches; a failure here is always unhealthy.
	probeCtx, probeCancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer probeCancel()
	mcpProbeErr := probeSharedMCPFunc(probeCtx)
	renderMCPEndpoint(cmd, mcpProbeErr)

	// Detect long-lived stdio mcp subprocesses (threshold: 60 s).
	mcpProcs, _ := check.ListLongLivedMCPProcesses(60)
	if len(mcpProcs) > 0 {
		renderMCPProcesses(cmd, mcpProcs)
	}

	ok, total := result.Summary()
	// Shared MCP endpoint reachability counts as one item.
	total++
	if mcpProbeErr == nil {
		ok++
	}
	// Long-lived stdio mcp processes each count as one unhealthy item.
	total += len(mcpProcs)

	pct := 0
	if total > 0 {
		pct = ok * 100 / total
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nHealth: %d/%d OK (%d%%)\n", ok, total, pct)

	// Print deduplicated remediation hints.
	hints := collectRemediationHints(result, mcpProcs, mcpProbeErr)
	if len(hints) > 0 {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Remediation:")
		for _, h := range hints {
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", h)
		}
	}

	if pct < 100 {
		return errUnhealthy
	}
	return nil
}

// collectRemediationHints scans the audit result and MCP process list and returns
// deduplicated hint strings.
func collectRemediationHints(result *check.AuditResult, mcpProcs []check.MCPProcess, mcpProbeErr error) []string {
	seen := map[string]bool{}
	var hints []string

	add := func(h string) {
		if !seen[h] {
			seen[h] = true
			hints = append(hints, h)
		}
	}

	for _, h := range result.Global {
		for _, s := range h.Skills {
			if s.Status == check.StatusMissing {
				add("run `kas skills sync` to create missing skill links")
			}
			if s.Status == check.StatusCopy {
				add("remove copy-only skill dirs and re-run `kas skills sync` to replace with symlinks")
			}
		}
	}

	for _, p := range result.Project {
		if !p.InCanonical {
			continue
		}
		if !p.HasSkillMD {
			add("add SKILL.md to skills missing documentation (e.g. .agents/skills/<name>/SKILL.md)")
		}
		for _, st := range p.HarnessStatus {
			if st == check.StatusMissing {
				add("run `kas skills sync` to create missing skill links")
			}
			if st == check.StatusCopy {
				add("remove copy-only skill dirs and re-run `kas skills sync` to replace with symlinks")
			}
		}
	}

	// Binary path skew hints — shared http entries are healthy and don't trigger this.
	if result.BinaryPath != nil {
		hasMismatch := false
		for _, ref := range result.BinaryPath.References {
			if !ref.NotInstalled && !ref.Healthy {
				hasMismatch = true
				break
			}
		}
		if hasMismatch {
			add("re-run `kas scaffold sync` to update config files with the current binary path, or reinstall service units")
		}
	}

	// Long-lived stdio mcp process hint.
	if len(mcpProcs) > 0 {
		pids := make([]string, len(mcpProcs))
		for i, p := range mcpProcs {
			pids[i] = fmt.Sprintf("%d", p.PID)
		}
		add("kill " + strings.Join(pids, " "))
	}

	// Shared MCP endpoint unreachable hint.
	if mcpProbeErr != nil {
		add("start the shared mcp endpoint: " + platform.DaemonStartCommand())
	}

	return hints
}

// renderMCPEndpoint prints a dedicated section reporting shared HTTP MCP
// endpoint reachability.
func renderMCPEndpoint(cmd *cobra.Command, probeErr error) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nshared mcp endpoint:\n")
	if probeErr == nil {
		fmt.Fprintf(out, "  ✓ %s reachable\n", mcpclient.SharedEndpointURL)
		return
	}
	fmt.Fprintf(out, "  ✗ %s unreachable (%s)\n", mcpclient.SharedEndpointURL, probeErr)
}

// renderBinaryPath prints a dedicated binary-path section before the health summary.
func renderBinaryPath(cmd *cobra.Command, bp *check.BinaryPathResult) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nBinary path:\n")

	if bp.RunningErr != "" {
		fmt.Fprintf(out, "  running: (unresolved: %s)\n", bp.RunningErr)
	} else {
		fmt.Fprintf(out, "  running: %s\n", bp.RunningExecutable)
		if bp.RunningCanonical != bp.RunningExecutable && bp.RunningCanonical != "" {
			fmt.Fprintf(out, "  canonical: %s\n", bp.RunningCanonical)
		}
	}

	if len(bp.References) > 0 {
		fmt.Fprintf(out, "  configured:\n")
		for _, ref := range bp.References {
			glyph := "✓"
			annotation := ""
			if ref.NotInstalled {
				glyph = "–"
				annotation = " (not installed)"
			} else if ref.Transport == binpath.TransportSharedHTTP {
				// Shared HTTP is always healthy — show the transport label.
				annotation = " (shared http)"
			} else if !ref.Healthy {
				glyph = "✗"
				if ref.Note != "" {
					annotation = " (" + ref.Note + ")"
				} else if ref.Normalized != "" {
					annotation = " (mismatch)"
				} else {
					annotation = " (stale)"
				}
			}
			fmt.Fprintf(out, "    %s %-30s %s%s\n",
				glyph,
				ref.File+":"+ref.Label,
				ref.RawPath,
				annotation,
			)
		}
	}
}

// renderMCPProcesses prints a warning block listing long-lived stdio kas mcp subprocesses.
func renderMCPProcesses(cmd *cobra.Command, procs []check.MCPProcess) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nstdio mcp processes:\n")
	for _, p := range procs {
		fmt.Fprintf(out, "  ✗ pid %-6d age %-6ds rss %-8d KB  %s\n",
			p.PID, p.AgeSeconds, p.RSSKB, p.Command)
	}
}

func renderGlobal(cmd *cobra.Command, results []check.HarnessResult, verbose bool) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nGlobal skills (~/.agents/skills):\n")

	// Collect orphans for summary display
	type orphanEntry struct {
		harness string
		name    string
		detail  string
	}
	var orphans []orphanEntry

	for _, h := range results {
		counts := map[check.SkillStatus]int{}
		for _, s := range h.Skills {
			counts[s.Status]++
		}

		fmt.Fprintf(out, "  %-12s %d synced  %d skipped  %d missing  %d orphan",
			h.Name,
			counts[check.StatusSynced],
			counts[check.StatusSkipped],
			counts[check.StatusMissing],
			counts[check.StatusOrphan],
		)
		if counts[check.StatusBroken] > 0 {
			fmt.Fprintf(out, "  %d broken", counts[check.StatusBroken])
		}
		if counts[check.StatusCopy] > 0 {
			fmt.Fprintf(out, "  %d copy", counts[check.StatusCopy])
		}
		fmt.Fprintln(out)

		if verbose {
			// Sort skills by name for stable output
			skills := make([]check.SkillEntry, len(h.Skills))
			copy(skills, h.Skills)
			sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })

			for _, s := range skills {
				glyph := statusGlyph(s.Status)
				detail := ""
				if s.Detail != "" && verbose {
					detail = " (" + s.Detail + ")"
				}
				fmt.Fprintf(out, "    %s %s%s\n", glyph, s.Name, detail)
			}
		}

		for _, s := range h.Skills {
			if s.Status == check.StatusOrphan {
				orphans = append(orphans, orphanEntry{harness: h.Name, name: s.Name, detail: s.Detail})
			}
		}
	}

	if len(orphans) > 0 {
		fmt.Fprintf(out, "\n  Orphans:\n")
		for _, o := range orphans {
			target := o.detail
			if target == "" {
				target = "(deleted)"
			}
			fmt.Fprintf(out, "    [%s] %s → %s\n", o.harness, o.name, target)
		}
	}
}

func renderProject(cmd *cobra.Command, entries []check.ProjectSkillEntry, verbose bool) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nProject skills (.agents/skills):\n")

	for _, e := range entries {
		if !e.InCanonical {
			fmt.Fprintf(out, "  ✗ %-22s MISSING from .agents/skills/\n", e.Name)
			continue
		}

		// Collect harness statuses in stable order
		harnessNames := sortedKeys(e.HarnessStatus)
		parts := make([]string, 0, len(harnessNames))
		allOK := true
		for _, h := range harnessNames {
			st := e.HarnessStatus[h]
			glyph := statusGlyph(st)
			parts = append(parts, fmt.Sprintf("%s %s", h, glyph))
			// StatusSynced and StatusCopy both count as healthy for harness checks
			if st != check.StatusSynced && st != check.StatusCopy {
				allOK = false
			}
		}

		// A skill is still overall unhealthy if SKILL.md is missing
		overallGlyph := "✓"
		if !allOK || !e.HasSkillMD {
			overallGlyph = "✗"
		}

		annotation := ""
		if e.InCanonical && !e.HasSkillMD {
			annotation = "  no SKILL.md"
		}

		fmt.Fprintf(out, "  %s %-22s %s%s\n", overallGlyph, e.Name, strings.Join(parts, "  "), annotation)
	}
}

func statusGlyph(s check.SkillStatus) string {
	switch s {
	case check.StatusSynced:
		return "✓"
	case check.StatusSkipped:
		return "⊘"
	case check.StatusMissing:
		return "✗"
	case check.StatusOrphan:
		return "✗"
	case check.StatusBroken:
		return "✗"
	case check.StatusCopy:
		return "≈"
	default:
		return "?"
	}
}

func sortedKeys(m map[string]check.SkillStatus) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func init() {
	rootCmd.AddCommand(newCheckCmd())
}
