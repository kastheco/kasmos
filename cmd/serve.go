package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/kastheco/kasmos/config/architectaudit"
	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/configactions"
	"github.com/kastheco/kasmos/config/taskactions"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linearruntime"
	"github.com/kastheco/kasmos/internal/livepreview"
	"github.com/kastheco/kasmos/internal/mcpserver"
	"github.com/kastheco/kasmos/internal/mcpserver/routing"
	kaslog "github.com/kastheco/kasmos/log"
	webassets "github.com/kastheco/kasmos/web"
	"github.com/spf13/cobra"
)

// MCPVersion is the version advertised in MCP initialize responses.
var MCPVersion = "0.1.0"

type serveRepoRegistration struct {
	valid          map[string]struct{}
	projects       []string
	roots          []string
	rootsByProject map[string]string
	loadProjects   routing.ProjectLoader
}

func buildServeRepoRegistration(repoPaths []string) (serveRepoRegistration, error) {
	reg := serveRepoRegistration{
		valid:          make(map[string]struct{}, len(repoPaths)),
		projects:       make([]string, 0, len(repoPaths)),
		roots:          make([]string, 0, len(repoPaths)),
		rootsByProject: make(map[string]string, len(repoPaths)),
	}
	seenRoots := make(map[string]struct{}, len(repoPaths))
	seenProjects := make(map[string]string, len(repoPaths))

	for _, repoPath := range repoPaths {
		root := canonicalRepoPath(repoPath)
		if root == "" {
			return serveRepoRegistration{}, fmt.Errorf("resolve repo path %q: unable to determine canonical path", repoPath)
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
		reg.rootsByProject[project] = root
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

func dynamicProjectValidationMiddleware(loadProjects routing.ProjectLoader, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project, ok := projectFromServePath(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		projects, err := loadProjects(r.Context())
		if err != nil {
			http.Error(w, "project catalog unavailable", http.StatusServiceUnavailable)
			return
		}
		for _, candidate := range projects {
			if candidate == project {
				next.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "project not found: " + project})
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

// resolveServeRepoPaths returns the repo paths to use for multi-repo bootstrap.
// If explicit paths are provided via --repo, they are returned unchanged. When
// --db is set, the empty slice is returned so the single-DB path stays
// authoritative. Otherwise the daemon is queried for registered repos.
func resolveServeRepoPaths(cmd *cobra.Command, repoPaths []string) ([]string, error) {
	if len(repoPaths) > 0 {
		return repoPaths, nil
	}
	if cmd.Flags().Changed("db") {
		return repoPaths, nil
	}
	repos, err := listDaemonRepoStatuses()
	if err != nil || len(repos) == 0 {
		return repoPaths, nil
	}
	roots := make([]string, 0, len(repos))
	for _, r := range repos {
		roots = append(roots, r.Path)
	}
	return roots, nil
}

func newServeMCPServer(store taskstore.Store, gw taskstore.SignalGateway, sharedDB *sql.DB, repoPaths []string, projectLoaders ...routing.ProjectLoader) (*mcpserver.Server, error) {
	return newConfiguredMCPServer(store, gw, sharedDB, repoPaths, projectLoaders...)
}

const serveMCPRequestLogEnv = "KASMOS_MCP_REQUEST_LOG"

var serveMCPFallbackLogger = stdlog.New(os.Stderr, "", stdlog.LstdFlags)

func wrapServeMCPRequestLogging(next http.Handler) http.Handler {
	if !serveMCPRequestLoggingEnabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rpcMethod, bodyBytes := sniffServeMCPRPCMethod(r)
		sessionID := r.Header.Get("Mcp-Session-Id")
		start := time.Now()
		next.ServeHTTP(w, r)
		serveMCPRequestLogf(
			"mcp http request http_method=%s path=%s rpc_method=%s session=%s bytes=%d duration=%s",
			r.Method,
			r.URL.Path,
			emptyServeMCPLogField(rpcMethod),
			emptyServeMCPLogField(sessionID),
			bodyBytes,
			time.Since(start).Round(time.Millisecond),
		)
	})
}

func newServeMCPHTTPHandler(next http.Handler) http.Handler {
	if next == nil {
		return http.NotFoundHandler()
	}
	return wrapServeMCPRequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		// Standalone widget previews are opened from file:// and therefore send
		// Origin: null. Limit cross-origin MCP access to that preview origin.
		if r.Header.Get("Origin") == "null" {
			w.Header().Set("Access-Control-Allow-Origin", "null")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id")
			w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	}))
}

func serveMCPRequestLoggingEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(serveMCPRequestLogEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func sniffServeMCPRPCMethod(r *http.Request) (string, int) {
	if r == nil || r.Body == nil || r.Method != http.MethodPost {
		return "", 0
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "read_error", 0
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 {
		return "", 0
	}
	var envelope struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "decode_error", len(body)
	}
	return envelope.Method, len(body)
}

func emptyServeMCPLogField(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func serveMCPRequestLogf(format string, args ...any) {
	if kaslog.InfoLog != nil {
		kaslog.InfoLog.Printf(format, args...)
		return
	}
	if serveMCPFallbackLogger != nil {
		serveMCPFallbackLogger.Printf(format, args...)
	}
}

// openServeSQLiteBackends opens a single shared *sql.DB at dbPath and derives
// the task store, signal gateway, and audit logger from it. All three subsystems
// share the same connection pool, eliminating SQLITE_BUSY contention.
//
// On any constructor failure the shared pool is closed before the error is
// returned, so callers never receive a leaked *sql.DB in the error path.
// The caller owns the returned *sql.DB and must call sharedDB.Close() when done;
// the Store/SignalGateway/Logger Close() methods are no-ops (ownsDB=false) and
// exist only to make the ownership boundary explicit.
func openServeSQLiteBackends(dbPath string) (*sql.DB, taskstore.Store, taskstore.SignalGateway, auditlog.Logger, error) {
	sharedDB, err := taskstore.OpenSharedDB(dbPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("open shared db: %w", err)
	}
	store, err := taskstore.NewSQLiteStoreFromDB(sharedDB)
	if err != nil {
		sharedDB.Close()
		return nil, nil, nil, nil, fmt.Errorf("open task store: %w", err)
	}
	gw, err := taskstore.NewSQLiteSignalGatewayFromDB(sharedDB)
	if err != nil {
		sharedDB.Close()
		return nil, nil, nil, nil, fmt.Errorf("open signal gateway: %w", err)
	}
	logger, err := auditlog.NewSQLiteLoggerFromDB(sharedDB)
	if err != nil {
		sharedDB.Close()
		return nil, nil, nil, nil, fmt.Errorf("open audit logger: %w", err)
	}
	return sharedDB, store, gw, logger, nil
}

// newProjectListHandler returns an HTTP handler that responds with a sorted,
// deduplicated JSON array of project names. In repo-scoped mode (validProjects
// non-empty), it returns exactly the registered project names so that stale
// DB-only entries are excluded and newly registered repos with no task rows
// still appear. In bare-DB mode it queries the shared SQLite database.
func newProjectListHandler(sharedDB *sql.DB, validProjects map[string]struct{}, projectLoaders ...routing.ProjectLoader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if len(projectLoaders) > 0 && projectLoaders[0] != nil {
			projects, err := projectLoaders[0](r.Context())
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "project catalog unavailable"})
				return
			}
			_ = json.NewEncoder(w).Encode(projects)
			return
		}

		// Repo-scoped mode: return exactly the registered project names.
		if len(validProjects) > 0 {
			projects := make([]string, 0, len(validProjects))
			for p := range validProjects {
				projects = append(projects, p)
			}
			sort.Strings(projects)
			_ = json.NewEncoder(w).Encode(projects)
			return
		}

		// Bare-DB mode: query all distinct projects from the database.
		if sharedDB == nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "projects db unavailable"})
			return
		}
		projects, err := taskstore.ListDistinctProjectsFromDB(sharedDB)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if projects == nil {
			projects = []string{}
		}
		_ = json.NewEncoder(w).Encode(projects)
	}
}

