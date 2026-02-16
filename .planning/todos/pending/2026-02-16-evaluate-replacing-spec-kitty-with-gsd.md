---
created: 2026-02-16T06:40:41.460Z
title: Evaluate replacing spec-kitty with GSD
area: planning
files:
  - crates/kasmos/
  - kitty-specs/
  - .kittify/
  - .planning/
---

## Problem

Kasmos currently orchestrates workflow around spec-kitty artifacts and conventions. A strategic question came up: should the project migrate from spec-kitty to the GSD workflow stack, and if that migration happens, does kasmos still provide unique value or become redundant.

This impacts roadmap direction, architecture investment, and maintenance scope. Without a feasibility assessment, future implementation work risks optimizing around a framework that may be replaced.

## Solution

Run a focused feasibility study that compares:

1. Functional overlap between kasmos and GSD orchestration/planning capabilities.
2. Migration complexity from spec-kitty artifacts (`kitty-specs/`, `.kittify/`) to GSD equivalents.
3. What kasmos capabilities remain differentiated after migration (for example Zellij orchestration, MCP integration, and workspace-specific workflow automation).
4. Options matrix:
   - Keep kasmos + spec-kitty
   - Keep kasmos, swap spec-kitty for GSD
   - De-scope kasmos into a thinner layer over GSD
   - Sunset kasmos if GSD fully subsumes its role

Deliverable should include recommendation, risks, and a phased migration/no-migration plan.
