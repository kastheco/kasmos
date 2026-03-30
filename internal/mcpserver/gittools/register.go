// Package gittools provides git tool handlers for the kasmos MCP server.
// It exposes git_status, git_diff, and git_log tools that operate within
// the sandboxed workspace, reusing fstools.Sandbox and fstools.CmdRunner for
// path sandboxing and testable exec.
package gittools

import (
	"github.com/kastheco/kasmos/internal/mcpserver/fstools"
	"github.com/mark3labs/mcp-go/server"
)

// registrarFn is a function that registers a single tool group with the MCP
// server. Each tool file calls addRegistrar in its init function so that
// RegisterTools can wire everything without importing each tool file individually.
type registrarFn func(srv *server.MCPServer, sb *fstools.Sandbox, runner fstools.CmdRunner)

// registrars holds all tool registrar functions added via addRegistrar.
var registrars []registrarFn

// addRegistrar appends fn to the list of registrar functions that
// RegisterTools calls. Tool files should call this from their init function.
func addRegistrar(fn registrarFn) {
	registrars = append(registrars, fn)
}

// RegisterTools wires all registered git tools into srv using the given
// allowed directories for sandboxing. It is safe to call with a nil srv; in
// that case it returns without panicking or registering anything. When runner
// is nil it defaults to &fstools.ExecRunner{}.
func RegisterTools(srv *server.MCPServer, allowedDirs []string, runner fstools.CmdRunner) {
	if srv == nil {
		return
	}
	if runner == nil {
		runner = &fstools.ExecRunner{}
	}
	sb := fstools.NewSandbox(allowedDirs)
	for _, fn := range registrars {
		fn(srv, sb, runner)
	}
}
