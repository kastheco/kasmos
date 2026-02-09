#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

FEATURE=""
ACTION="start"
TARGET_WP=""
FEEDBACK_FILE=""
REJECT_REASON=""
DRY_RUN=0
AGENT=""
REVIEW_AGENT=""
EXTRA_DEPS="${SK_EXTRA_DEPS-__AUTO__}"
DO_CLEANUP=0
ACCEPT_MODE="auto"

declare -a ALL_WPS=()
declare -a WAVE_IDS=()
declare -A WP_LANE=()
declare -A WP_WAVE=()
declare -A WP_FRONT_DEPS=()
declare -A WP_ALL_DEPS=()
declare -A WP_TASK_FILE=()
declare -A WAVE_MEMBERS=()

usage() {
  cat <<'EOF'
Usage: sk-start.sh <feature-prefix> [options]

  <feature-prefix>          Prefix or full slug (e.g. 001, 003, 001-automated-dictation-benchmarking)
                            Resolved by matching kitty-specs/<prefix>*/ directories.

Options:
  --agent <name>            Implementer agent label for lane tracking (default: from .kittify/config.yaml)
  --review-agent <name>     Reviewer agent label (default: same as --agent)
  --dry-run                 Preview actions without mutating state
  --cleanup                 Run orphan context cleanup before starting

Actions (default: start next wave):
  --status                  Show spec-kitty board plus computed waves
  --review <WPxx>           Move WP to for_review, generate REVIEW.md prompt
  --done <WPxx>             Mark WP done
  --reject <WPxx>           Move WP back to planned with feedback
  --accept                  Run acceptance check (validates feature branch for PR mode)
  --feedback <path>         Feedback file for --reject
  --reason <text>           Inline feedback text for --reject (alternative to --feedback)

Environment:
  SK_EXTRA_DEPS             Extra dependency edges (overrides auto-detect).
                            Format: "WP06:WP03,WP04;WP07:WP04"

Examples:
  just swarm 001                            # start next wave for feature 001-...
  just swarm 003 --status                   # show status for feature 003-...
  just swarm 001 --cleanup                  # start with orphan cleanup
  just swarm 001 --review WP02              # submit WP02 for review
  just swarm 001 --done WP01                # mark WP01 done
  just swarm 001 --reject WP02 --feedback /tmp/review.md
  just swarm 001 --reject WP02 --reason "Fix validation in models.py"
  just swarm 001 --dry-run                  # preview without mutations
  just swarm 001 --accept                   # run acceptance validation
  just swarm 001 --accept --accept-mode pr   # force PR mode acceptance
EOF
}

log() {
  printf '%s\n' "$*"
}

