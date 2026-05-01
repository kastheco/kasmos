#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
HOOK_SRC="$ROOT/scripts/git-hooks/pre-push"
INSTALL_SRC="$ROOT/scripts/git-hooks/install.sh"
DETECTOR_SRC="$ROOT/scripts/detect-docs-drift.sh"
MAP_SRC="$ROOT/docs/docs-drift-map.yml"
ZERO_SHA="0000000000000000000000000000000000000000"

SCENARIO_TMP=""
SCENARIO_STDERR=""
SCENARIO_STATUS=0
SCENARIO_PATH=""
SCENARIO_REMOTE_NAME="origin"

fail() {
  printf '%s\n' "$1"
  return 1
}

git_commit() {
  git -c user.email=test@test -c user.name=test commit -q -m "$1"
}

seed_repo() {
  local repo
  repo="$(mktemp -d)"
  git -C "$repo" init -q --initial-branch=main

  mkdir -p "$repo/docs" "$repo/scripts" "$repo/scripts/git-hooks" \
    "$repo/cmd" "$repo/web/docs/docs/cli-reference" "$repo/test-bin"
  cp "$MAP_SRC" "$repo/docs/docs-drift-map.yml"
  cp "$DETECTOR_SRC" "$repo/scripts/detect-docs-drift.sh"
  chmod +x "$repo/scripts/detect-docs-drift.sh"
  write_yq_adapter "$repo/test-bin/yq"

  if [ ! -f "$HOOK_SRC" ]; then
    fail "missing scripts/git-hooks/pre-push; Task 1 must land before behavioral scenarios can run"
    return 1
  fi
  cp "$HOOK_SRC" "$repo/scripts/git-hooks/pre-push"
  chmod +x "$repo/scripts/git-hooks/pre-push"
  cp "$INSTALL_SRC" "$repo/scripts/git-hooks/install.sh"
  chmod +x "$repo/scripts/git-hooks/install.sh"

  printf 'package main\n' >"$repo/cmd/task.go"
  printf '# task docs\n' >"$repo/web/docs/docs/cli-reference/task.mdx"
  printf '# index docs\n' >"$repo/web/docs/docs/cli-reference/index.mdx"
  git -C "$repo" add docs scripts cmd web
  git -C "$repo" -c user.email=test@test -c user.name=test commit -q -m "initial"
  printf '%s\n' "$repo"
}

write_yq_adapter() {
  local dest="$1"
  cat >"$dest" <<'YQ'
#!/usr/bin/env bash
set -euo pipefail

if [ "${1:-}" = "e" ] && [ "${2:-}" = "-o=json" ] && [ "${3:-}" = "-I=0" ] && [ "${4:-}" = ".[]" ]; then
  python3 - "$5" <<'PY'
import ast
import json
import sys

data = []
current = None
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    for raw in fh:
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("- "):
            if current is not None:
                data.append(current)
            current = {}
            line = line[2:].strip()
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        key = key.strip()
        value = value.strip()
        if current is None:
            current = {}
        current[key] = ast.literal_eval(value) if value else []
if current is not None:
    data.append(current)
for entry in data:
    print(json.dumps(entry, separators=(",", ":")))
PY
  exit 0
fi

if [ "${1:-}" = "-r" ]; then
  jq -r "$2"
  exit 0
fi

echo "unsupported test yq invocation: $*" >&2
exit 2
YQ
  chmod +x "$dest"
}

run_hook() {
  local repo="$1"
  local stdin_line="$2"
  shift 2
  local hook_path="${SCENARIO_PATH:-$repo/test-bin:$PATH}"
  local remote_name="${SCENARIO_REMOTE_NAME:-origin}"

  SCENARIO_STDERR="$repo/stderr.txt"
  SCENARIO_STATUS=0
  (
    cd "$repo"
    "$@" PATH="$hook_path" bash scripts/git-hooks/pre-push "$remote_name" "git@example.test:$remote_name/repo.git" >/dev/null 2>"$SCENARIO_STDERR" <<<"$stdin_line"
  ) || SCENARIO_STATUS=$?
}

