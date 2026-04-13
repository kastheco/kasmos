package check

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// MCPProcess describes a running kas mcp subprocess found by the process inspector.
type MCPProcess struct {
	PID        int
	AgeSeconds int
	RSSKB      int
	Command    string
}

// psOutputFn is the seam for obtaining raw ps output. Replaced in tests.
var psOutputFn = func() (string, error) {
	out, err := exec.Command("ps", "-eo", "pid=,etimes=,rss=,args=").Output()
	if err != nil {
		return "", fmt.Errorf("ps: %w", err)
	}
	return string(out), nil
}

// SetPSOutputFnForTest replaces the ps output seam with fn and returns the
// previous value so callers can restore it in t.Cleanup. For use in tests only.
func SetPSOutputFnForTest(fn func() (string, error)) func() (string, error) {
	prev := psOutputFn
	psOutputFn = fn
	return prev
}

// ListLongLivedMCPProcesses returns all running kas mcp subprocesses whose age
// is at least minAgeSeconds. It never auto-kills any process.
func ListLongLivedMCPProcesses(minAgeSeconds int) ([]MCPProcess, error) {
	raw, err := psOutputFn()
	if err != nil {
		return nil, err
	}
	return parseMCPProcesses(raw, minAgeSeconds), nil
}

// parseMCPProcesses parses the output of `ps -eo pid=,etimes=,rss=,args=` and
// returns processes that look like `kas mcp` (or `/abs/path/to/kas mcp`) and
// are at least minAgeSeconds old.
func parseMCPProcesses(raw string, minAgeSeconds int) []MCPProcess {
	var procs []MCPProcess
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		age, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		rss, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		command := strings.Join(fields[3:], " ")

		if !isKasMCPCommand(command) {
			continue
		}
		if age < minAgeSeconds {
			continue
		}
		procs = append(procs, MCPProcess{
			PID:        pid,
			AgeSeconds: age,
			RSSKB:      rss,
			Command:    command,
		})
	}
	return procs
}

// isKasMCPCommand returns true when command matches `kas mcp ...` or
// `/abs/path/to/kas mcp ...` where kas/path-to-kas is the first token.
// It explicitly excludes `kas serve` and any process where "kas mcp" only
// appears as embedded text in arguments.
func isKasMCPCommand(command string) bool {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		return false
	}
	bin := parts[0]
	if bin != "kas" && !strings.HasSuffix(bin, "/kas") {
		return false
	}
	return parts[1] == "mcp"
}
