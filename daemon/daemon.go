package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/binpath"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
)

// ---------------------------------------------------------------------------
// Daemon
// ---------------------------------------------------------------------------

// Daemon is the multi-repo background orchestrator. It polls registered
// repositories for signal files and executes the resulting actions via the
// configured AgentSpawner.
type Daemon struct {
	cfg             *DaemonConfig
	repos           *RepoManager
	spawner         *TmuxSpawner
	logger          *slog.Logger
	pidLock         *PIDLock
	broadcaster     *api.EventBroadcaster
	prMonitor       *PRMonitor
	pushBranch      func(*session.Instance) error
	killAgent       func(repoPath, planFile, agentType string) error
	forceKillAgent  func(repoPath, planFile, agentType string) error
	spawnPlanner    func(context.Context, loop.SpawnOpts) error
	spawnReviewer   func(context.Context, loop.SpawnOpts) error
	spawnCoder      func(context.Context, loop.SpawnOpts) error
	spawnElaborator func(context.Context, loop.SpawnOpts) error
	spawnFixer      func(context.Context, loop.SpawnOpts) error
	spawnMaster     func(context.Context, loop.SpawnOpts) error
	spawnWaveTask   func(context.Context, loop.SpawnOpts, taskparser.Task, string, int, int) error
	killWaveAgents  func(repoPath, planFile string, wave int) error
	createPR        func(RepoEntry, string, string) error
	mu              sync.RWMutex
	startedAt       time.Time
}

// daemonStateAdapter adapts the Daemon to the api.StateProvider interface.
type daemonStateAdapter struct {
	d *Daemon
}

// activePlansByProject counts distinct plan files currently running per project.
func (a *daemonStateAdapter) activePlansByProject() map[string]int {
	counts := map[string]int{}
	for _, inst := range a.d.spawner.RunningInstances() {
		if inst.Project == "" {
			continue
		}
		key := inst.Project + "\x00" + inst.PlanFile
		counts[key]++
	}
	// Collapse to unique-plan count per project.
	perProject := map[string]int{}
	seen := map[string]struct{}{}
	for k := range counts {
		parts := strings.SplitN(k, "\x00", 2)
		proj := parts[0]
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			perProject[proj]++
		}
	}
	return perProject
}

func (a *daemonStateAdapter) Status() api.StatusResponse {
	a.d.mu.RLock()
	defer a.d.mu.RUnlock()
	uptime := ""
	if !a.d.startedAt.IsZero() {
		uptime = time.Since(a.d.startedAt).Round(time.Second).String()
	}
	repos := a.d.repos.List()
	active := a.activePlansByProject()
	repoStatuses := make([]api.RepoStatus, len(repos))
	for i, r := range repos {
		repoStatuses[i] = api.RepoStatus{Path: r.Path, Project: r.Project, ActivePlans: active[r.Project]}
	}
	return api.StatusResponse{
		Running:   true,
		Repos:     repoStatuses,
		RepoCount: len(repoStatuses),
		Uptime:    uptime,
	}
}

func (a *daemonStateAdapter) ListRepos() []api.RepoStatus {
	active := a.activePlansByProject()
	repos := a.d.repos.List()
	out := make([]api.RepoStatus, len(repos))
	for i, r := range repos {
		out[i] = api.RepoStatus{Path: r.Path, Project: r.Project, ActivePlans: active[r.Project]}
	}
	return out
}

func (a *daemonStateAdapter) AddRepo(path string) error {
	return a.d.repos.Add(path)
}

func (a *daemonStateAdapter) RemoveRepo(project string) error {
	return a.d.repos.RemoveByProject(project)
}

func (a *daemonStateAdapter) ListPlans(project string) ([]taskstore.TaskEntry, error) {
	store, err := a.TaskStoreForProject(project)
	if err != nil {
		return nil, err
	}
	return store.List(project)
}

func taskStatusFromEntry(entry taskstore.TaskEntry) api.TaskStatus {
	return api.TaskStatus{
		Filename:       entry.Filename,
		Status:         string(entry.Status),
		ExecutionState: entry.ExecutionState,
		Branch:         entry.Branch,
		PRURL:          entry.PRURL,
		ReviewCycle:    entry.ReviewCycle,
		Description:    entry.Description,
		Topic:          entry.Topic,
	}
}

func (a *daemonStateAdapter) ListTasks(project string) ([]api.TaskStatus, error) {
	store, err := a.TaskStoreForProject(project)
	if err != nil {
		return nil, err
	}
	entries, err := store.List(project)
	if err != nil {
		return nil, err
	}
	tasks := make([]api.TaskStatus, 0, len(entries))
	for _, entry := range entries {
		tasks = append(tasks, taskStatusFromEntry(entry))
	}
	return tasks, nil
}

func (a *daemonStateAdapter) TaskStoreForProject(project string) (taskstore.Store, error) {
	entries := a.d.repos.List()
	for _, e := range entries {
		if e.Project != project {
			continue
		}
		if e.Store == nil {
			return nil, fmt.Errorf("%w: %s", api.ErrTaskStoreUnavailable, project)
		}
		return e.Store, nil
	}
	return nil, fmt.Errorf("%w: %s", api.ErrProjectNotFound, project)
}

func (a *daemonStateAdapter) SignalGatewayForProject(project string) (taskstore.SignalGateway, error) {
	entries := a.d.repos.List()
	for _, e := range entries {
		if e.Project != project {
			continue
		}
		if e.SignalGateway == nil {
			return nil, fmt.Errorf("%w: %s", api.ErrTaskStoreUnavailable, project)
		}
		return e.SignalGateway, nil
	}
	return nil, fmt.Errorf("%w: %s", api.ErrProjectNotFound, project)
}

func (a *daemonStateAdapter) ListInstances(project string) []api.InstanceStatus {
	entries := a.d.repos.List()
	for _, entry := range entries {
		if entry.Project != project {
			continue
		}
		tracked := a.d.spawner.InstancesForRepo(entry.Path)
		out := make([]api.InstanceStatus, 0, len(tracked))
		for _, inst := range tracked {
			if inst == nil {
				continue
			}
			active := !inst.Paused() && !inst.Exited && (inst.Started() || inst.Status == session.Loading)
			ready := active && inst.Status == session.Ready
			out = append(out, api.InstanceStatus{
				ID:            inst.Title,
				Project:       project,
				Plan:          inst.TaskFile,
				Role:          inst.AgentType,
				Active:        active,
				Loading:       inst.Status == session.Loading,
				Ready:         ready,
				Title:         inst.Title,
				Branch:        inst.Branch,
				Program:       inst.Program,
				TaskNumber:    inst.TaskNumber,
				WaveNumber:    inst.WaveNumber,
				ReviewCycle:   inst.ReviewCycle,
				WaveTaskIndex: inst.WaveTaskIndex,
				WaveTaskCount: inst.WaveTaskCount,
				ExecutionMode: string(session.NormalizeExecutionMode(inst.ExecutionMode)),
			})
		}
		return out
	}
	return nil
}

func (a *daemonStateAdapter) StartPlan(project, filename, prompt, program string) error {
	return a.d.StartPlan(project, filename, prompt, program)
}

func (a *daemonStateAdapter) EventStream() <-chan api.Event {
	if a.d.broadcaster != nil {
		return a.d.broadcaster.Subscribe()
	}
	return make(chan api.Event)
}

// repoPathByProject returns the file-system path for the registered repo with the
// given project name. Returns ("", false) when no matching project is registered.
func (a *daemonStateAdapter) repoPathByProject(project string) (string, bool) {
	for _, e := range a.d.repos.List() {
		if e.Project == project {
			return e.Path, true
		}
	}
	return "", false
}

// mapSpawnerInstanceErr translates internal spawner sentinel errors to the
// api-layer sentinels that daemon/api/server.go maps to HTTP status codes.
func (a *daemonStateAdapter) mapSpawnerInstanceErr(err error, project, title string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errSpawnerInstanceNotFound) {
		return fmt.Errorf("%w: %s/%s", api.ErrInstanceNotFound, project, title)
	}
	if errors.Is(err, errSpawnerInvalidTransition) {
		return fmt.Errorf("%w: %s/%s", api.ErrInvalidTransition, project, title)
	}
	return err
}

// PauseInstance implements StateProvider.
func (a *daemonStateAdapter) PauseInstance(project, title string) error {
	repoPath, ok := a.repoPathByProject(project)
	if !ok {
		return fmt.Errorf("%w: project %s", api.ErrProjectNotFound, project)
	}
	return a.mapSpawnerInstanceErr(a.d.spawner.PauseInstance(repoPath, title), project, title)
}

// ResumeInstance implements StateProvider.
func (a *daemonStateAdapter) ResumeInstance(project, title string) error {
	repoPath, ok := a.repoPathByProject(project)
	if !ok {
		return fmt.Errorf("%w: project %s", api.ErrProjectNotFound, project)
	}
	return a.mapSpawnerInstanceErr(a.d.spawner.ResumeInstance(repoPath, title), project, title)
}

// RestartInstance implements StateProvider.
func (a *daemonStateAdapter) RestartInstance(project, title string) error {
	repoPath, ok := a.repoPathByProject(project)
	if !ok {
		return fmt.Errorf("%w: project %s", api.ErrProjectNotFound, project)
	}
	return a.mapSpawnerInstanceErr(a.d.spawner.RestartInstance(repoPath, title), project, title)
}

