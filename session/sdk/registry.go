package sdk

import "github.com/kastheco/kasmos/session/common"

// transportFactory is the function used to create a Transport for a program.
// It is a package-level variable so tests can inject a mock without touching
// the real transport constructors.
var transportFactory = func(program string) (Transport, bool) {
	switch common.DetectProgramKind(program) {
	case common.ProgramClaude:
		return NewClaudeTransport(), true
	case common.ProgramCodex:
		return NewCodexTransport(), true
	default:
		return nil, false
	}
}

// SupportsProgram reports whether program has an SDK transport implementation.
// Only Claude and Codex are currently supported.
func SupportsProgram(program string) bool {
	switch common.DetectProgramKind(program) {
	case common.ProgramClaude, common.ProgramCodex:
		return true
	default:
		return false
	}
}

// NewTransport returns a fresh Transport for the given program string.
// Returns (nil, false) when program is not supported by the SDK backend.
func NewTransport(program string) (Transport, bool) {
	return transportFactory(program)
}
