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
| [system-context.md](system-context.md) | the two runtime services (`kas serve` + kasmos daemon), the five operator surfaces, and how they interconnect |
| [daemon-topology.md](daemon-topology.md) | daemon process internals: unix-socket control API routes, orchestration loop, and shared SQLite wiring |
| [signal-flow.md](signal-flow.md) | four ingress paths that create a pending gateway row and the daemon's claim-process-mark cycle that consumes it |
| [task-fsm.md](task-fsm.md) | the seven-status task FSM: every valid `(status, event) → next-status` transition plus execution sub-phases |
| [source-of-truth.md](source-of-truth.md) | authoritative ownership map for every piece of mutable state: which file or DB table holds it and who writes it |
| [wave-execution.md](wave-execution.md) | architect pass → parallel coder waves → user confirmation → review/verify pipeline, with blueprint-skip short-circuit |
| [review-cycle.md](review-cycle.md) | reviewer-agent and master-agent sequence, fail-closed fix loops, and config toggles that control the cycle |

---

## reading order

new to the codebase? a useful path through the docs:

1. **[system-context.md](system-context.md)** — understand the two services and five surfaces
2. **[daemon-topology.md](daemon-topology.md)** — see how the daemon is structured internally
3. **[signal-flow.md](signal-flow.md)** — trace how an agent action reaches the daemon
4. **[task-fsm.md](task-fsm.md)** — understand what happens to a task when a signal fires
5. **[wave-execution.md](wave-execution.md)** — follow a plan from planner-finished to code complete
6. **[review-cycle.md](review-cycle.md)** — follow code-complete to done
7. **[source-of-truth.md](source-of-truth.md)** — look up where any specific datum lives

---

## source of authority

[`FACTS.md`](FACTS.md) is a reference extracted from live source code. it lists canonical types, routes, and state definitions with exact file citations. diagrams defer to `FACTS.md` when there is any ambiguity. if a diagram contradicts a cited source file, the source file wins.
