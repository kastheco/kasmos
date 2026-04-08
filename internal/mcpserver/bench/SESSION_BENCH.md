# Session Benchmark Guide

This guide documents the **manual** procedure for recording MCP vs. built-in vs.
Bash tool latency inside a live Claude Code session, and how to compare those
observations to the automated Go benchmark report.

---

## Why manual observation matters

The Go benchmark suite (`internal/mcpserver/bench/`) drives all three arms
programmatically and produces a JSON report, but it cannot observe Claude Code's
own built-in tool dispatch overhead — that code runs inside the harness, not
inside this repo.  The session procedure here lets you record those timings by
hand (or via your harness's debug output) and compare them to the automated
baselines.

---

## Step 1 — run the shell-side arm

From the repo root:

```sh
bash internal/mcpserver/bench/session_bench.sh
```

The script will:

1. Print the **canonical probe file paths** used for all arms.
2. Time `cat -n`, `rg`, and `fd` against those files and report min/mean/max.
3. Optionally run the full Go benchmark suite and emit a JSON report to `/tmp/`.

Copy the probe file paths printed by the script — you will paste them into the
Claude Code session prompt below.

---

## Step 2 — record timings in a live Claude Code session

Open a Claude Code session in this repo and paste the following prompt, filling
in the `<SMALL>`, `<MEDIUM>`, and `<LARGE>` placeholders with the paths printed
by the script:

```
I am benchmarking kasmos MCP tool latency vs. built-in and Bash alternatives.
Please perform the following operations and note the wall-clock time each one
takes (use your harness's tool-timing or debug output if available; otherwise
estimate from response timestamps):

Probe files:
  SMALL  = <SMALL>
  MEDIUM = <MEDIUM>
  LARGE  = <LARGE>

For each file, please run these three tool variants in order and record how
long each call takes:

1. MCP arm — use the kasmos MCP tools:
   - mcp__kasmos__read_file on each file
   - mcp__kasmos__grep with pattern "func " on each file
   - mcp__kasmos__find_files with pattern "*.go" in the file's parent directory

2. Built-in arm — use the Claude Code built-in tools directly:
   - Read on each file
   - Grep with pattern "func " on each file
   - Glob with pattern "*.go" in the file's parent directory

3. Bash arm — use the Bash tool to run shell commands:
   - cat -n <file>
   - rg 'func ' <file>
   - fd -e go . <parent-dir>

After all operations, summarise the observations in a table:

| operation     | file   | mcp   | built-in | bash  |
|---------------|--------|-------|----------|-------|
| read_file     | small  | ?ms   | ?ms      | ?ms   |
| read_file     | medium | ?ms   | ?ms      | ?ms   |
| read_file     | large  | ?ms   | ?ms      | ?ms   |
| grep func     | small  | ?ms   | ?ms      | ?ms   |
| grep func     | medium | ?ms   | ?ms      | ?ms   |
| grep func     | large  | ?ms   | ?ms      | ?ms   |
| find *.go     | small  | ?ms   | ?ms      | ?ms   |
| find *.go     | medium | ?ms   | ?ms      | ?ms   |
| find *.go     | large  | ?ms   | ?ms      | ?ms   |

Note any warm-vs-cold differences if the harness re-uses tool connections
across calls.
```

### Notes on built-in tool timings

- Built-in tool dispatch is internal to the Claude Code harness. Exact overhead
  depends on your harness version and machine.
- If your harness exposes tool timing (e.g. via debug output or a timing log),
  use that. Otherwise use the difference between consecutive response timestamps
  as an approximation.
- **Do not hard-code harness environment variable names** from this repo into
  the prompt — they are not defined here. Instead ask Claude to use "your
  harness's tool-timing or debug output if available."

---

## Step 3 — compare to the JSON report

If the script produced a JSON report (default path printed to stdout), open it:

```sh
cat /tmp/mcp_bench_report_<hash>.json | jq .
```

Key fields to compare:

| JSON field | meaning |
|------------|---------|
| `.operations[].key` | scenario identifier, e.g. `read_file/small/cold` |
| `.arms[] where .arm=="mcp" | .latency.p50_ns` | MCP p50 latency (ns) |
| `.arms[] where .arm=="direct" | .latency.p50_ns` | direct function-call p50 |
| `.arms[] where .arm=="bash" | .latency.p50_ns` | shell-subprocess p50 |
| `.overhead_mcp_vs_direct` | multiplier: MCP p50 / direct p50 |
| `.overhead_mcp_vs_bash` | multiplier: MCP p50 / bash p50 |

The "direct" arm simulates the cost of a built-in tool (in-process Go call with
no subprocess). Compare it to the built-in timings you recorded in Step 2 to
validate that the synthetic baseline is in the right ballpark.

---

## Step 4 — re-run with cache disabled (optional)

To isolate the ristretto cache benefit, re-run the Go suite with:

```sh
KAS_MCP_NOCACHE=1 KAS_MCP_BENCH_REPORT=/tmp/report_nocache.json \
  go test ./internal/mcpserver/bench/... -run '^$' -bench . -benchtime=1x
```

Then diff the two JSON reports:

```sh
jq -r '.operations[] | "\(.key)  p50=\(.arms[] | select(.arm=="mcp") | .latency.p50_ns)"' \
  /tmp/mcp_bench_report_*.json
```

A large p50 increase with `KAS_MCP_NOCACHE=1` confirms that caching is
contributing meaningfully to warm-path latency.

---

## Interpreting results

| overhead_mcp_vs_direct | interpretation |
|------------------------|----------------|
| < 2×                   | MCP overhead is negligible; policy is clearly safe |
| 2–5×                   | Acceptable for infrequent calls; monitor hot loops |
| > 5×                   | Investigate: possible subprocess cold-start or IPC bottleneck |

The MCP-first policy's benefits (caching, consistency, code-awareness) are not
captured in raw latency. Use the overhead multiplier as a worst-case cost signal,
not as a rejection criterion on its own.