// KillInstance implements StateProvider.
func (a *daemonStateAdapter) KillInstance(project, title string) error {
	repoPath, ok := a.repoPathByProject(project)
	if !ok {
		return fmt.Errorf("%w: project %s", api.ErrProjectNotFound, project)
	}
	return a.mapSpawnerInstanceErr(a.d.spawner.KillInstance(repoPath, title), project, title)
}

// NewDaemon creates a new Daemon from the given configuration. The daemon is
// not started until Run is called.
func NewDaemon(cfg *DaemonConfig) (*Daemon, error) {
	if cfg == nil {
		cfg = defaultDaemonConfig()
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.SocketPath == "" {
		cfg.SocketPath = defaultSocketPath()
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	repos := NewRepoManager()
	repos.autoAdvance = cfg.AutoAdvance
	repos.autoReviewFix = cfg.AutoReviewFix
	repos.autoReadinessReview = cfg.AutoReadinessReview
	repos.maxReviewFixCycles = cfg.MaxReviewFixCycles

	d := &Daemon{
		cfg:         cfg,
		repos:       repos,
		spawner:     newTmuxSpawner(logger),
		logger:      logger,
		broadcaster: api.NewEventBroadcaster(),
	}

	// Pre-register repos from config.
	for _, r := range cfg.Repos {
		if err := d.repos.Add(r); err != nil {
			return nil, fmt.Errorf("daemon: add repo %s: %w", r, err)
		}
	}

	if cfg.PRMonitor.Enabled {
		d.prMonitor = NewPRMonitor(cfg.PRMonitor, cfg.MaxReviewFixCycles, repos, d.broadcaster, logger, d.executeAction)
	}

	return d, nil
}

// defaultSocketPath returns the default Unix domain socket path for the daemon.
// It prefers $XDG_RUNTIME_DIR/kasmos/kas.sock, then falls back to
// /tmp/kasmos-<uid>/kas.sock.
func defaultSocketPath() string {
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "kasmos", "kas.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("kasmos-%d", os.Getuid()), "kas.sock")
}

// DefaultSocketPath returns the default Unix domain socket path for the daemon.
func DefaultSocketPath() string {
	return defaultSocketPath()
}

// AddRepo registers a repository root with the daemon. The repo will be
// polled on the next tick. Safe to call concurrently.
func (d *Daemon) AddRepo(root string) error {
	return d.repos.Add(root)
}

// StartPlan asks the daemon to spawn a planner session for the given plan.
func (d *Daemon) StartPlan(project, planFile, prompt, program string) error {
	var entry RepoEntry
	var found bool
	for _, repo := range d.repos.List() {
		if repo.Project == project {
			entry = repo
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("project not found: %s", project)
	}
	if entry.Store == nil {
		return fmt.Errorf("task store unavailable for project %s", project)
	}
	if _, err := entry.Store.Get(project, planFile); err != nil {
		return fmt.Errorf("load task entry for %s: %w", planFile, err)
	}
	go d.startPlanAsync(entry, planFile, prompt, program)
	return nil
}

func (d *Daemon) startPlanAsync(entry RepoEntry, planFile, prompt, program string) {
	killAgent := d.killAgent
	if killAgent == nil {
		killAgent = d.spawner.KillAgent
	}
	spawnPlanner := d.spawnPlanner
	if spawnPlanner == nil {
		spawnPlanner = d.spawner.SpawnPlanner
	}

	if err := killAgent(entry.Path, planFile, session.AgentTypePlanner); err != nil {
		d.logger.Error("kill existing planner failed", "project", entry.Project, "plan", planFile, "err", err)
		return
	}
	if err := spawnPlanner(context.Background(), loop.SpawnOpts{
		PlanFile: planFile,
		RepoPath: entry.Path,
		Project:  entry.Project,
		Prompt:   prompt,
		Program:  program,
	}); err != nil {
		d.logger.Error("spawn planner failed", "project", entry.Project, "plan", planFile, "err", err)
		return
	}
	if d.broadcaster != nil {
		d.broadcaster.Emit(api.Event{
			Kind:      api.EventKindAgentSpawned,
			Message:   "planner spawned for " + planFile,
			Repo:      entry.Path,
			PlanFile:  planFile,
			AgentType: session.AgentTypePlanner,
		})
	}
}

// ListRepos returns the current list of registered repo root paths.
func (d *Daemon) ListRepos() []string {
	entries := d.repos.List()
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Path
	}
	return out
}

// Run starts the daemon event loop. It blocks until ctx is cancelled, then
// performs a clean shutdown.
//
// The event loop:
//  1. Creates a ticker at cfg.PollInterval.
//  2. Listens on the Unix domain socket (cfg.SocketPath) and serves the
//     control API via api.Handler.
//  3. On each tick: scans all registered repos for signal files, feeds results
//     to per-repo Processor.Tick(), and executes the returned actions.
//  4. On context cancellation: releases the PID lock and closes the socket.
func (d *Daemon) Run(ctx context.Context) error {
	d.logger.Info("daemon starting", "poll_interval", d.cfg.PollInterval, "socket", d.cfg.SocketPath)

	d.mu.Lock()
	d.startedAt = time.Now()
	d.mu.Unlock()

	// Ensure the socket directory exists.
	if err := os.MkdirAll(filepath.Dir(d.cfg.SocketPath), 0o700); err != nil {
		return fmt.Errorf("daemon: create socket dir: %w", err)
	}

	lock, err := AcquirePIDLock(d.pidLockPath())
	if err != nil {
		return fmt.Errorf("daemon: acquire pid lock: %w", err)
	}
	d.pidLock = lock

	// Ensure signal directories exist and recover any in-flight signals that
	// were interrupted by a previous crash before beginning the poll loop.
	for _, e := range d.repos.List() {
		allSignalDirs := []string{filepath.Join(e.Path, ".kasmos", "signals")}
		for _, wt := range sharedWorktreePaths(e.Path) {
			allSignalDirs = append(allSignalDirs, filepath.Join(wt, ".kasmos", "signals"))
		}
		for _, sd := range allSignalDirs {
			if ensErr := taskfsm.EnsureSignalDirs(sd); ensErr != nil {
				d.logger.Warn("ensure signal dirs failed on startup", "dir", sd, "err", ensErr)
				continue
			}
			if n := taskfsm.RecoverInFlight(sd); n > 0 {
				d.logger.Info("recovered in-flight signals", "dir", sd, "count", n)
			}
		}
	}

	if recovered, recErr := d.RecoverSessions(); recErr != nil {
		d.logger.Warn("recover sessions failed", "err", recErr)
	} else if recovered > 0 {
		d.logger.Info("recovered orphan sessions", "count", recovered)
	}

	// Warn about binary-path skew in each registered repo's project files.
	// This is best-effort: errors are swallowed so they never block startup.
	for _, e := range d.repos.List() {
		warnBinaryPathSkew(d.logger, e.Path)
	}

	// Remove any stale socket file before listening.
	_ = os.Remove(d.cfg.SocketPath)

	ln, err := net.Listen("unix", d.cfg.SocketPath)
	if err != nil {
		_ = d.pidLock.Release()
		d.pidLock = nil
		return fmt.Errorf("daemon: listen unix %s: %w", d.cfg.SocketPath, err)
	}
	defer func() {
		ln.Close()
		_ = os.Remove(d.cfg.SocketPath)
	}()

	// Build and start the HTTP server on the control socket.
	// Use NewHandlerWithBroadcaster so each connecting client gets its own
	// subscription to the live event stream rather than a dead channel.
	state := &daemonStateAdapter{d: d}
	handler := api.NewHandlerWithBroadcaster(state, d.broadcaster)
	srv := &http.Server{Handler: handler}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			d.logger.Error("control socket server error", "err", serveErr)
		}
	}()

	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	// Reaper goroutine: reset signals stuck in "processing" for >60s.
	reaperTicker := time.NewTicker(30 * time.Second)
	defer reaperTicker.Stop()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-reaperTicker.C:
				reapStuckSignals(d.repos.List(), 60*time.Second, d.logger)
			}
		}
	}()

	// PR monitor goroutine: poll open pull requests for new review comments.
	if d.prMonitor != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := d.prMonitor.Run(ctx); err != nil {
				if ctx.Err() == nil {
					// Monitor exited while context is still live — log as a warning.
					d.logger.Warn("pr monitor exited unexpectedly", "err", err)
				}
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("daemon shutting down")

			// Drain all running agent instances before closing the control socket.
			drainCtx, drainCancel := context.WithTimeout(context.Background(), 35*time.Second)
			d.spawner.DrainAll(drainCtx)
			drainCancel()

			// Close the broadcaster first so any active SSE handlers on
			// /v1/events see their subscription channel close and return,
			// otherwise srv.Shutdown would block on them indefinitely.
			d.broadcaster.Close()

			shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if shutErr := srv.Shutdown(shutCtx); shutErr != nil {
				d.logger.Warn("control socket shutdown error", "err", shutErr)
			}
			shutCancel()
			wg.Wait()
			if d.pidLock != nil {
				_ = d.pidLock.Release()
				d.pidLock = nil
			}
			// Release the shared global taskstore connection pool.
			d.repos.Close()
			return nil

		case <-ticker.C:
			d.tick(ctx)
		}
	}
}

// tick executes one poll cycle across all registered repos using atomic
// per-signal processing: each signal is moved to processing/ before handling,
// then either completed (deleted) or dead-lettered into failed/.
func (d *Daemon) tick(ctx context.Context) {
	for _, e := range d.repos.List() {
		d.tickRepo(ctx, e)
	}
}

