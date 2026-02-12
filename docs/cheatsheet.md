# kasmos Cheatsheet

> For the full end-to-end workflow (spec-kitty planning → kasmos execution → merge), see [Workflow Cheatsheet](./workflow-cheatsheet.md).

## Quick Reference

### CLI Commands

Launch a new orchestration:
```bash
kasmos start <feature_dir> [--mode continuous|wave-gated]
```

Show status:
```bash
kasmos status [feature_dir]
```

Attach to existing session:
```bash
kasmos attach <feature_dir>
```

Stop orchestration:
```bash
kasmos stop [feature_dir]
```

Send controller commands (works outside the feature dir with `--feature`):
```bash
kasmos cmd status
kasmos cmd focus WP02
kasmos cmd --feature 004 advance
```

---

## FIFO Commands

Preferred:

```bash
kasmos cmd <command> [WP_ID] [--feature <feature_dir_or_prefix>]
```

Write commands to `.kasmos/cmd.pipe`:

```bash
echo "<command>" > .kasmos/cmd.pipe
```

| Command | Effect |
|---------|--------|
| `status` | Display orchestration state table |
| `restart <WP_ID>` | Restart failed/crashed work package |
| `pause <WP_ID>` | Pause a running work package |
| `resume <WP_ID>` | Resume a paused work package |
| `focus <WP_ID>` | Navigate to work package pane |
| `zoom <WP_ID>` | Focus and zoom pane to full view |
| `abort` | Gracefully shutdown |
| `advance` | Confirm wave advancement (wave-gated only) |
| `force-advance <WP_ID>` | Skip failed WP, unblock dependents |
| `retry <WP_ID>` | Re-run a failed work package |
| `help` | Show command help |

---

## Zellij Keybinds (Inside Session)

### Global Mode Keys

| Action | Keys |
|--------|------|
| Enter pane mode | `Ctrl+p` |
| Enter tab mode | `Ctrl+t` |
| Enter move mode | `Ctrl+h` |
| Enter resize mode | `Ctrl+n` |
| Enter scroll mode | `Ctrl+s` |
| Enter session mode | `Ctrl+o` |
| Quit Zellij | `Ctrl+q` |

For other mode details, check your Zellij help or documentation.

### Pane Mode (after `Ctrl+p`)

| Action | Keys |
|--------|------|
| New pane | `n` |
| Close pane | `d` |
| Toggle fullscreen | `f` |
| Toggle floating pane | `w` |
| Move focus left | `h` |
| Move focus down | `j` |
| Move focus up | `k` |
| Move focus right | `l` |
| Toggle focus between panes | `Tab` |

### Session Mode (after `Ctrl+o`)

| Action | Keys |
|--------|------|
| Detach from session | `d` |
| Open session manager | `w` |

---

## Configuration

### Environment Variables

Set defaults before launching:

```bash
# Max concurrent agent panes (1-16, default: 8)
export KASMOS_MAX_PANES=12

# Progression mode (default: continuous)
export KASMOS_MODE=wave_gated

# Paths to binaries
export KASMOS_ZELLIJ=/usr/bin/zellij
export KASMOS_OPENCODE=/usr/bin/opencode
export KASMOS_SPEC_KITTY=/usr/bin/spec-kitty

# State directory (default: .kasmos)
export KASMOS_DIR=.kasmos

# Poll interval for crash detection (seconds, default: 5)
export KASMOS_POLL_INTERVAL=10

# Completion detection debounce (ms, default: 200)
export KASMOS_DEBOUNCE=500

# Controller pane width as % (10-90, default: 40)
export KASMOS_CONTROLLER_WIDTH=35
```

### Config File

Create `.kasmos/config.toml` in the feature directory:

```toml
max_agent_panes = 8
progression_mode = "continuous"
zellij_binary = "zellij"
opencode_binary = "opencode"
spec_kitty_binary = "spec-kitty"
kasmos_dir = ".kasmos"
poll_interval_secs = 5
debounce_ms = 200
controller_width_pct = 40
```

---

## Common Tasks

### Check Status from Outside Session

