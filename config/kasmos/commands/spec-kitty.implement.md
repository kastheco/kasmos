---
description: Create an isolated workspace (worktree) for implementing a specific work package.
---

> **Skill Required**: Before executing this command, load the **spec-kitty** skill (`.opencode/skills/spec-kitty/SKILL.md`). If a Skill loading tool is available, use it with name `spec-kitty`. Otherwise, read the skill file directly. The skill provides essential context about the spec-kitty workflow, CLI commands, worktree management, and the `spec-kitty agent` programmatic API.

## Agent Routing (Cost Tier)

- Route implementation execution through the `coder` agent (light tier).
- Keep the command runner in controller mode: set up the WP and delegate coding work to `coder`.
- If implementation fails twice or touches shared/public API seams, escalate that WP to `reviewer` before continuing.
- Profile default: `coder` -> `openai/gpt-5.3-codex` with `reasoningEffort: high`.

## ⚠️ CRITICAL: Working Directory Requirement

**After running `spec-kitty implement WP##`, you MUST:**

1. **Run the cd command shown in the output** - e.g., `cd .worktrees/###-feature-WP##/`
2. **ALL file operations happen in this directory** - Read, Write, Edit tools must target files in the workspace
3. **NEVER write deliverable files to the main repository** - This is a critical workflow error

**Why this matters:**
- Each WP has an isolated worktree with its own branch
- Changes in main repository will NOT be seen by reviewers looking at the WP worktree
- Writing to main instead of the workspace causes review failures and merge conflicts

---

**IMPORTANT**: After running the command below, you'll see a LONG work package prompt (~1000+ lines).

**You MUST scroll to the BOTTOM** to see the completion command!

Run this command to get the work package prompt and implementation instructions:

```bash
spec-kitty agent workflow implement $ARGUMENTS --agent <your-name>
```

**CRITICAL**: You MUST provide `--agent <your-name>` to track who is implementing!

If no WP ID is provided, it will automatically find the first work package with `lane: "planned"` and move it to "doing" for you.

---

## Commit Workflow

**BEFORE moving to for_review**, you MUST commit your implementation:

```bash
cd .worktrees/###-feature-WP##/
git add -A
git commit -m "feat(WP##): <describe your implementation>"
```

**Then move to review:**
```bash
spec-kitty agent tasks move-task WP## --to for_review --note "Ready for review: <summary>"
```

**Why this matters:**
- `move-task` validates that your worktree has commits beyond main
- Uncommitted changes will block the move to for_review
- This prevents lost work and ensures reviewers see complete implementations

---

**The Python script handles all file updates automatically - no manual editing required!**

**NOTE**: If `/spec-kitty.status` shows your WP in "doing" after you moved it to "for_review", don't panic - a reviewer may have moved it back (changes requested), or there's a sync delay. Focus on your WP.
