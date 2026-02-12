# CLI Contract: kasmos tui

**Date**: 2026-02-12
**Introduced in**: Feature 009-standalone-tui-preview

## Command Signature

```
kasmos tui [--count <N>]
```

## Arguments

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `--count` | `usize` | No | `12` | `clap::value_parser!(usize).range(1..)` — must be ≥ 1 | Clap rejects with: "error: invalid value '0' for '--count <COUNT>': 0 is not in 1..=18446744073709551615" |

## Behavior Contract

| Condition | Expected Behavior |
|-----------|------------------|
| `kasmos tui` (no args) | Launches TUI with 12 mock WPs, animated |
| `kasmos tui --count 3` | Launches TUI with 3 mock WPs, animated |
| `kasmos tui --count 25` | Launches TUI with 25 mock WPs, animated |
| `kasmos tui --count 0` | Clap error, non-zero exit code |
| `kasmos tui --count abc` | Clap error: "invalid value 'abc'" |
| `kasmos tui --count` (missing value) | Clap error: "a value is required for '--count <COUNT>'" |
| Press `q` during TUI | Clean exit, terminal restored |
| Terminal resize during TUI | TUI redraws correctly (handled by existing `tui::run`) |

## External Dependencies

**None.** This command does NOT require:
- Zellij binary or session
- Git repository or worktrees
- `kitty-specs/` directory
- Running orchestration or `.kasmos/` state directory
- Network access

## Channel Contracts

| Channel | Direction | Type | Behavior in Preview |
|---------|-----------|------|-------------------|
| `watch::Receiver<OrchestrationRun>` | Engine → TUI | State updates | Fed by `animation_loop` background task |
| `mpsc::Sender<EngineAction>` | TUI → Engine | User commands | Receiver dropped; `try_send` returns `Err`, silently ignored by TUI |

## Versioning

- **Introduced**: v0.2 (feature 009)
- **Breaking change policy**: Removing `kasmos tui` or renaming `--count` is a breaking change requiring a major version bump per the CLI Contract in `kitty-specs/009-standalone-tui-preview/spec.md`.
