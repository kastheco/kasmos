#!/usr/bin/env bash
# session_bench.sh — live-session validation kit for kasmos MCP latency
#
# Automates the repo-local (shell-side) timing arm and optionally runs the
# Go benchmark suite. Use SESSION_BENCH.md for the manual Claude Code steps.
#
# Usage:
#   bash internal/mcpserver/bench/session_bench.sh [--skip-go-bench]
#
# Options:
#   --skip-go-bench   Skip the Go benchmark suite (faster, shell timings only)

set -euo pipefail

# ── repo root detection ────────────────────────────────────────────────────────
REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
BENCH_DIR="${REPO_ROOT}/internal/mcpserver/bench"

SKIP_GO_BENCH=0
for arg in "$@"; do
  [[ "$arg" == "--skip-go-bench" ]] && SKIP_GO_BENCH=1
done

# ── canonical probe files ──────────────────────────────────────────────────────
# These are stable, checked-in files used for repeatable cross-arm comparison.
# The markdown guide references these exact paths — keep them in sync.
FILE_SMALL="${REPO_ROOT}/internal/mcpserver/fstools/find.go"
FILE_MEDIUM="${REPO_ROOT}/internal/mcpserver/fstools/grep.go"
FILE_LARGE="${REPO_ROOT}/app/app_state.go"

echo "══════════════════════════════════════════════════════════════"
echo " kasmos MCP session bench — shell-side timing arm"
echo "══════════════════════════════════════════════════════════════"
echo ""
echo "repo root : ${REPO_ROOT}"
echo ""
echo "canonical probe files (copy these paths into your Claude Code session):"
echo "  small  : ${FILE_SMALL}"
echo "  medium : ${FILE_MEDIUM}"
echo "  large  : ${FILE_LARGE}"
echo ""

# sanity-check that the probe files exist
for f in "$FILE_SMALL" "$FILE_MEDIUM" "$FILE_LARGE"; do
  if [[ ! -f "$f" ]]; then
    echo "WARNING: probe file not found: $f" >&2
  fi
done

# ── helper: portable nanosecond timestamp ─────────────────────────────────────
# macOS date(1) does not support %N; fall back to python3 or perl.
if date +%s%N 2>/dev/null | grep -qv N; then
  _now_ns() { date +%s%N; }
elif command -v python3 &>/dev/null; then
  _now_ns() { python3 -c 'import time; print(int(time.time()*1e9))'; }
else
  _now_ns() { perl -MTime::HiRes=time -e 'printf "%d\n",time()*1e9'; }
fi

# ── helper: time a command N times and report min/mean/max in ms ──────────────
time_cmd() {
  local label="$1"; shift
  local runs=5
  local total=0 min=999999 max=0

  for i in $(seq 1 "$runs"); do
    local t0 t1 elapsed
    t0=$(_now_ns)
    "$@" > /dev/null 2>&1
    t1=$(_now_ns)
    elapsed=$(( (t1 - t0) / 1000000 ))  # ns → ms
    (( elapsed < min )) && min=$elapsed
    (( elapsed > max )) && max=$elapsed
    total=$(( total + elapsed ))
  done
  local mean=$(( total / runs ))
  printf "  %-52s  min=%dms  mean=%dms  max=%dms\n" "$label" "$min" "$mean" "$max"
}

echo "── shell-side timings (${runs:-5} runs each) ──────────────────────────────"
echo ""

echo "cat -n (read):"
time_cmd "cat -n find.go (small)"  cat -n "$FILE_SMALL"
time_cmd "cat -n grep.go (medium)" cat -n "$FILE_MEDIUM"
time_cmd "cat -n app_state.go (large)" cat -n "$FILE_LARGE"
echo ""

echo "rg (grep):"
time_cmd "rg 'func ' find.go (small)"  rg 'func ' "$FILE_SMALL"
time_cmd "rg 'func ' grep.go (medium)" rg 'func ' "$FILE_MEDIUM"
time_cmd "rg 'func ' app_state.go (large)" rg 'func ' "$FILE_LARGE"
echo ""

echo "fd (find) — searching from repo root:"
time_cmd "fd -e go fstools/ (small subtree)" \
  fd --base-directory "$REPO_ROOT" -e go . internal/mcpserver/fstools
time_cmd "fd -e go app/ (medium subtree)" \
  fd --base-directory "$REPO_ROOT" -e go . app
time_cmd "fd -e go . (full repo)" \
  fd --base-directory "$REPO_ROOT" -e go .
echo ""

# ── optional: Go benchmark suite ──────────────────────────────────────────────
if [[ "$SKIP_GO_BENCH" -eq 0 ]]; then
  REPORT_FILE="$(mktemp /tmp/mcp_bench_report_XXXXXX.json)"
  echo "── Go benchmark suite ────────────────────────────────────────────────"
  echo "report will be written to: ${REPORT_FILE}"
  echo "(set KAS_MCP_NOCACHE=1 to disable the ristretto cache arm)"
  echo ""

  if KAS_MCP_BENCH_REPORT="$REPORT_FILE" \
       go test ./internal/mcpserver/bench/... \
         -run '^$' \
         -bench . \
         -benchtime=1x \
         -count=1 \
         -timeout 120s \
         2>&1; then
    echo ""
    echo "JSON report: ${REPORT_FILE}"
    if command -v jq &>/dev/null && [[ -s "$REPORT_FILE" ]]; then
      echo ""
      echo "── report summary (jq) ────────────────────────────────────────────"
      jq -r '
        .operations[] |
        "  \(.key)\n" +
        "    mcp_warm p50=\(.arms[] | select(.arm=="mcp_warm") | .latency.p50_ns / 1e6 | round)ms" +
        "  direct p50=\(.arms[] | select(.arm=="direct") | .latency.p50_ns / 1e6 | round)ms" +
        "  bash   p50=\(.arms[] | select(.arm=="bash") | .latency.p50_ns / 1e6 | round)ms" +
        "  (overhead mcp/direct=\(.mcp_vs_direct)x  mcp/bash=\(.mcp_vs_bash)x)"
      ' "$REPORT_FILE" 2>/dev/null || true
    fi
  else
    echo "WARNING: Go benchmark suite exited non-zero (kas mcp may not be on PATH)" >&2
    echo "Run manually: KAS_MCP_BENCH_REPORT=/tmp/report.json go test ./internal/mcpserver/bench/... -run '^\$' -bench . -benchtime=1x" >&2
  fi
  echo ""
fi

echo "══════════════════════════════════════════════════════════════"
echo " next step: follow SESSION_BENCH.md to record MCP / built-in"
echo " timings inside a live Claude Code session, then compare to"
echo " the JSON report above."
echo "══════════════════════════════════════════════════════════════"
