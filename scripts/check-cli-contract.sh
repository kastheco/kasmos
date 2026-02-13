#!/usr/bin/env bash
# check-cli-contract.sh — Verify contracts/cli-contract.md matches the implemented CLI.
#
# Usage:
#   scripts/check-cli-contract.sh          # check only (exit 0/1)
#   scripts/check-cli-contract.sh --diff   # show detailed drift
#
# Requires: kasmos binary built and in PATH (or cargo run works).
# This script extracts subcommand names and FIFO commands from the live binary
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
# 2. Extract FIFO subcommands from the binary
# ---------------------------------------------------------------------------
get_live_fifo_commands() {
    # Extract from cmd --help, plus grep the FifoCommand enum for variants
    # that clap hides (like Help via disable_help_subcommand).
    {
        cargo run -p kasmos --quiet -- cmd --help 2>/dev/null \
            | sed -n '/^Commands:/,/^$/p' \
            | grep -E '^\s+\w' \
            | awk '{print $1}'
        # Also extract from source: FifoCommand enum variants -> kebab-case
        grep -oP '^\s+///.*\n\s+(\w+)' "$REPO_ROOT/crates/kasmos/src/cmd.rs" 2>/dev/null \
            | grep -v '///' | sed 's/^\s*//' | head -0  # fallback: noop
        # Explicit: check if Help variant exists in source
        if grep -q '^\s*Help,' "$REPO_ROOT/crates/kasmos/src/cmd.rs" 2>/dev/null; then
            echo "help"
        fi
    } | sort -u
}

# ---------------------------------------------------------------------------
# 3. Extract documented subcommands from the contract
# ---------------------------------------------------------------------------
get_contract_subcommands() {
    # Top-Level Commands table: lines like "| `kasmos xxx` |"
    grep -oP '`kasmos (\w+)' "$CONTRACT" \
        | sed 's/`kasmos //' \
        | sort -u
}

get_contract_fifo_commands() {
    # FIFO Subcommands table: lines like "| `status` |" or "| `restart <wp_id>` |"
    sed -n '/### FIFO Subcommands/,/^## /p' "$CONTRACT" \
        | grep -oP '^\| `(\w[\w-]*)' \
        | sed 's/^| `//' \
        | sort -u
}

# ---------------------------------------------------------------------------
# Compare
# ---------------------------------------------------------------------------
LIVE_CMDS=$(get_live_subcommands)
CONTRACT_CMDS=$(get_contract_subcommands)
LIVE_FIFO=$(get_live_fifo_commands)
CONTRACT_FIFO=$(get_contract_fifo_commands)

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

# Check FIFO commands
MISSING_FIFO=$(comm -23 <(echo "$LIVE_FIFO") <(echo "$CONTRACT_FIFO"))
EXTRA_FIFO=$(comm -13 <(echo "$LIVE_FIFO") <(echo "$CONTRACT_FIFO"))

if [[ -n "$MISSING_FIFO" ]]; then
    echo "DRIFT: FIFO commands in binary but NOT in contract:"
    echo "$MISSING_FIFO" | sed 's/^/  - /'
    ERRORS=1
fi

if [[ -n "$EXTRA_FIFO" ]]; then
    echo "DRIFT: FIFO commands in contract but NOT in binary:"
    echo "$EXTRA_FIFO" | sed 's/^/  - /'
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
    echo ""
    echo "=== Live FIFO commands ==="
    echo "$LIVE_FIFO"
    echo ""
    echo "=== Contract FIFO commands ==="
    echo "$CONTRACT_FIFO"
fi

if [[ $ERRORS -eq 0 ]]; then
    echo "OK: CLI contract is in sync with binary ($(echo "$LIVE_CMDS" | wc -l) subcommands, $(echo "$LIVE_FIFO" | wc -l) FIFO commands)"
    exit 0
else
    echo ""
    echo "ACTION: Update contracts/cli-contract.md to match the current CLI surface."
    exit 1
fi
