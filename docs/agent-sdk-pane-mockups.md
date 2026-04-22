# agent sdk pane mockups

These mockups are for the existing kasmos `agent output` / preview pane, not a new full-screen mode. The left nav, tabbed window shell, and status bar remain as they are today.

## codex cli cues to borrow

From the screenshots, the most valuable things to copy are not the exact layout but the treatment:

- **muted lavender frame lines** rather than bright borders
- **soft section dividers** that break up phases of a turn without boxing everything
- a **clear divider before assistant prose** so tools and setup feel separate from the actual answer
- **dim tool/system text** and **bright response text**
- **rose/pink accent** reserved for the active footer mode and the most important warnings
- a **quiet composer**: no heavy textbox chrome, just a prompt line plus a slim footer/help strip

Suggested kasmos color roles:

- frame and dividers: muted lavender
- metadata and timestamps: cool gray-lavender
- tool call labels: desaturated cyan
- tool results: desaturated mint for success, dusty red for failure
- thinking rows: muted violet-gray
- assistant prose: near-white
- permission/warning rows: rose/salmon
- active footer strip: codex-style pink

## constraints from the current sdk stream

- current structured sdk events are `text_delta`, `tool_call`, `tool_result`, `permission`, `system`, and turn lifecycle markers
- the current renderer is line-based and intentionally compresses tool noise into one-line summaries
- explicit `thinking` is not a first-class renderer event today
- the `thinking` rows below therefore assume a derived UI state from turn metadata, timing gaps, or future reasoning summaries

## shared scenario

Each variant below renders the same turn:

- the agent inspects `session/sdk/renderer.go`
- the agent uses three tools
- the agent replies with two short paragraphs
- a permission request interrupts the turn once

## variant a: compact log-first

visual thesis: keep the pane feeling like a terminal transcript, but make turns readable at a glance.

interaction thesis:

- `thinking` is one compact row and expands inline
- tool calls stay in the stream, not in a side drawer
- the input box is always one screen-row tall until the user explicitly adds a newline

best fit:

- closest to the current renderer model
- lowest implementation cost
- best when the pane is narrow or stacked beside other kasmos chrome

risk:

- still looks like a log, not a conversation
- long turns may feel dense

```text
+--------------------------------------------------------------------------------------------------+
| agent output                                                       codex gpt-5.4 xhigh  running |
| turn 184                                                           00:09              3 tools  |
|--------------------------------------------------------------------------------------------------|
| thinking 6.2s  planning renderer changes in session/sdk/renderer.go                      [show] |
| * grep renderer.go                                                                  -> 4 items |
| * read_file renderer.go                                                             -> 297 lines|
| * go test ./session/sdk                                                             -> ok       |
|                                                                                                  |
| ------------------------------------------ response -------------------------------------------- |
|                                                                                                  |
| i found that the current renderer keeps tool output readable by compressing results into one    |
| line, but it loses turn-level grouping. the fastest path is to keep the log layout and add a    |
| lightweight turn header plus a collapsible tool summary.                                         |
|                                                                                                  |
| that would preserve the current density while making it much easier to scan one completed turn   |
| versus the next.                                                                                  |
|                                                                                                  |
| permission: allow network access for https://api.render.com                      [y] [n] [always]|
|--------------------------------------------------------------------------------------------------|
| > continue with the grouped-turn version                                                         |
|   enter send   shift+enter newline   esc stop                                 idle prompt        |
+--------------------------------------------------------------------------------------------------+
```

implementation notes:

- can evolve directly from `session/sdk/renderer.go`
- needs only light turn chrome, better spacing, and a real input footer
- `thinking` can ship as a synthetic line even before richer reasoning support exists
- should borrow codex cli's dimmer treatment for tool rows and brighter treatment for prose after the response divider

## variant b: chat-first with collapsible tools

visual thesis: make the assistant prose the primary object and demote tool traffic into collapsible support sections.

interaction thesis:

- each turn is a conversation block
- `thinking` and `tools used` default to collapsed summaries
- the input box reads like a composer, not a shell prompt

best fit:

- best readability for long agent answers
- strongest "assistant" feel
- easiest for non-operator users to understand quickly

risk:

- more departure from current kasmos terminal texture
- requires more structure than the current line renderer has
- hidden tools may feel too opaque for debugging-heavy workflows

```text
+--------------------------------------------------------------------------------------------------+
| agent output                                                       codex gpt-5.4 xhigh  running |
|--------------------------------------------------------------------------------------------------|
| assistant                                                                                turn 184|
|                                                                                                  |
| [thinking 6.2s: planning renderer changes in session/sdk/renderer.go]                    [expand]|
| [tools used: 3 calls, all ok]                                                               [open]|
|                                                                                                  |
| ------------------------------------------ response -------------------------------------------- |
|                                                                                                  |
| i found that the current renderer keeps tool output readable by compressing results into one    |
| line, but it loses turn-level grouping. the fastest path is to keep the log layout and add a    |
| lightweight turn header plus a collapsible tool summary.                                         |
|                                                                                                  |
| that would preserve the current density while making it much easier to scan one completed turn   |
| versus the next.                                                                                  |
|                                                                                                  |
| permission needed: network access for https://api.render.com                    allow  deny  save |
|--------------------------------------------------------------------------------------------------|
| message                                                                                          |
| continue with the grouped-turn version                                                           |
|                                                                                                  |
| enter send   shift+enter newline   / for commands                              prompt ready      |
+--------------------------------------------------------------------------------------------------+
```

expanded tools state:

```text
[tools used: 3 calls, all ok]                                                                   [hide]
  grep renderer.go                                                                          -> 4 items
  read_file renderer.go                                                                     -> 297 lines
  go test ./session/sdk                                                                     -> ok
```