// tickRepo executes one poll cycle for a single repo entry.
// Prefer the DB-backed signal gateway when available; otherwise process the
// repo's filesystem sentinels directly.
func (d *Daemon) tickRepo(ctx context.Context, e RepoEntry) {
	if e.Store == nil || e.Processor == nil {
		// Processor requires a store; skip repos whose store is unavailable.
		return
	}

	if e.SignalGateway == nil {
		// Filesystem-only path for repos without a usable signal gateway.
		scan := loop.ScanAllSignals(e.Path, sharedWorktreePaths(e.Path))

		var actions []loop.Action

		// --- FSM signals ---
		for _, sig := range scan.FSMSignals {
			sigDir := sig.Dir()
			sigFile := sig.Filename()

			if err := taskfsm.EnsureSignalDirs(sigDir); err != nil {
				d.logger.Warn("ensure signal dirs failed", "file", sigFile, "repo", e.Path, "err", err)
				continue
			}
			procPath, err := taskfsm.BeginProcessing(sigDir, sigFile)
			if err != nil {
				d.logger.Warn("begin processing failed", "file", sigFile, "repo", e.Path, "err", err)
				continue
			}
			d.logger.Info("processing fsm signal", "file", sigFile, "event", sig.Event, "repo", e.Path)

			acts := e.Processor.ProcessFSMSignals([]taskfsm.Signal{sig})
			if len(acts) > 0 {
				actions = append(actions, acts...)
				taskfsm.CompleteProcessing(procPath)
			} else if sig.Event == taskfsm.ImplementFinished {
				// Benign suppressed/duplicate implement-finished — wave orchestrator
				// owns this transition; silently complete without dead-lettering.
				d.logger.Info("suppressed implement-finished signal", "file", sigFile, "plan", sig.TaskFile, "repo", e.Path)
				taskfsm.CompleteProcessing(procPath)
			} else {
				d.logger.Warn("dead-lettering fsm signal", "file", sigFile, "event", sig.Event, "repo", e.Path)
				taskfsm.FailProcessing(sigDir, sigFile, "signal rejected by processor")
			}
		}

		// --- Task signals ---
		for _, ts := range scan.TaskSignals {
			sigDir := ts.Dir()
			sigFile := ts.Filename()

			if err := taskfsm.EnsureSignalDirs(sigDir); err != nil {
				d.logger.Warn("ensure signal dirs failed", "file", sigFile, "repo", e.Path, "err", err)
				continue
			}
			procPath, err := taskfsm.BeginProcessing(sigDir, sigFile)
			if err != nil {
				d.logger.Warn("begin processing failed", "file", sigFile, "repo", e.Path, "err", err)
				continue
			}
			d.logger.Info("processing task signal", "file", sigFile, "repo", e.Path)

			acts := e.Processor.ProcessTaskSignals([]taskfsm.TaskSignal{ts})
			if len(acts) > 0 {
				actions = append(actions, acts...)
				taskfsm.CompleteProcessing(procPath)
			} else {
				d.logger.Warn("dead-lettering task signal", "file", sigFile, "repo", e.Path)
				taskfsm.FailProcessing(sigDir, sigFile, "no active orchestrator / wrong wave / already-finished task")
			}
		}

		// --- Wave signals ---
		for _, ws := range scan.WaveSignals {
			sigDir := ws.Dir()
			sigFile := ws.Filename()

			if err := taskfsm.EnsureSignalDirs(sigDir); err != nil {
				d.logger.Warn("ensure signal dirs failed", "file", sigFile, "repo", e.Path, "err", err)
				continue
			}
			procPath, err := taskfsm.BeginProcessing(sigDir, sigFile)
			if err != nil {
				d.logger.Warn("begin processing failed", "file", sigFile, "repo", e.Path, "err", err)
				continue
			}
			d.logger.Info("processing wave signal", "file", sigFile, "repo", e.Path)

			acts := e.Processor.ProcessWaveSignals([]taskfsm.WaveSignal{ws})
			if len(acts) > 0 {
				actions = append(actions, acts...)
				taskfsm.CompleteProcessing(procPath)
			} else {
				d.logger.Warn("dead-lettering wave signal", "file", sigFile, "repo", e.Path)
				taskfsm.FailProcessing(sigDir, sigFile, "processor could not start the requested wave")
			}
		}

		// --- Architect-pass completion signals ---
		for _, es := range scan.ElaborationSignals {
			sigDir := es.Dir()
			sigFile := es.Filename()

			if err := taskfsm.EnsureSignalDirs(sigDir); err != nil {
				d.logger.Warn("ensure signal dirs failed", "file", sigFile, "repo", e.Path, "err", err)
				continue
			}
			procPath, err := taskfsm.BeginProcessing(sigDir, sigFile)
			if err != nil {
				d.logger.Warn("begin processing failed", "file", sigFile, "repo", e.Path, "err", err)
				continue
			}
			d.logger.Info("processing architect completion signal", "file", sigFile, "repo", e.Path)

			acts := e.Processor.ProcessElaborationSignals([]taskfsm.ElaborationSignal{es})
			if len(acts) > 0 {
				actions = append(actions, acts...)
				taskfsm.CompleteProcessing(procPath)
			} else {
				d.logger.Warn("dead-lettering architect completion signal", "file", sigFile, "repo", e.Path)
				taskfsm.FailProcessing(sigDir, sigFile, "no active architect pass to resume")
			}
		}

		for _, action := range actions {
			d.logger.Info("executing action", "kind", action.Kind(), "repo", e.Path)
			if err := d.executeAction(ctx, e, action); err != nil {
				d.logger.Error("execute action failed", "kind", action.Kind(), "repo", e.Path, "err", err)
			}
		}
		d.monitorRunningInstances(ctx, e)
		return
	}

	// DB-backed gateway path.
	workerID := fmt.Sprintf("daemon:%s:%d", e.Project, os.Getpid())

	if _, err := loop.BridgeFilesystemSignals(e.SignalGateway, e.Project, e.Path, sharedWorktreePaths(e.Path)); err != nil {
		d.logger.Error("bridge filesystem signals failed", "repo", e.Path, "err", err)
		return
	}

	for {
		entry, err := e.SignalGateway.Claim(e.Project, workerID)
		if err != nil {
			d.logger.Error("claim gateway signal failed", "repo", e.Path, "err", err)
			return
		}
		if entry == nil {
			break
		}

		var scan loop.ScanResult
		if err := loop.ConvertSignalEntry(entry, &scan); err != nil {
			if markErr := e.SignalGateway.MarkProcessed(entry.ID, taskstore.SignalFailed, err.Error()); markErr != nil {
				d.logger.Error("mark failed signal failed", "repo", e.Path, "id", entry.ID, "err", markErr)
			}
			continue
		}

		actions := e.Processor.Tick(scan)
		if len(actions) == 0 {
			status, result := gatewayNoopOutcome(entry)
			if err := e.SignalGateway.MarkProcessed(entry.ID, status, result); err != nil {
				d.logger.Error("mark noop signal failed", "repo", e.Path, "id", entry.ID, "err", err)
			}
			continue
		}

		for _, action := range actions {
			if err := d.executeAction(ctx, e, action); err != nil {
				d.logger.Error("execute action failed", "kind", action.Kind(), "repo", e.Path, "err", err)
			}
		}

		if err := e.SignalGateway.MarkProcessed(entry.ID, taskstore.SignalDone, ""); err != nil {
			d.logger.Error("mark processed failed", "repo", e.Path, "id", entry.ID, "err", err)
		}
	}

	d.monitorRunningInstances(ctx, e)
}

func setRepoExecutionState(e RepoEntry, planFile string, state taskstore.ExecutionState) error {
	if e.Store == nil {
		return fmt.Errorf("task store unavailable for %s", planFile)
	}
	ps, err := taskstate.Load(e.Store, e.Project, "")
	if err != nil {
		return err
	}
	return ps.SetExecutionState(planFile, state)
}

func clearRepoExecutionState(e RepoEntry, planFile string) error {
	if e.Store == nil {
		return fmt.Errorf("task store unavailable for %s", planFile)
	}
	ps, err := taskstate.Load(e.Store, e.Project, "")
	if err != nil {
		return err
	}
	return ps.ClearExecutionState(planFile)
}

func (d *Daemon) plannerStillTracked(repoPath, planFile string) bool {
	if d.spawner == nil {
		return false
	}

	key := instanceKey(repoPath, planFile, session.AgentTypePlanner)
	for _, inst := range d.spawner.RunningInstances() {
		if inst.Key == key {
			return true
		}
	}
	return false
}

func (d *Daemon) blueprintSkipThreshold(repoPath string) int {
	const defaultThreshold = 2

	path := filepath.Join(repoPath, ".kasmos", config.TOMLConfigFileName)
	result, err := config.LoadTOMLConfigFrom(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			d.logger.Warn("load repo config for blueprint-skip failed", "repo", repoPath, "err", err)
		}
		return defaultThreshold
	}
	if result == nil || result.BlueprintSkipThreshold == nil {
		return defaultThreshold
	}
	return *result.BlueprintSkipThreshold
}

