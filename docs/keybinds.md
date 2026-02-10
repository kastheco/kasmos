# Zellij Keybinds for kasmos

This reference covers Zellij keybinds relevant to kasmos operations. Zellij uses **mode-based keybinds**: press a prefix key to enter a mode, then use action keys.

## Global Mode Keys

These work anytime inside a Zellij session:

| Key | Action |
|-----|--------|
| `Ctrl+t` | Enter tab mode |
| `Ctrl+p` | Enter pane mode |
| `Ctrl+h` | Enter move mode |
| `Ctrl+n` | Enter resize mode |
| `Ctrl+s` | Enter scroll mode |
| `Ctrl+o` | Enter session mode |
| `Ctrl+q` | Quit Zellij immediately |

For additional mode details, consult your Zellij configuration or help.

## Pane Mode (`Ctrl+p`)

Navigate and control panes:

| Key | Action |
|-----|--------|
| `n` | New pane |
| `d` | Close focused pane |
| `f` | Toggle fullscreen |
| `w` | Toggle floating pane |
| `h` | Move focus left |
| `j` | Move focus down |
| `k` | Move focus up |
| `l` | Move focus right |
| `Tab` | Toggle focus between panes |

**Example:** Press `Ctrl+p`, then `h` to move focus left.

## Session Mode (`Ctrl+o`)

Manage Zellij session:

| Key | Action |
|-----|--------|
| `d` | Detach from session |
| `w` | Open session manager |

**Example:** Press `Ctrl+o`, then `d` to detach (session continues running).

## Additional Modes

Zellij provides additional modes for tab management, resizing, moving, and scrolling operations. Check your Zellij help for the full keybind map:

```bash
zellij help
```

Or consult the Zellij documentation for details on Tab Mode (`Ctrl+t`), scroll operations, and pane reorganization.

## Floating Panes

Create temporary overlay windows over the main layout:

**To create a floating pane:**
- Enter pane mode: `Ctrl+p`
- Press `w` to toggle floating pane

**To close a floating pane:**
- Focus it: `Ctrl+p` → `w` (cycle)
- Close it: `Ctrl+p` → `d`

### Floating Pane Use Cases for kasmos

**Quick log viewer overlay:**
```bash
# Create floating pane
Ctrl+p → w

# In the floating pane:
tail -f .kasmos/state.json | jq '.work_packages[] | {id, state}'

# Overlay shows live state updates over the main grid
# Press Ctrl+p → w to cycle back to main layout
```

**Help reference window:**
```bash
# Create floating pane
Ctrl+p → w

# In the floating pane:
echo "status" > .kasmos/cmd.pipe
# Shows current orchestration state without affecting main view
```

## Common Workflows

### Monitor Multiple Agents

1. **Initial layout** — Grid of agents visible
2. **Navigate** — `Ctrl+p` → `h/j/k/l` to move focus
3. **Fullscreen** — `Ctrl+p` → `f` to maximize current pane
4. **Return** — `Ctrl+p` → `f` again to restore grid
5. **Next agent** — `Ctrl+p` → `Tab` to switch panes

### Switch Between Tabs

Use `Ctrl+t` to enter tab mode. Check your Zellij help for tab navigation and creation commands.

### Scroll Through Output

Use `Ctrl+s` to enter scroll mode. Refer to Zellij documentation for scroll keys and search operations.

### Detach and Reconnect

1. **Detach** — `Ctrl+o` → `d` (leaves session running)
2. **From another terminal** — `kasmos attach <feature_dir>`
3. **Reconnect** — You're back in the session

## Integration with kasmos

### FIFO Command Dispatch

Use `.kasmos/cmd.pipe` to send commands from any terminal (not just keybinds):

```bash
# From another terminal:
echo "focus WP01" > .kasmos/cmd.pipe
echo "zoom WP02" > .kasmos/cmd.pipe
echo "status" > .kasmos/cmd.pipe
```

### Manual Pane Navigation

The `focus` and `zoom` FIFO commands are accepted by kasmos but do not control Zellij pane focus today (due to Zellij 0.41+ API limitations). Use Zellij keybinds to navigate manually:

```bash
# Navigate to an agent pane:
Ctrl+p → h/j/k/l    # Move focus using arrow keys
Ctrl+p → f           # Fullscreen to see output clearly
Ctrl+p → f           # Return to normal grid
```

See [getting-started.md](./getting-started.md) for workflow examples and [cheatsheet.md](./cheatsheet.md) for a condensed command reference.