assert_status() {
  local expected="$1"
  if [ "$SCENARIO_STATUS" -ne "$expected" ]; then
    fail "expected exit $expected, got $SCENARIO_STATUS; stderr: $(tr '\n' ' ' <"$SCENARIO_STDERR")"
    return 1
  fi
}

assert_stderr_contains() {
  local expected="$1"
  local stderr
  stderr="$(cat "$SCENARIO_STDERR")"
  if [[ "$stderr" != *"$expected"* ]]; then
    fail "stderr missing '$expected'; got: $(tr '\n' ' ' <"$SCENARIO_STDERR")"
    return 1
  fi
}

assert_stderr_empty() {
  if [ -s "$SCENARIO_STDERR" ]; then
    fail "expected empty stderr, got: $(tr '\n' ' ' <"$SCENARIO_STDERR")"
    return 1
  fi
}

make_drift_commit() {
  local repo="$1"
  printf '\nfunc changed() {}\n' >>"$repo/cmd/task.go"
  git -C "$repo" add cmd/task.go
  git -C "$repo" -c user.email=test@test -c user.name=test commit -q -m "${2:-change task cli}"
}

make_docs_commit() {
  local repo="$1"
  printf '\nfunc changed() {}\n' >>"$repo/cmd/task.go"
  printf '\nupdated task docs\n' >>"$repo/web/docs/docs/cli-reference/task.mdx"
  git -C "$repo" add cmd/task.go web/docs/docs/cli-reference/task.mdx
  git -C "$repo" -c user.email=test@test -c user.name=test commit -q -m "change task cli and docs"
}

with_feature_branch() {
  local repo="$1"
  git -C "$repo" switch -q -c feature
}

scenario_drift_detected() {
  local repo remote local_sha
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  with_feature_branch "$repo"
  remote="$(git -C "$repo" rev-parse HEAD)"
  make_drift_commit "$repo"
  local_sha="$(git -C "$repo" rev-parse HEAD)"
  run_hook "$repo" "refs/heads/feature $local_sha refs/heads/feature $remote" env
  assert_status 1 || return 1
  assert_stderr_contains "web/docs/docs/cli-reference/task.mdx"
}

scenario_drift_absent() {
  local repo remote local_sha
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  with_feature_branch "$repo"
  remote="$(git -C "$repo" rev-parse HEAD)"
  make_docs_commit "$repo"
  local_sha="$(git -C "$repo" rev-parse HEAD)"
  run_hook "$repo" "refs/heads/feature $local_sha refs/heads/feature $remote" env
  assert_status 0 || return 1
  assert_stderr_empty
}

scenario_deletion_ref() {
  local repo remote
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  with_feature_branch "$repo"
  remote="$(git -C "$repo" rev-parse HEAD)"
  run_hook "$repo" "refs/heads/feature $ZERO_SHA refs/heads/feature $remote" env
  assert_status 0 || return 1
  assert_stderr_empty
}

scenario_tag_push() {
  local repo remote local_sha
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  remote="$(git -C "$repo" rev-parse HEAD)"
  git -C "$repo" tag v2.9.0
  local_sha="$(git -C "$repo" rev-parse v2.9.0)"
  run_hook "$repo" "refs/tags/v2.9.0 $local_sha refs/tags/v2.9.0 $remote" env
  assert_status 0 || return 1
  assert_stderr_empty
}

scenario_new_branch() {
  local repo base local_sha
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  base="$(git -C "$repo" rev-parse HEAD)"
  git -C "$repo" update-ref refs/remotes/origin/main "$base"
  with_feature_branch "$repo"
  make_drift_commit "$repo"
  local_sha="$(git -C "$repo" rev-parse HEAD)"
  run_hook "$repo" "refs/heads/feature $local_sha refs/heads/feature $ZERO_SHA" env
  assert_status 1 || return 1
  assert_stderr_contains "web/docs/docs/cli-reference/task.mdx"
}