func (d *Daemon) autoImplementPlan(ctx context.Context, e RepoEntry, planFile string) error {
	if e.Store == nil {
		return fmt.Errorf("task store unavailable for %s", planFile)
	}

	forceKillAgent := d.forceKillAgent
	if forceKillAgent == nil {
		forceKillAgent = d.spawner.ForceKillAgent
	}
	killErr := forceKillAgent(e.Path, planFile, session.AgentTypePlanner)
	if killErr != nil {
		d.logger.Warn("kill planner before auto-implement failed", "repo", e.Path, "plan", planFile, "err", killErr)
	}
	if d.plannerStillTracked(e.Path, planFile) {
		if killErr != nil {
			return fmt.Errorf("planner agent still tracked for %s after cleanup failure: %w", planFile, killErr)
		}
		return fmt.Errorf("planner agent still tracked for %s after cleanup", planFile)
	}

	content, err := e.Store.GetContent(e.Project, planFile)
	if err != nil {
		return fmt.Errorf("load plan content for %s: %w", planFile, err)
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("load plan content for %s: empty content", planFile)
	}

	plan, err := taskparser.Parse(content)
	if err != nil {
		return fmt.Errorf("parse plan content for %s: %w", planFile, err)
	}

	if err := taskfsm.New(e.Store, e.Project, "").Transition(planFile, taskfsm.ImplementStart); err != nil {
		return fmt.Errorf("transition %s to implementing: %w", planFile, err)
	}

	if orchestration.ShouldBlueprintSkip(plan, d.blueprintSkipThreshold(e.Path)) {
		if e.Processor != nil {
			e.Processor.ClearWaveOrchestrator(planFile)
		}
		if err := setRepoExecutionState(e, planFile, taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseSingleAgentImplementing),
			ActiveAgentType: session.AgentTypeCoder,
		}); err != nil {
			return fmt.Errorf("persist single-agent execution state for %s: %w", planFile, err)
		}
		entry, err := e.Store.Get(e.Project, planFile)
		if err != nil {
			return fmt.Errorf("load task entry for %s: %w", planFile, err)
		}
		prompt := orchestration.BuildBlueprintSkipPrompt(planFile, plan, e.Project)
		return d.executeAction(ctx, e, loop.SpawnCoderAction{
			PlanFile: entry.Filename,
			Feedback: prompt,
		})
	}

	return d.executeAction(ctx, e, loop.SpawnElaboratorAction{PlanFile: planFile})
}

func (d *Daemon) pushInstanceBranch(inst *session.Instance) error {
	if d.pushBranch != nil {
		return d.pushBranch(inst)
	}
	worktree, err := inst.GetGitWorktree()
	if err != nil {
		return err
	}
	return worktree.Push(false)
}

func (d *Daemon) autoAdvanceCompletedImplementer(e RepoEntry, inst *session.Instance, tmuxAlive bool) (bool, error) {
	if e.Store == nil || inst == nil || inst.TaskFile == "" {
		return false, nil
	}

	entry, err := e.Store.Get(e.Project, inst.TaskFile)
	if err != nil {
		return false, fmt.Errorf("load task entry for %s: %w", inst.TaskFile, err)
	}
	if !session.ShouldAutoAdvanceLifecycleImplementer(string(entry.Status), entry.ExecutionState, inst, tmuxAlive) {
		return false, nil
	}

	if err := d.pushInstanceBranch(inst); err != nil {
		return false, fmt.Errorf("push branch for %s: %w", inst.Title, err)
	}

	fsm := taskfsm.New(e.Store, e.Project, "")
	if err := fsm.Transition(inst.TaskFile, taskfsm.ImplementFinished); err != nil {
		return false, fmt.Errorf("transition %s to reviewing: %w", inst.TaskFile, err)
	}
	if err := setRepoExecutionState(e, inst.TaskFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseReviewing),
		ActiveAgentType: session.AgentTypeReviewer,
	}); err != nil {
		return false, fmt.Errorf("persist reviewing execution state for %s: %w", inst.TaskFile, err)
	}

	return true, nil
}

func shouldProcessWaveTaskCompletion(entry taskstore.TaskEntry, inst *session.Instance, tmuxAlive bool) (taskfsm.TaskSignal, bool) {
	if inst == nil || inst.TaskFile == "" || inst.TaskNumber < 1 {
		return taskfsm.TaskSignal{}, false
	}
	if strings.TrimSpace(string(entry.Status)) != string(taskfsm.StatusImplementing) {
		return taskfsm.TaskSignal{}, false
	}
	if taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase) != taskfsm.ExecutionPhaseWaveRunning {
		return taskfsm.TaskSignal{}, false
	}
	if inst.ImplementationComplete || !inst.HasWorked {
		return taskfsm.TaskSignal{}, false
	}

	waveNumber := inst.WaveNumber
	if waveNumber == 0 {
		waveNumber = entry.ExecutionState.ActiveWave
	}
	if waveNumber < 1 {
		return taskfsm.TaskSignal{}, false
	}
	if entry.ExecutionState.ActiveWave > 0 && waveNumber != entry.ExecutionState.ActiveWave {
		return taskfsm.TaskSignal{}, false
	}

	finished := false
	if inst.HasStableCompletionPrompt(time.Now()) {
		finished = true
	}
	if !tmuxAlive {
		finished = true
	}
	if !finished {
		return taskfsm.TaskSignal{}, false
	}

	return taskfsm.TaskSignal{TaskFile: inst.TaskFile, WaveNumber: waveNumber, TaskNumber: inst.TaskNumber}, true
}

func (d *Daemon) processCompletedWaveTask(ctx context.Context, e RepoEntry, inst *session.Instance, tmuxAlive bool) (bool, error) {
	if e.Store == nil || e.Processor == nil || inst == nil || inst.TaskFile == "" {
		return false, nil
	}

	entry, err := e.Store.Get(e.Project, inst.TaskFile)
	if err != nil {
		return false, fmt.Errorf("load task entry for %s: %w", inst.TaskFile, err)
	}

	ts, ok := shouldProcessWaveTaskCompletion(entry, inst, tmuxAlive)
	if !ok {
		return false, nil
	}

	actions := e.Processor.ProcessTaskSignals([]taskfsm.TaskSignal{ts})
	if len(actions) == 0 {
		return false, nil
	}

	for _, action := range actions {
		if err := d.executeAction(ctx, e, action); err != nil {
			return false, err
		}
	}

	inst.ImplementationComplete = true
	if !tmuxAlive {
		inst.Exited = true
		if inst.Status == session.Running {
			inst.SetStatus(session.Ready)
		}
	}

	return true, nil
}

func (d *Daemon) monitorRunningInstances(ctx context.Context, e RepoEntry) {
	now := time.Now()
	for _, inst := range d.spawner.InstancesForRepo(e.Path) {
		if inst == nil || inst.Paused() || !inst.Started() {
			continue
		}

		md := inst.CollectMetadata()
		if md.TmuxAlive && inst.Exited {
			inst.Exited = false
		}
		if md.ContentCaptured {
			if md.HasPrompt {
				inst.SetStatus(session.Ready)
				inst.PromptDetected = true
				// Skip TapEnter for permission prompts — avoid sending
				// literal "1" as text when both auto_yes and permission
				// bypass are active.
				if md.PermissionPrompt == nil {
					inst.TapEnter()
				}
			} else if md.Updated {
				inst.SetStatus(session.Running)
				if inst.TaskNumber > 0 && !inst.HasWorked && inst.QueuedPrompt == "" {
					inst.HasWorked = true
				}
			} else {
				inst.SetStatus(session.Ready)
			}
		}

		inst.PermissionBlocked = (md.PermissionPrompt != nil)
		inst.UpdateCompletionPromptState(now)

		completedWaveTask, err := d.processCompletedWaveTask(ctx, e, inst, md.TmuxAlive)
		if err != nil {
			d.logger.Warn("wave task completion handling failed", "repo", e.Path, "plan", inst.TaskFile, "instance", inst.Title, "err", err)
			continue
		}
		if completedWaveTask {
			continue
		}

		advanced, err := d.autoAdvanceCompletedImplementer(e, inst, md.TmuxAlive)
		if err != nil {
			d.logger.Warn("auto-advance implementer failed", "repo", e.Path, "plan", inst.TaskFile, "instance", inst.Title, "err", err)
			continue
		}
		if !advanced {
			if md.TmuxAlive || inst.Exited {
				continue
			}

			inst.Exited = true
			if inst.Status == session.Running {
				inst.SetStatus(session.Ready)
			}

			if e.Store == nil || inst.TaskFile == "" {
				continue
			}

			entry, err := e.Store.Get(e.Project, inst.TaskFile)
			if err != nil {
				d.logger.Warn("load task entry for exited instance failed", "repo", e.Path, "plan", inst.TaskFile, "instance", inst.Title, "err", err)
				continue
			}
			if session.IsStuck(entry, inst, md.TmuxAlive) {
				d.broadcaster.Emit(api.Event{
					Kind:      api.EventKindStuckDetected,
					Message:   "agent exited without an auto-advance path for " + inst.TaskFile,
					Repo:      e.Path,
					PlanFile:  inst.TaskFile,
					AgentType: inst.AgentType,
				})
			}
			continue
		}

		d.logger.Info("implementer completed; starting review", "repo", e.Path, "plan", inst.TaskFile, "instance", inst.Title, "agent", inst.AgentType)
		if err := d.spawner.KillAgent(e.Path, inst.TaskFile, inst.AgentType); err != nil {
			d.logger.Warn("kill completed implementer failed", "repo", e.Path, "plan", inst.TaskFile, "agent", inst.AgentType, "err", err)
		}
		_ = d.executeAction(ctx, e, loop.TransitionAction{PlanFile: inst.TaskFile, Event: taskfsm.ImplementFinished})
		if err := d.executeAction(ctx, e, loop.SpawnReviewerAction{PlanFile: inst.TaskFile}); err != nil {
			d.logger.Error("spawn reviewer after implementer completion failed", "repo", e.Path, "plan", inst.TaskFile, "err", err)
		}
	}
}

