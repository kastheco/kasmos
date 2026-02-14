# Research: MCP Agent Swarm Orchestration

## Decision 1: Serve Runtime Model

- Decision: Run `kasmos serve` as an MCP stdio subprocess owned by the manager agent.
- Rationale: Keeps lifecycle tied to manager ownership, removes extra Zellij process wiring, and aligns with OpenCode MCP subprocess support.
- Alternatives considered:
  - Dedicated MCP tab process: easier visual inspection, but adds lifecycle split and startup ordering complexity.
  - Hybrid tab plus subprocess: duplicates responsibility and increases failure modes.

## Decision 2: Feature Ownership Lock Scope

- Decision: Enforce one active owner per feature repository-wide (`<repo_root>::<feature_slug>`).
- Rationale: Prevents conflicting managers from separate tabs/sessions/processes from mutating the same task state concurrently.
- Alternatives considered:
  - Same-session-only lock: misses collisions across sessions and detached terminals.
  - Machine-global lock without repo key: risks false conflicts across unrelated repositories.

## Decision 3: Stale Lock Recovery

- Decision: Mark locks stale after 15 minutes of missed heartbeat; allow takeover only after explicit user confirmation.
- Rationale: Balances safety and operability by preventing silent ownership transfer while still recovering from abandoned sessions.
- Alternatives considered:
  - Manual unlock only: safest but creates operational dead-ends after crashes.
  - Automatic takeover: faster but can hide race conditions and user intent mismatches.

## Decision 4: Launch Failure Policy

- Decision: `kasmos` launch path must fail before creating tabs/sessions if dependencies are missing.
- Rationale: Prevents partial orchestration state and gives deterministic setup feedback.
- Alternatives considered:
  - Warning-only behavior: users reach broken sessions with delayed failures.
  - Auto-run setup and continue: can mask root causes and produce inconsistent startup timing.

## Decision 5: Audit Log Location and Retention

- Decision: Persist audit logs at `kitty-specs/<feature>/.kasmos/messages.jsonl`; rotate/prune when either file size exceeds 512MB or entry age exceeds 14 days.
- Rationale: Feature-local logs improve traceability and code review context; dual threshold controls disk growth and stale data.
- Alternatives considered:
  - Repo-level shared log: harder to isolate incidents by feature.
  - Append-only no rotation: unbounded growth and slower diagnostics.

## Decision 6: Audit Payload Depth

- Decision: Metadata-only logging by default; full payload logging enabled only in debug mode.
- Rationale: Reduces sensitive prompt/context retention while preserving enough telemetry for normal operations.
- Alternatives considered:
  - Always full payload: highest forensic value, but larger privacy and storage risk.
  - Strict metadata only with no debug option: insufficient for deep incident triage.

## Decision 7: No-Inference Feature Selection UX

- Decision: Run feature selection in CLI before any session or tab is created.
- Rationale: Avoids launching orchestration UI in an unknown context and keeps startup behavior deterministic.
- Alternatives considered:
  - In-manager selector: delays resolution until after tab launch and complicates rollback.
  - Dual prompt CLI plus manager: redundant confirmation with little extra safety.

## Best-Practice Notes

- Use bounded blocking waits (`wait_for_event` timeout) so manager loops remain resumable.
- Use atomic writes plus advisory locking for task lane transitions.
- Keep structured message format stable and machine-parseable (`[KASMOS:<sender>:<event>] <json>`).
- Treat lock conflicts, dependency failures, and transition validation errors as first-class user-facing events.
