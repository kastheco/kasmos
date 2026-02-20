---
description: Perform structured code review and kanban transitions for completed task prompt files
---

## Agent Routing (Cost Tier)

- Route review execution through the `reviewer` agent (medium tier).
- Keep the command runner in controller mode: delegate review analysis and verdict drafting to `reviewer`.
- Escalate back to controller only for unresolved dependency conflicts or ambiguous review outcomes.
- Profile default: `reviewer` -> `anthropic/claude-opus-4-6` with `reasoningEffort: high`.

**IMPORTANT**: After running the command below, you'll see a LONG work package prompt (~1000+ lines).

**You MUST scroll to the BOTTOM** to see the completion commands!

Run this command to get the work package prompt and review instructions:

```bash
spec-kitty agent workflow review $ARGUMENTS --agent <your-name>
```

**CRITICAL**: You MUST provide `--agent <your-name>` to track who is reviewing!

If no WP ID is provided, it will automatically find the first work package with `lane: "for_review"` and move it to "doing" for you.

## Phase 1: Spec-Kitty workflow checks

### Dependency checks (required)

- dependency_check: If the WP frontmatter lists `dependencies`, confirm each dependency WP is merged to main before you review this WP.
- dependent_check: Identify any WPs that list this WP as a dependency and note their current lanes.
- rebase_warning: If you request changes AND any dependents exist, warn those agents to rebase and provide a concrete command (example: `cd .worktrees/FEATURE-WP02 && git rebase FEATURE-WP01`).
- verify_instruction: Confirm dependency declarations match actual code coupling (imports, shared modules, API contracts).

### Integration seam check (required)

When a WP modifies any package `__init__.py`, re-export file, or public API surface:

1. **Cross-WP import verification**: Run a smoke import of every modified package to catch name mismatches between the `__init__.py` re-exports and actual module exports:
   ```bash
   # For each __init__.py modified in this WP's diff:
   python -c "from <package> import <every_name_in___all__>"
   ```
2. **Name drift check**: Compare every name in `__init__.py` imports and `__all__` against the actual `def`/`class` names in the source modules. Flag any that don't match (spec names vs implementation names).
3. **Cross-WP consumer check**: Search for any other WP branches that import from this package's public API. Verify those imports still resolve.

**Reject** the WP if any import fails -- this is a **Critical** severity finding. Name mismatches between WPs are integration bugs that won't surface in per-WP tests.

## Phase 2: /kas.verify tiered code review

After completing Phase 1, run the full `/kas.verify` tiered review on the WP's changes. This catches quality issues that spec-kitty's workflow checks don't cover.

### 2a) Build review context

Run:

```bash
git status
git diff --stat
git diff
```

If no meaningful changes: skip Phase 2 (rely on Phase 1 outcome only).

From diff + task context, classify change profile:

- `has_code_changes`
- `has_error_handling_changes`
- `has_comments_or_docs_changes`
- `has_type_or_schema_changes`
- `has_test_changes`

### 2b) Tier 1 - Static analysis (required)

Always run core review; add targeted passes per profile.

- Core: correctness, security, readability, performance, maintainability.
- Error handling pass if relevant.
- Comment/docs accuracy pass if relevant.
- Type/schema quality pass if relevant.
- Test sufficiency pass if relevant.

If your runtime supports parallel sub-runs/subagents, run Tier 1 checks in parallel.

Tier 1 exits:

- Critical/High found -> `BLOCKED`
- Medium found -> `NEEDS_CHANGES`
- Otherwise continue

### 2c) Tier 2 - Reality assessment (required when changes exist)

Validate claimed completion vs actual behavior:

- behavior correctness end-to-end
- integration completeness (not stubs)
- tests cover real outcomes + edge cases
- gaps between WP/spec claims and implementation

Tier 2 exits:

- Severe gaps -> `BLOCKED`
- Actionable gaps -> `NEEDS_CHANGES`
- Clean -> `VERIFIED`

### 2d) Tier 3 - Simplification (optional; only if VERIFIED)

For code changes, provide non-blocking simplification suggestions.

## Phase 3: Unified verdict

Combine Phase 1 (spec-kitty workflow) and Phase 2 (tiered code review) into a single verdict.

- If EITHER phase produces a rejection, the overall verdict is reject.
- Phase 1 failures (dependency, integration seam) are always Critical severity.
- Phase 2 severity follows the tiered exit rules above.

### Output format (strict)

```markdown
DECISION: VERIFIED | NEEDS_CHANGES | BLOCKED
TIER_REACHED: 1 | 2 | 3
SEVERITY_SUMMARY: Critical=<n>, High=<n>, Medium=<n>, Low=<n>

SCOPE:
- spec: <inferred-or-explicit>
- wp: <WP## or none>

SK_WORKFLOW_CHECKS:
- dependency_check: PASS | FAIL <detail>
- integration_seam: PASS | FAIL <detail>

FINDINGS:
- [severity] file:line - issue and impact

REALITY_GAPS:
- [gap severity] claim vs actual behavior

SIMPLIFICATION_SUGGESTIONS:
- optional improvements (only if DECISION=VERIFIED)

NEXT_ACTION:
- one concrete operator step

AUTOMATION:
- feedback_file: <path or none>
- lane_update: <command run + result, or skipped>
```

## Phase 4: Post-review actions

### If VERIFIED (approved)

Run:

```bash
spec-kitty agent tasks move-task WP## --to done --note "Review passed: <summary>"
```

Also update the task file frontmatter `lane` from `for_review` to `done`:

```yaml
---
lane: done
review_status: "approved"
reviewed_by: "${KAS_REVIEW_AGENT:-reviewer}"
---
```

Write the review output to `.kas/review-<WP>.last.txt` for the record.

### If NEEDS_CHANGES or BLOCKED (rejected)

1. Ensure `.kas/` exists in current workspace root.
2. Write full structured review output to:
   - `.kas/review-<WP>.feedback.md`
   - `.kas/review-<WP>.last.txt`
3. Update the task file frontmatter `lane` from `for_review` to `planned` with rejection reason:

```yaml
---
lane: planned
review_status: "rejected"
reviewed_by: "${KAS_REVIEW_AGENT:-reviewer}"
review_rejection: "<one-line summary of highest severity finding>"
---
```

4. Write feedback to the temp file path shown in the prompt, then run:

```bash
spec-kitty agent tasks move-task WP## --to planned \
  --review-feedback-file .kas/review-<WP>.feedback.md \
  --reviewer ${KAS_REVIEW_AGENT:-reviewer} \
  --force --no-auto-commit
```

**The Python script handles all file updates automatically - no manual editing required!**

## Rules

- Default read-only review, except required automation above for post-review lane updates.
- Do not auto-fix code; wait for explicit user approval.
- Use concise, concrete findings with file references.
- Phase 1 failures short-circuit: if dependency or integration seam checks fail, you may skip Phase 2 and go straight to reject.
