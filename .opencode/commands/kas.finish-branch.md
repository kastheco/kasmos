---
description: Merge a plan's branch to main or create a PR
agent: custodial
---

# /kas.finish-branch

Finish a development branch by merging to main or creating a pull request.

## Arguments

```
$ARGUMENTS
```

Optional: plan filename. If omitted, infer from current git branch.

## Process

1. Resolve the plan and its branch:
   - If argument provided, look up branch from `kq plan list`
   - If no argument, detect from `git branch --show-current` and match to a plan
2. Verify the branch has commits ahead of main:
   ```bash
   git log main..<branch> --oneline
   ```
   If no commits ahead, report "branch is up to date with main" and stop.
3. Run tests:
   ```bash
   go test ./...
   ```
   If tests fail, report failures and stop.
4. Present options:
   ```
   branch '<branch>' has N commits ahead of main.

   1. merge to main locally
   2. push and create a pull request
   3. keep as-is
   ```
5. Execute chosen option:
   - **Merge**: `git checkout main && git merge <branch> && git branch -d <branch>`
   - **PR**: `git push -u origin <branch> && gh pr create --title "<plan-name>" --body "..."`
   - **Keep**: do nothing
6. On merge or PR, update plan status:
   ```bash
   kq plan set-status <plan-file> done --force
   ```
7. If worktree exists for this branch, offer to clean it up:
   ```bash
   git worktree remove <path>
   ```