func gatewayNoopOutcome(entry *taskstore.SignalEntry) (taskstore.SignalStatus, string) {
	canonicalType, err := taskfsm.CanonicalGatewaySignalType(entry.SignalType)
	if err != nil {
		return taskstore.SignalFailed, "signal rejected by processor"
	}
	internalType := canonicalType
	if canonicalType == "elaborator_finished" {
		internalType = string(taskfsm.ArchitectFinished)
	}
	switch internalType {
	case "implement_finished":
		return taskstore.SignalDone, "suppressed implement-finished signal"
	case "implement_task_finished":
		return taskstore.SignalFailed, "no active orchestrator / wrong wave / already-finished task"
	case "implement_wave":
		return taskstore.SignalFailed, "processor could not start the requested wave"
	case string(taskfsm.ArchitectFinished):
		return taskstore.SignalFailed, "no active architect pass to resume"
	case string(taskfsm.VerifyApproved), string(taskfsm.VerifyFailed):
		return taskstore.SignalFailed, "signal rejected outside verifying state"
	default:
		return taskstore.SignalFailed, "signal rejected by processor"
	}
}

// reapStuckSignals resets signals that have been stuck in "processing" for
// longer than timeout across all repos with a SignalGateway. Returns the
// total count of signals reset.
func reapStuckSignals(repos []RepoEntry, timeout time.Duration, logger *slog.Logger) int {
	total := 0
	for _, e := range repos {
		if e.SignalGateway == nil {
			continue
		}
		n, err := e.SignalGateway.ResetStuck(timeout)
		if err != nil {
			logger.Error("reap stuck signals failed", "repo", e.Path, "project", e.Project, "err", err)
			continue
		}
		total += n
	}
	return total
}

// executeAction dispatches a single action to the configured spawner.
// It resolves RepoPath from e.Path and looks up Branch from the task store so
// that spawnInSharedWorktree has the required context.
// Returns an error if the action fails so that callers (e.g. PRMonitor) can
// decide whether to persist side-effects such as MarkReviewFixerDispatched.
func (d *Daemon) executeAction(ctx context.Context, e RepoEntry, action loop.Action) error {
	// branchFor looks up the git branch for a plan from the task store.
	branchFor := func(planFile string) string {
		if e.Store == nil {
			return ""
		}
		entry, err := e.Store.Get(e.Project, planFile)
		if err != nil {
			return ""
		}
		return entry.Branch
	}
	entryFor := func(planFile string) taskstore.TaskEntry {
		if e.Store == nil {
			return taskstore.TaskEntry{}
		}
		entry, err := e.Store.Get(e.Project, planFile)
		if err != nil {
			return taskstore.TaskEntry{}
		}
		return entry
	}

	switch a := action.(type) {
	case loop.ReviewChangesAction:
		if e.Store != nil {
			ps, err := taskstate.Load(e.Store, e.Project, "")
			if err == nil {
				if setErr := ps.SetLatestReviewFeedback(a.PlanFile, a.Feedback); setErr != nil {
					d.logger.Warn("persist latest review feedback failed", "plan", a.PlanFile, "err", setErr)
				}
			}
		}
		return nil
	case loop.SpawnReviewerAction:
		if err := setRepoExecutionState(e, a.PlanFile, taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		}); err != nil {
			return fmt.Errorf("persist reviewer execution state: %w", err)
		}
		opts := reviewerSpawnOpts(e, entryFor(a.PlanFile))
		spawnReviewer := d.spawnReviewer
		if spawnReviewer == nil {
			spawnReviewer = d.spawner.SpawnReviewer
		}
		if err := spawnReviewer(ctx, opts); err != nil {
			d.logger.Error("spawn reviewer failed", "plan", a.PlanFile, "err", err)
			return err
		}
		d.broadcaster.Emit(api.Event{
			Kind:      api.EventKindAgentSpawned,
			Message:   "reviewer spawned for " + a.PlanFile,
			Repo:      e.Path,
			PlanFile:  a.PlanFile,
			AgentType: "reviewer",
		})
		return nil
	case loop.SpawnCoderAction:
		opts := coderSpawnOpts(e, a.PlanFile, branchFor(a.PlanFile), a.Feedback)
		spawnCoder := d.spawnCoder
		if spawnCoder == nil {
			spawnCoder = d.spawner.SpawnCoder
		}
		if err := spawnCoder(ctx, opts); err != nil {
			d.logger.Error("spawn coder failed", "plan", a.PlanFile, "err", err)
			return err
		}
		d.broadcaster.Emit(api.Event{
			Kind:      api.EventKindAgentSpawned,
			Message:   "coder spawned for " + a.PlanFile,
			Repo:      e.Path,
			PlanFile:  a.PlanFile,
			AgentType: "coder",
		})
		return nil
	case loop.SpawnPlannerAction:
		// Kill any existing planner for this plan before spawning a new one —
		// matches the StartPlan path the TUI uses so a retry from the admin UI
		// never races with a stale planner.
		killAgent := d.killAgent
		if killAgent == nil {
			killAgent = d.spawner.KillAgent
		}
		if err := killAgent(e.Path, a.PlanFile, session.AgentTypePlanner); err != nil {
			d.logger.Error("kill existing planner failed", "plan", a.PlanFile, "err", err)
			return err
		}
		entry := entryFor(a.PlanFile)
		spec := orchestration.BuildPlannerAgentSpec(a.PlanFile, e.Project, entry.Description)
		opts := loop.SpawnOpts{
			PlanFile: a.PlanFile,
			RepoPath: e.Path,
			Project:  e.Project,
			Program:  programForAgent(e.Path, session.AgentTypePlanner),
			Prompt:   spec.Prompt,
		}
		spawnPlanner := d.spawnPlanner
		if spawnPlanner == nil {
			spawnPlanner = d.spawner.SpawnPlanner
		}
		if err := spawnPlanner(ctx, opts); err != nil {
			d.logger.Error("spawn planner failed", "plan", a.PlanFile, "err", err)
			return err
		}
		d.broadcaster.Emit(api.Event{
			Kind:      api.EventKindAgentSpawned,
			Message:   "planner spawned for " + a.PlanFile,
			Repo:      e.Path,
			PlanFile:  a.PlanFile,
			AgentType: session.AgentTypePlanner,
		})
		return nil
	case loop.SpawnElaboratorAction:
		if err := setRepoExecutionState(e, a.PlanFile, taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseArchitecting),
			ActiveAgentType: session.AgentTypeElaborator,
		}); err != nil {
			return fmt.Errorf("persist architect execution state: %w", err)
		}
		spec := orchestration.BuildArchitectAgentSpec(a.PlanFile, e.Project)
		opts := loop.SpawnOpts{
			PlanFile: a.PlanFile,
			RepoPath: e.Path,
			Project:  e.Project,
			Program:  programForAgent(e.Path, session.AgentTypeElaborator),
			Prompt:   spec.Prompt,
		}
		spawnElaborator := d.spawnElaborator
		if spawnElaborator == nil {
			spawnElaborator = d.spawner.SpawnElaborator
		}
		if err := spawnElaborator(ctx, opts); err != nil {
			d.logger.Error("spawn architect failed", "plan", a.PlanFile, "err", err)
			return err
		}
		d.broadcaster.Emit(api.Event{
			Kind:      api.EventKindAgentSpawned,
			Message:   "architect spawned for " + a.PlanFile,
			Repo:      e.Path,
			PlanFile:  a.PlanFile,
			AgentType: session.AgentTypeElaborator,
		})
		return nil
	case loop.SpawnFixerAction:
		if err := setRepoExecutionState(e, a.PlanFile, taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseFixing),
			ActiveAgentType: session.AgentTypeFixer,
		}); err != nil {
			return fmt.Errorf("persist fixer execution state: %w", err)
		}
		opts := fixerSpawnOpts(e, a.PlanFile, branchFor(a.PlanFile), a.Feedback)
		spawnFixer := d.spawnFixer
		if spawnFixer == nil {
			spawnFixer = d.spawner.SpawnFixer
		}
		if err := spawnFixer(ctx, opts); err != nil {
			d.logger.Error("spawn fixer failed", "plan", a.PlanFile, "err", err)
			return err
		}
		d.broadcaster.Emit(api.Event{
			Kind:      api.EventKindAgentSpawned,
			Message:   "fixer spawned for " + a.PlanFile,
			Repo:      e.Path,
			PlanFile:  a.PlanFile,
			AgentType: "fixer",
		})
		return nil
	case loop.SpawnMasterAction:
		if err := setRepoExecutionState(e, a.PlanFile, taskstore.ExecutionState{
			ActiveAgentType: session.AgentTypeMaster,
		}); err != nil {
			return fmt.Errorf("persist master execution state: %w", err)
		}
		opts := masterSpawnOpts(e, entryFor(a.PlanFile))
		spawnMaster := d.spawnMaster
		if spawnMaster == nil {
			spawnMaster = d.spawner.SpawnMaster
		}
		if err := spawnMaster(ctx, opts); err != nil {
			d.logger.Error("spawn master failed", "plan", a.PlanFile, "err", err)
			return err
		}
		d.broadcaster.Emit(api.Event{
			Kind:      api.EventKindAgentSpawned,
			Message:   "master spawned for " + a.PlanFile,
			Repo:      e.Path,
			PlanFile:  a.PlanFile,
			AgentType: session.AgentTypeMaster,
		})
		return nil
	case loop.PausePlanAgentAction:
		if err := d.spawner.KillAgent(e.Path, a.PlanFile, a.AgentType); err != nil {
			d.logger.Error("kill agent failed", "plan", a.PlanFile, "type", a.AgentType, "err", err)
			return err
		}
		d.broadcaster.Emit(api.Event{
			Kind:      "agent_killed",
			Message:   a.AgentType + " killed for " + a.PlanFile,
			Repo:      e.Path,
			PlanFile:  a.PlanFile,
			AgentType: a.AgentType,
		})
		return nil
	case loop.AutoImplementAction:
		if d.cfg == nil || !d.cfg.AutoAdvance {
			return nil
		}
		return d.autoImplementPlan(ctx, e, a.PlanFile)
	case loop.AdvanceWaveAction:
		return d.startWaveTasks(ctx, e, a.PlanFile)
	case loop.TaskCompleteAction:
		return d.handleWaveTaskComplete(ctx, e, a)
	case loop.TransitionAction:
		d.logger.Info("fsm transition", "plan", a.PlanFile, "event", a.Event, "repo", e.Path)
		d.broadcaster.Emit(api.Event{
			Kind:     "transition_applied",
			Message:  fmt.Sprintf("fsm event %v for %s", a.Event, a.PlanFile),
			Repo:     e.Path,
			PlanFile: a.PlanFile,
		})
		return nil
	case loop.ReviewApprovedAction:
		// Lightweight: reviewer side-effects only. Execution state is cleared in
		// VerifyApprovedAction so that recovery works during the verifying window.
		d.logger.Info("reviewer approved", "plan", a.PlanFile, "repo", e.Path)
		d.broadcaster.Emit(api.Event{
			Kind:     "review_approved",
			Message:  "reviewer approved " + a.PlanFile,
			Repo:     e.Path,
			PlanFile: a.PlanFile,
		})
		return nil
	case loop.VerifyApprovedAction:
		// Terminal lifecycle event: verification passed, task moves to done.
		if err := clearRepoExecutionState(e, a.PlanFile); err != nil {
			return fmt.Errorf("clear execution state after verify approval: %w", err)
		}
		d.logger.Info("verify approved", "plan", a.PlanFile, "repo", e.Path)
		d.broadcaster.Emit(api.Event{
			Kind:     "signal_processed",
			Message:  "verify approved for " + a.PlanFile,
			Repo:     e.Path,
			PlanFile: a.PlanFile,
		})
		return nil
	case loop.VerifyFailedAction:
		// Persist latest feedback so fixer agents receive it via SpawnFixer opts.
		if e.Store != nil {
			ps, err := taskstate.Load(e.Store, e.Project, "")
			if err == nil {
				if setErr := ps.SetLatestReviewFeedback(a.PlanFile, a.Feedback); setErr != nil {
					d.logger.Warn("persist verify failed feedback failed", "plan", a.PlanFile, "err", setErr)
				}
			}
		}
		d.logger.Info("verify failed", "plan", a.PlanFile, "repo", e.Path)
		d.broadcaster.Emit(api.Event{
			Kind:     "signal_processed",
			Message:  "verify failed for " + a.PlanFile,
			Repo:     e.Path,
			PlanFile: a.PlanFile,
		})
		return nil
	case loop.CreatePRAction:
		createPR := d.createPR
		if createPR == nil {
			createPR = d.createPRForApprovedTask
		}
		if err := createPR(e, a.PlanFile, a.ReviewBody); err != nil {
			d.logger.Warn("create pr after approval failed", "plan", a.PlanFile, "repo", e.Path, "err", err)
		}
		d.broadcaster.Emit(api.Event{
			Kind:     "signal_processed",
			Message:  "create PR for " + a.PlanFile,
			Repo:     e.Path,
			PlanFile: a.PlanFile,
		})
		return nil
	case loop.ReviewCycleLimitAction:
		d.logger.Warn("review-fix cycle limit reached",
			"plan", a.PlanFile, "cycle", a.Cycle, "limit", a.Limit, "repo", e.Path)
		d.broadcaster.Emit(api.Event{
			Kind:     "review_cycle_limit",
			Message:  fmt.Sprintf("review-fix cycle limit reached (%d/%d) for %s", a.Cycle, a.Limit, a.PlanFile),
			Repo:     e.Path,
			PlanFile: a.PlanFile,
		})
		return nil
	case loop.IncrementReviewCycleAction:
		if e.Store == nil {
			return fmt.Errorf("task store unavailable for %s", a.PlanFile)
		}
		ps, err := taskstate.Load(e.Store, e.Project, "")
		if err != nil {
			return fmt.Errorf("load task state for review cycle increment: %w", err)
		}
		if err := ps.IncrementReviewCycle(a.PlanFile); err != nil {
			return fmt.Errorf("increment review cycle: %w", err)
		}
		return nil
	default:
		d.logger.Debug("unhandled action", "kind", action.Kind(), "repo", e.Path)
		return nil
	}
}