warn() {
  printf 'WARN: %s\n' "$*" >&2
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

normalize_wp() {
  local raw="${1^^}"
  if [[ "$raw" =~ ^WP([0-9]+)$ ]]; then
    printf 'WP%02d\n' "$((10#${BASH_REMATCH[1]}))"
    return 0
  fi
  die "Invalid work package ID: $1 (expected WPxx)"
}

set_action() {
  local requested="$1"
  if [[ "$ACTION" != "start" ]]; then
    die "Only one action can be selected"
  fi
  ACTION="$requested"
}

resolve_feature() {
  local prefix="$1"
  local specs_dir="$ROOT/kitty-specs"
  [[ -d "$specs_dir" ]] || die "kitty-specs directory not found: $specs_dir"

  # Exact match first
  if [[ -d "$specs_dir/$prefix" ]]; then
    printf '%s\n' "$prefix"
    return 0
  fi

  # Glob match: prefix*
  local matches=()
  local candidate
  for candidate in "$specs_dir/${prefix}"*/; do
    [[ -d "$candidate" ]] || continue
    # Must have a tasks/ subdirectory with WP files to qualify
    if [[ -d "$candidate/tasks" ]] && compgen -G "$candidate/tasks/WP*.md" >/dev/null 2>&1; then
      matches+=("$(basename "$candidate")")
    fi
  done

  if [[ "${#matches[@]}" -eq 0 ]]; then
    die "No feature found matching prefix: $prefix (searched $specs_dir/${prefix}*/)"
  fi

  if [[ "${#matches[@]}" -gt 1 ]]; then
    warn "Multiple features match prefix '$prefix':"
    for m in "${matches[@]}"; do
      warn "  - $m"
    done
    die "Ambiguous prefix. Be more specific."
  fi

  printf '%s\n' "${matches[0]}"
}

feature_dir() {
  printf '%s/kitty-specs/%s\n' "$ROOT" "$FEATURE"
}

tasks_dir() {
  printf '%s/tasks\n' "$(feature_dir)"
}

worktree_path_for_wp() {
  local wp="$1"
  printf '%s/.worktrees/%s-%s\n' "$ROOT" "$FEATURE" "$wp"
}

csv_to_array() {
  local csv="$1"
  local -n out_ref="$2"
  out_ref=()
  if [[ -z "$csv" ]]; then
    return 0
  fi
  IFS=',' read -r -a out_ref <<< "$csv"
}

detect_default_agent() {
  python3 - "$ROOT/.kittify/config.yaml" <<'PY'
import re
import sys
from pathlib import Path

config_path = Path(sys.argv[1])
if not config_path.exists():
    print("opencode")
    raise SystemExit(0)

text = config_path.read_text(encoding="utf-8")
match = re.search(r"^\s*preferred_implementer\s*:\s*([^\n#]+)", text, flags=re.MULTILINE)
if not match:
    print("opencode")
    raise SystemExit(0)

value = match.group(1).strip().strip('"').strip("'")
print(value or "opencode")
PY
}

default_extra_deps() {
  local feature="$1"
  case "$feature" in
    001-automated-dictation-benchmarking)
      printf '%s\n' "WP06:WP03,WP04;WP07:WP04;WP09:WP02,WP03,WP04,WP05"
      ;;
    *)
      printf '%s\n' ""
      ;;
  esac
}

