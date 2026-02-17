#!/usr/bin/env bash
# check-cli-contract.sh — Verify contracts/cli-contract.md matches the implemented CLI.
#
# Usage:
#   scripts/check-cli-contract.sh          # check only (exit 0/1)
#   scripts/check-cli-contract.sh --diff   # show detailed drift
#
# Requires: kasmos binary built and in PATH (or cargo run works).
# This script extracts top-level subcommand names from the live binary
# and compares them against the contract document.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONTRACT="$REPO_ROOT/contracts/cli-contract.md"
SHOW_DIFF="${1:-}"

if [[ ! -f "$CONTRACT" ]]; then
    echo "ERROR: Contract file not found: $CONTRACT"
    exit 1
fi

# ---------------------------------------------------------------------------
# 1. Extract top-level subcommands from the binary
# ---------------------------------------------------------------------------
get_live_subcommands() {
    cargo run -p kasmos --quiet -- --help 2>/dev/null \
        | sed -n '/^Commands:/,/^$/p' \
        | grep -E '^\s+\w' \
        | awk '{print $1}' \
        | grep -v '^help$' \
        | sort
}

# ---------------------------------------------------------------------------
# 2. Extract documented subcommands from the contract
# ---------------------------------------------------------------------------
get_contract_subcommands() {
    awk '
        /^## Top-Level Commands$/ { in_table = 1; next }
        in_table && /^## / { in_table = 0 }
        in_table {
            if (match($0, /`kasmos ([^` ]+)/, m)) {
                cmd = m[1]
                if (cmd ~ /^[a-z][a-z0-9-]*$/) {
                    print cmd
                }
            }
        }
    ' "$CONTRACT" | sort -u
}

# ---------------------------------------------------------------------------
# Compare
# ---------------------------------------------------------------------------
LIVE_CMDS=$(get_live_subcommands)
CONTRACT_CMDS=$(get_contract_subcommands)

ERRORS=0

# Check top-level commands
MISSING_FROM_CONTRACT=$(comm -23 <(echo "$LIVE_CMDS") <(echo "$CONTRACT_CMDS"))
EXTRA_IN_CONTRACT=$(comm -13 <(echo "$LIVE_CMDS") <(echo "$CONTRACT_CMDS"))

if [[ -n "$MISSING_FROM_CONTRACT" ]]; then
    echo "DRIFT: Subcommands in binary but NOT in contract:"
    echo "$MISSING_FROM_CONTRACT" | sed 's/^/  - /'
    ERRORS=1
fi

if [[ -n "$EXTRA_IN_CONTRACT" ]]; then
    echo "DRIFT: Subcommands in contract but NOT in binary:"
    echo "$EXTRA_IN_CONTRACT" | sed 's/^/  - /'
    ERRORS=1
fi

# Show detailed diff if requested
if [[ "$SHOW_DIFF" == "--diff" ]]; then
    echo ""
    echo "=== Live subcommands ==="
    echo "$LIVE_CMDS"
    echo ""
    echo "=== Contract subcommands ==="
    echo "$CONTRACT_CMDS"
fi

if [[ $ERRORS -eq 0 ]]; then
    echo "OK: CLI contract is in sync with binary ($(echo "$LIVE_CMDS" | wc -l) subcommands)"
    exit 0
else
    echo ""
    echo "ACTION: Update contracts/cli-contract.md to match the current CLI surface."
    exit 1
fi
