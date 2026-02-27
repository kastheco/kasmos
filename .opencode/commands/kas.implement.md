---
description: Trigger implementation of a specific wave for a plan
agent: custodial
---

# /kas.implement

Trigger wave-based implementation for a plan via signal file.

## Arguments

```
$ARGUMENTS
```

Expected format: `<plan-file> [--wave N]`
Default wave: 1

## Process

1. Parse arguments for plan filename and optional wave number
2. If no arguments, show available plans:
   ```bash
   kq plan list --status ready
   kq plan list --status implementing
   ```
3. Verify plan exists and has wave headers:
   ```bash
   head -50 docs/plans/<plan-file>
   ```
4. Execute:
   ```bash
   kq plan implement <plan-file> --wave <N>
   ```
5. Confirm:
   ```
   implementation triggered: <plan-file> wave <N>
   the TUI will pick up the signal on the next tick (~2s).
   ```
