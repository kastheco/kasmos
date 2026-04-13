package check

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsKasMCPCommand(t *testing.T) {
	cases := []struct {
		command string
		want    bool
		desc    string
	}{
		{"kas mcp", true, "bare kas mcp"},
		{"/home/kas/go/bin/kas mcp", true, "absolute path kas mcp"},
		{"/usr/local/bin/kas mcp --some-flag", true, "kas mcp with flags"},
		{"kas serve", false, "kas serve ignored"},
		{"/home/kas/go/bin/kas serve", false, "absolute kas serve ignored"},
		{"kasmos mcp", false, "different binary"},
		{"kas", false, "kas alone"},
		{"mcp", false, "mcp alone"},
		{"", false, "empty command"},
		{"claude --agent kas mcp", false, "kas mcp embedded in other command args"},
		{"/usr/bin/env kas mcp", false, "env wrapper not matched as kas binary"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.want, isKasMCPCommand(tc.command))
		})
	}
}

func TestParseMCPProcesses_Basic(t *testing.T) {
	raw := `  4242  95  38124 /home/kas/go/bin/kas mcp
  4243  30  12000 kas mcp
  4244  120  50000 kas serve
  4245  200  10000 kas mcp --extra-flag
`
	// minAge = 60: 4242 (age 95) and 4245 (age 200) qualify; 4243 too young; 4244 is serve
	procs := parseMCPProcesses(raw, 60)

	assert.Len(t, procs, 2, "only kas mcp processes with age >= 60")

	assert.Equal(t, 4242, procs[0].PID)
	assert.Equal(t, 95, procs[0].AgeSeconds)
	assert.Equal(t, 38124, procs[0].RSSKB)
	assert.Equal(t, "/home/kas/go/bin/kas mcp", procs[0].Command)

	assert.Equal(t, 4245, procs[1].PID)
	assert.Equal(t, 200, procs[1].AgeSeconds)
	assert.Equal(t, 10000, procs[1].RSSKB)
	assert.Equal(t, "kas mcp --extra-flag", procs[1].Command)
}

func TestParseMCPProcesses_TooYoung(t *testing.T) {
	raw := "  100  30  5000 kas mcp\n"
	procs := parseMCPProcesses(raw, 60)
	assert.Empty(t, procs, "process younger than minAge should be excluded")
}

func TestParseMCPProcesses_ExactAge(t *testing.T) {
	raw := "  101  60  5000 kas mcp\n"
	procs := parseMCPProcesses(raw, 60)
	assert.Len(t, procs, 1, "process at exactly minAge should be included")
}

func TestParseMCPProcesses_EmptyAndMalformed(t *testing.T) {
	raw := "\n\n   \nnotanumber 10 5000 kas mcp\n10 notanumber 5000 kas mcp\n"
	procs := parseMCPProcesses(raw, 0)
	assert.Empty(t, procs, "malformed lines should be silently skipped")
}

func TestParseMCPProcesses_IgnoresKasServe(t *testing.T) {
	raw := "  200  999  50000 kas serve\n  201  999  50000 /opt/kas serve\n"
	procs := parseMCPProcesses(raw, 0)
	assert.Empty(t, procs, "kas serve processes must not be reported")
}

func TestListLongLivedMCPProcesses_Seam(t *testing.T) {
	orig := psOutputFn
	t.Cleanup(func() { psOutputFn = orig })

	psOutputFn = func() (string, error) {
		return "  9999  120  20000 kas mcp\n", nil
	}

	procs, err := ListLongLivedMCPProcesses(60)
	assert.NoError(t, err)
	assert.Len(t, procs, 1)
	assert.Equal(t, 9999, procs[0].PID)
}
