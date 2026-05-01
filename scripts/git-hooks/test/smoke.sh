#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
HOOK="$ROOT/scripts/git-hooks/pre-push"
DETECTOR="$ROOT/scripts/detect-docs-drift.sh"
TMP_DIR=""

if [ ! -f "$HOOK" ]; then
  echo "missing scripts/git-hooks/pre-push; Task 1 must land before smoke can run" >&2
  exit 1
fi

head_sha="$(git -C "$ROOT" rev-parse HEAD)"
base_sha="$(git -C "$ROOT" rev-parse origin/main)"
stdin_line="$(printf 'refs/heads/HEAD %s refs/heads/main %s\n' "$head_sha" "$base_sha")"
stderr_file="$(mktemp)"
trap 'rm -f "$stderr_file"; [ -z "$TMP_DIR" ] || rm -rf "$TMP_DIR"' EXIT

ensure_detector_yq() {
  if (cd "$ROOT" && yq e -o=json -I=0 '.[]' docs/docs-drift-map.yml >/dev/null 2>&1); then
    return 0
  fi

  TMP_DIR="$(mktemp -d)"
  cat >"$TMP_DIR/yq" <<'YQ'
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
  chmod +x "$TMP_DIR/yq"
  PATH="$TMP_DIR:$PATH"
  export PATH
}

ensure_detector_yq

hook_status=0
(
  cd "$ROOT"
  bash "$HOOK" origin git@example.test:kastheco/kasmos.git >/dev/null 2>"$stderr_file" <<<"$stdin_line"
) || hook_status=$?

drift_count="$(
  cd "$ROOT"
  BASE_REF="$base_sha" TARGET_REF="$head_sha" bash "$DETECTOR" | jq '.drift | length'
)"

if [ "$drift_count" -eq 0 ]; then
  if [ "$hook_status" -ne 0 ]; then
    echo "expected hook success for zero drift entries, got exit $hook_status" >&2
    tr '\n' ' ' <"$stderr_file" >&2
    echo >&2
    exit 1
  fi
  echo "smoke passed: hook allowed push and detector reported zero drift entries"
  exit 0
fi

if [ "$hook_status" -eq 0 ]; then
  echo "expected hook failure for $drift_count drift entries, got success" >&2
  exit 1
fi

stderr_contents="$(cat "$stderr_file")"
if [[ "$stderr_contents" != *"docs-drift: push blocked"* ]]; then
  echo "expected hook stderr to contain docs-drift block message" >&2
  tr '\n' ' ' <"$stderr_file" >&2
  echo >&2
  exit 1
fi

echo "smoke passed: hook blocked push and detector reported $drift_count drift entries"
