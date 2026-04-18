package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CLIToolsEnforcementScript is a PreToolUse hook body shared by harnesses that
// expose Bash tool execution. Claude installs it at ~/.claude/hooks and codex
// installs it at <repo>/.codex/hooks — the script itself is harness-agnostic.
const CLIToolsEnforcementScript = `#!/bin/bash
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
`

// Claude implements Harness for the Claude Code CLI.
type Claude struct{}

func (c *Claude) Name() string { return "claude" }

func (c *Claude) Detect() (string, bool) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", false
	}
	return path, true
}

// ListModels returns the static set of Claude models.
func (c *Claude) ListModels() ([]string, error) {
	return []string{
		"claude-sonnet-4-6",
		"claude-opus-4-6",
		"claude-sonnet-4-5",
		"claude-haiku-4-5",
	}, nil
}

func (c *Claude) BuildFlags(agent AgentConfig) []string {
	var flags []string
	if agent.Model != "" {
		flags = append(flags, "--model", agent.Model)
	}
	if agent.Effort != "" {
		flags = append(flags, "--effort", agent.Effort)
	}
	flags = append(flags, agent.ExtraFlags...)
	return flags
}

func (c *Claude) InstallEnforcement() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	hooksDir := filepath.Join(home, ".claude", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create claude hooks dir: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "enforce-cli-tools.sh")
	if err := os.WriteFile(hookPath, []byte(CLIToolsEnforcementScript), 0o755); err != nil {
		return fmt.Errorf("write claude enforcement hook: %w", err)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	settingsRaw, err := os.ReadFile(settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read claude settings: %w", err)
		}
		settingsRaw = []byte(`{"hooks":{}}`)
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsRaw, &settings); err != nil {
		return fmt.Errorf("parse claude settings: %w", err)
	}

	hooksVal, ok := settings["hooks"]
	if !ok {
		hooksVal = map[string]any{}
	}

	hooks, ok := hooksVal.(map[string]any)
	if !ok {
		return fmt.Errorf("claude settings hooks has unexpected type %T", hooksVal)
	}

	preToolUseVal, ok := hooks["PreToolUse"]
	if !ok {
		preToolUseVal = []any{}
	}

	preToolUse, ok := preToolUseVal.([]any)
	if !ok {
		return fmt.Errorf("claude settings hooks.PreToolUse has unexpected type %T", preToolUseVal)
	}

	if !hasClaudeEnforcementHook(preToolUse) {
		preToolUse = append(preToolUse, map[string]any{
			"matcher": "Bash",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": hookPath,
				},
			},
		})
	}

	hooks["PreToolUse"] = preToolUse
	settings["hooks"] = hooks

	merged, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal claude settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, merged, 0o644); err != nil {
		return fmt.Errorf("write claude settings: %w", err)
	}

	return nil
}

func (c *Claude) UninstallEnforcement() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	hookPath := filepath.Join(home, ".claude", "hooks", "enforce-cli-tools.sh")
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	settingsRaw, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Nothing to clean up in settings.json; still try to remove the script.
			return removeHookScript(hookPath)
		}
		return fmt.Errorf("read claude settings: %w", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsRaw, &settings); err != nil {
		return fmt.Errorf("parse claude settings: %w", err)
	}

	hooksVal, ok := settings["hooks"]
	if ok {
		hooks, ok := hooksVal.(map[string]any)
		if !ok {
			return fmt.Errorf("claude settings hooks has unexpected type %T", hooksVal)
		}

		preToolUseVal, ok := hooks["PreToolUse"]
		if ok {
			preToolUse, ok := preToolUseVal.([]any)
			if !ok {
				return fmt.Errorf("claude settings hooks.PreToolUse has unexpected type %T", preToolUseVal)
			}

			filtered := removeClaudeEnforcementHooks(preToolUse)
			if len(filtered) == 0 {
				delete(hooks, "PreToolUse")
			} else {
				hooks["PreToolUse"] = filtered
			}
		}

		settings["hooks"] = hooks
	}

	merged, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal claude settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, merged, 0o644); err != nil {
		return fmt.Errorf("write claude settings: %w", err)
	}

	return removeHookScript(hookPath)
}

// removeHookScript deletes the managed enforcement script. A missing file is
// treated as success; any other error (e.g. permission denied) is propagated so
// callers do not report a successful uninstall while leaving the script in place.
func removeHookScript(path string) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove claude hook script: %w", err)
	}
	return nil
}

// removeClaudeEnforcementHooks returns a copy of preToolUse with all matcher
// groups that reference enforce-cli-tools.sh removed. Groups whose hooks slice
// becomes empty after filtering are dropped entirely.
func removeClaudeEnforcementHooks(preToolUse []any) []any {
	var out []any
	for _, entry := range preToolUse {
		group, ok := entry.(map[string]any)
		if !ok {
			out = append(out, entry)
			continue
		}

		hooks, ok := group["hooks"].([]any)
		if !ok {
			out = append(out, entry)
			continue
		}

		var kept []any
		for _, hook := range hooks {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				kept = append(kept, hook)
				continue
			}
			command, _ := hookMap["command"].(string)
			if !strings.Contains(command, "enforce-cli-tools.sh") {
				kept = append(kept, hook)
			}
		}

		if len(kept) == 0 {
			// Drop the entire matcher group.
			continue
		}

		// Return a shallow copy with filtered hooks.
		newGroup := make(map[string]any, len(group))
		for k, v := range group {
			newGroup[k] = v
		}
		newGroup["hooks"] = kept
		out = append(out, newGroup)
	}
	return out
}

func hasClaudeEnforcementHook(preToolUse []any) bool {
	for _, entry := range preToolUse {
		group, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		hooks, ok := group["hooks"].([]any)
		if !ok {
			continue
		}

		for _, hook := range hooks {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				continue
			}

			command, _ := hookMap["command"].(string)
			if strings.Contains(command, "enforce-cli-tools.sh") {
				return true
			}
		}
	}

	return false
}

func (c *Claude) SupportsTemperature() bool { return false }
func (c *Claude) SupportsEffort() bool      { return true }

func (c *Claude) ListEffortLevels(_ string) []string {
	return []string{"", "low", "medium", "high", "max"}
}
