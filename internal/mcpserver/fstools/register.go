package fstools

import (
	"time"

	"github.com/kastheco/kasmos/internal/mcpserver/symbols"
	"github.com/mark3labs/mcp-go/server"
)

// FileCache is the optional read-file cache used by future fstool handlers.
type FileCache interface {
	Get(path string, from, lines int, mtime time.Time) (content string, totalLines int, ok bool)
	Set(path string, from, lines int, mtime time.Time, content string, totalLines int)
}

// SymbolLookup is the optional symbol index used by future fstool handlers.
type SymbolLookup interface {
	LookupAt(path string, line int) *symbols.Symbol
}

// RegisterOptions controls optional dependencies injected into fstool
// registrars. Zero values preserve the existing default behavior.
type RegisterOptions struct {
	Runner    CmdRunner
	FileCache FileCache
	Symbols   SymbolLookup
}

// registrarFn is a function that registers a single tool group with the MCP
// server. Each tool file calls addRegistrar in its init function so that
// RegisterTools can wire everything without importing each tool file individually.
type registrarFn func(srv *server.MCPServer, sb *Sandbox, opts RegisterOptions)

// registrars holds all tool registrar functions added via addRegistrar.
var registrars []registrarFn

// addRegistrar appends fn to the list of registrar functions that
// RegisterTools calls. Tool files should call this from their init function.
func addRegistrar(fn any) {
	switch typed := fn.(type) {
	case registrarFn:
		registrars = append(registrars, typed)
	case func(srv *server.MCPServer, sb *Sandbox, opts RegisterOptions):
		registrars = append(registrars, typed)
	case func(srv *server.MCPServer, sb *Sandbox, runner CmdRunner):
		registrars = append(registrars, func(srv *server.MCPServer, sb *Sandbox, opts RegisterOptions) {
			typed(srv, sb, opts.Runner)
		})
	default:
		panic("fstools: unsupported registrar signature")
	}
}

// RegisterTools wires all registered filesystem tools into srv using the given
// allowed directories for sandboxing. It is safe to call with a nil srv; in
// that case it returns without panicking or registering anything. When
// opts.Runner is nil it defaults to &ExecRunner{}.
func RegisterTools(srv *server.MCPServer, allowedDirs []string, opts RegisterOptions) {
	if srv == nil {
		return
	}
	sb := NewSandbox(allowedDirs)
	if opts.Runner == nil {
		opts.Runner = &ExecRunner{}
	}
	for _, fn := range registrars {
		fn(srv, sb, opts)
	}
}