func (d *Daemon) createPRForApprovedTask(e RepoEntry, planFile, reviewBody string) error {
	if e.Store == nil {
		return fmt.Errorf("task store unavailable for %s", planFile)
	}

	entry, err := e.Store.Get(e.Project, planFile)
	if err != nil {
		return fmt.Errorf("load task entry for %s: %w", planFile, err)
	}
	if entry.Branch == "" {
		d.logger.Warn("no branch for approved task; skipping pr creation", "plan", planFile, "repo", e.Path)
		return nil
	}

	shared := gitpkg.NewSharedTaskWorktree(e.Path, entry.Branch)
	if err := shared.Setup(); err != nil {
		return fmt.Errorf("setup shared worktree for %s: %w", planFile, err)
	}

	subtasks := []taskstore.SubtaskEntry(nil)
	if subtasksFromStore, err := e.Store.GetSubtasks(e.Project, planFile); err == nil {
		subtasks = subtasksFromStore
	} else {
		d.logger.Warn("load subtasks for pr creation failed", "plan", planFile, "repo", e.Path, "err", err)
	}

	base := shared.GetBaseCommitSHA()
	gitChanges, gitCommits, gitStats := "", "", ""
	if base != "" {
		worktreePath := shared.GetWorktreePath()
		runGit := func(args ...string) string {
			cmd := exec.Command("git", append([]string{"-C", worktreePath}, args...)...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(out))
		}
		gitChanges = runGit("diff", "--name-only", base)
		gitCommits = runGit("log", "--oneline", base+"..HEAD")
		gitStats = runGit("diff", "--stat", base)
	}

	meta := gitpkg.AssemblePRMetadata(entry, subtasks, reviewBody, entry.ReviewCycle, gitChanges, gitCommits, gitStats)
	planName := taskstate.DisplayName(planFile)
	title := gitpkg.BuildPRTitle(entry.Description, planName)
	body := gitpkg.BuildPRBody(meta)
	commitMsg := fmt.Sprintf("[kas] implementation of '%s'", planName)
	if err := shared.CreatePR(title, body, commitMsg); err != nil {
		return fmt.Errorf("create pr for %s: %w", planFile, err)
	}

	state, err := shared.QueryPRState()
	if err != nil {
		return fmt.Errorf("query pr state for %s: %w", planFile, err)
	}
	if state.URL == "" {
		d.logger.Warn("empty pr url after creation", "plan", planFile, "repo", e.Path)
		return nil
	}

	if err := e.Store.SetPRURL(e.Project, planFile, state.URL); err != nil {
		return fmt.Errorf("persist pr url for %s: %w", planFile, err)
	}

	if state.Number > 0 {
		if err := shared.PostGitHubReview(state.Number, body, true); err != nil {
			d.logger.Warn("post approving review failed", "plan", planFile, "repo", e.Path, "pr", state.Number, "err", err)
		}
	}

	return nil
}

