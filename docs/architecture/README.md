# architecture

this section documents the **current runtime topology** of kasmos — how signals, data, and state actually flow between the TUI, admin UI, `kas` CLI, MCP server, daemon, task store, signal gateway, and agent sessions.

these are not aspirational diagrams. every node, edge, and data-flow shown here reflects real code paths. when prose and visuals disagree, the cited source files are authoritative (see [source of authority](#source-of-authority) below).

---

## legend

all diagrams in this section follow the same notation:

| shape | meaning |
|-------|---------|
| rectangle | process or component |
| cylinder | persisted store |
| rounded box (stadium) | operator surface (TUI, CLI, web UI) |
| solid arrow `-->` | direct call / synchronous request |
| dashed arrow `-.->` | async handoff / eventual processing |
| dotted arrow `...>` | filesystem sentinel compatibility path |

---

## pages

| page | what it shows |
|------|--------------|
| [system-context.md](system-context.md) | top-level view: which processes exist and how operators reach them |
| [daemon-topology.md](daemon-topology.md) | daemon internals: http mux routes, signal gateway, task-store wiring |
| [signal-flow.md](signal-flow.md) | lifecycle of a signal from agent → gateway → daemon loop → task FSM |
| [task-fsm.md](task-fsm.md) | task state machine: states, events, and guard conditions |
| [source-of-truth.md](source-of-truth.md) | where each piece of state lives (task store, config, worktree, tmux) |
| [wave-execution.md](wave-execution.md) | orchestration loop: wave phases, agent spawning, barrier logic |
| [review-cycle.md](review-cycle.md) | review and merge flow: reviewer agents, PR creation, readiness check |

---

## source of authority

[`FACTS.md`](FACTS.md) is a reference extracted from live source code. it lists canonical types, routes, and state definitions with exact file citations. diagrams defer to `FACTS.md` when there is any ambiguity. if a diagram contradicts a cited source file, the source file wins.
