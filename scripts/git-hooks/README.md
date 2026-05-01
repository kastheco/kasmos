# kasmos git hooks

This directory holds the checked-in client-side git hooks for kasmos contributors. Install with `just hooks` (sets `git config core.hooksPath`). `kas check` treats both `scripts/git-hooks` and an absolute path to this directory as configured.

## pre-push

Blocks `git push` when code changes in the push range have no matching documentation edits, per `docs/docs-drift-map.yml`. Reuses `scripts/detect-docs-drift.sh` and the same drift map that CI uses.

### stdin protocol

`git push` invokes `pre-push` with one line per ref being pushed, on stdin:

```
<local ref> <local sha> <remote ref> <remote sha>
```

The hook iterates every line. Special cases:

- `<local sha>` is the all-zero sha → ref deletion, allow.
- `<remote ref>` matches `refs/tags/*` → tag push, allow.
- `<remote sha>` is the all-zero sha → new branch, compare against the pushed remote's default branch, or `${KASMOS_DEFAULT_BRANCH}` when set. If the local remote-tracking ref is missing, the hook fetches that branch; if it still cannot find a merge base, the push is blocked.

### bypass

| how | when |
|-----|------|
| `git push --no-verify` | one-off escape, skips all client hooks |
| `KASMOS_SKIP_DOCS_DRIFT=1` env | scoped to docs-drift only |
| `Docs-Drift-Skip: <reason>` commit trailer | auditable, preserved in history, honored by CI |

CI runs the same check as a required status. `--no-verify` and `KASMOS_SKIP_DOCS_DRIFT=1` do not bypass CI.

### dependencies

`bash`, `git`, `yq`, `jq` — all required by `scripts/detect-docs-drift.sh`. The hook hard-fails with an install hint if any are missing.

### tests

- `scripts/git-hooks/test/run.sh` — synthetic-repo unit scenarios.
- `scripts/git-hooks/test/smoke.sh` — runs hook against the real repo HEAD and asserts agreement with the detector.

Both are invoked from `.github/workflows/docs-drift.yml`.