// newServeAPIRootMux builds the API route mux from pre-constructed handlers.
// Task-action routes are registered before the generic taskstore prefix so that
// the more-specific method+path patterns (e.g. PUT /content) win over the plain
// prefix delegated to taskAPI. auditAPI keeps its own exact route for
// GET /audit-events; everything else falls through to taskAPI.
// previewAPI serves live-preview instance routes, configAPI serves project
// config and scaffold-sync routes, and architectAuditAPI serves cached architect
// decisions; all are registered before the generic taskstore prefix.
func newServeAPIRootMux(sharedDB *sql.DB, repoRegs serveRepoRegistration, taskAPI, auditAPI, actionsAPI, previewAPI, configAPI, architectAuditAPI http.Handler, linearWebhookAPI ...http.Handler) *http.ServeMux {
	rootMux := http.NewServeMux()
	rootMux.Handle("/v1/ping", taskAPI)
	rootMux.Handle("GET /v1/projects", newProjectListHandler(sharedDB, repoRegs.valid, repoRegs.loadProjects))

	// Task-action routes registered first — more-specific than the taskAPI prefix.
	rootMux.Handle("GET /v1/projects/{project}/tasks/{filename}/available-actions", actionsAPI)
	rootMux.Handle("POST /v1/projects/{project}/tasks/{filename}/transition", actionsAPI)
	rootMux.Handle("PUT /v1/projects/{project}/tasks/{filename}/status", actionsAPI)
	rootMux.Handle("POST /v1/projects/{project}/tasks/{filename}/rename", actionsAPI)
	rootMux.Handle("PUT /v1/projects/{project}/tasks/{filename}/topic", actionsAPI)
	rootMux.Handle("PUT /v1/projects/{project}/tasks/{filename}/goal", actionsAPI)
	rootMux.Handle("PUT /v1/projects/{project}/tasks/{filename}/content", actionsAPI)

	// Live-preview instance routes, before the generic taskstore prefix.
	rootMux.Handle("GET /v1/projects/{project}/instances", previewAPI)
	rootMux.Handle("GET /v1/projects/{project}/instances/{title}/capture", previewAPI)
	rootMux.Handle("POST /v1/projects/{project}/instances/{title}/send", previewAPI)
	rootMux.Handle("GET /v1/projects/{project}/instances/{title}/presentation", previewAPI)
	rootMux.Handle("POST /v1/projects/{project}/instances/{title}/permission", previewAPI)

	// Instance lifecycle action routes — forward to previewAPI which handles dispatch.
	rootMux.Handle("POST /v1/projects/{project}/instances/{title}/pause", previewAPI)
	rootMux.Handle("POST /v1/projects/{project}/instances/{title}/resume", previewAPI)
	rootMux.Handle("POST /v1/projects/{project}/instances/{title}/restart", previewAPI)
	rootMux.Handle("POST /v1/projects/{project}/instances/{title}/kill", previewAPI)

	// Config endpoints — before the generic taskstore prefix.
	rootMux.Handle("GET /v1/projects/{project}/config", configAPI)
	rootMux.Handle("PUT /v1/projects/{project}/config", configAPI)
	rootMux.Handle("POST /v1/projects/{project}/scaffold-sync", configAPI)

	// Cached architect decisions route — before the generic taskstore prefix.
	rootMux.Handle("GET /v1/projects/{project}/tasks/{filename}/architect-decisions", architectAuditAPI)

	// Exact audit route, then generic taskstore prefix.
	rootMux.Handle("GET /v1/projects/{project}/audit-events", auditAPI)
	if len(linearWebhookAPI) > 0 && linearWebhookAPI[0] != nil {
		rootMux.Handle("POST /v1/projects/{project}/linear/webhook", linearWebhookAPI[0])
	}
	rootMux.Handle("/v1/projects/", taskAPI)

	return rootMux
}