implementation notes:

- likely wants a richer per-turn view model rather than plain line accumulation
- strongest candidate if we expect lots of streamed prose and fewer operator-debug sessions
- should keep the codex-style response divider even if tools stay collapsed

## variant c: threaded turn blocks

visual thesis: keep the operator/debugging strength of a terminal, but group every turn into a strict timeline block.

interaction thesis:

- every turn is rendered as a stack of typed rows
- tools and results are paired visually
- the input box stays compact so the timeline remains dominant

best fit:

- best balance between operator clarity and conversational grouping
- strongest audit/debug feel
- scales well if we later add retries, interrupts, diffs, or patch summaries

risk:

- more chrome than variant a
- denser than variant b
- needs more careful spacing rules to avoid looking busy

```text
+--------------------------------------------------------------------------------------------------+
| agent output                                                       codex gpt-5.4 xhigh  running |
|--------------------------------------------------------------------------------------------------|
| turn 184  00:09                                                                                 |
|   thinking   6.2s  planning renderer changes in session/sdk/renderer.go                 [show]  |
|   tool       Edit session/sdk/renderer.go                                                     |
|   diff  ─────────────────────────────────── renderer.go ──────────────────────────────────    |
|         1   - func renderTurn(t *Turn) string {                                               |
|         1   + func renderSDKTurn(t *PresentationTurn, width int) []string {                   |
|         2   + 	if t == nil { return nil }                                                   |
|         ··· 44 lines hidden                                                                    |
|   result     ok                                                                               |
|   tool       Bash: go test ./session/sdk/...                                                  |
|   preview ───────────────────────────────── output ───────────────────────────────────────    |
|         ok   github.com/kastheco/kasmos/session/sdk   (cached)                               |
|   -------------------------------------- response -------------------------------------------   |
|     i found that the current renderer keeps tool output readable by compressing results into   |
|     one line, but it loses turn-level grouping. the fastest path is to keep the log layout     |
|     and add a lightweight turn header plus a collapsible tool summary.                         |
|                                                                                                |
|     that would preserve the current density while making it much easier to scan one completed  |
|     turn versus the next.                                                                      |
|                                                                                                |
|   permission  network access for https://api.render.com                          [y] [n] [all] |
|--------------------------------------------------------------------------------------------------|
| > continue with the grouped-turn version                                                        |
|   enter send   shift+enter newline   esc stop                                 idle prompt       |
|  ✺ editing renderer.go  00:12                                                                  |
+--------------------------------------------------------------------------------------------------+
```

implementation notes:

- easiest variant for future additions like `patch`, `diff`, `retry`, `interrupted`, or `applied`
- probably needs the pane to understand turns as grouped objects, not just lines
- this is the variant that best matches codex cli's visual rhythm: dim setup rows, a divider, then bright prose
- **inline diff blocks** (`RowToolDiff`) are emitted by the in-process LCS differ in
  `session/sdk/tool_diff.go`; no external `difft` binary is required
- **inline text preview blocks** (`RowToolPreview`) show a capped slice of non-error
  tool result text extracted by `session/sdk/tool_preview.go`
- the **pinned `working + elapsed` strip** (bottom line above the composer) is rendered by
  `ui/preview.go:buildSDKPresentationView`; it is derived from the last running turn's
  `Activity` field on every preview tick and disappears when the turn completes
- hard-coded line caps (both defaulting to 50):
  - `diffPreviewMaxLines = 50` — maximum visible diff lines per `ToolDiffPayload`
  - `textPreviewMaxLines = 50` — maximum visible lines per `ToolPreviewPayload`
    (reuses `diffPreviewMaxLines`; both tools use the same constant)

## codex-inspired styling pass

Regardless of which layout wins, I would apply these exact appearance rules:

1. every turn starts with a dim metadata row
   Example: `turn 184   00:09   3 tools`

2. pre-response content is visually quieter
   Thinking, tool calls, tool results, and system notes should all use dimmer color roles than the final prose.

3. assistant prose always begins after a dedicated divider
   Not just blank space. The divider is the visual cue that the setup phase is over and the answer begins.

4. the divider text should be simple
   Use `response`, not `assistant response`, `final answer`, or anything noisy.

5. the composer should stay visually light
   Prefer:
   - one prompt line
   - one subtle top divider
   - one thin footer strip for shortcuts/status
   Avoid a boxed multiline textarea unless expanded.

6. permission rows should interrupt with color, not extra chrome
   A rose/salmon row is enough. Avoid modal-card styling inside the pane.

7. successful tool results should be less loud than tool calls
   The user mostly needs confirmation that the tool returned something; they do not need a second high-contrast row unless it failed.

## recommended direction after seeing codex cli

The recommendation changes slightly with the screenshots in mind:

- **layout architecture:** `variant c`
- **implementation starting point:** `variant a`
- **visual treatment:** codex cli-inspired for both

If we do the first implementation pass, I would explicitly build these three things:

1. codex-style color hierarchy
2. a real `response` divider before prose starts
3. a light composer/footer treatment instead of a heavy input box

## recommendation

If the goal is to ship a good first version quickly, start from **variant a** and borrow one idea from **variant b**:

- keep the stream/log feel
- add a strong turn header
- make `thinking` and `tools used` collapsible at the turn level
- keep the input box compact and always visible

If the goal is to build the long-term sdk pane we will keep expanding, **variant c** is the best target architecture.

My honest ranking for kasmos:

1. `variant c` for the best final product
2. `variant a` for the best first implementation
3. `variant b` for the best prose readability, but the weakest fit for kasmos's operator/debugging personality
