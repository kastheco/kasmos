---
description: Tiered verification alias with argument-aware scope (WP/spec/path)
---

# /kas:verify

Compatibility alias for `/kas:review` with the same argument-aware behavior.

## Supported input

- `/kas:verify WP05` (spec inferred from branch)
- `/kas:verify 002-ratatui-tui-controller-panel WP05`
- `/kas:verify crates/kasmos/src/engine.rs`

Use `$ARGUMENTS` exactly as scope hints and run the full `/kas:review` workflow with those inputs.

## Required behavior

1. Resolve spec/WP/path scope from `$ARGUMENTS` (infer spec from branch when missing).
2. Run Tier 1 static checks.
3. Run Tier 2 reality assessment.
4. Run Tier 3 simplification suggestions only if VERIFIED.
5. Return strict structured output:
   - `DECISION`, `TIER_REACHED`, `SEVERITY_SUMMARY`, `SCOPE`, `FINDINGS`, `REALITY_GAPS`, `NEXT_ACTION`, `AUTOMATION`.
6. Apply the same post-review automation rules as `/kas:review`:
   - if `DECISION` is `NEEDS_CHANGES` or `BLOCKED`, and `spec` + `WP` resolve and lane is not `done`,
   - export feedback to `.kas/review-<WP>.feedback.md` and `.kas/review-<WP>.last.txt`,
   - run `spec-kitty agent tasks move-task <WP> --feature <spec> --to doing --review-feedback-file .kas/review-<WP>.feedback.md --reviewer ${KAS_REVIEW_AGENT:-reviewer} --force --no-auto-commit`.

Default reviewer run settings:

- model: `anthropic/claude-opus-4-6`
- variant/reasoning: `high`
- override with `KAS_REVIEW_MODEL`, `KAS_REVIEW_VARIANT`