func buildLinearWebhookRuntimes(ctx context.Context, repoRegs serveRepoRegistration, store taskstore.Store, gw taskstore.SignalGateway, logger auditlog.Logger) (map[string]*linearruntime.Resolved, error) {
	runtimes := make(map[string]*linearruntime.Resolved)
	for project, repoPath := range repoRegs.rootsByProject {
		runtime, err := linearruntime.Resolve(ctx, repoPath, project, linearruntime.Options{
			Store:   store,
			Gateway: gw,
			Audit:   logger,
			Now:     time.Now,
			Logger:  slog.Default().With("monitor", "linear_webhook", "project", project),
		})
		if err != nil {
			return nil, fmt.Errorf("resolve linear webhook runtime for %s: %w", project, err)
		}
		if runtime == nil || runtime.Ingestor == nil {
			continue
		}
		runtimes[project] = runtime
	}
	return runtimes, nil
}

const nonLoopbackWarningText = "kas serve: admin API has no built-in auth; front it with tailscale, ssh tunnel, or a reverse proxy. built-in bearer-token auth is tracked in the serve-bearer-auth plan."

// nonLoopbackAdminWarning returns the warning message and true when the given
// addr is bound to a non-loopback interface (e.g. 0.0.0.0, ::, or a public IP).
// It returns ("", false) for loopback addresses (127.0.0.1, ::1, localhost).
func nonLoopbackAdminWarning(addr string) (string, bool) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Can't parse — treat as warnable to be safe.
		return nonLoopbackWarningText, true
	}
	// Empty host or wildcard binds are non-loopback.
	if host == "" || host == "0.0.0.0" || host == "::" {
		return nonLoopbackWarningText, true
	}
	if host == "localhost" {
		return "", false
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return "", false
	}
	return nonLoopbackWarningText, true
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

			// Auto-detect daemon repos when neither --repo nor --db is set.
			repoPaths, err := resolveServeRepoPaths(cmd, repoPaths)
			if err != nil {
				return err
			}

			var repoRegs serveRepoRegistration

			if len(repoPaths) > 0 {
				repoRegs, err = buildServeRepoRegistration(repoPaths)
				if err != nil {
					return err
				}
			}

			// Open a single shared *sql.DB and derive all subsystems from
			// it. This eliminates SQLITE_BUSY contention that occurred when
			// store, gateway, and audit logger each opened independent
			// connection pools on the same file.
			sharedDB, store, gw, logger, err := openServeSQLiteBackends(db)
			if err != nil {
				return err
			}
			defer sharedDB.Close()
			switch {
			case cmd.Flags().Changed("repo"):
				// Explicit repo mode remains a fixed allowlist.
			case cmd.Flags().Changed("db"):
				repoRegs.loadProjects = newMCPProjectLoader(sharedDB, false)
			default:
				repoRegs.loadProjects = newMCPProjectLoader(sharedDB, true)
			}
			// store/gw/logger Close() are no-ops (ownsDB=false);
			// sharedDB.Close() handles the actual connection teardown.
			defer store.Close()
			defer gw.Close()
			defer logger.Close()

			taskAPI := taskstore.NewHandler(store)
			auditAPI := auditlog.NewHandler(logger)
			hooks := buildTaskTransitionHooks("", store)
			actionsAPI := taskactions.NewHandler(store, gw, hooks)

			// Build the live-preview handler.
			//
			//   --repo (explicit): static resolver built from the flag values;
			//                      project validation middleware enforces the allow-list.
			//   --db (bare-DB):    preview unavailable (no filesystem repo root known).
			//   default (daemon):  dynamic resolver queries the daemon per-request so
			//                      repos registered after kas serve starts are visible
			//                      without a restart.
			//
			// Daemon-spawned (plan-associated) instances live in the daemon's
			// TmuxSpawner map and are never persisted to state.json. The preview
			// handler is given a daemonInstanceLister so it can union those
			// live records with whatever is on disk — otherwise the admin UI
			// only shows TUI-spawned standalone instances.
			daemonLister := newDaemonInstanceLister()
			var previewAPI http.Handler
			var configAPI http.Handler
			var architectAuditAPI http.Handler
			switch {
			case cmd.Flags().Changed("repo"):
				previewResolve := func(project string) (string, error) {
					root, ok := repoRegs.rootsByProject[project]
					if !ok {
						return "", fmt.Errorf("project not found: %s", project)
					}
					return root, nil
				}
				previewHandler := livepreview.NewHTTPHandlerWithDaemon(previewResolve, &livepreview.ExecPaneRunner{}, daemonLister, daemonLister)
				previewAPI = projectValidationMiddleware(repoRegs.valid, previewHandler)

				configResolve := func(project string) (string, error) {
					root, ok := repoRegs.rootsByProject[project]
					if !ok {
						return "", fmt.Errorf("project not found: %s", project)
					}
					return root, nil
				}
				configHandler := configactions.NewHandler(configResolve)
				configAPI = projectValidationMiddleware(repoRegs.valid, configHandler)
				architectAuditHandler := architectaudit.NewHandler(store, architectaudit.ProjectRootResolver(configResolve))
				architectAuditAPI = projectValidationMiddleware(repoRegs.valid, architectAuditHandler)
			case cmd.Flags().Changed("db"):
				previewAPI = livepreview.NewHTTPHandlerWithDaemon(func(string) (string, error) {
					return "", livepreview.ErrPreviewUnavailable
				}, &livepreview.ExecPaneRunner{}, daemonLister, daemonLister)
				configAPI = configactions.NewHandler(func(string) (string, error) {
					return "", configactions.ErrRepoNotRegistered
				})
				architectAuditAPI = architectaudit.NewHandler(store, func(string) (string, error) {
					return "", architectaudit.ErrRepoNotRegistered
				})
			default:
				previewAPI = livepreview.NewHTTPHandlerWithDaemon(newDynamicProjectRootResolver(), &livepreview.ExecPaneRunner{}, daemonLister, daemonLister)
				configAPI = configactions.NewHandler(
					configactions.ProjectRootResolver(newDynamicProjectRootResolverWithUnavailable(configactions.ErrRepoNotRegistered)),
				)
				architectAuditAPI = architectaudit.NewHandler(
					store,
					architectaudit.ProjectRootResolver(newDynamicProjectRootResolverWithUnavailable(architectaudit.ErrRepoNotRegistered)),
				)
			}
			if len(repoPaths) > 0 {
				if repoRegs.loadProjects != nil {
					taskAPI = dynamicProjectValidationMiddleware(repoRegs.loadProjects, taskAPI)
					auditAPI = dynamicProjectValidationMiddleware(repoRegs.loadProjects, auditAPI)
					actionsAPI = dynamicProjectValidationMiddleware(repoRegs.loadProjects, actionsAPI)
				} else {
					taskAPI = projectValidationMiddleware(repoRegs.valid, taskAPI)
					auditAPI = projectValidationMiddleware(repoRegs.valid, auditAPI)
					actionsAPI = projectValidationMiddleware(repoRegs.valid, actionsAPI)
				}
			}

			runtimeByProject, err := buildLinearWebhookRuntimes(cmd.Context(), repoRegs, store, gw, logger)
			if err != nil {
				return err
			}
			drainCh := make(chan string, 64)
			var linearWebhookAPI http.Handler
			if len(repoPaths) > 0 {
				linearWebhookAPI = projectValidationMiddleware(repoRegs.valid, newLinearWebhookProjectHandler(runtimeByProject, drainCh, time.Now))
			} else {
				linearWebhookAPI = newLinearWebhookUnavailableHandler()
			}

			rootMux := newServeAPIRootMux(sharedDB, repoRegs, taskAPI, auditAPI, actionsAPI, previewAPI, configAPI, architectAuditAPI, linearWebhookAPI)

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

			if msg, warn := nonLoopbackAdminWarning(addr); warn {
				if kaslog.WarningLog != nil {
					kaslog.WarningLog.Printf("%s", msg)
				} else {
					fmt.Printf("WARNING: %s\n", msg)
				}
			}

			if len(repoPaths) > 0 {
				fmt.Printf("task store listening on http://%s (repos: %s)\n", addr, strings.Join(repoRegs.projects, ", "))
			} else {
				fmt.Printf("task store listening on http://%s (db: %s)\n", addr, db)
			}

			// Graceful shutdown on SIGINT/SIGTERM.
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			// errCh has capacity 3 so server goroutines and the drain worker never
			// block on send when multiple failures happen at once.
			errCh := make(chan error, 3)

			go runLinearWebhookDrainer(ctx, runtimeByProject, drainCh, errCh)

			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
			}()

			var mcpHTTP *http.Server
			if mcpEnabled {
				mcpSrv, err := newServeMCPServer(store, gw, sharedDB, repoRegs.roots, repoRegs.loadProjects)
				if err != nil {
					return err
				}
				mcpAddr := fmt.Sprintf("%s:%d", bind, mcpPort)
				mcpHTTP = &http.Server{Addr: mcpAddr, Handler: newServeMCPHTTPHandler(mcpSrv.Handler())}
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
