# AGENTS.md

## Startup checklist
- Read `README.md` for project overview.
- Read `.kittify/memory/` for project constitution, architecture knowledge, and workflow intelligence.
- Check `kitty-specs/` for feature specifications.
- This is a Go module. Source lives at the repository root.
- Primary binary: `cmd/kasmos/` - the TUI agent orchestrator.

## Repository layout
- `cmd/kasmos/`: Entry point (main.go)
- `internal/tui/`: bubbletea TUI (model, update, view, styles, keys)
- `internal/worker/`: Worker backend interface, subprocess backend, output buffer
- `internal/task/`: Task source adapters (spec-kitty, GSD, ad-hoc)
- `internal/persist/`: Session persistence (JSON)
- `internal/setup/`: `kasmos setup` subcommand (agent scaffolding, dep validation)
- `kitty-specs/`: Feature specifications (spec-kitty)
- `design-artifacts/`: TUI visual design (mockups, layout, styles, keybinds)
- `.kittify/memory/`: Persistent project memory (constitution, architecture, workflow learnings)
- `.kittify/`: spec-kitty project configuration, scripts, missions

## Build / run commands
- Build: `go build ./cmd/kasmos`
- Run: `go run ./cmd/kasmos`
- Test: `go test ./...`
- Test (integration): `KASMOS_INTEGRATION=1 go test ./...`
- Lint: `golangci-lint run`

## Code style (Go)
- Follow standard Go conventions (gofmt, go vet)
- Use `internal/` for non-exported packages
- Prefer explicit error handling with `fmt.Errorf` wrapping
- Use table-driven tests
- Follow standard Go naming: camelCase unexported, PascalCase exported
- Keep packages small and focused

## External tools
- `opencode`: AI coding agent harness (workers spawned via `opencode run`)
- `spec-kitty`: Feature specification tool
- `git`: Version control

## Agent harness: OpenCode only

kasmos uses **OpenCode** as the sole agent harness for spawning worker agents. This is a hard rule:

- Workers are spawned via `opencode run --agent <role> "prompt"`.
- **Never invoke a model-specific CLI** (e.g., `claude`, `gemini`, `aider`) directly. OpenCode is the abstraction layer.
- kasmos is **model-agnostic**. The model running behind OpenCode is configured in OpenCode's own config, not in kasmos.
- Session continuation uses `opencode run --continue -s <session_id> "follow-up"`.

## Persistent memory

When you discover something significant about the codebase architecture, runtime behavior, or integration quirks, record it in `.kittify/memory/`.

- `constitution.md`: Project technical standards and governance (do not modify without discussion).
- `architecture.md`: Codebase structure, type locations, subsystem interactions, known issues.
- `workflow-intelligence.md`: Lessons from the spec-kitty planning lifecycle.

## Automatic skill loading

When your prompt, file paths, or content you read contains certain keywords, automatically load the corresponding skill before proceeding. Use the Skill loading tool if available, otherwise read the skill file directly.

| Keywords | Skill | Skill file |
|----------|-------|------------|
| `kitty`, `kittify`, `spec-kitty`, `kitty-specs` | spec-kitty | `.opencode/skills/spec-kitty/SKILL.md` |
| `tmux`, `pane` | tmux-orchestration | `.opencode/skills/tmux-orchestration/SKILL.md` |
| `tui`, `worker`, `app`, `launch`, `settings`, `keybind` | TUI Design | `.opencode/skills/tui-design/SKILL.md` |

Rules:
- Match keywords case-insensitively in the user's prompt, file paths being read/edited, or content encountered during the task.
- Load the skill once at the start of the task; do not reload on every keyword match.
- If multiple skills match, load all matching skills.
- Skill loading is additive to any skill instructions already present in command files.