```bash
kasmos status ./features/my-feature
```

### Force-Restart a Failed Agent

```bash
# Restart (resume from checkpoint if available)
echo "restart WP01" > .kasmos/cmd.pipe

# Or retry (clean slate)
echo "retry WP01" > .kasmos/cmd.pipe
```

### Unblock Wave Dependencies

If a work package fails and you want to skip it:

```bash
echo "force-advance WP01" > .kasmos/cmd.pipe
# WP02, WP03 (which depend on WP01) now unblock
```

### Pause for Review

In continuous mode, pause a running WP:

```bash
echo "pause WP02" > .kasmos/cmd.pipe
# Work on another agent, then:
echo "resume WP02" > .kasmos/cmd.pipe
```

### View One Agent Closely

```bash
# Fullscreen a pane
Ctrl+p → f

# Interact with opencode, then press Ctrl+p → f again to restore grid
```

### Graceful Shutdown

```bash
echo "abort" > .kasmos/cmd.pipe
# Saves state to .kasmos/state.json
# Allows resume/attach later
```

---

## Directory Structure

After `kasmos launch`:

```
<feature_dir>/
├── .kasmos/
│   ├── cmd.pipe              # FIFO for command input
│   ├── layout.kdl            # Generated Zellij layout
│   ├── state.json            # Persistent orchestration state
│   ├── prompts/
│   │   ├── WP01/prompt.md
│   │   ├── WP02/prompt.md
│   │   └── ...
│   └── logs/                 # Optional log files
├── spec.md                   # Feature specification
└── ...
```

---

## Troubleshooting Quick Fixes

| Problem | Solution |
|---------|----------|
| Agent pane stuck | `echo "focus WP01" > .kasmos/cmd.pipe` |
| FIFO permission denied | `chmod 700 .kasmos` |
| Zellij crashes | `kasmos attach <dir>` to reconnect |
| State seems stale | `kasmos status <dir>` for live status |
| Can't launch (binary not found) | Check `zellij`, `opencode` in `PATH` |
| Wave-gated stuck waiting | `echo "advance" > .kasmos/cmd.pipe` |

---

## Layout Grid Rules

For N agents, the grid is arranged as:

- **Columns** = ceil(sqrt(N))
- **Rows** = ceil(N / columns)

Examples:
- 1 agent: 1×1
- 4 agents: 2×2
- 5 agents: 2×3 (one empty slot)
- 8 agents: 3×3 (one empty slot)
- 9 agents: 3×3

---

## Default Config Values

```
max_agent_panes:        8
progression_mode:       continuous
zellij_binary:          zellij
opencode_binary:        opencode
spec_kitty_binary:      spec-kitty
kasmos_dir:             .kasmos
poll_interval_secs:     5
debounce_ms:            200
controller_width_pct:   40
```

---

## State Machine Quick View

### Work Package States
- **Pending** → **Active** (launch)
- **Active** → **Completed** (success)
- **Active** → **Failed** (crash/error)
- **Active** → **Paused** (pause command)
- **Paused** → **Active** (resume)
- **Failed** → **Pending** (retry)
- **Failed** → **Active** (restart)

### Run States
- **Initializing** → **Running** → **Completed**
- **Running** → **Paused** (wave-gated boundary)
- **Running** → **Aborted** (abort command)
- **Running** → **Failed** (unrecoverable error)

---

## Known Limitations

- **launch uses NoOpChecker for health monitor** — Cannot list live panes (Zellij 0.41+ removed `list-panes`); pane count must be pre-configured via `max_agent_panes`. Crash detection relies on external signals (pane exit, spec-kitty completion, git status).

- **launch wires NoOpSessionCtrl for focus/zoom** — Commands `focus <WP_ID>` and `zoom <WP_ID>` are accepted but do not actually control Zellij pane focus. Use Zellij keybinds (`Ctrl+p → h/j/k/l` or `Tab`) to navigate manually.

- **Zellij 0.41+ API gaps** — Missing `list-panes`, `focus-terminal-pane --pane-id`, `write-chars-to-pane-id`. kasmos works around these with internal tracking and manual navigation.
