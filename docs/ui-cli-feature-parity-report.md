# ui/cli feature parity report (archival)

> historical cleanup artifact: this path is not current product guidance.
>
> the original parity report targeted an older cli surface during the plan-to-task transition; use `README.md` and `go run . --help` for the current command surface instead.
>
> legacy-cleanup-audit currently classifies this document as a stale-doc candidate pending task 6's final keep/delete decision.

## current audit note

- current user-facing command terminology is `kas task ...`, not `kas plan ...`.
- project-local runtime state lives under `<repo-root>/.kasmos/`, with `taskstore.db` as the default local store.
- signals are gateway-backed first; `.kasmos/signals/` remains a compatibility path, not the primary control plane.

If this file stays in-tree after the cleanup, it should be rewritten as explicit historical context rather than live parity guidance.