scenario_new_branch_uses_push_remote() {
  local repo base local_sha
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  base="$(git -C "$repo" rev-parse HEAD)"
  git -C "$repo" update-ref refs/remotes/upstream/main "$base"
  SCENARIO_REMOTE_NAME="upstream"
  with_feature_branch "$repo"
  make_drift_commit "$repo"
  local_sha="$(git -C "$repo" rev-parse HEAD)"
  run_hook "$repo" "refs/heads/feature $local_sha refs/heads/feature $ZERO_SHA" env
  assert_status 1 || return 1
  assert_stderr_contains "web/docs/docs/cli-reference/task.mdx"
}

scenario_new_branch_fetches_missing_remote_base() {
  local repo remote_repo local_sha
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  remote_repo="$repo/remote.git"
  git init --bare -q --initial-branch=main "$remote_repo"
  git -C "$repo" remote add upstream "$remote_repo"
  git -C "$repo" push -q upstream main
  SCENARIO_REMOTE_NAME="upstream"
  with_feature_branch "$repo"
  make_drift_commit "$repo"
  local_sha="$(git -C "$repo" rev-parse HEAD)"
  run_hook "$repo" "refs/heads/feature $local_sha refs/heads/feature $ZERO_SHA" env
  assert_status 1 || return 1
  assert_stderr_contains "web/docs/docs/cli-reference/task.mdx"
}

scenario_new_branch_fails_when_base_unavailable() {
  local repo local_sha
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  SCENARIO_REMOTE_NAME="missing-remote"
  with_feature_branch "$repo"
  make_drift_commit "$repo"
  local_sha="$(git -C "$repo" rev-parse HEAD)"
  run_hook "$repo" "refs/heads/feature $local_sha refs/heads/feature $ZERO_SHA" env
  assert_status 1 || return 1
  assert_stderr_contains "refusing to skip docs drift check"
}

scenario_non_head_push_ref() {
  local repo remote feature_sha
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  remote="$(git -C "$repo" rev-parse HEAD)"
  with_feature_branch "$repo"
  make_drift_commit "$repo"
  feature_sha="$(git -C "$repo" rev-parse HEAD)"
  git -C "$repo" switch -q main
  run_hook "$repo" "refs/heads/feature $feature_sha refs/heads/feature $remote" env
  assert_status 1 || return 1
  assert_stderr_contains "web/docs/docs/cli-reference/task.mdx"
}

scenario_bypass_env() {
  local repo remote local_sha
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  with_feature_branch "$repo"
  remote="$(git -C "$repo" rev-parse HEAD)"
  make_drift_commit "$repo"
  local_sha="$(git -C "$repo" rev-parse HEAD)"
  run_hook "$repo" "refs/heads/feature $local_sha refs/heads/feature $remote" env KASMOS_SKIP_DOCS_DRIFT=1
  assert_status 0 || return 1
  assert_stderr_contains "bypassed via KASMOS_SKIP_DOCS_DRIFT"
}

scenario_bypass_trailer() {
  local repo remote local_sha
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  with_feature_branch "$repo"
  remote="$(git -C "$repo" rev-parse HEAD)"
  make_drift_commit "$repo" $'change task cli\n\nDocs-Drift-Skip: ticket-123'
  local_sha="$(git -C "$repo" rev-parse HEAD)"
  run_hook "$repo" "refs/heads/feature $local_sha refs/heads/feature $remote" env
  assert_status 0 || return 1
  assert_stderr_contains "bypassed via trailer" || return 1
  assert_stderr_contains "ticket-123"
}

