#!/usr/bin/env bash
set -euo pipefail

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

log() {
  printf '%s\n' "$*"
}

normalize_wp() {
  local raw="${1^^}"
  if [[ "$raw" =~ ^WP([0-9]+)$ ]]; then
    printf 'WP%02d\n' "$((10#${BASH_REMATCH[1]}))"
    return 0
  fi
  die "Invalid WP id: $1 (expected WP##)"
}

resolve_feature() {
  local root="$1"
  local input="$2"

  if [[ -d "$root/kitty-specs/$input" ]]; then
    printf '%s\n' "$input"
    return 0
  fi

  local matches=()
  local candidate
  for candidate in "$root/kitty-specs/${input}"*/; do
    [[ -d "$candidate" ]] || continue
    if [[ -d "$candidate/tasks" ]] && compgen -G "$candidate/tasks/WP*.md" >/dev/null 2>&1; then
      matches+=("$(basename "$candidate")")
    fi
  done

  if [[ "${#matches[@]}" -eq 0 ]]; then
    die "No feature found matching '$input'"
  fi

  if [[ "${#matches[@]}" -gt 1 ]]; then
    die "Ambiguous feature '$input': ${matches[*]}"
  fi

  printf '%s\n' "${matches[0]}"
}

trim() {
  local value="$1"
  value="${value#${value%%[![:space:]]*}}"
  value="${value%${value##*[![:space:]]}}"
  printf '%s\n' "$value"
}

main() {
  command -v git >/dev/null 2>&1 || die "git is required"
  command -v just >/dev/null 2>&1 || die "just is required"
  command -v ocx >/dev/null 2>&1 || die "ocx is required"
  command -v python3 >/dev/null 2>&1 || die "python3 is required"

  local arg_feature=""
  local arg_wp=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      WP*|wp*)
        arg_wp="$(normalize_wp "$1")"
        ;;
      *)
        if [[ -n "$arg_feature" ]]; then
          die "Unexpected extra argument: $1"
        fi
        arg_feature="$1"
        ;;
    esac
    shift
  done

  local git_common_dir
  git_common_dir="$(git rev-parse --git-common-dir 2>/dev/null)" || die "Run from a git worktree"

  local root
  root="$(cd "$git_common_dir/.." && pwd)"
  [[ -f "$root/Justfile" ]] || die "Could not locate main repo root from git-common-dir: $git_common_dir"

  local branch
  branch="$(git branch --show-current)"

  local branch_feature=""
  local branch_wp=""
  if [[ "$branch" =~ ^([0-9]{3}-[a-z0-9-]+)-WP([0-9]+)$ ]]; then
    branch_feature="${BASH_REMATCH[1]}"
    branch_wp="WP$(printf '%02d' "$((10#${BASH_REMATCH[2]}))")"
  elif [[ "$branch" =~ ^([0-9]{3}-[a-z0-9-]+)$ ]]; then
    branch_feature="${BASH_REMATCH[1]}"
  fi

  local feature_input="${arg_feature:-$branch_feature}"
  [[ -n "$feature_input" ]] || die "Could not infer feature. Pass it explicitly (e.g. 002 or 002-... )."

  local feature
  feature="$(resolve_feature "$root" "$feature_input")"

  local wp="${arg_wp:-$branch_wp}"
  [[ -n "$wp" ]] || die "Could not infer WP from branch '$branch'. Pass WP explicitly (e.g. WP02)."

  local prefix="${feature%%-*}"
  local worktree="$root/.worktrees/${feature}-${wp}"
  [[ -d "$worktree" ]] || die "Expected worktree not found: $worktree"

  log "==> Submitting $feature $wp for review"
  just -f "$root/Justfile" swarm "$feature" --review "$wp"

  log "==> Launching reviewer session (agent=reviewer)"
  local review_output
  if ! review_output="$(cd "$worktree" && ocx oc -- run --agent reviewer --variant high "/kas:review $feature $wp" 2>&1)"; then
    printf '%s\n' "$review_output" >&2
    die "Reviewer session failed"
  fi

  local kas_dir="$worktree/.kas"
  mkdir -p "$kas_dir"
  local last_file="$kas_dir/review-${wp}.last.txt"
  printf '%s\n' "$review_output" > "$last_file"

  local decision
  decision="$(python3 - "$last_file" <<'PY'
import re
import sys
from pathlib import Path

text = Path(sys.argv[1]).read_text(encoding="utf-8", errors="ignore")
# Strip ANSI sequences so regex anchors match clean lines.
text = re.sub(r"\x1b\[[0-?]*[ -/]*[@-~]", "", text)

decision = ""

# Preferred explicit decision emitted by /kas:review automation.
match = re.search(r"^\s*DECISION:\s*([A-Z_]+)\b", text, flags=re.MULTILINE | re.IGNORECASE)
if match:
    decision = match.group(1).upper()
else:
    # Fallback for reviewer outputs that use an assessment label instead.
    assess = re.search(r"^\s*ASSESSMENT:\s*([A-Z_ ]+)\b", text, flags=re.MULTILINE | re.IGNORECASE)
    if assess:
        label = assess.group(1).strip().upper().replace(" ", "_")
        mapping = {
            "APPROVE": "VERIFIED",
            "APPROVED": "VERIFIED",
            "REQUEST_CHANGES": "NEEDS_CHANGES",
            "REQUESTED_CHANGES": "NEEDS_CHANGES",
            "NEEDS_CHANGES": "NEEDS_CHANGES",
            "BLOCKED": "BLOCKED",
            "REJECT": "NEEDS_CHANGES",
            "REJECTED": "NEEDS_CHANGES",
        }
        decision = mapping.get(label, label)

print(decision)
PY
)"
  decision="$(trim "$decision")"

  if [[ "$decision" == "VERIFIED" ]]; then
    log "==> Review VERIFIED; marking $wp done"
    just -f "$root/Justfile" swarm "$feature" --done "$wp"
  elif [[ "$decision" == "NEEDS_CHANGES" || "$decision" == "BLOCKED" ]]; then
    log "==> Review $decision; WP moved back to doing with feedback by /kas:review automation"
  else
    log "==> Could not parse review decision from reviewer output"
    log "    Expected DECISION: <VERIFIED|NEEDS_CHANGES|BLOCKED>"
    log "    or ASSESSMENT: <APPROVE|REQUEST_CHANGES|...>"
    log "    See: $last_file"
  fi

  log "==> Current status"
  just -f "$root/Justfile" swarm "$prefix" --status
}

main "$@"
