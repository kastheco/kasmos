package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/mcpserver"
	webassets "github.com/kastheco/kasmos/web"
	"github.com/spf13/cobra"
)

// MCPVersion is the version advertised in MCP initialize responses.
var MCPVersion = "0.1.0"

type serveRepoRegistration struct {
	configs  []taskstore.RepoConfig
	valid    map[string]struct{}
	projects []string
	roots    []string
}

func buildServeRepoRegistration(repoPaths []string) (serveRepoRegistration, error) {
	reg := serveRepoRegistration{
		configs:  make([]taskstore.RepoConfig, 0, len(repoPaths)),
		valid:    make(map[string]struct{}, len(repoPaths)),
		projects: make([]string, 0, len(repoPaths)),
		roots:    make([]string, 0, len(repoPaths)),
	}
	seenRoots := make(map[string]struct{}, len(repoPaths))
	seenProjects := make(map[string]string, len(repoPaths))

	for _, repoPath := range repoPaths {
		root, err := filepath.Abs(filepath.Clean(repoPath))
		if err != nil {
			return serveRepoRegistration{}, fmt.Errorf("resolve repo path %q: %w", repoPath, err)
		}
		if _, exists := seenRoots[root]; exists {
			return serveRepoRegistration{}, fmt.Errorf("repo already registered: %s", root)
		}

		project := filepath.Base(root)
		if existingPath, exists := seenProjects[project]; exists {
			return serveRepoRegistration{}, fmt.Errorf("repo with basename %q already registered (path: %s); rename one of the directories or use distinct names", project, existingPath)
		}

		seenRoots[root] = struct{}{}
		seenProjects[project] = root
		reg.valid[project] = struct{}{}
		reg.projects = append(reg.projects, project)
		reg.roots = append(reg.roots, root)
		reg.configs = append(reg.configs, taskstore.RepoConfig{Path: root})
	}

	return reg, nil
}

