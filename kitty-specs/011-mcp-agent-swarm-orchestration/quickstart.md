# Quickstart: MCP Agent Swarm Orchestration

## Prerequisites

- Rust stable toolchain (2024 edition support)
- `zellij` in `PATH`
- `opencode` in `PATH`
- zellij pane tracker tooling available to manager/worker agents

## 1) Validate Environment

Run:

```bash
kasmos setup
```

Expected behavior:

- Validates required binaries and integration prerequisites
- Creates missing baseline config/profile assets if needed

## 2) Launch with Explicit Feature

Run:

```bash
kasmos 011
```

Expected behavior:

- Launch preflight runs before any tab/session creation
- If dependencies are missing, launch aborts with actionable guidance and non-zero exit
- Orchestration tab opens with manager, message-log, dashboard, and worker area
- Manager spawns `kasmos serve` as MCP stdio subprocess

## 3) Launch Without Feature Prefix

Run:

```bash
kasmos
```

Expected behavior:

- If feature can be inferred (branch/path), launch continues
- If not inferable, CLI selector appears before any tab/session creation
- If no feature specs exist, CLI reports this and exits cleanly

## 4) Lock Conflict and Stale Recovery

If another process already owns the feature lock:

- Fresh lock: bind is refused and current owner details are shown
- Stale lock (older than 15 minutes): takeover is offered but requires explicit confirmation

## 5) Audit Logging Behavior

- Log file path: `kitty-specs/<feature>/.kasmos/messages.jsonl`
- Default mode: metadata-only entries
- Debug mode: optional full payload capture for deep troubleshooting
- Rotation/pruning triggers when either threshold is met:
  - file size exceeds 512MB
  - entry age exceeds 14 days

## 6) Basic Verification Checklist

- `cargo build`
- `cargo test`
- Manual check: launch + lock conflict + stale takeover prompt + message-log event flow
