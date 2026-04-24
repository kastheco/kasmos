#!/usr/bin/env bash
# bench_tests.sh — reproducible test suite timing for kasmos
# prints cold-path, warm-path, and per-package timings using only bash builtins for timing.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ── helpers ──────────────────────────────────────────────────────────────────

hr() { printf '%0.s─' {1..60}; printf '\n'; }

# shell-portable elapsed time using $SECONDS (integer) for display;
# TIMEFORMAT gives sub-second precision inside a `time` compound command.
run_timed() {
    local label="$1"; shift
    printf '\n%s\n' "$label"
    hr
    TIMEFORMAT='elapsed: %Rs'
    time "$@"
    hr
}

# ── 1. priming run (populates the build cache) ───────────────────────────────
printf '\n[bench] priming build cache…\n'
go test ./... >/dev/null 2>&1 || true   # errors are expected if tests fail; we just want the cache

# ── 2. warm-cache timing ──────────────────────────────────────────────────────
run_timed '[bench] warm-cache suite (go test ./...)' \
    go test ./...

# ── 3. cold-path timing ───────────────────────────────────────────────────────
run_timed '[bench] cold-path suite (go test -count=1 ./...)' \
    go test -count=1 ./...

# ── 4. per-package breakdown ──────────────────────────────────────────────────
printf '\n[bench] per-package timings (descending)\n'
hr

if ! command -v jq >/dev/null 2>&1; then
    printf 'warning: jq not found — skipping per-package table\n'
else
    { printf "%-10s  %s\n%-10s  %s\n" "elapsed" "package" "-------" "-------";
      go test -json -count=1 ./... 2>/dev/null \
        | jq -r 'select(.Action=="pass" and .Package and (.Test|not)) | [.Elapsed, .Package] | @tsv' \
        | sort -rn \
        | while IFS=$'\t' read -r elapsed pkg; do printf "%-10ss  %s\n" "$elapsed" "$pkg"; done; }
fi

hr
printf '[bench] done\n\n'