func (d *Daemon) startWaveTasks(ctx context.Context, e RepoEntry, planFile string) error {
	orch := e.Processor.WaveOrchestrator(planFile)
	if orch == nil {
		return fmt.Errorf("wave orchestrator not found for %s", planFile)
	}
	if e.Store == nil {
		return fmt.Errorf("task store unavailable for %s", planFile)
	}

	entry, err := e.Store.Get(e.Project, planFile)
	if err != nil {
		return fmt.Errorf("load task entry for %s: %w", planFile, err)
	}

	tasks := orch.CurrentWaveTasks()
	if orch.State() != orchestration.WaveStateRunning {
		tasks = orch.StartNextWave()
	}
	if len(tasks) == 0 {
		return nil
	}

	waveNum := orch.CurrentWaveNumber()
	spawnWaveTask := d.spawnWaveTask
	if spawnWaveTask == nil {
		spawnWaveTask = d.spawner.SpawnWaveTask
	}
	if err := setRepoExecutionState(e, planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      waveNum,
	}); err != nil {
		return fmt.Errorf("persist wave execution state for %s: %w", planFile, err)
	}
	peerCount := len(tasks)
	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	for i, task := range tasks {
		task := task
		waveTaskIndex := i + 1
		prompt := orch.BuildTaskPrompt(task, peerCount)
		opts := loop.SpawnOpts{
			PlanFile: planFile,
			RepoPath: e.Path,
			Project:  e.Project,
			Branch:   entry.Branch,
			Program:  programForAgent(e.Path, session.AgentTypeCoder),
			Wave:     waveNum,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := spawnWaveTask(ctx, opts, task, prompt, waveTaskIndex, peerCount); err != nil {
				errOnce.Do(func() {
					firstErr = err
				})
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}

	planName := taskstate.DisplayName(planFile)
	d.logger.Info("wave advanced", "plan", planFile, "wave", waveNum, "repo", e.Path)
	d.broadcaster.Emit(api.Event{
		Kind:     "wave_advanced",
		Message:  fmt.Sprintf("%s: wave %d started", planName, waveNum),
		Repo:     e.Path,
		PlanFile: planFile,
	})
	return nil
}

func (d *Daemon) handleWaveTaskComplete(ctx context.Context, e RepoEntry, action loop.TaskCompleteAction) error {
	orch := e.Processor.WaveOrchestrator(action.PlanFile)
	if orch == nil {
		return nil
	}

	planName := taskstate.DisplayName(action.PlanFile)
	d.broadcaster.Emit(api.Event{
		Kind:     "task_completed",
		Message:  fmt.Sprintf("%s: task %d in wave %d completed", planName, action.TaskNumber, action.WaveNumber),
		Repo:     e.Path,
		PlanFile: action.PlanFile,
	})

	switch orch.State() {
	case orchestration.WaveStateRunning:
		return nil

	case orchestration.WaveStateWaveComplete:
		waveNum := orch.CurrentWaveNumber()
		completed := orch.CompletedTaskCount()
		failed := orch.FailedTaskCount()
		total := completed + failed

		killWaveAgents := d.killWaveAgents
		if killWaveAgents == nil {
			killWaveAgents = d.spawner.KillWaveAgents
		}
		if err := killWaveAgents(e.Path, action.PlanFile, waveNum); err != nil {
			return err
		}

		if failed > 0 {
			if err := setRepoExecutionState(e, action.PlanFile, taskstore.ExecutionState{
				Phase:           string(taskfsm.ExecutionPhaseWaveWaiting),
				ActiveAgentType: session.AgentTypeCoder,
				ActiveWave:      waveNum,
			}); err != nil {
				return fmt.Errorf("persist failed-wave waiting state for %s: %w", action.PlanFile, err)
			}
			e.Processor.ClearWaveOrchestrator(action.PlanFile)
			d.broadcaster.Emit(api.Event{
				Kind:     "wave_failed",
				Message:  fmt.Sprintf("%s: wave %d finished with %d/%d failed tasks", planName, waveNum, failed, total),
				Repo:     e.Path,
				PlanFile: action.PlanFile,
			})
			return nil
		}

		d.broadcaster.Emit(api.Event{
			Kind:     "wave_completed",
			Message:  fmt.Sprintf("%s: wave %d complete (%d/%d)", planName, waveNum, completed, total),
			Repo:     e.Path,
			PlanFile: action.PlanFile,
		})

		autoAdvanceWaves := d.cfg != nil && d.cfg.AutoAdvanceWaves
		if !autoAdvanceWaves {
			if err := setRepoExecutionState(e, action.PlanFile, taskstore.ExecutionState{
				Phase:           string(taskfsm.ExecutionPhaseWaveWaiting),
				ActiveAgentType: session.AgentTypeCoder,
				ActiveWave:      waveNum,
			}); err != nil {
				return fmt.Errorf("persist waiting wave state for %s: %w", action.PlanFile, err)
			}
			e.Processor.ClearWaveOrchestrator(action.PlanFile)
			return nil
		}

		tasks := orch.StartNextWave()
		if len(tasks) == 0 {
			e.Processor.ClearWaveOrchestrator(action.PlanFile)
			return nil
		}
		return d.startWaveTasks(ctx, e, action.PlanFile)

	case orchestration.WaveStateAllComplete:
		waveNum := orch.CurrentWaveNumber()
		killWaveAgents := d.killWaveAgents
		if killWaveAgents == nil {
			killWaveAgents = d.spawner.KillWaveAgents
		}
		if err := killWaveAgents(e.Path, action.PlanFile, waveNum); err != nil {
			return err
		}
		e.Processor.ClearWaveOrchestrator(action.PlanFile)

		fsm := taskfsm.New(e.Store, e.Project, "")
		if err := fsm.Transition(action.PlanFile, taskfsm.ImplementFinished); err != nil {
			return err
		}
		if err := setRepoExecutionState(e, action.PlanFile, taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		}); err != nil {
			return fmt.Errorf("persist reviewing execution state for %s: %w", action.PlanFile, err)
		}

		d.broadcaster.Emit(api.Event{
			Kind:     "wave_completed",
			Message:  fmt.Sprintf("all waves complete for %s", planName),
			Repo:     e.Path,
			PlanFile: action.PlanFile,
		})
		return d.executeAction(ctx, e, loop.SpawnReviewerAction{PlanFile: action.PlanFile})
	}

	return nil
}

func (d *Daemon) pidLockPath() string {
	return d.cfg.SocketPath + ".pid"
}

func sharedWorktreePaths(repoPath string) []string {
	entries, err := os.ReadDir(filepath.Join(repoPath, ".worktrees"))
	if err != nil {
		return nil
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(repoPath, ".worktrees", entry.Name()))
	}
	return paths
}

// programForAgent resolves the configured program command for a given agent
// type by reading the repo's config.toml profiles. Returns an empty string
// when no config exists or the profile is unconfigured, letting the spawner
// apply its own default.
func programForAgent(repoPath, agentType string) string {
	return programForAgentWithRegistry(repoPath, agentType, harness.NewRegistry())
}

// programForAgentWithRegistry is the testable core of programForAgent.
func programForAgentWithRegistry(repoPath, agentType string, registry *harness.Registry) string {
	configPath := filepath.Join(repoPath, ".kasmos", config.TOMLConfigFileName)
	result, err := config.LoadTOMLConfigFrom(configPath)
	if err != nil {
		return ""
	}
	defaultProgram := result.DefaultProgram

	cfg := &config.Config{
		PhaseRoles: result.PhaseRoles,
		Profiles:   result.Profiles,
	}

	var phase string
	switch agentType {
	case session.AgentTypeCoder:
		phase = "implementing"
	case session.AgentTypePlanner:
		phase = "planning"
	case session.AgentTypeReviewer:
		phase = "quality_review"
	case session.AgentTypeFixer:
		phase = "fixer"
	case session.AgentTypeElaborator:
		phase = "elaborating"
	case session.AgentTypeMaster:
		phase = "readiness_review"
	default:
		return defaultProgram
	}

	profile := cfg.ResolveProfile(phase, defaultProgram)
	return buildProgramCommand(profile, registry)
}

// buildProgramCommand synthesizes the full command string for an agent profile
// using the harness registry to generate harness-aware flags (model, effort,
// temperature). Falls back to the generic BuildCommand() for unknown programs
// or inline program strings that already contain spaces.
func buildProgramCommand(profile config.AgentProfile, registry *harness.Registry) string {
	program := strings.TrimSpace(profile.Program)
	if program == "" {
		return ""
	}

	// If the program string already contains spaces (inline arguments),
	// preserve legacy behavior — do not reinterpret it.
	if strings.Contains(program, " ") {
		return profile.BuildCommand()
	}

	// Look up the harness adapter by the basename of the program path.
	adapter := registry.Get(filepath.Base(program))
	if adapter == nil {
		// Unknown program — fall back to generic join.
		return profile.BuildCommand()
	}

	// Build harness-aware flags from the profile's model/effort/temperature fields.
	agentCfg := harness.AgentConfig{
		Harness:     adapter.Name(),
		Model:       profile.Model,
		Effort:      profile.Effort,
		Temperature: profile.Temperature,
		ExtraFlags:  profile.Flags,
	}
	flags := adapter.BuildFlags(agentCfg)

	if len(flags) == 0 {
		return program
	}
	return program + " " + strings.Join(flags, " ")
}

func coderSpawnOpts(e RepoEntry, planFile, branch, feedback string) loop.SpawnOpts {
	return loop.SpawnOpts{
		PlanFile: planFile,
		RepoPath: e.Path,
		Project:  e.Project,
		Branch:   branch,
		Program:  programForAgent(e.Path, session.AgentTypeCoder),
		Prompt:   feedback,
		Feedback: feedback,
	}
}

func reviewerSpawnOpts(e RepoEntry, entry taskstore.TaskEntry) loop.SpawnOpts {
	spec := orchestration.BuildReviewerAgentSpec(entry.Filename, e.Project, entry.ReviewCycle, entry.LatestReviewFeedback)
	return loop.SpawnOpts{
		PlanFile:    entry.Filename,
		RepoPath:    e.Path,
		Project:     e.Project,
		Branch:      entry.Branch,
		Program:     programForAgent(e.Path, session.AgentTypeReviewer),
		ReviewCycle: spec.ReviewCycle,
		Prompt:      spec.Prompt,
	}
}

func fixerSpawnOpts(e RepoEntry, planFile, branch, feedback string) loop.SpawnOpts {
	reviewCycle := 1
	if e.Store != nil {
		if entry, err := e.Store.Get(e.Project, planFile); err == nil && entry.ReviewCycle > 0 {
			reviewCycle = entry.ReviewCycle
		}
	}
	spec := orchestration.BuildFixerAgentSpec(planFile, e.Project, reviewCycle, feedback)
	return loop.SpawnOpts{
		PlanFile:    planFile,
		RepoPath:    e.Path,
		Project:     e.Project,
		Branch:      branch,
		Program:     programForAgent(e.Path, session.AgentTypeFixer),
		ReviewCycle: spec.ReviewCycle,
		Prompt:      spec.Prompt,
		Feedback:    feedback,
	}
}

