# kasmos 🌌

Zellij-based agent orchestrator for managing concurrent AI coding sessions.

## Overview

kasmos drives [Zellij](https://zellij.dev) programmatically to orchestrate multiple OpenCode agent sessions in a structured terminal layout. It manages work package execution, monitors agent progress, and provides the operator with a unified view of all running agents.

## Architecture

- **Zellij** as the terminal session runtime (panes, tabs, layouts)
- **kasmos** as the orchestrator binary (Rust) — generates layouts, manages pane lifecycle
- **OpenCode** agents running in Zellij panes as interactive TUI sessions

## Status

🚧 Under development — see `kitty-specs/` for feature specifications.
