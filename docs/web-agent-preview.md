# web agent preview

the web admin's agent preview replaces the ANSI-in-`<pre>` terminal emulation used for tmux
instances with a structured, turn-by-turn React timeline for daemon-managed SDK instances.
the visual lineage follows the variant-c treatment from `docs/agent-sdk-pane-mockups.md`:
prose-first, tools/setup dimmed, and a clear `response` divider separating tool activity
from the assistant's final output.

## route flow

the presentation data flows through two API layers:

```
daemon control API             kas serve (browser-facing)
──────────────────             ─────────────────────────
GET /v1/repos/{project}/instances/{title}/presentation
                          →    GET /v1/projects/{project}/instances/{title}/presentation
                               POST /v1/projects/{project}/instances/{title}/permission
```

1. `daemon/api` exposes the `sdk.PresentationTurn` model as JSON on the daemon control API.
2. `cmd/livepreview_daemon.go` bridges the daemon response through the `kas serve` stack,
   translating the internal model to the wire format consumed by the SPA.
3. the SPA calls `getInstancePresentation(project, title)` and
   `sendInstancePermission(project, title, choice)` from `web/admin/src/api.ts`.

## execution-mode and daemon-managed distinction

`InstanceEntry.execution_mode` drives which preview component renders:

| execution_mode | daemon-managed | component |
|---------------|----------------|-----------|
| `sdk`         | yes            | `AgentPreview` (structured timeline) |
| `sdk`         | no             | preview unavailable message |
| `tmux`        | —              | `TerminalPreview` |

the `headless` legacy value is normalised to `sdk` at the API boundary so older rows do
not fall through to an unknown state (see `normalizeExecutionMode` in `web/admin/src/api.ts`).

daemon-managed vs standalone is detected using the instance-list metadata — specifically
the presence of daemon-provided `valid_actions` on an `sdk` row. **daemon-managed** means
the daemon owns the row, which covers two cases:

1. **plan-driven SDK agents** — wave execution agents started by the orchestrator.
2. **TUI-spawned SDK agents** when the daemon owns the repo (i.e. started via
   `POST /v1/repos/{project}/instances/solo`; the TUI sends this request and waits
   for the title to appear in `ListInstances`).

truly standalone SDK rows — legacy `state.json` records or manual/test rows not routed
through the daemon — still have no web actions because the web path has no daemon to
delegate to. they render the preview-unavailable placeholder while tmux instances continue
to use `TerminalPreview` unchanged.

## filter storage key

filter state is persisted to `localStorage` under the key:

```
kasmos.agentPreview.filters
```

the value is a JSON object with three boolean fields:

```json
{ "hideTools": false, "hideThinking": false, "hideSystem": false }
```

permission rows are **never** affected by these filters — they always remain visible.

## visual lineage from variant-c

the component hierarchy mirrors the variant-c mockup hierarchy:

```
AgentPreview
  FilterToolbar          ← thin strip, lowercase labels, persisted toggles
  scroll container
    TurnTimeline
      TurnBlock (×N)
        turn header      ← #N, elapsed, tool count, running pill, copy/collapse/anchor
        turn rows
          ResponseDivider  ← "response" label + rule separating tools from prose
          ProseMarkdown    ← react-markdown + remark-gfm, no raw HTML
          PermissionCard   ← allow / always / deny, only first unresolved is interactive
          TextRow          ← monospace dimmed rows (thinking, tool, result, system, status)
```

prose output uses `react-markdown` with `remark-gfm` so agent responses render with
proper formatting (bold, lists, fenced code, tables) rather than pre-formatted text.
raw HTML passthrough is disabled (no `allowDangerousHtml`).

## collapse and copy

each turn block has two header controls:

- **collapse / expand**: hides `thinking`, `tool`, `result`, and `system` rows for that
  turn. `response` dividers, `prose`, `permission`, and `status` rows remain visible.
  collapse state is session-local and does not persist across refresh.

- **copy**: writes a plain-text summary (turn number, elapsed time, row text) to
  `navigator.clipboard`. when the clipboard API is unavailable (e.g. non-secure context),
  an inline textarea is shown so the operator can manually select and copy the text.

- **#N anchor**: clicking the `#N` link in the header updates `location.hash` to
  `#turn-N` and scrolls the matching block into view.

## permission cards

the `PermissionCard` component replaces the plain permission text row with an inline
action card:

- **allow**: sends `choice: 0` (`allow_once`) to the daemon permission endpoint.
- **always**: sends `choice: 1` (`allow_always`).
- **deny**: sends `choice: 2` (`reject`).

while a choice is in-flight all three buttons are disabled. on success the card is
dismissed locally (the next poll reconciles the authoritative state). on error the buttons
are re-enabled and a lowercase inline error message is shown.

only the first unresolved permission card in the snapshot is interactive. later cards
render read-only until the first decision resolves.