load_metadata() {
  local fdir
  fdir="$(feature_dir)"
  local output
  if ! output="$(python3 - "$fdir" "$EXTRA_DEPS" <<'PY'
import re
import sys
from pathlib import Path


def strip_quotes(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
        return value[1:-1]
    return value


def normalize_wp(value: str) -> str:
    value = strip_quotes(value).upper()
    match = re.match(r"^WP(\d+)$", value)
    if not match:
        raise ValueError(f"Invalid WP id: {value}")
    return f"WP{int(match.group(1)):02d}"


def parse_frontmatter(path: Path):
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---"):
        raise ValueError(f"Missing frontmatter in {path}")
    parts = text.split("---", 2)
    if len(parts) < 3:
        raise ValueError(f"Malformed frontmatter in {path}")
    lines = parts[1].splitlines()

    wp_id = None
    lane = "planned"
    dependencies = []

    i = 0
    while i < len(lines):
        line = lines[i]

        wp_match = re.match(r"^work_package_id\s*:\s*(.+?)\s*$", line)
        if wp_match:
            wp_id = normalize_wp(wp_match.group(1))
            i += 1
            continue

        lane_match = re.match(r"^lane\s*:\s*(.+?)\s*$", line)
        if lane_match:
            lane = strip_quotes(lane_match.group(1)).strip() or "planned"
            i += 1
            continue

        dep_match = re.match(r"^dependencies\s*:\s*(.*?)\s*$", line)
        if dep_match:
            tail = dep_match.group(1).strip()
            if tail.startswith("[") and tail.endswith("]"):
                inner = tail[1:-1].strip()
                if inner:
                    dependencies = [normalize_wp(part.strip()) for part in inner.split(",") if part.strip()]
            else:
                j = i + 1
                parsed = []
                while j < len(lines):
                    item_match = re.match(r"^\s*-\s*(.+?)\s*$", lines[j])
                    if item_match:
                        parsed.append(normalize_wp(item_match.group(1)))
                        j += 1
                        continue
                    key_match = re.match(r"^[A-Za-z_][A-Za-z0-9_]*\s*:\s*", lines[j])
                    if key_match:
                        break
                    if lines[j].strip() == "":
                        j += 1
                        continue
                    break
                dependencies = parsed
            i += 1
            continue

        i += 1

    if wp_id is None:
        raise ValueError(f"Missing work_package_id in {path}")

    seen = set()
    deduped = []
    for dep in dependencies:
        if dep not in seen:
            deduped.append(dep)
            seen.add(dep)

    return wp_id, lane, deduped


def wp_sort_key(wp_id: str):
    m = re.match(r"^WP(\d+)$", wp_id)
    return (int(m.group(1)) if m else 10**9, wp_id)


def parse_extra_deps(raw: str):
    extra = {}
    if not raw.strip():
        return extra
    for clause in raw.split(";"):
        clause = clause.strip()
        if not clause:
            continue
        if ":" not in clause:
            raise ValueError(f"Invalid SK_EXTRA_DEPS clause: {clause}")
        wp_raw, deps_raw = clause.split(":", 1)
        wp_id = normalize_wp(wp_raw)
        deps = [normalize_wp(part.strip()) for part in deps_raw.split(",") if part.strip()]
        bucket = extra.setdefault(wp_id, [])
        bucket.extend(deps)
    return extra


feature_dir = Path(sys.argv[1])
extra_raw = sys.argv[2]
tasks_dir = feature_dir / "tasks"

if not tasks_dir.exists():
    raise SystemExit(f"Feature tasks directory not found: {tasks_dir}")

records = {}
for path in sorted(tasks_dir.glob("WP*.md")):
    wp_id, lane, deps = parse_frontmatter(path)
    records[wp_id] = {
        "lane": lane,
        "front_deps": deps,
        "task_file": str(path),
    }

if not records:
    raise SystemExit(f"No WP task files found in {tasks_dir}")

extra_deps = parse_extra_deps(extra_raw)

for wp_id, data in records.items():
    merged = []
    seen = set()
    for dep in data["front_deps"] + extra_deps.get(wp_id, []):
        if dep not in seen:
            merged.append(dep)
            seen.add(dep)
    data["all_deps"] = merged

unknown = []
for wp_id, data in records.items():
    for dep in data["all_deps"]:
        if dep not in records:
            unknown.append((wp_id, dep))

if unknown:
    details = ", ".join(f"{wp}->{dep}" for wp, dep in unknown)
    raise SystemExit(f"Unknown dependencies in graph: {details}")

dependents = {wp: [] for wp in records}
indegree = {wp: 0 for wp in records}
for wp_id, data in records.items():
    indegree[wp_id] = len(data["all_deps"])
    for dep in data["all_deps"]:
        dependents[dep].append(wp_id)

for dep in dependents:
    dependents[dep].sort(key=wp_sort_key)

ready = sorted([wp for wp, count in indegree.items() if count == 0], key=wp_sort_key)
waves = []
processed = 0
while ready:
    wave = ready
    waves.append(wave)
    next_ready = []
    for parent in wave:
        processed += 1
        for child in dependents[parent]:
            indegree[child] -= 1
            if indegree[child] == 0:
                next_ready.append(child)
    ready = sorted(next_ready, key=wp_sort_key)

if processed != len(records):
    raise SystemExit("Dependency cycle detected while computing waves")

for idx, wave in enumerate(waves, start=1):
    for wp_id in wave:
        record = records[wp_id]
        print(
            "\x1f".join(
                [
                    wp_id,
                    str(idx),
                    record["lane"],
                    ",".join(record["front_deps"]),
                    ",".join(record["all_deps"]),
                    record["task_file"],
                ]
            )
        )
PY
)"; then
    die "Failed to load metadata for feature: $FEATURE"
  fi

  ALL_WPS=()
  WAVE_IDS=()
  WP_LANE=()
  WP_WAVE=()
  WP_FRONT_DEPS=()
  WP_ALL_DEPS=()
  WP_TASK_FILE=()
  WAVE_MEMBERS=()

  local last_wave=""
  while IFS=$'\x1f' read -r row_wp row_wave row_lane row_front_deps row_all_deps row_task_file; do
    [[ -z "$row_wp" ]] && continue
    ALL_WPS+=("$row_wp")
    WP_LANE["$row_wp"]="$row_lane"
    WP_WAVE["$row_wp"]="$row_wave"
    WP_FRONT_DEPS["$row_wp"]="$row_front_deps"
    WP_ALL_DEPS["$row_wp"]="$row_all_deps"
    WP_TASK_FILE["$row_wp"]="$row_task_file"

    if [[ "$row_wave" != "$last_wave" ]]; then
      WAVE_IDS+=("$row_wave")
      last_wave="$row_wave"
    fi

    local existing="${WAVE_MEMBERS[$row_wave]-}"
    if [[ -n "$existing" ]]; then
      WAVE_MEMBERS["$row_wave"]="$existing,$row_wp"
    else
      WAVE_MEMBERS["$row_wave"]="$row_wp"
    fi
  done <<< "$output"

  if [[ "${#ALL_WPS[@]}" -eq 0 ]]; then
    die "No work packages detected for feature: $FEATURE"
  fi
}

