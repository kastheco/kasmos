Treat work in this repository as kasmos-style agent work: correctness first, evidence first, minimal scope, and explicit verification. When reviewing pull requests, behave like the `kasmos-reviewer` skill first and the `kasmos-fixer` skill when evaluating follow-up fixes.

This repository is `kasmos`, a Go 1.24.4 TUI/CLI that orchestrates multiple AI agent sessions in isolated git worktrees and tmux sessions. The main code areas are `app/`, `cmd/`, `config/`, `daemon/`, `internal/`, `orchestration/`, `session/`, `ui/`, and `web/`.

Review and change guidance:
- Check correctness, spec compliance, and edge cases before style.
- Avoid scope creep and preserve the existing architecture. PRs from automated agents should implement ONLY their assigned task — flag files modified outside the task's stated scope.
- Cite concrete findings with `path:line` references.
- Investigate failures before proposing fixes; prefer the smallest root-cause change.
- Do not approve changes if required validation is missing or failing.
- Keep user-visible labels, toasts, confirmations, and overlay titles lowercase.
- Keep overlay navigation arrow-key friendly; do not introduce j/k-only behavior where typing or arrows are expected.
- Prefer table-driven tests with `testify`, and do not depend on real tmux, git, or network behavior in tests.
- Preserve non-blocking Bubble Tea I/O patterns: perform I/O in `tea.Cmd` goroutines and return results as messages.
- Respect shared-worktree safety around `KASMOS_TASK` and `KASMOS_PEERS`; avoid destructive git advice.

Security-critical patterns:
- Shell-escape ALL values injected into tmux/exec command strings using `shellEscapeSingleQuote` or the `'` + `strings.ReplaceAll(s, "'", "'\\''")` + `'` pattern. Unquoted `KASMOS_PROJECT=`, path, or config values in `program = "FOO=" + value + " " + program` constructs are injection vectors.
- Permission prompt detection (`claudeprompt.Find`) must only match numbered choices at the bottom of the pane — stale scrollback causes false positives. Description search is limited to 8 lines above the choices.
- Task store operations must include the `project` parameter for multi-repo daemon support.
- Signal emission must go through the gateway (MCP `signal_create`), not filesystem sentinels in `.kasmos/signals/`.
- Config reads from `<repo-root>/.kasmos/config.toml`; task store lives at `~/.config/kasmos/taskstore.db` — these are different paths, do not conflate them.

What NOT to flag:
- Do not suggest adding comments, docstrings, or type annotations to code that was not changed in the PR.
- Do not suggest `fmt.Fprintf` over `sb.WriteString(fmt.Sprintf(...))` — that is an established pattern in the prompt builders.
- Do not flag lowercase user-visible strings — intentional TUI aesthetic.
- Do not suggest model version updates (e.g. `claude-sonnet-4-20250514`) — we use shorthand aliases like `claude-sonnet-4-6`.
- Do not flag empty `model:` fields in `.claude/agents/*.md` — the harness handles the default.

Validation expectations:
- For Go changes, run `gofmt -w` on touched files, then `go test ./...`.
- Run `go build ./...` when build, packaging, startup, or command wiring changes.
- CI also runs `go test -v ./...`, `golangci-lint` fast mode, and a formatting check.
- For `web/**` changes, use Node 20 and run `cd web && npm ci && npm run build`.
- For release changes, a `v*` tag must match the `version` constant in `main.go`.

If a change touches agent prompts, skills, scaffold templates, or other instruction files, also follow the matching path-specific instructions in `.github/instructions/`.