scenario_missing_yq() {
  local repo remote local_sha mask_path
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  with_feature_branch "$repo"
  remote="$(git -C "$repo" rev-parse HEAD)"
  make_drift_commit "$repo"
  local_sha="$(git -C "$repo" rev-parse HEAD)"

  mask_path="$repo/path-without-yq"
  mkdir -p "$mask_path"
  # Keep the hook runnable with a narrow PATH, but intentionally do not provide yq.
  ln -s "$(command -v bash)" "$mask_path/bash"
  ln -s "$(command -v git)" "$mask_path/git"
  ln -s "$(command -v jq)" "$mask_path/jq"
  ln -s "$(command -v env)" "$mask_path/env"
  ln -s "$(command -v dirname)" "$mask_path/dirname"
  ln -s "$(command -v mktemp)" "$mask_path/mktemp"
  ln -s "$(command -v rm)" "$mask_path/rm"
  ln -s "$(command -v tr)" "$mask_path/tr"
  ln -s "$(command -v grep)" "$mask_path/grep"
  ln -s "$(command -v head)" "$mask_path/head"

  SCENARIO_PATH="$mask_path"
  run_hook "$repo" "refs/heads/feature $local_sha refs/heads/feature $remote" env
  assert_status 1 || return 1
  assert_stderr_contains "yq and jq required"
}

scenario_install_from_subdirectory() {
  local repo
  repo="$(seed_repo)" || return 1
  SCENARIO_TMP="$repo"
  mkdir -p "$repo/nested/dir"
  (
    cd "$repo/nested/dir"
    bash "$repo/scripts/git-hooks/install.sh" >/dev/null
  )
  if [ "$(git -C "$repo" config --get core.hooksPath)" != "scripts/git-hooks" ]; then
    fail "installer did not configure core.hooksPath"
    return 1
  fi
  if [ ! -x "$repo/scripts/git-hooks/pre-push" ]; then
    fail "installer did not chmod repo-root pre-push hook"
    return 1
  fi
}

run_scenario() {
  local name="$1"
  local fn="$2"
  SCENARIO_TMP=""
  SCENARIO_STDERR=""
  SCENARIO_STATUS=0
  SCENARIO_PATH=""
  SCENARIO_REMOTE_NAME="origin"

  local message
  if message="$($fn 2>&1)"; then
    printf 'PASS %s\n' "$name"
    [ -z "$SCENARIO_TMP" ] || rm -rf "$SCENARIO_TMP"
    return 0
  fi

  printf 'FAIL %s %s\n' "$name" "$message"
  [ -z "$SCENARIO_TMP" ] || rm -rf "$SCENARIO_TMP"
  return 1
}

main() {
  local passed=0
  local total=13

  local scenarios=(
    "drift_detected:scenario_drift_detected"
    "drift_absent:scenario_drift_absent"
    "deletion_ref:scenario_deletion_ref"
    "tag_push:scenario_tag_push"
    "new_branch:scenario_new_branch"
    "new_branch_uses_push_remote:scenario_new_branch_uses_push_remote"
    "new_branch_fetches_missing_remote_base:scenario_new_branch_fetches_missing_remote_base"
    "new_branch_fails_when_base_unavailable:scenario_new_branch_fails_when_base_unavailable"
    "non_head_push_ref:scenario_non_head_push_ref"
    "bypass_env:scenario_bypass_env"
    "bypass_trailer:scenario_bypass_trailer"
    "missing_yq:scenario_missing_yq"
    "install_from_subdirectory:scenario_install_from_subdirectory"
  )

  for scenario in "${scenarios[@]}"; do
    local name="${scenario%%:*}"
    local fn="${scenario#*:}"
    if run_scenario "$name" "$fn"; then
      passed=$((passed + 1))
      continue
    fi
    printf 'RESULT: %d/%d passed\n' "$passed" "$total"
    exit 1
  done

  printf 'RESULT: %d/%d passed\n' "$passed" "$total"
}

main "$@"