require_wp_exists() {
  local wp="$1"
  if [[ -z "${WP_WAVE[$wp]-}" ]]; then
    die "Unknown work package: $wp"
  fi
}

cleanup_contexts() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "[dry-run] spec-kitty context cleanup --dry-run"
    spec-kitty context cleanup --dry-run
  else
    spec-kitty context cleanup
  fi
}

ensure_feature_branch() {
  local feature_branch="$FEATURE"
  local current_branch
  current_branch="$(git branch --show-current)"

  # Guard against detached HEAD
  [[ -n "$current_branch" ]] || die "Cannot create feature branch from detached HEAD"

  # If already on the feature branch, nothing to do
  if [[ "$current_branch" == "$feature_branch" ]]; then
    return 0
  fi

  # If the feature branch already exists, just switch to it
  if git rev-parse --verify "$feature_branch" >/dev/null 2>&1; then
    if [[ "$DRY_RUN" -eq 1 ]]; then
      log "[dry-run] git checkout $feature_branch"
    else
      git checkout "$feature_branch" || die "Failed to switch to feature branch '$feature_branch' (dirty worktree?)"
      log "Switched to existing feature branch: $feature_branch"
    fi
  else
    # Create the feature branch from current position (should be master/main)
    if [[ "$DRY_RUN" -eq 1 ]]; then
      log "[dry-run] git checkout -b $feature_branch"
    else
      git checkout -b "$feature_branch" || die "Failed to create feature branch '$feature_branch'"
      log "Created feature branch: $feature_branch (from $current_branch)"
    fi
  fi

  # Ensure meta.json target_branch matches the feature branch
  local meta_file
  meta_file="$(feature_dir)/meta.json"
  if [[ -f "$meta_file" ]]; then
    local current_target
    current_target="$(python3 -c "import json,sys; print(json.load(open(sys.argv[1])).get('target_branch',''))" "$meta_file")"
    if [[ "$current_target" != "$feature_branch" ]]; then
      if [[ "$DRY_RUN" -eq 1 ]]; then
        log "[dry-run] Update meta.json target_branch → $feature_branch"
      else
        python3 -c "
import json, sys
path = sys.argv[1]
branch = sys.argv[2]
with open(path) as f:
    data = json.load(f)
data['target_branch'] = branch
with open(path, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\\n')
" "$meta_file" "$feature_branch"
        git add "$meta_file"
        git commit -m "chore: set target_branch to feature branch $feature_branch" || die "Failed to commit meta.json update"
        log "Updated meta.json target_branch → $feature_branch"
      fi
    fi
  fi
}

ensure_dependencies_done() {
  local wp="$1"
  local deps_csv="${WP_ALL_DEPS[$wp]-}"
  local deps=()
  csv_to_array "$deps_csv" deps
  for dep in "${deps[@]}"; do
    local dep_lane="${WP_LANE[$dep]-unknown}"
    if [[ "$dep_lane" != "done" ]]; then
      return 1
    fi
  done
  return 0
}

setup_wp() {
  local wp="$1"
  local lane="${WP_LANE[$wp]}"
  local worktree
  worktree="$(worktree_path_for_wp "$wp")"

  if [[ "$lane" != "planned" ]]; then
    log "- $wp is already in lane '$lane', skipping setup"
    return 0
  fi

  local front_deps_csv="${WP_FRONT_DEPS[$wp]-}"
  local base_wp=""
  if [[ -n "$front_deps_csv" ]]; then
    base_wp="${front_deps_csv%%,*}"
  fi

  if [[ ! -d "$worktree" ]]; then
    local impl_cmd=(spec-kitty implement "$wp" --feature "$FEATURE")
    if [[ -n "$base_wp" ]]; then
      impl_cmd+=(--base "$base_wp")
    fi

    if [[ "$DRY_RUN" -eq 1 ]]; then
      log "[dry-run] ${impl_cmd[*]}"
    else
      "${impl_cmd[@]}"
    fi
  else
    log "- Reusing existing worktree for $wp: $worktree"
  fi

  local workflow_cmd=(spec-kitty agent workflow implement "$wp" --feature "$FEATURE" --agent "$AGENT")
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "[dry-run] ${workflow_cmd[*]}"
    return 0
  fi

  local prompt_output
  if ! prompt_output="$("${workflow_cmd[@]}" 2>&1)"; then
    die "Failed to generate implement prompt for $wp"
  fi

  [[ -d "$worktree" ]] || die "Expected worktree not found for $wp: $worktree"
  local prompt_file="$worktree/IMPLEMENT.md"
  printf '%s\n' "$prompt_output" > "$prompt_file"
  cat >> "$prompt_file" <<'EOF'

## Spec-kitty lane policy

- Use spec-kitty lane commands only (not Vibe Kanban APIs).
- Mark all WP subtasks done before lane move:
  - `spec-kitty agent tasks mark-status <TIDs...> --status done`
- Rebase worktree branch onto `master` before lane move:
  - `git rebase master`
- Move the WP to review lane with spec-kitty:
  - `spec-kitty agent tasks move-task <WPID> --to for_review`
  - Optional note: `--note "Ready for review"`
EOF
  log "- Wrote implement prompt: $prompt_file"
}

print_next_wave_instructions() {
  local wave="$1"
  local wave_csv="${WAVE_MEMBERS[$wave]}"
  local wave_wps=()
  csv_to_array "$wave_csv" wave_wps

  log ""
  log "Wave $wave ready: ${wave_csv//,/ }"
  log ""
  for wp in "${wave_wps[@]}"; do
    local lane="${WP_LANE[$wp]}"
    if [[ "$lane" == "done" ]]; then
      log "- $wp: done"
      continue
    fi

    local worktree
    worktree="$(worktree_path_for_wp "$wp")"
    local prompt_file="$worktree/IMPLEMENT.md"
    log "- $wp"
    log "  lane: $lane"
    log "  worktree: $worktree"
    log "  prompt: $prompt_file"
    log "  launch: cd \"$worktree\" && ocx opencode -p ws -- --agent coder --prompt \"@IMPLEMENT.md\""
    local feat_prefix="${FEATURE%%-*}"
    log "  when done: just swarm $feat_prefix --review $wp"
  done
}

start_next_wave() {
  if [[ "$DO_CLEANUP" -eq 1 ]]; then
    cleanup_contexts
  fi
  ensure_feature_branch
  load_metadata

  local next_wave=""
  for wave in "${WAVE_IDS[@]}"; do
    local wave_csv="${WAVE_MEMBERS[$wave]}"
    local wave_wps=()
    csv_to_array "$wave_csv" wave_wps

    local all_done=1
    for wp in "${wave_wps[@]}"; do
      if [[ "${WP_LANE[$wp]}" != "done" ]]; then
        all_done=0
        break
      fi
    done

    if [[ "$all_done" -eq 0 ]]; then
      next_wave="$wave"
      break
    fi
  done

  if [[ -z "$next_wave" ]]; then
    log "All work packages are done for feature: $FEATURE"
    return 0
  fi

  local blocked=0
  local wave_csv="${WAVE_MEMBERS[$next_wave]}"
  local wave_wps=()
  csv_to_array "$wave_csv" wave_wps
  for wp in "${wave_wps[@]}"; do
    local lane="${WP_LANE[$wp]}"
    if [[ "$lane" == "done" ]]; then
      continue
    fi
    if ! ensure_dependencies_done "$wp"; then
      blocked=1
      local deps_csv="${WP_ALL_DEPS[$wp]-}"
      local deps=()
      csv_to_array "$deps_csv" deps
      for dep in "${deps[@]}"; do
        local dep_lane="${WP_LANE[$dep]-unknown}"
        if [[ "$dep_lane" != "done" ]]; then
          log "Blocked: $wp waits on $dep (lane=$dep_lane)"
        fi
      done
    fi
  done

  if [[ "$blocked" -eq 1 ]]; then
    die "Wave $next_wave is blocked by unfinished dependencies"
  fi

  log "Starting setup for wave $next_wave"
  for wp in "${wave_wps[@]}"; do
    setup_wp "$wp"
  done

  load_metadata
  print_next_wave_instructions "$next_wave"
}

show_status() {
  spec-kitty agent tasks status --feature "$FEATURE"
  load_metadata

  log ""
  log "Computed wave plan for $FEATURE:"
  for wave in "${WAVE_IDS[@]}"; do
    local wave_csv="${WAVE_MEMBERS[$wave]}"
    local wave_wps=()
    csv_to_array "$wave_csv" wave_wps
    local items=()
    for wp in "${wave_wps[@]}"; do
      items+=("$wp(${WP_LANE[$wp]})")
    done
    log "- Wave $wave: ${items[*]}"
  done
}

start_review() {
  local wp="$1"
  local prefix="${FEATURE%%%-*}"

  load_metadata
  require_wp_exists "$wp"
  local lane="${WP_LANE[$wp]}"
  local worktree
  worktree="$(worktree_path_for_wp "$wp")"

  if [[ "$lane" == "planned" ]]; then
    die "$wp is still planned. Run 'just swarm ${FEATURE%%-*}' first."
  fi
  if [[ "$lane" == "done" ]]; then
    log "$wp is already done"
    return 0
  fi

  if [[ "$lane" == "doing" ]]; then
    if [[ "$DRY_RUN" -eq 1 ]]; then
      log "[dry-run] spec-kitty agent tasks move-task $wp --feature $FEATURE --to for_review --agent $AGENT --no-auto-commit"
    else
      spec-kitty agent tasks move-task "$wp" --feature "$FEATURE" --to for_review --agent "$AGENT" --note "Submitted for review via swarm" --no-auto-commit
    fi
  fi

  local review_cmd=(spec-kitty agent workflow review "$wp" --feature "$FEATURE" --agent "$REVIEW_AGENT")
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "[dry-run] ${review_cmd[*]}"
    return 0
  fi

  local prompt_output
  if ! prompt_output="$("${review_cmd[@]}" 2>&1)"; then
    die "Failed to generate review prompt for $wp"
  fi

  [[ -d "$worktree" ]] || die "Missing worktree for $wp: $worktree"
  printf '%s\n' "$prompt_output" > "$worktree/REVIEW.md"

  log "Review prompt written: $worktree/REVIEW.md"
  log "Launch reviewer: cd \"$worktree\" && ocx opencode -p ws -- --agent reviewer --prompt \"@REVIEW.md\""
  log "Then: just swarm $prefix --done $wp  OR  just swarm $prefix --reject $wp --feedback /path/to/feedback.md"
}

mark_done() {
  local wp="$1"
  local prefix="${FEATURE%%%-*}"

  load_metadata
  require_wp_exists "$wp"
  local lane="${WP_LANE[$wp]}"
  if [[ "$lane" == "done" ]]; then
    log "$wp is already done"
    return 0
  fi
  if [[ "$lane" == "planned" ]]; then
    die "$wp is still planned"
  fi

  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "[dry-run] spec-kitty agent tasks move-task $wp --feature $FEATURE --to done --agent $REVIEW_AGENT --force --no-auto-commit"
    return 0
  fi

  spec-kitty agent tasks move-task "$wp" --feature "$FEATURE" --to done --agent "$REVIEW_AGENT" --note "Review passed via swarm" --force --no-auto-commit

  load_metadata
  local wave="${WP_WAVE[$wp]}"
  local wave_csv="${WAVE_MEMBERS[$wave]}"
  local wave_wps=()
  csv_to_array "$wave_csv" wave_wps

  local remaining=()
  for member in "${wave_wps[@]}"; do
    if [[ "${WP_LANE[$member]}" != "done" ]]; then
      remaining+=("$member")
    fi
  done

  if [[ "${#remaining[@]}" -eq 0 ]]; then
    log "Wave $wave is complete. Run 'just swarm $prefix' to begin the next wave."
  else
    log "Wave $wave still has open WPs: ${remaining[*]}"
  fi
}

reject_wp() {
  local wp="$1"
  local effective_feedback="$FEEDBACK_FILE"

  # --reason writes inline text to a temp file
  if [[ -n "$REJECT_REASON" ]]; then
    if [[ -n "$FEEDBACK_FILE" ]]; then
      die "Use --feedback or --reason, not both"
    fi
    effective_feedback="$(mktemp /tmp/swarm-reject-XXXXXX.md)"
    printf '%s\n' "$REJECT_REASON" > "$effective_feedback"
  fi

  [[ -n "$effective_feedback" ]] || die "--feedback or --reason is required with --reject"
  [[ -f "$effective_feedback" ]] || die "Feedback file not found: $effective_feedback"

  load_metadata
  require_wp_exists "$wp"

  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "[dry-run] spec-kitty agent tasks move-task $wp --feature $FEATURE --to planned --review-feedback-file $effective_feedback --reviewer $REVIEW_AGENT --force --no-auto-commit"
    [[ -n "$REJECT_REASON" ]] && rm -f "$effective_feedback"
    return 0
  fi

  spec-kitty agent tasks move-task "$wp" \
    --feature "$FEATURE" \
    --to planned \
    --review-feedback-file "$effective_feedback" \
    --reviewer "$REVIEW_AGENT" \
    --force \
    --no-auto-commit

  log "$wp moved back to planned with feedback"
  [[ -n "$REJECT_REASON" ]] && log "Reason: $REJECT_REASON" && rm -f "$effective_feedback"
  [[ -z "$REJECT_REASON" ]] && log "Feedback file: $effective_feedback"
}

run_accept() {
  local current_branch
  current_branch="$(git branch --show-current)"

  # Warn if on master/main with PR mode
  if [[ "$ACCEPT_MODE" == "pr" || "$ACCEPT_MODE" == "auto" ]]; then
    if [[ "$current_branch" == "master" || "$current_branch" == "main" ]]; then
      warn "Running acceptance on '$current_branch' — PR mode expects a feature branch."
      warn "Future features should use 'just swarm <prefix>' to auto-create a feature branch."
      warn "Falling back to --mode local since there is no feature branch to PR."
      ACCEPT_MODE="local"
    fi
  fi

  local accept_cmd=(spec-kitty accept --feature "$FEATURE" --mode "$ACCEPT_MODE" --actor "$AGENT")

  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "[dry-run] ${accept_cmd[*]}"
    return 0
  fi

  "${accept_cmd[@]}"
}

parse_args() {
  # First non-flag argument is the feature prefix
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --agent)
        [[ $# -ge 2 ]] || die "--agent requires a value"
        AGENT="$2"
        shift 2
        ;;
      --review-agent)
        [[ $# -ge 2 ]] || die "--review-agent requires a value"
        REVIEW_AGENT="$2"
        shift 2
        ;;
      --status)
        set_action "status"
        shift
        ;;
      --cleanup)
        DO_CLEANUP=1
        shift
        ;;
      --review)
        [[ $# -ge 2 ]] || die "--review requires a WP ID"
        set_action "review"
        TARGET_WP="$(normalize_wp "$2")"
        shift 2
        ;;
      --done)
        [[ $# -ge 2 ]] || die "--done requires a WP ID"
        set_action "done"
        TARGET_WP="$(normalize_wp "$2")"
        shift 2
        ;;
      --reject)
        [[ $# -ge 2 ]] || die "--reject requires a WP ID"
        set_action "reject"
        TARGET_WP="$(normalize_wp "$2")"
        shift 2
        ;;
      --accept)
        set_action "accept"
        shift
        ;;
      --accept-mode)
        [[ $# -ge 2 ]] || die "--accept-mode requires a value (pr, local, checklist)"
        case "$2" in
          pr|local|checklist|auto) ;;
          *) die "Invalid --accept-mode: $2 (expected: pr, local, checklist, auto)" ;;
        esac
        ACCEPT_MODE="$2"
        shift 2
        ;;
      --feedback)
        [[ $# -ge 2 ]] || die "--feedback requires a path"
        FEEDBACK_FILE="$2"
        shift 2
        ;;
      --reason)
        [[ $# -ge 2 ]] || die "--reason requires text"
        shift
        # Consume all remaining non-flag args as the reason text
        local reason_parts=()
        while [[ $# -gt 0 ]] && [[ "$1" != --* ]]; do
          reason_parts+=("$1")
          shift
        done
        [[ ${#reason_parts[@]} -gt 0 ]] || die "--reason requires text"
        REJECT_REASON="${reason_parts[*]}"
        ;;
      --dry-run)
        DRY_RUN=1
        shift
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      --*)
        die "Unknown option: $1"
        ;;
      *)
        # Positional: feature prefix
        if [[ -z "$FEATURE" ]]; then
          FEATURE="$1"
        else
          die "Unexpected argument: $1 (feature already set to '$FEATURE')"
        fi
        shift
        ;;
    esac
  done
}

main() {
  parse_args "$@"

  [[ -n "$FEATURE" ]] || die "Feature prefix is required as first argument"

  cd "$ROOT"

  require_cmd git
  require_cmd python3
  require_cmd spec-kitty

  # Resolve prefix to full slug
  FEATURE="$(resolve_feature "$FEATURE")"

  if [[ -z "$AGENT" ]]; then
    AGENT="$(detect_default_agent)"
  fi
  if [[ -z "$REVIEW_AGENT" ]]; then
    REVIEW_AGENT="$AGENT"
  fi

  if [[ "$EXTRA_DEPS" == "__AUTO__" ]]; then
    EXTRA_DEPS="$(default_extra_deps "$FEATURE")"
  fi

  case "$ACTION" in
    start)
      start_next_wave
      ;;
    status)
      show_status
      ;;
    review)
      start_review "$TARGET_WP"
      ;;
    done)
      mark_done "$TARGET_WP"
      ;;
    reject)
      reject_wp "$TARGET_WP"
      ;;
    accept)
      run_accept
      ;;
    *)
      die "Unknown action: $ACTION"
      ;;
  esac
}

main "$@"
