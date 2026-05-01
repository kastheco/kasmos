#!/usr/bin/env bash
set -euo pipefail

force=0
if [ "${1:-}" = "--force" ]; then
  force=1
fi

repo_root="$(git rev-parse --show-toplevel)"
if [ ! -f "$repo_root/docs/docs-drift-map.yml" ]; then
  echo "error: scripts/git-hooks/install.sh must run inside a kasmos clone (missing docs/docs-drift-map.yml)" >&2
  exit 1
fi

is_kasmos_hooks_path() {
  local path="$1"
  local expected
  local configured

  if [ -z "$path" ]; then
    return 1
  fi

  expected="$(cd "$repo_root/scripts/git-hooks" && pwd -P)"
  case "$path" in
    /*)
      configured="$(cd "$path" 2>/dev/null && pwd -P)" || return 1
      ;;
    *)
      configured="$(cd "$repo_root/$path" 2>/dev/null && pwd -P)" || return 1
      ;;
  esac

  [ "$configured" = "$expected" ]
}

current="$(git config --get core.hooksPath || true)"
if [ -n "$current" ] && ! is_kasmos_hooks_path "$current" && [ "$force" -ne 1 ]; then
  echo "error: core.hooksPath is already set to '$current'." >&2
  echo "refusing to overwrite. re-run with --force, or unset manually:" >&2
  echo "  git config --unset core.hooksPath" >&2
  exit 1
fi

git config core.hooksPath scripts/git-hooks
chmod +x scripts/git-hooks/pre-push
git fetch origin "${KASMOS_DEFAULT_BRANCH:-main}" --quiet >/dev/null 2>&1 || true

echo "installed: core.hooksPath=scripts/git-hooks"
