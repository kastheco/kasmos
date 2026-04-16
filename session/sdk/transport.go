// Package sdk provides an SDK-based execution backend that drives agent programs
// (claude, codex) via their app-server JSON-RPC protocols, giving kasmos typed
// bidirectional control (query, interrupt, approvals, structured stream) instead
// of tmux terminal scraping.
//
// The parent session package must not be imported from here to avoid import
// cycles — session/execution.go imports session/sdk, so the dependency edge must
// be one-way: session → session/sdk.
package sdk

import (
	"context"

	"github.com/kastheco/kasmos/session/tmux"
)

// LaunchConfig carries the parameters required to start an SDK-managed agent
// session. It mirrors the builder setters on headless.Session and tmux.TmuxSession
// so the instance layer can use a single config struct for all backends.
type LaunchConfig struct {
	// Name is the human-readable session name (used for log file naming).
	Name string
	// Program is the command string for the agent (e.g. "claude --model sonnet").
	Program string
	// WorkDir is the working directory for the agent subprocess.
	WorkDir string
	// SkipPermissions, when true, instructs the agent to bypass permission prompts.
	SkipPermissions bool
	// AgentType is an optional identifier appended via --agent <type> for
	// programs that support it (e.g. claude, opencode).
	AgentType string
	// InitialPrompt is delivered by the transport after startup when supported.
	InitialPrompt string
	// Project is the repository base name injected as KASMOS_PROJECT.
	Project string
	// TaskNumber, WaveNumber, PeerCount are the wave task identity env vars.
	// When TaskNumber > 0, all three are injected as KASMOS_TASK/WAVE/PEERS.
	TaskNumber int
	WaveNumber int
	PeerCount  int
	// NoFlicker controls whether CLAUDE_CODE_NO_FLICKER is set to 1 for Claude.
	NoFlicker bool
	// ExtraEnv holds additional environment variables to inject into the child
	// process environment on top of the standard kasmos vars. Each entry must be
	// in KEY=VALUE form. Transport implementations use this to add program-specific
	// variables (e.g. CLAUDE_CODE_NO_FLICKER) without modifying the generic
	// buildEnv helper in process.go.
	ExtraEnv []string
}

// Transport is the bidirectional control interface over an SDK-driven agent
// process. Implementations speak the agent's app-server protocol over stdio
// (for example Claude's JSON-RPC 2.0 surface or Codex's JSONL/JSON-RPC-lite
// App Server protocol).
//
// Import note: tmux.PermissionChoice is safe to use here because session/tmux
// is a sibling package with no dependency on session/sdk.
type Transport interface {
	// Start launches the agent subprocess and establishes the JSON-RPC connection.
	// Returns an error if the program string is empty or the process cannot start.
	Start(ctx context.Context, cfg LaunchConfig) error

	// SendPrompt delivers a new user prompt to the running agent turn.
	SendPrompt(ctx context.Context, prompt string) error

	// Interrupt requests the agent to stop its current turn.
	Interrupt(ctx context.Context) error

	// RespondPermission forwards a harness-specific permission-prompt response.
	RespondPermission(ctx context.Context, choice tmux.PermissionChoice) error

	// Events returns the channel of structured events produced by the agent.
	// The channel is closed when the transport is closed or the process exits.
	Events() <-chan Event

	// PID returns the OS process ID of the running agent.
	// Returns 0 before a successful Start call.
	PID() int

	// Close shuts down the agent process and releases all resources.
	// Safe to call before Start and after the process has already exited.
	Close() error
}
