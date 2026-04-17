#!/bin/bash
# PreToolUse hook: block legacy CLI tools, enforce modern replacements.
# Installed by kasmos setup. Source of truth: cli-tools skill.
# Reads Bash tool_input.command from stdin JSON and rejects banned commands.

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

[ -z "$COMMAND" ] && exit 0

PYTHON_BIN=""
if command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="python3"
elif command -v python >/dev/null 2>&1 && python -c 'import sys; raise SystemExit(0 if sys.version_info[0] >= 3 else 1)' >/dev/null 2>&1; then
  PYTHON_BIN="python"
fi

SCAN_COMMAND="$COMMAND"
if [ -n "$PYTHON_BIN" ]; then
  if SANITIZED=$(COMMAND="$COMMAND" "$PYTHON_BIN" <<'PY'
import os
import re
import shlex

OPS = {"|", "||", "&", "&&", ";"}
SSH_VALUE_OPTIONS = {
    "-b", "-c", "-D", "-E", "-e", "-F", "-I", "-i", "-J", "-L", "-l",
    "-m", "-O", "-o", "-p", "-Q", "-R", "-S", "-W", "-w",
}
ENV_VALUE_OPTIONS = {"-u"}


def is_assignment(token):
    return re.match(r"^[A-Za-z_][A-Za-z0-9_]*=.*$", token) is not None


def is_ssh(token):
    return token.rsplit("/", 1)[-1] == "ssh"


def tokenize(command):
    lexer = shlex.shlex(command, posix=True, punctuation_chars="|&;")
    lexer.whitespace_split = True
    lexer.commenters = ""
    return list(lexer)


def command_start(tokens):
    index = 0
    while index < len(tokens) and is_assignment(tokens[index]):
        index += 1

    if index < len(tokens) and tokens[index] == "env":
        index += 1
        while index < len(tokens):
            token = tokens[index]
            if token == "--":
                index += 1
                break
            if is_assignment(token):
                index += 1
                continue
            if token in ENV_VALUE_OPTIONS:
                index += 2
                continue
            if token.startswith("-"):
                index += 1
                continue
            break

    return index


def has_local_shell_expansion(tokens):
    for token in tokens:
        if "$(" in token or chr(96) in token:
            return True
    return False


def strip_ssh_remote(tokens):
    start = command_start(tokens)
    if start >= len(tokens) or not is_ssh(tokens[start]):
        return tokens

    index = start + 1
    target_seen = False
    while index < len(tokens):
        token = tokens[index]
        if token in OPS:
            return tokens

        if not target_seen:
            if token == "--":
                index += 1
                continue
            if token in SSH_VALUE_OPTIONS:
                index += 2
                continue
            if token.startswith("-"):
                index += 1
                continue
            target_seen = True
            index += 1
            continue

        remote_end = index
        while remote_end < len(tokens) and tokens[remote_end] not in OPS:
            remote_end += 1

        if has_local_shell_expansion(tokens[index:remote_end]):
            return tokens

        return tokens[:index] + tokens[remote_end:]

    return tokens


command = os.environ.get("COMMAND", "")
try:
    tokens = tokenize(command)
except ValueError:
    print(command, end="")
else:
    print(" ".join(strip_ssh_remote(tokens)), end="")
PY
  ); then
    SCAN_COMMAND="$SANITIZED"
  fi
fi

[ -z "$SCAN_COMMAND" ] && exit 0

# grep -> rg (ripgrep)
# Word-boundary match avoids false positives (e.g. ast-grep). ssh remote argv
# is stripped above so quoted remote pipelines do not look like local grep use.
if echo "$SCAN_COMMAND" | grep -qP '(^|[|;&\x60]\s*|\$\(\s*)\bgrep\b'; then
  echo "BLOCKED: 'grep' is banned. Use 'rg' (ripgrep) instead. rg is faster, respects .gitignore, and has better defaults." >&2
  exit 2
fi

# sed -> sd or comby
if echo "$SCAN_COMMAND" | grep -qP '(^|[|;&\x60]\s*|\$\(\s*)\bsed\b'; then
  echo "BLOCKED: 'sed' is banned. Use 'sd' for simple replacements or 'comby' for structural/multi-line rewrites." >&2
  exit 2
fi

# awk -> yq/jq, sd, or comby
if echo "$SCAN_COMMAND" | grep -qP '(^|[|;&\x60]\s*|\$\(\s*)\bawk\b'; then
  echo "BLOCKED: 'awk' is banned. Use 'yq'/'jq' for structured data, 'sd' for text, or 'comby' for code patterns." >&2
  exit 2
fi

# standalone diff (not git diff) -> difft
if echo "$SCAN_COMMAND" | grep -qP '(^|[|;&\x60]\s*|\$\(\s*)\bdiff\b' && \
   ! echo "$SCAN_COMMAND" | grep -qP '\bgit\s+diff\b'; then
  echo "BLOCKED: standalone 'diff' is banned. Use 'difft' (difftastic) for syntax-aware structural diffs. 'git diff' is allowed." >&2
  exit 2
fi

# wc -l -> scc
if echo "$SCAN_COMMAND" | grep -qP '\bwc\s+(-\w*l|--lines)\b|\bwc\b.*\s-l\b'; then
  echo "BLOCKED: 'wc -l' is banned. Use 'scc' for language-aware line counts with complexity estimates." >&2
  exit 2
fi

# pip --break-system-packages -> use venv or uv
if echo "$SCAN_COMMAND" | grep -qP '\bpip3?\b.*--break-system-packages'; then
  echo "BLOCKED: 'pip --break-system-packages' is banned. Use a virtual environment (python -m venv) or uv instead." >&2
  exit 2
fi

exit 0