func masterSpawnOpts(e RepoEntry, entry taskstore.TaskEntry) loop.SpawnOpts {
	spec := orchestration.BuildMasterAgentSpec(entry.Filename, e.Project, entry.ReviewCycle)
	return loop.SpawnOpts{
		PlanFile:    entry.Filename,
		RepoPath:    e.Path,
		Project:     e.Project,
		Branch:      entry.Branch,
		Program:     programForAgent(e.Path, session.AgentTypeMaster),
		Prompt:      spec.Prompt,
		ReviewCycle: entry.ReviewCycle + 1,
	}
}

// RecoverSessions discovers orphaned kas_ tmux sessions and attempts to
// re-adopt them into the spawner's tracking map. This should be called once
// on daemon startup, before the first tick.
//
// The recovery process:
//  1. Calls spawner.DiscoverOrphanSessions() to list kas_ sessions not tracked
//     by the spawner.
//  2. Cross-references orphan session names with task filenames in each
//     registered repo's task store to identify sessions this daemon owns.
//  3. Logs and counts matched sessions. Full re-hydration of Instance objects
//     from stored task metadata is a future enhancement.
//
// Returns the number of sessions matched to known tasks and logged as recovered.
func (d *Daemon) RecoverSessions() (int, error) {
	orphans := d.spawner.DiscoverOrphanSessions()
	if len(orphans) == 0 {
		return 0, nil
	}

	d.logger.Info("discovered orphaned sessions", "count", len(orphans))

	// Build a set of orphan session titles (without the kas_ prefix) for lookup.
	orphanTitles := make(map[string]struct{}, len(orphans))
	for _, o := range orphans {
		orphanTitles[o.Title] = struct{}{}
	}

	recovered := 0

	// Cross-reference orphan sessions with tasks in each registered repo.
	entries := d.repos.List()
	for _, e := range entries {
		if e.Store == nil {
			continue
		}
		tasks, err := e.Store.List(e.Project)
		if err != nil {
			d.logger.Warn("recover sessions: list tasks failed", "repo", e.Path, "err", err)
			continue
		}
		for _, task := range tasks {
			content := ""
			phase := taskfsm.NormalizeExecutionPhase(task.ExecutionState.Phase)
			if taskfsm.IsWaveExecutionPhase(phase) && e.Store != nil {
				if stored, getErr := e.Store.GetContent(e.Project, task.Filename); getErr == nil {
					content = stored
				}
			}
			for orphanTitle := range orphanTitles {
				candidate, ok := orchestration.MatchRecoveryCandidateByTitle(task, content, orphanTitle)
				if !ok {
					continue
				}

				data := session.InstanceData{
					Title:         candidate.Title,
					Path:          e.Path,
					Branch:        candidate.Branch,
					Status:        session.Running,
					Program:       "opencode",
					ExecutionMode: session.ExecutionModeTmux,
					AutoYes:       true,
					TaskFile:      task.Filename,
					AgentType:     candidate.AgentType,
					TaskNumber:    candidate.TaskNumber,
					WaveNumber:    candidate.WaveNumber,
					ReviewCycle:   candidate.ReviewCycle,
					WaveTaskIndex: candidate.WaveTaskIndex,
					WaveTaskCount: candidate.WaveTaskCount,
				}
				if candidate.Branch != "" {
					shared := gitpkg.NewSharedTaskWorktree(e.Path, candidate.Branch)
					data.Worktree = session.GitWorktreeData{
						RepoPath:     shared.GetRepoPath(),
						WorktreePath: shared.GetWorktreePath(),
						SessionName:  candidate.Title,
						BranchName:   candidate.Branch,
					}
				}

				if err := d.spawner.RestoreTrackedInstance(e.Path, e.Project, task.Filename, candidate.AgentType, data); err != nil {
					if errors.Is(err, errInstanceAlreadyTracked) {
						delete(orphanTitles, orphanTitle)
						continue
					}
					d.logger.Warn("recover sessions: restore instance failed",
						"session", candidate.Title, "repo", e.Path, "plan", task.Filename, "err", err)
					continue
				}

				d.logger.Info("re-adopted orphan session",
					"session", candidate.Title, "repo", e.Path, "plan", task.Filename, "agent", candidate.AgentType)
				delete(orphanTitles, orphanTitle)
				recovered++
			}
		}
	}

	return recovered, nil
}

// ---------------------------------------------------------------------------
// Legacy API (deprecated)
// ---------------------------------------------------------------------------

// RunDaemon is the legacy auto-accept daemon entry point. Kept for backward
// compatibility.
//
// Deprecated: use NewDaemon + Run instead.
func RunDaemon(cfg *config.Config) error {
	log.InfoLog.Printf("daemon starting")

	state := config.LoadState()

	storage, err := session.NewStorage(state)
	if err != nil {
		return fmt.Errorf("daemon: storage init failed: %w", err)
	}

	instances, err := storage.LoadInstances()
	if err != nil {
		return fmt.Errorf("daemon: load instances failed: %w", err)
	}

	// Daemon always operates in auto-accept mode.
	for _, inst := range instances {
		inst.AutoYes = true
	}

	pollInterval := time.Duration(cfg.DaemonPollInterval) * time.Millisecond

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		t := time.NewTimer(pollInterval)
		for {
			for _, inst := range instances {
				if inst.Started() && !inst.Paused() {
					if _, hasPrompt := inst.HasUpdated(); hasPrompt {
						inst.TapEnter()
					}
				}
			}

			// Check for stop before blocking on the timer.
			select {
			case <-stopCh:
				return
			default:
			}

			<-t.C
			t.Reset(pollInterval)
		}
	}()

	// Block until a termination signal arrives.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	received := <-sigCh
	log.InfoLog.Printf("daemon received signal: %s", received)

	// Signal the poll goroutine and wait for it to exit before persisting state.
	close(stopCh)
	wg.Wait()

	if saveErr := storage.SaveInstances(instances); saveErr != nil {
		log.ErrorLog.Printf("daemon: failed to save instances on shutdown: %v", saveErr)
	}

	return nil
}

// LaunchDaemon forks a detached daemon child process running
// `kas daemon start --foreground` and records its PID.
//
// The child is placed in a new session (Setsid=true on Unix) so it survives
// the parent terminal closing. Use `kas daemon start --foreground` directly
// when running under systemd.
func LaunchDaemon() error {
	bpInfo, err := binpath.Resolve()
	if err != nil {
		return fmt.Errorf("daemon: could not resolve executable path: %w", err)
	}

	cmd := exec.Command(bpInfo.Executable, "daemon", "start", "--foreground")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = getSysProcAttr()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("daemon: failed to start process: %w", err)
	}

	log.InfoLog.Printf("daemon child process started, PID=%d", cmd.Process.Pid)

	cfgDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("daemon: failed to locate config directory: %w", err)
	}

	pidPath := filepath.Join(cfgDir, "daemon.pid")
	pidContent := fmt.Sprintf("%d", cmd.Process.Pid)
	if err := os.WriteFile(pidPath, []byte(pidContent), 0644); err != nil {
		return fmt.Errorf("daemon: failed to write PID file: %w", err)
	}

	return nil
}

// StopDaemon terminates a running daemon process identified by its PID file.
// If no PID file exists the function returns without error (daemon is not running).
func StopDaemon() error {
	cfgDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("daemon: failed to locate config directory: %w", err)
	}

	pidPath := filepath.Join(cfgDir, "daemon.pid")
	raw, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("daemon: could not read PID file: %w", err)
	}

	var pid int
	if _, err := fmt.Sscanf(string(raw), "%d", &pid); err != nil {
		return fmt.Errorf("daemon: malformed PID file: %w", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("daemon: could not find process %d: %w", pid, err)
	}

	if err := proc.Kill(); err != nil {
		return fmt.Errorf("daemon: kill process %d failed: %w", pid, err)
	}

	if err := os.Remove(pidPath); err != nil {
		return fmt.Errorf("daemon: failed to remove PID file: %w", err)
	}

	log.InfoLog.Printf("daemon stopped (PID=%d)", pid)
	return nil
}

// warnBinaryPathSkew inspects the project files in repoPath and logs a Warn
// entry for every kasmos binary reference that does not match the running kas
// executable. Missing files and healthy matches are silently ignored.
// Errors during resolution are also silently swallowed so startup is never
// blocked.
func warnBinaryPathSkew(logger *slog.Logger, repoPath string) {
	info, err := binpath.Resolve()
	if err != nil {
		// Cannot determine running path — skip the audit rather than spamming noise.
		return
	}

	refs := binpath.InspectProjectFiles(repoPath)
	for _, ref := range refs {
		if ref.Note != "" && strings.Contains(ref.Note, "not installed") {
			continue
		}
		// Healthy: normalized path matches running canonical.
		if ref.Normalized != "" && ref.Normalized == info.Canonical {
			continue
		}
		// Nothing to report if both paths are empty (parse failure, etc.).
		if ref.RawPath == "" && ref.Normalized == "" {
			continue
		}
		logger.Warn("binary path skew detected",
			"repo", repoPath,
			"file", ref.File,
			"configured_path", ref.RawPath,
			"running_path", info.Executable,
		)
	}
}
