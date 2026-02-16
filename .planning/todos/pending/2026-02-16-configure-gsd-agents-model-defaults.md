---
created: 2026-02-16T06:43:15.602Z
title: Configure GSD agents model defaults
area: planning
files:
  - /home/kas/.config/opencode/get-shit-done/workflows/
  - /home/kas/.config/opencode/get-shit-done/bin/gsd-tools.cjs
  - .planning/
---

## Problem

GSD agents are currently defaulting to `opus 4.6 max` across the board. That configuration likely ignores role-specific model needs and can hurt cost/performance tradeoffs for orchestration, mapping, planning, and execution tasks.

Without explicit per-agent model configuration, sessions may run with the wrong model tier and produce inconsistent behavior relative to expected GSD workflow design.

## Solution

Audit and correct GSD model selection so each agent type has an intentional default.

1. Identify where agent model defaults are defined and propagated to workflow steps.
2. Map each GSD subagent role to an appropriate model tier (quality-critical vs throughput-oriented tasks).
3. Verify `init` outputs and workflow prompts pass the intended model value to spawned agents.
4. Add validation/checks so future workflow runs detect accidental global fallback to a single model.

Deliverable: documented model matrix + config/code changes that enforce correct defaults for all GSD agents.