func projectValidationMiddleware(valid map[string]struct{}, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project, ok := projectFromServePath(r.URL.Path)
		if ok {
			if _, exists := valid[project]; !exists {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "project not found: " + project})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func projectFromServePath(path string) (string, bool) {
	const prefix = "/v1/projects/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" {
		return "", false
	}
	if idx := strings.IndexByte(rest, '/'); idx >= 0 {
		rest = rest[:idx]
	}
	if rest == "" {
		return "", false
	}
	return rest, true
}

func newServeMCPServer(store taskstore.Store, gw taskstore.SignalGateway, repoPaths []string) (*mcpserver.Server, error) {
	if len(repoPaths) == 0 {
		return newConfiguredMCPServer(store, gw, "")
	}

	return newConfiguredMCPServer(store, gw, repoPaths[0])
}

// NewServeCmd returns the `kas serve` cobra command.
// It starts an HTTP server backed by a SQLite task store, and optionally
// an MCP server on a second port sharing the same store and signal gateway.
func NewServeCmd() *cobra.Command {
	var (
		port       int
		db         string
		repoPaths  []string
		bind       string
		mcpEnabled bool
		mcpPort    int
		adminDir   string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "start the task store HTTP server",
		Long:  "Start an HTTP server that exposes task state over a REST API backed by SQLite.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("db") && cmd.Flags().Changed("repo") {
				return fmt.Errorf("--db and --repo are mutually exclusive")
			}

			var (
				store    taskstore.Store
				gw       taskstore.SignalGateway
				logger   auditlog.Logger
				repoRegs serveRepoRegistration
				err      error
			)

			if len(repoPaths) > 0 {
				repoRegs, err = buildServeRepoRegistration(repoPaths)
				if err != nil {
					return err
				}
				store, err = taskstore.NewMultiStore(repoRegs.configs)
				if err != nil {
					return fmt.Errorf("open task store: %w", err)
				}
				gw, err = taskstore.NewMultiSignalGateway(repoRegs.configs)
				if err != nil {
					_ = store.Close()
					return fmt.Errorf("open signal gateway: %w", err)
				}
				logger, err = auditlog.NewMultiLogger(repoPaths)
				if err != nil {
					_ = gw.Close()
					_ = store.Close()
					return fmt.Errorf("open audit logger: %w", err)
				}
			} else {
				store, err = taskstore.NewSQLiteStore(db)
				if err != nil {
					return fmt.Errorf("open task store: %w", err)
				}
				gw, err = taskstore.NewSQLiteSignalGateway(db)
				if err != nil {
					_ = store.Close()
					return fmt.Errorf("open signal gateway: %w", err)
				}
				logger, err = auditlog.NewSQLiteLogger(db)
				if err != nil {
					_ = gw.Close()
					_ = store.Close()
					return fmt.Errorf("open audit logger: %w", err)
				}
			}
			defer store.Close()
			defer gw.Close()
			defer logger.Close()

			taskAPI := taskstore.NewHandler(store)
			auditAPI := auditlog.NewHandler(logger)
			if len(repoPaths) > 0 {
				taskAPI = projectValidationMiddleware(repoRegs.valid, taskAPI)
				auditAPI = projectValidationMiddleware(repoRegs.valid, auditAPI)
			}

			rootMux := http.NewServeMux()
			rootMux.Handle("/v1/ping", taskAPI)
			// Route audit-events exactly, then fall through to the task API for everything else.
			// Go 1.22+ mux gives the more-specific method+path pattern precedence over the
			// plain prefix, so GET audit-events is handled by auditAPI and all other
			// /v1/projects/* requests continue to taskAPI.
			rootMux.Handle("GET /v1/projects/{project}/audit-events", auditAPI)
			rootMux.Handle("/v1/projects/", taskAPI)

			// Resolve the admin filesystem: --admin-dir flag overrides embedded assets.
			// Require the directory to contain index.html so users aren't accidentally
			// served a source tree (e.g. web/admin/) instead of the built output (dist/).
			var adminFS http.FileSystem
			if adminDir != "" {
				if _, err := os.Stat(adminDir); err != nil {
					return fmt.Errorf("stat admin dir: %w", err)
				}
				if _, err := os.Stat(filepath.Join(adminDir, "index.html")); err != nil {
					return fmt.Errorf("admin dir must contain index.html (point --admin-dir at the dist/ directory): %w", err)
				}
				adminFS = http.Dir(adminDir)
			} else {
				adminFS = webassets.AdminFS()
			}

			rootMux.Handle("/admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently))
			rootMux.Handle("/admin/", http.StripPrefix("/admin", adminFallbackHandler(adminFS)))
			fmt.Println("admin UI available at /admin/")

			addr := fmt.Sprintf("%s:%d", bind, port)

			srv := &http.Server{
				Addr:    addr,
				Handler: rootMux,
			}

			if len(repoPaths) > 0 {
				fmt.Printf("task store listening on http://%s (repos: %s)\n", addr, strings.Join(repoRegs.projects, ", "))
			} else {
				fmt.Printf("task store listening on http://%s (db: %s)\n", addr, db)
			}

			// Graceful shutdown on SIGINT/SIGTERM.
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			// errCh has capacity 2 so neither goroutine blocks on send when both fail.
			errCh := make(chan error, 2)

			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
			}()

			var mcpHTTP *http.Server
			if mcpEnabled {
				mcpSrv, err := newServeMCPServer(store, gw, repoRegs.roots)
				if err != nil {
					return err
				}
				mcpAddr := fmt.Sprintf("%s:%d", bind, mcpPort)
				mcpHTTP = &http.Server{Addr: mcpAddr, Handler: mcpSrv.Handler()}
				fmt.Printf("mcp server listening on http://%s/mcp\n", mcpAddr)
				go func() {
					if err := mcpHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						errCh <- err
					}
				}()
			}

			select {
			case err := <-errCh:
				return err
			case <-ctx.Done():
				fmt.Println("\nshutting down...")
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if mcpHTTP != nil {
					_ = mcpHTTP.Shutdown(shutdownCtx)
				}
				return srv.Shutdown(shutdownCtx)
			}
		},
	}

	defaultDB := taskstore.ResolvedDBPath()

	cmd.Flags().IntVar(&port, "port", 7433, "port to listen on")
	cmd.Flags().StringVar(&db, "db", defaultDB, "path to the SQLite database file")
	cmd.Flags().StringSliceVar(&repoPaths, "repo", nil, "repo root to serve; repeat for multiple repos")
	cmd.Flags().StringVar(&bind, "bind", "0.0.0.0", "address to bind to")
	cmd.Flags().BoolVar(&mcpEnabled, "mcp", true, "enable the MCP server (Streamable HTTP on --mcp-port)")
	cmd.Flags().IntVar(&mcpPort, "mcp-port", 7434, "port for the MCP server")
	cmd.Flags().StringVar(&adminDir, "admin-dir", "", "path to the built admin SPA dist/ directory (overrides embedded assets)")

	return cmd
}
