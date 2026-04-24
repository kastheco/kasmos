package docstools

import (
	"net/http"
	"os"
	"time"

	"github.com/kastheco/kasmos/internal/mcpserver/fstools"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterOptions controls optional dependencies injected into docstools registrars.
// Zero values trigger sensible defaults.
type RegisterOptions struct {
	Runner     fstools.CmdRunner // default &fstools.ExecRunner{}
	HTTPClient *http.Client      // default http.Client{Timeout: 5s}
	BaseURL    string            // default from KASMOS_DOCS_BASE_URL or "https://kasmos.kasthe.co/docs/"
}

// registrarFn is a function that registers a single tool with the MCP server.
// Each tool file calls addRegistrar in its init function so that RegisterTools
// can wire everything without importing each file individually.
type registrarFn func(srv *server.MCPServer, sb *fstools.Sandbox, d *Dispatcher)

// registrars holds all tool registrar functions appended via addRegistrar.
var registrars []registrarFn

// addRegistrar appends fn to the list of registrar functions that RegisterTools
// calls. Tool files should call this from their init function.
func addRegistrar(fn registrarFn) {
	registrars = append(registrars, fn)
}

// RegisterTools wires all registered docs tools into srv using the given allowed
// directories for sandboxing. It is safe to call with a nil srv; in that case it
// returns without panicking or registering anything.
func RegisterTools(srv *server.MCPServer, allowedDirs []string, opts RegisterOptions) {
	if srv == nil {
		return
	}
	if opts.Runner == nil {
		opts.Runner = &fstools.ExecRunner{}
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	if opts.BaseURL == "" {
		if env := os.Getenv("KASMOS_DOCS_BASE_URL"); env != "" {
			opts.BaseURL = env
		} else {
			opts.BaseURL = "https://kasmos.kasthe.co/docs/"
		}
	}
	sb := fstools.NewSandbox(allowedDirs)
	d := NewDispatcher(sb, allowedDirs, opts.Runner, opts.HTTPClient, opts.BaseURL)
	for _, fn := range registrars {
		fn(srv, sb, d)
	}
}
