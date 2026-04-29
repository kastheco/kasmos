package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"reflect"
	"strings"

	"database/sql"

	cmd2 "github.com/kastheco/kasmos/cmd"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	daemonpkg "github.com/kastheco/kasmos/daemon"
	daemonapi "github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/clickup"
	"github.com/kastheco/kasmos/internal/mcpclient"
	sentrypkg "github.com/kastheco/kasmos/internal/sentry"
	theme "github.com/kastheco/kasmos/internal/theme"
	"github.com/kastheco/kasmos/keys"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
	sessionsdk "github.com/kastheco/kasmos/session/sdk"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"os"
	"path/filepath"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
)

const GlobalInstanceLimit = 50

const clickUpOpTimeout = 30 * time.Second

var repoManagedByDaemon = func(repoPath string) bool {
	if repoPath == "" {
		return false
	}
	client := daemonpkg.NewSocketClient(taskstore.ResolvedDaemonSocketPath())
	repos, err := client.ListRepos()
	if err != nil {
		return false
	}

	cleanRepoPath := canonicalRepoPath(repoPath)
	for _, repo := range repos {
		if canonicalRepoPath(repo.Path) == cleanRepoPath {
			return true
		}
	}
	return false
}

var setTerminalBackground = ui.SetTerminalBackground

var runTeaProgram = func(model tea.Model) error {
	p := tea.NewProgram(model)
	_, err := p.Run()
	return err
}

func daemonHTTPClient(socketPath string, timeout time.Duration) *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}

// Run is the main entrypoint into the application.
func Run(ctx context.Context, program string, autoYes bool, version string) error {
	defer sentrypkg.RecoverPanic()

	appConfig := config.LoadConfig()
	themeResult := resolveStartupTheme(ctx, appConfig)
	applyStartupTheme(themeResult)

	// Set the terminal's default background to the resolved theme base color so
	// every ANSI reset and unstyled cell falls back to the active palette.
	restore := setTerminalBackground(string(themeResult.Palette.Base))
	defer restore()

	zone.NewGlobal()
	h := newHomeWithConfig(ctx, program, autoYes, version, appConfig)
	if h.sharedDB != nil {
		defer h.sharedDB.Close()
	}
	defer h.auditLogger.Close()
	if h.permissionStore != nil {
		defer h.permissionStore.Close()
	}
	return runTeaProgram(h)
}

func resolveStartupTheme(ctx context.Context, cfg *config.Config) theme.Result {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return theme.Resolve(ctx, theme.Options{
		Source:      cfg.ThemeSource,
		Provider:    cfg.SystemThemeProvider,
		PaletteFile: cfg.ThemePaletteFile,
	}, theme.Dependencies{
		ReadFile: os.ReadFile,
		HomeDir:  os.UserHomeDir,
		RunCommand: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).Output()
		},
	})
}

func applyStartupTheme(result theme.Result) {
	theme.SetCurrent(result.Palette)
	ui.ApplyPalette(result.Palette)
	overlay.ApplyPalette(result.Palette)
	rebuildHelpStyles()
}

type state int

const (
	stateDefault state = iota
	// stateNew is the state when the user is creating a new instance.
	stateNew
	// statePrompt is the state when the user is entering a prompt.
	statePrompt
	// stateHelp is the state when a help screen is displayed.
	stateHelp
	// stateConfirm is the state when a confirmation modal is displayed.
	stateConfirm
	// stateSearch is the state when the user is searching topics/instances.
	stateSearch
	// stateNewPlan is the state when the user is creating a new plan (name + description form).
	stateNewPlan
	// stateNewPlanDeriving is the state when the AI is deriving a plan title.
	stateNewPlanDeriving
	// stateNewPlanTopic is the state when the user is picking a topic for a new plan.
	stateNewPlanTopic
	// stateSpawnHarnessPicker is the state when the user is selecting a harness program for ad-hoc spawn.
	stateSpawnHarnessPicker
	// stateSpawnExecutionModePicker is the state when the user is picking tmux vs sdk for an ad-hoc spawn.
	stateSpawnExecutionModePicker
	// stateSpawnAgent is the state when the user is spawning an ad-hoc agent session.
	stateSpawnAgent
	// statePRTitle is the state when the user is entering a PR title.
	statePRTitle
	// statePRBody is the state when the user is editing the PR body/description.
	statePRBody
	// stateRenameInstance is the state when the user is renaming an instance.
	stateRenameInstance
	// stateRenameTask is the state when the user is renaming a plan.
	stateRenameTask
	// stateRenameTopic is the state when the user is renaming a topic.
	stateRenameTopic
	// stateSendPrompt is the state when the user is sending a prompt via text overlay.
	stateSendPrompt
	// stateFocusAgent is the state when the user is typing directly into the agent pane.
	stateFocusAgent
	// stateContextMenu is the state when a right-click context menu is shown.
	stateContextMenu
	// stateChangeTopic is the state when the user is changing a plan's topic via picker.
	stateChangeTopic
	// stateSetStatus is the state when the user is force-overriding a plan's status via picker.
	stateSetStatus
	// stateClickUpSearch is the state when the user is typing a ClickUp search query.
	stateClickUpSearch
	// stateClickUpPicker is the state when the user is picking from ClickUp search results.
	stateClickUpPicker
	// stateClickUpFetching is when kasmos is fetching a full task from ClickUp.
	stateClickUpFetching
	// stateClickUpWorkspacePicker is when the user must pick a ClickUp workspace.
	stateClickUpWorkspacePicker
	// statePermission is when an opencode permission prompt is detected and the modal is shown.
	statePermission
	// stateTmuxBrowser is the state when the tmux session browser overlay is shown.
	stateTmuxBrowser
	// stateChatAboutTask is the state when the user is typing a question about a plan.
	stateChatAboutTask
	// stateAuditCursor is the state when the user is navigating log lines in the
	// audit pane to open per-line context menus.
	stateAuditCursor
	// stateLauncher is the state when the global command launcher overlay is shown.
	stateLauncher
	// stateKeybindBrowser is the state when the keybind browser overlay is shown.
	stateKeybindBrowser
	// stateWaveDecision is the state when an intermediate wave decision overlay is shown.
	stateWaveDecision
)

type home struct {
	ctx context.Context

	// -- Storage and Configuration --

	program string
	version string
	autoYes bool

	// activeRepoPath is the currently active repository path for filtering and new instances
	activeRepoPath string

	// storage is the interface for saving/loading data to/from the app's state
	storage *session.Storage
	// appConfig stores persistent application configuration
	appConfig *config.Config
	// appState stores persistent application state like seen help screens
	appState config.AppState

	// -- State --

	// allInstances stores every instance across all repos (master list)
	allInstances []*session.Instance
	// dismissedInstanceTitles suppresses daemon re-hydration of rows the user
	// explicitly deleted from the sidebar until the daemon stops reporting them.
	dismissedInstanceTitles map[string]struct{}

	// state is the current discrete state of the application
	state state
	// seenNotified tracks an instance whose Notified flag was visible when selected.
	// The flag is cleared when the user navigates away, not immediately on focus.
	seenNotified *session.Instance

	// instanceFinalizers stores per-instance finalizers keyed by instance pointer.
	// Each finalizer registers the repo name after the instance has started.
	// Supports concurrent batch spawns (e.g. wave tasks) where multiple
	// instances start in parallel. Lazily initialized via addInstanceFinalizer.
	instanceFinalizers map[*session.Instance]func()
	// abortedInstanceStartGroups records async start batches whose rollback
	// already ran, so late success messages from the same batch are ignored.
	abortedInstanceStartGroups map[string]struct{}
	plannerFanoutSeq           int
	// newInstance is the instance currently being named in stateNew.
	// Set when entering stateNew, cleared on Enter/Esc/ctrl+c.
	newInstance *session.Instance

	// promptAfterName tracks if we should enter prompt mode after naming
	promptAfterName bool

	// keySent is used to manage underlining menu items
	keySent bool

	// doubleTap tracks conflict-free double-tap state (k, K, u, d). Lazily
	// initialised on first key press so tests using bare &home{} keep working.
	doubleTap *keys.DoubleTapTracker

	// pendingDoubleTapKey is the canonical key awaiting its debounce timeout (s, space).
	pendingDoubleTapKey string
	// pendingDoubleTapAction is the single-press action queued for the pending key.
	pendingDoubleTapAction keys.KeyName
	// pendingDoubleTapSeq is incremented each time a new debounce is started so
	// that stale doubleTapTimeoutMsg values can be detected and dropped.
	pendingDoubleTapSeq int

	// -- UI Components --

	// nav displays plans + instances
	nav *ui.NavigationPanel
	// auditPane displays recent audit events below the nav panel
	auditPane         *ui.AuditPane
	auditBootstrapped bool // true after first audit query on boot
	// menu displays the bottom menu
	menu *ui.Menu
	// statusBar displays the top contextual status bar
	statusBar *ui.StatusBar
	// tabbedWindow displays the tabbed window with preview and info panes
	tabbedWindow *ui.TabbedWindow
	// toastManager manages toast notifications
	toastManager *overlay.ToastManager
	// global spinner instance. we plumb this down to where it's needed
	spinner spinner.Model
	// overlays manages the single active modal overlay.
	overlays *overlay.Manager
	// pendingConfirmAction stores the tea.Cmd to run asynchronously when confirmed
	pendingConfirmAction tea.Cmd

	// contextMenuTaskFile tracks the plan file backing the current task context menu.
	// Set when opening a task context menu; cleared on every stateContextMenu exit path.
	// The metadata tick compares it against the live FSM state to detect drift.
	contextMenuTaskFile string
	// contextMenuTaskStatus and contextMenuTaskPhase capture the lifecycle state at
	// menu-open time so the metadata tick can detect FSM changes while the menu is open.
	contextMenuTaskStatus taskstate.Status
	contextMenuTaskPhase  string

	// nav handles unified navigation state
	// focusSlot tracks which pane has keyboard focus in the Tab ring:
	// 0=nav, 1=tabs (center pane)
	focusSlot int
	// pendingPlanName stores the plan name during the two-step plan creation flow
	pendingPlanName string
	// pendingPlanDesc stores the plan description during the two-step plan creation flow
	pendingPlanDesc string
	// pendingPRTitle stores the PR title during the two-step PR creation flow
	pendingPRTitle string
	// pendingPRWorktree is a GitWorktree built from taskState for plan-level PR
	// creation flows where no running instance is available. Cleared after use.
	pendingPRWorktree *gitpkg.GitWorktree
	// pendingSpawnProgram stores the selected harness program during the spawn flow
	pendingSpawnProgram string
	// pendingSpawnExecutionMode stores the chosen execution mode during the spawn flow
	pendingSpawnExecutionMode session.ExecutionMode
	// pendingSpawnSpeedTier stores the chosen speed tier during the spawn flow
	// ("" or an explicit Codex tier such as "fast" / "flex")
	pendingSpawnSpeedTier string
	// pendingChangeTopicTask stores the plan filename during the change-topic flow
	pendingChangeTopicTask string
	// pendingSetStatusTask stores the plan filename during the set-status flow
	pendingSetStatusTask string
	// pendingChatAboutTask stores the plan filename during the chat-about-plan flow
	pendingChatAboutTask string
	// pendingLogEvent stores the audit event that triggered the log-action context
	// menu. Consumed by executeContextAction for "log_*" actions.
	pendingLogEvent *ui.AuditEventDisplay
	// pendingPRToastID stores the toast ID for the in-progress PR creation
	pendingPRToastID string
	// pendingAttachInstance is the instance queued for tea.Exec attach after the
	// help overlay is dismissed. Set in the keys.KeyEnter handler; consumed and
	// cleared in handleHelpState once the user acknowledges the attach help screen.
	pendingAttachInstance *session.Instance

	// tmuxSessionCount is the latest count of kas_-prefixed tmux sessions.
	tmuxSessionCount int
	// clickUpConfig stores the detected ClickUp MCP server config (nil if not detected)
	clickUpConfig *clickup.MCPServerConfig
	// clickUpImporter handles search/fetch via MCP (nil until first use)
	clickUpImporter *clickup.Importer
	// clickUpCommenter handles posting progress comments to ClickUp tasks (nil until first use)
	clickUpCommenter *clickup.Commenter
	// clickUpMCPClient is the raw MCP caller shared by importer and commenter
	clickUpMCPClient clickup.MCPCaller
	// clickUpResults stores the latest search results for the picker
	clickUpResults []clickup.SearchResult
	// clickUpPendingQuery stores the search query to retry after workspace selection
	clickUpPendingQuery string
	// clickUpWorkspaceMap maps picker labels ("name (id)") back to bare workspace IDs.
	clickUpWorkspaceMap map[string]string

	// Layout dimensions for mouse hit-testing
	navWidth      int
	tabsWidth     int
	contentHeight int

	// sidebarHidden tracks whether the nav is collapsed (ctrl+s toggle)
	sidebarHidden bool

	// Terminal dimensions for the global background fill.
	termWidth  int
	termHeight int

	// previewTerminal is the VT emulator for the selected instance's preview.
	// Also used for focus mode — entering focus just forwards keys to this terminal.
	previewTerminal         *session.EmbeddedTerminal
	previewTerminalInstance string // identity key of the instance the terminal is attached to
	previewRequested        bool   // true once the app should keep the live agent preview attached
	previewClipboardPending bool
	previewClipboardTarget  byte

	// taskState holds the parsed task state from the store for the active repo.
	taskState *taskstate.TaskState
	// taskStateLoadedAt records when taskState was last refreshed from the store.
	taskStateLoadedAt time.Time
	// taskStateDir is the legacy plans directory path. Retained only for JSON migration.
	// New code should not depend on this path existing on disk.
	taskStateDir string
	// signalsDir is the directory where agent sentinel files are written.
	// Defaults to <repoRoot>/.kasmos/signals/ (project-local, gitignored).
	signalsDir string
	// sharedDB is the single *sql.DB connection pool shared by the task store,
	// signal gateway, audit logger, and permission store. Owned by Run(); nil
	// when a remote task store is configured.
	sharedDB *sql.DB
	// taskStore is the authoritative task store client. It points at the daemon or
	// a configured remote store, and may be nil until daemon registration completes.
	taskStore taskstore.Store
	// taskStoreProject is the project name used with the remote store (derived from repo basename).
	taskStoreProject string
	// signalGateway is the authoritative signal gateway, shared across all signal
	// actions. Nil when a remote task store is configured (signals go through HTTP).
	signalGateway taskstore.SignalGateway
	// auditLogger records structured audit events in the local taskstore SQLite database.
	// Falls back to NopLogger when the SQLite audit logger cannot be opened.
	auditLogger auditlog.Logger

	// previewTickCount counts preview ticks for throttled banner animation
	previewTickCount int

	// metadataTickCount counts metadata ticks for throttled PR state polling.
	metadataTickCount int

	// cachedPlanFile is the filename of the last rendered plan (for cache hit).
	cachedPlanFile string
	// cachedPlanRendered is the glamour-rendered markdown of cachedPlanFile.
	cachedPlanRendered string

	// waveOrchestrators tracks active wave orchestrations by plan filename.
	waveOrchestrators map[string]*orchestration.WaveOrchestrator

	// pendingAllComplete holds plan files whose all-waves-complete prompt was
	// deferred because an overlay was active when the orchestrator finished.
	// Drained on each metadata tick once the overlay clears.
	pendingAllComplete []string

	// allCompleteDismissed tracks plan files where the user cancelled the
	// "all waves complete" confirmation. Prevents rebuildOrphanedOrchestrators
	// from recreating the orchestrator (which would re-show the prompt).
	allCompleteDismissed map[string]bool
	// allCompleteAdvancing tracks plans whose final review prompt was already
	// accepted and are currently pushing / transitioning to review.
	allCompleteAdvancing map[string]bool
	// allCompleteToastIDs tracks sticky toast notifications for plans whose final
	// review prompt is deferred while focus mode is active.
	allCompleteToastIDs map[string]string
	// pendingAllCompleteTaskFile is set while an all-waves-complete confirmation
	// overlay is showing, so cancel can record the dismissal.
	pendingAllCompleteTaskFile string

	// waveConfirmDismissedAt is the time the wave confirm dialog was last dismissed
	// via Esc. Used to impose a cooldown before re-showing the dialog.
	waveConfirmDismissedAt time.Time

	// plannerPrompted tracks plan files whose planner-exit dialog has been
	// answered (yes or no). Prevents re-prompting every metadata tick.
	// NOT set on esc — allows re-prompt.
	plannerPrompted map[string]bool

	// coderPushPrompted tracks plan files whose coder-exit push dialog has
	// been answered (yes or no) or dismissed (esc). Prevents the dialog from
	// re-firing on every metadata tick while the coder instance is still in
	// the list. Cleared when a new coder is spawned for the same plan
	// (e.g. via spawnFixerWithFeedback) so the next round can prompt again.
	coderPushPrompted map[string]bool

	// deferredPlannerDialogs holds plan files whose PlannerFinished dialog
	// could not be shown because an overlay was active at signal-processing time.
	// On each metadata tick, any queued plans are shown once the overlay clears.
	deferredPlannerDialogs []string
	// deferredPlannerToastIDs tracks sticky planner-ready notifications while
	// focus mode remains active.
	deferredPlannerToastIDs map[string]string
	// deferredCoderPushDialogs holds plan files whose implementer-finished push
	// prompt is waiting for the user to leave focus mode.
	deferredCoderPushDialogs []string
	// deferredCoderPushToastIDs tracks sticky implementer-finished notifications
	// while focus mode remains active.
	deferredCoderPushToastIDs map[string]string
	// deferredWaveDialogs holds plan files whose wave-complete decision dialog is
	// waiting for the user to leave focus mode.
	deferredWaveDialogs []string
	// deferredWaveToastIDs tracks sticky wave-complete notifications while focus
	// mode remains active.
	deferredWaveToastIDs map[string]string

	// pendingPlannerInstanceTitle is the title of the planner instance that
	// triggered the current planner-exit confirmation dialog.
	pendingPlannerInstanceTitle string

	// pendingPlannerTaskFile is the plan file associated with the planner instance
	// that triggered the current planner-exit confirmation dialog. Set by the
	// PlannerFinished signal handler so cancel/esc handlers can mark plannerPrompted
	// without needing to look up the (possibly already removed) instance by title.
	pendingPlannerTaskFile string

	// fsm is the sole writer of task state. All task status mutations flow
	// through fsm.Transition — direct SetStatus calls are not allowed.
	fsm *taskfsm.TaskStateMachine

	// processor is the signal processing engine that converts FSM sentinel signals
	// into typed Action values. Lazily initialized via ensureProcessor() on first use.
	// Nil when taskStore is not set (e.g. in tests that don't need signal processing).
	processor *loop.Processor
	// daemonStatusChecker verifies that the daemon is reachable and this repo is registered.
	// Nil disables daemon gating, which keeps narrow unit tests lightweight.
	daemonStatusChecker func(string) daemonStatusMsg
	// daemonRepoRegistrar registers the active repo with the daemon on demand.
	// Nil disables in-app repo registration, which keeps narrow unit tests lightweight.
	daemonRepoRegistrar func(string) error
	// planBrowserOpener starts or reuses kas serve and opens the admin plan browser.
	// Injected for testability.
	planBrowserOpener func(repoRoot, project, planFile string) (string, bool, error)

	// pendingReviewFeedback holds review feedback from sentinel files, keyed by
	// plan filename, to be injected as context for the next coder session.
	pendingReviewFeedback map[string]string

	// -- Permission prompt handling --

	// pendingPermissionInstance is the instance that triggered the permission modal.
	pendingPermissionInstance *session.Instance
	// pendingPermissionPattern is the pattern from the active permission overlay.
	// Captured at detection time so it's available after the overlay is dismissed.
	pendingPermissionPattern string
	// pendingPermissionDesc is the description from the active permission overlay.
	// Captured at detection time so it's available after the overlay is dismissed.
	pendingPermissionDesc string
	// permissionStore persists "allow always" decisions in the shared SQLite database.
	permissionStore config.PermissionStore
	// permissionHandled tracks in-flight auto-approvals: instance → pattern.
	// Prevents duplicate key sequences when the pane still shows the prompt
	// across multiple metadata ticks while opencode processes the first response.
	// Cleared when the pane no longer contains a permission prompt for that instance.
	permissionHandled map[*session.Instance]string
	// deferredPermissionPrompts queues permission overlays until the user leaves
	// focus mode explicitly.
	deferredPermissionPrompts []deferredPermissionPrompt
	// deferredPermissionToastIDs tracks sticky permission notifications keyed by
	// instance title while focus mode remains active.
	deferredPermissionToastIDs map[string]string
	// preOverlayNavID is the nav row id captured just before a permission
	// overlay auto-focused a different instance. It works for any nav row
	// kind — instance, plan header, history row, etc. — so non-instance
	// selections are restored on dismissal. Restored when the last queued
	// permission prompt is dismissed so the user's original selection is
	// preserved. First-write-wins via preOverlayCaptured: set only while
	// captured is false so a burst of queued prompts preserves the original
	// selection, not the interim selection from the previous prompt.
	preOverlayNavID    string
	preOverlayCaptured bool
}

type deferredPermissionPrompt struct {
	instance *session.Instance
	pattern  string
	desc     string
}

func newHome(ctx context.Context, program string, autoYes bool, version string) *home {
	appConfig := config.LoadConfig()
	return newHomeWithConfig(ctx, program, autoYes, version, appConfig)
}

func newHomeWithConfig(ctx context.Context, program string, autoYes bool, version string, appConfig *config.Config) *home {
	if appConfig == nil {
		appConfig = config.DefaultConfig()
	}
	// Load application state
	appState := config.LoadState()

	// Initialize storage
	storage, err := session.NewStorage(appState)
	if err != nil {
		fmt.Printf("Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}

	activeRepoPath, err := filepath.Abs(".")
	if err != nil {
		fmt.Printf("Failed to get current directory: %v\n", err)
		os.Exit(1)
	}
	if repoRoot, repoErr := config.ResolveRepoRoot(activeRepoPath); repoErr == nil && repoRoot != "" {
		activeRepoPath = repoRoot
	}

	project := resolveTaskStoreProject(activeRepoPath)
	h := &home{
		ctx:                        ctx,
		spinner:                    spinner.New(spinner.WithSpinner(spinner.Dot)),
		menu:                       ui.NewMenu(),
		auditPane:                  ui.NewAuditPane(),
		statusBar:                  ui.NewStatusBar(),
		tabbedWindow:               ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		storage:                    storage,
		appConfig:                  appConfig,
		program:                    program,
		version:                    version,
		autoYes:                    autoYes,
		state:                      stateDefault,
		appState:                   appState,
		activeRepoPath:             activeRepoPath,
		taskStateDir:               filepath.Join(activeRepoPath, "docs", "plans"), // legacy: only for JSON migration
		signalsDir:                 filepath.Join(activeRepoPath, ".kasmos", "signals"),
		taskStoreProject:           project,
		daemonStatusChecker:        checkDaemonStatus,
		daemonRepoRegistrar:        registerRepoWithDaemon,
		planBrowserOpener:          cmd2.OpenPlanBrowser,
		instanceFinalizers:         make(map[*session.Instance]func()),
		dismissedInstanceTitles:    make(map[string]struct{}),
		waveOrchestrators:          make(map[string]*orchestration.WaveOrchestrator),
		allCompleteDismissed:       make(map[string]bool),
		allCompleteAdvancing:       make(map[string]bool),
		allCompleteToastIDs:        make(map[string]string),
		plannerPrompted:            make(map[string]bool),
		deferredPlannerToastIDs:    make(map[string]string),
		deferredCoderPushToastIDs:  make(map[string]string),
		deferredWaveToastIDs:       make(map[string]string),
		coderPushPrompted:          make(map[string]bool),
		pendingReviewFeedback:      make(map[string]string),
		deferredPermissionToastIDs: make(map[string]string),
		abortedInstanceStartGroups: make(map[string]struct{}),
	}

	if appConfig.DatabaseURL != "" {
		remoteStore := taskstore.NewHTTPStore(appConfig.DatabaseURL, project)
		if pingErr := remoteStore.Ping(); pingErr != nil {
			fmt.Printf("Failed to connect to remote task store: %v\n", pingErr)
			os.Exit(1)
		}
		h.taskStore = remoteStore
		// Audit logger and permission store still use local SQLite even
		// with a remote task store — open their own shared pool.
		h.sharedDB, h.auditLogger, h.permissionStore = openLocalSharedBackends()
	} else {
		h.sharedDB, h.taskStore, h.signalGateway, h.auditLogger, h.permissionStore = openAllSharedBackends()
		if h.taskStore != nil {
			// Migrate repo-local taskstore.db into the global DB (idempotent no-op
			// when no local DB exists or data is already present globally).
			repoKasmosDir := filepath.Join(activeRepoPath, ".kasmos")
			if n, migErr := taskstore.MigrateRepoLocalToGlobal(h.taskStore, project, repoKasmosDir); migErr != nil {
				log.WarningLog.Printf("repo-local to global taskstore migration failed: %v", migErr)
			} else if n > 0 {
				log.InfoLog.Printf("migrated %d tasks from repo-local taskstore to global DB", n)
			}
		}
	}
	h.fsm = taskfsm.New(h.taskStore, project, h.taskStateDir)

	h.nav = ui.NewNavigationPanel(&h.spinner)
	h.toastManager = overlay.NewToastManager(&h.spinner)
	h.overlays = overlay.NewManager()

	// Don't show a startup error toast here. Local SQLite is authoritative for
	// repo-backed task state, and daemon readiness only gates agent workflows.

	permCacheDir := filepath.Join(activeRepoPath, ".kasmos")
	if h.permissionStore != nil {
		if migrateErr := config.MigratePermissionCache(permCacheDir, project, h.permissionStore); migrateErr != nil {
			log.WarningLog.Printf("permission cache migration failed: %v", migrateErr)
		}
	}
	h.permissionHandled = make(map[*session.Instance]string)

	h.tabbedWindow.SetAnimateBanner(appConfig.AnimateBanner)
	h.setFocusSlot(slotNav)
	h.loadTaskState()

	// Load saved instances
	instances, err := storage.LoadInstances()
	if err != nil {
		fmt.Printf("Failed to load instances: %v\n", err)
		os.Exit(1)
	}

	h.allInstances = dedupeInstancesByRepoAndTitle(instances)

	// Add instances matching active repo to the nav
	for _, instance := range h.allInstances {
		repoPath := instance.GetRepoPath()
		if repoPath == "" || repoPath == h.activeRepoPath {
			h.nav.AddInstance(instance)()
			if autoYes {
				instance.AutoYes = true
			}
		}
	}

	h.updateSidebarTasks()

	// Reconstruct in-memory wave orchestrators for plans that were mid-wave
	// when kasmos was last restarted. Must run after loadTaskState and instance load.
	h.rebuildOrphanedOrchestrators()

	return h
}

func dedupeInstancesByRepoAndTitle(instances []*session.Instance) []*session.Instance {
	if len(instances) < 2 {
		return instances
	}

	seen := make(map[string]*session.Instance, len(instances))
	order := make([]string, 0, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		key := inst.GetRepoPath() + "\x00" + inst.Title
		if existing, ok := seen[key]; ok {
			if existing.CreatedAt.Before(inst.CreatedAt) {
				seen[key] = inst
			}
			continue
		}
		seen[key] = inst
		order = append(order, key)
	}

	result := make([]*session.Instance, 0, len(order))
	for _, key := range order {
		result = append(result, seen[key])
	}
	return result
}

// openAllSharedBackends opens a single shared *sql.DB and derives the task
// store, signal gateway, audit logger, and permission store from it. Used when
// no remote task store is configured so all four subsystems share one pool.
func openAllSharedBackends() (*sql.DB, taskstore.Store, taskstore.SignalGateway, auditlog.Logger, config.PermissionStore) {
	sharedDB, err := taskstore.OpenBackingSharedDB()
	if err != nil {
		log.WarningLog.Printf("shared db init failed: %v", err)
		return nil, nil, nil, auditlog.NopLogger(), nil
	}

	store, err := taskstore.NewSQLiteStoreFromDB(sharedDB)
	if err != nil {
		log.WarningLog.Printf("task store init from shared db failed: %v", err)
		sharedDB.Close()
		return nil, nil, nil, auditlog.NopLogger(), nil
	}

	gw, err := taskstore.NewSQLiteSignalGatewayFromDB(sharedDB)
	if err != nil {
		log.WarningLog.Printf("signal gateway init from shared db failed: %v", err)
		sharedDB.Close()
		return nil, nil, nil, auditlog.NopLogger(), nil
	}

	al, err := auditlog.NewSQLiteLoggerFromDB(sharedDB)
	if err != nil {
		log.WarningLog.Printf("audit logger init from shared db failed: %v", err)
		sharedDB.Close()
		return nil, nil, nil, auditlog.NopLogger(), nil
	}

	permStore, err := config.NewSQLitePermissionStoreFromDB(sharedDB)
	if err != nil {
		log.WarningLog.Printf("permission store init from shared db failed: %v", err)
		sharedDB.Close()
		return nil, nil, nil, auditlog.NopLogger(), nil
	}

	return sharedDB, store, gw, al, permStore
}

// openLocalSharedBackends opens a shared *sql.DB for the audit logger and
// permission store only (used when the task store is remote).
func openLocalSharedBackends() (*sql.DB, auditlog.Logger, config.PermissionStore) {
	sharedDB, err := taskstore.OpenBackingSharedDB()
	if err != nil {
		log.WarningLog.Printf("shared db init failed: %v", err)
		return nil, auditlog.NopLogger(), nil
	}

	al, err := auditlog.NewSQLiteLoggerFromDB(sharedDB)
	if err != nil {
		log.WarningLog.Printf("audit logger init from shared db failed: %v", err)
		sharedDB.Close()
		return nil, auditlog.NopLogger(), nil
	}

	permStore, err := config.NewSQLitePermissionStoreFromDB(sharedDB)
	if err != nil {
		log.WarningLog.Printf("permission store init from shared db failed: %v", err)
		sharedDB.Close()
		return nil, auditlog.NopLogger(), nil
	}

	return sharedDB, al, permStore
}

// activeProject returns the project name derived from the active repo path.
// This matches how the task store derives the project name (filepath.Base of the repo path).
func (m *home) activeProject() string {
	return filepath.Base(m.activeRepoPath)
}

// isUserInOverlay returns true when the user is actively interacting with
// any modal overlay. Used to prevent async metadata-tick handlers from
// clobbering the active overlay by showing a confirmation dialog.
func (m *home) isUserInOverlay() bool {
	switch m.state {
	case stateDefault, stateFocusAgent:
		return false
	}
	return true
}

// updateHandleWindowSizeEvent sets the sizes of the components.
// The components will try to render inside their bounds.
func (m *home) updateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	// Two-column layout: nav + preview
	var navWidth int
	if m.sidebarHidden {
		navWidth = 0
	} else {
		navWidth = msg.Width * 30 / 100
		if navWidth < 25 {
			navWidth = 25
		}
	}
	tabsWidth := msg.Width - navWidth

	// Keep the keybind rail compact and give the saved rows to the three columns.
	menuHeight := 1
	if msg.Height < 2 {
		menuHeight = 0
	}
	statusBarHeight := 1
	accentHeight := 0
	if m.appConfig != nil && m.appConfig.AccentColor != "" {
		accentHeight = 1
	}
	contentHeight := msg.Height - menuHeight - statusBarHeight - accentHeight
	if contentHeight < 1 {
		contentHeight = 1
	}
	// Detect actual terminal resize vs spurious tea.RequestWindowSize side-effects.
	termResized := msg.Width != m.termWidth || msg.Height != m.termHeight

	m.termWidth = msg.Width
	m.termHeight = msg.Height
	m.toastManager.SetSize(msg.Width, msg.Height)
	if m.statusBar != nil {
		m.statusBar.SetSize(msg.Width)
	}

	m.tabbedWindow.SetSize(tabsWidth, contentHeight)

	// Nav panel gets full content height — audit pane is rendered inside its border.
	m.nav.SetSize(navWidth, contentHeight)
	if m.auditPane != nil && m.auditPane.Visible() && navWidth > 0 {
		// Size audit pane for the nav panel's inner content area.
		// border (2) + border padding (2) + item padding (2) = 6
		auditInnerW := navWidth - 6
		// Pass full content height — the nav panel clamps to whatever space
		// remains below the list content (active/plans sections + legend).
		auditH := contentHeight
		if !m.auditBootstrapped {
			m.refreshAuditPane() // load historical events on first render
			m.auditBootstrapped = true
		}
		m.auditPane.SetSize(auditInnerW, auditH)
		m.nav.SetAuditView(m.auditPane.String(), m.auditPane.ContentLines())
	} else {
		m.nav.SetAuditView("", 0)
		if m.auditPane != nil {
			m.auditPane.SetSize(0, 0)
		}
	}

	// Store for mouse hit-testing
	m.navWidth = navWidth
	m.tabsWidth = tabsWidth
	m.contentHeight = contentHeight

	if navWidth == 0 && m.focusSlot == slotNav {
		m.setFocusSlot(slotAgent)
	}

	// Only resize overlays when the terminal dimensions actually changed.
	// Many handlers emit tea.RequestWindowSize as a batched side-effect (e.g.
	// instanceStartedMsg) — those fire with the same dimensions and should
	// not overwrite the overlay's explicit sizing.
	if termResized {
		m.overlays.SetSize(msg.Width, msg.Height)
	}

	previewWidth, previewHeight := m.tabbedWindow.GetPreviewSize()
	if m.previewTerminal != nil {
		m.previewTerminal.Resize(previewWidth, previewHeight)
	}
	if err := m.nav.SetSessionPreviewSize(previewWidth, previewHeight); err != nil {
		log.ErrorLog.Print(err)
	}
	m.menu.SetSize(msg.Width, menuHeight)
}

func (m *home) Init() tea.Cmd {
	m.audit(auditlog.EventSessionStarted, "kasmos started")
	m.previewRequested = true
	initialPreviewCmd := m.instanceChanged()

	// Upon starting, we want to start the spinner. Whenever we get a spinner.TickMsg, we
	// update the spinner, which sends a new spinner.TickMsg. I think this lasts forever lol.
	return tea.Batch(
		initialPreviewCmd,
		m.spinner.Tick,
		func() tea.Msg {
			time.Sleep(50 * time.Millisecond)
			return previewTickMsg{}
		},
		tickUpdateMetadataCmd,
		m.toastTickCmd(),
		m.daemonStartupCheckCmd(),
		detectClickUpCmd(m.activeRepoPath),
	)
}

func (m *home) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case overlay.ToastTickMsg:
		m.toastManager.Tick()
		if m.toastManager.HasActiveToasts() {
			return m, m.toastTickCmd()
		}
		return m, nil
	case prCreatedMsg:
		m.toastManager.Resolve(m.pendingPRToastID, overlay.ToastSuccess, "PR created!")
		m.pendingPRToastID = ""
		m.audit(auditlog.EventPRCreated, fmt.Sprintf("PR created: %s", msg.prTitle),
			auditlog.WithInstance(msg.instanceTitle),
		)
		return m, m.toastTickCmd()
	case daemonStatusMsg:
		if msg.ready && msg.autoRegistered {
			m.toastManager.Success("auto-registered repo with daemon")
			return m, m.toastTickCmd()
		}
		if !msg.ready {
			m.showDaemonRequiredDialog(msg)
		}
		return m, nil
	case daemonRepoRegisteredMsg:
		m.toastManager.Success("registered repo with daemon")
		return m, m.toastTickCmd()
	case planBrowserOpenedMsg:
		if msg.startedServer {
			m.toastManager.Success("started plan browser server")
		} else {
			m.toastManager.Success("opened plan browser")
		}
		return m, m.toastTickCmd()
	case prErrorMsg:
		log.ErrorLog.Printf("%v", msg.err)
		m.toastManager.Resolve(msg.id, overlay.ToastError, msg.err.Error())
		m.pendingPRToastID = ""
		return m, m.toastTickCmd()
	case prCreatedForPlanMsg:
		if msg.url != "" && m.taskStore != nil {
			if err := m.taskStore.SetPRURL(m.taskStoreProject, msg.planFile, msg.url); err != nil {
				log.WarningLog.Printf("prCreatedForPlanMsg: could not persist PR URL for %q: %v", msg.planFile, err)
			}
		}
		m.loadTaskState()
		m.updateInfoPane()
		planName := taskstate.DisplayName(msg.planFile)
		m.toastManager.Success(fmt.Sprintf("pr created for '%s'", planName))
		return m, m.toastTickCmd()
	case planRenderedMsg:
		if msg.err != nil {
			return m, m.handleError(msg.err)
		}
		m.cachedPlanFile = msg.planFile
		m.cachedPlanRendered = msg.rendered
		m.previewRequested = false
		m.tabbedWindow.SetDocumentContent(msg.rendered)
		return m, nil
	case previewTickMsg:
		// If previewTerminal is active, render from it (zero-latency VT emulator).
		if m.previewTerminal != nil && !m.tabbedWindow.IsDocumentMode() {
			if content, changed := m.previewTerminal.Render(); changed {
				m.tabbedWindow.SetPreviewContent(content)
			}
			if !m.previewClipboardPending {
				if selection, ok := m.previewTerminal.PollClipboardRequest(); ok {
					m.previewClipboardPending = true
					m.previewClipboardTarget = selection
					term := m.previewTerminal
					return m, tea.Batch(nextPreviewTickCmd(term), readClipboardCmd(selection))
				}
			}
		} else if m.previewTerminal == nil && !m.tabbedWindow.IsDocumentMode() {
			// No terminal — show appropriate fallback state.
			selected := m.nav.GetSelectedInstance()
			if m.shouldAttachPreviewTerminal(selected) {
				// Instance is running but terminal hasn't attached yet — show connecting indicator.
				m.tabbedWindow.SetConnectingState()
			} else {
				// nil, Loading, or Paused — delegate to UpdatePreview which renders the
				// correct fallback (banner, progress bar, or paused message).
				if err := m.tabbedWindow.UpdatePreview(selected); err != nil {
					log.ErrorLog.Printf("preview update error: %v", err)
				}
			}
		}
		// Advance spring animation every tick (20fps)
		m.tabbedWindow.TickSpring()
		// Banner animation (only when no terminal is active / fallback showing).
		m.previewTickCount++
		if m.previewTickCount%20 == 0 {
			m.tabbedWindow.TickBanner()
		}
		// Use event-driven wakeup when terminal is live, fall back to 50ms poll otherwise.
		term := m.previewTerminal
		return m, nextPreviewTickCmd(term)
	case keyupMsg:
		m.menu.ClearKeydown()
		return m, nil
	case clickUpDetectedMsg:
		m.clickUpConfig = &msg.Config
		m.nav.SetClickUpAvailable(true)
		return m, nil
	case clickUpSearchResultMsg:
		if msg.Err != nil {
			// Check if the error is a multiple-workspaces error — show picker instead of failing.
			var mwErr *clickup.MultipleWorkspacesError
			if errors.As(msg.Err, &mwErr) && len(mwErr.WorkspaceIDs) > 0 {
				m.clickUpPendingQuery = msg.Query
				m.state = stateClickUpWorkspacePicker
				// Build picker labels: "name (id)" when names are available, bare id otherwise.
				items := make([]string, len(mwErr.WorkspaceIDs))
				m.clickUpWorkspaceMap = make(map[string]string, len(mwErr.WorkspaceIDs))
				for i, id := range mwErr.WorkspaceIDs {
					if name, ok := mwErr.WorkspaceNames[id]; ok && name != "" {
						label := name + " (" + id + ")"
						items[i] = label
						m.clickUpWorkspaceMap[label] = id
					} else {
						items[i] = id
						m.clickUpWorkspaceMap[id] = id
					}
				}
				m.overlays.Show(overlay.NewPickerOverlay("select clickup workspace", items))
				return m, nil
			}
			m.toastManager.Error("clickup search failed: " + msg.Err.Error())
			m.state = stateDefault
			return m, m.toastTickCmd()
		}
		if len(msg.Results) == 0 {
			m.toastManager.Info("no clickup tasks found")
			m.state = stateDefault
			return m, m.toastTickCmd()
		}
		m.clickUpResults = msg.Results
		items := make([]string, len(msg.Results))
		for i, r := range msg.Results {
			label := r.ID + " · " + r.Name
			if r.Status != "" {
				label += " (" + r.Status + ")"
			}
			if r.ListName != "" {
				label += " — " + r.ListName
			}
			items[i] = label
		}
		m.state = stateClickUpPicker
		m.overlays.Show(overlay.NewPickerOverlay("select clickup task", items))
		return m, nil
	case tickUpdateMetadataMessage:
		// Snapshot the instance list for the goroutine. The slice header is
		// copied but the pointers are shared — CollectMetadata only reads
		// instance fields that don't change between ticks (started, Status,
		// tmuxSession, gitWorktree, Program).
		instances := m.nav.GetInstances()
		snapshots := make([]*session.Instance, len(instances))
		copy(snapshots, instances)
		taskStateDir := m.taskStateDir // snapshot for goroutine
		signalsDir := m.signalsDir     // snapshot for goroutine
		store := m.taskStore           // snapshot for goroutine
		gateway := m.signalGateway     // snapshot for goroutine
		project := m.taskStoreProject  // snapshot for goroutine
		repoPath := m.activeRepoPath   // snapshot for goroutine
		m.metadataTickCount++
		tickCount := m.metadataTickCount // capture by value for goroutine

		return m, func() tea.Msg {
			daemonManagedRepo := repoManagedByDaemon(repoPath)
			var daemonClient *daemonpkg.SocketClient
			if daemonManagedRepo && project != "" {
				daemonClient = daemonpkg.NewSocketClient(taskstore.ResolvedDaemonSocketPath())
			}
			results := make([]instanceMetadata, 0, len(snapshots))
			for _, inst := range snapshots {
				if inst.Paused() {
					continue
				}
				// Daemon-managed SDK placeholders have no local
				// executionSession, so inst.CollectMetadata() short-
				// circuits on !inst.Started(). Instead, pull the pane
				// content from the daemon's own renderer buffer via the
				// control-socket API so the TUI preview reflects what the
				// agent is actually doing.
				if daemonClient != nil && !inst.Started() && session.NormalizeExecutionMode(inst.ExecutionMode) == session.ExecutionModeSDK {
					presResp, presErr := daemonClient.CapturePresentationFull(project, inst.Title)
					if presErr == nil && presResp.Supported {
						var turns []*sessionsdk.PresentationTurn
						if len(presResp.Turns) > 0 && string(presResp.Turns) != "null" {
							_ = json.Unmarshal(presResp.Turns, &turns)
						}
						results = append(results, instanceMetadata{
							Title:              inst.Title,
							PresentationTurns:  turns,
							PresentationCached: true,
							RendererStats:      rendererStatsFromDaemon(presResp.Stats),
							TmuxAlive:          true,
						})
						continue
					}
					content, err := daemonClient.CaptureInstance(project, inst.Title, "", "")
					if err != nil {
						results = append(results, instanceMetadata{Title: inst.Title})
						continue
					}
					results = append(results, instanceMetadata{
						Title:           inst.Title,
						Content:         content,
						ContentCaptured: content != "",
						Updated:         content != inst.CachedContent,
						TmuxAlive:       true,
					})
					continue
				}
				if !inst.Started() {
					continue
				}
				md := inst.CollectMetadata()
				var rendererStats *sessionsdk.RendererStats
				if session.NormalizeExecutionMode(inst.ExecutionMode) == session.ExecutionModeSDK {
					stats := md.RendererStats
					rendererStats = &stats
				}
				results = append(results, instanceMetadata{
					Title:              inst.Title,
					Content:            md.Content,
					ContentCaptured:    md.ContentCaptured,
					Updated:            md.Updated,
					HasPrompt:          md.HasPrompt,
					CPUPercent:         md.CPUPercent,
					MemMB:              md.MemMB,
					ResourceUsageValid: md.ResourceUsageValid,
					RendererStats:      rendererStats,
					TmuxAlive:          md.TmuxAlive,
					PermissionPrompt:   md.PermissionPrompt,
				})
			}

			// Load task state — moved here from the synchronous Update handler
			// to avoid blocking the event loop every 500ms. The authoritative
			// store is the repo-local SQLite DB for local repos or the configured
			// remote store when database_url is set.
			var ps *taskstate.TaskState
			var planStateLoadedAt time.Time
			var daemonTaskStateLoaded bool
			if store != nil && taskStateDir != "" && project != "" {
				planStateLoadedAt = time.Now().UTC()
				loaded, err := taskstate.Load(store, project, taskStateDir)
				if err != nil {
					log.WarningLog.Printf("could not load plan state: %v", err)
				} else {
					ps = loaded
					daemonTaskStateLoaded = daemonManagedRepo
				}
			}

			daemonInstances := make([]*session.Instance, 0)
			daemonTitles := make([]string, 0)
			if daemonManagedRepo && project != "" {
				knownInstances := make(map[string]*session.Instance, len(snapshots))
				for _, inst := range snapshots {
					if inst != nil {
						knownInstances[inst.Title] = inst
					}
				}
				statuses, err := listDaemonInstances(project)
				if err != nil {
					log.WarningLog.Printf("daemon instance sync: list instances for %q: %v", project, err)
				} else {
					for _, status := range statuses {
						title := status.Title
						if title == "" {
							continue
						}
						daemonTitles = append(daemonTitles, title)
						if !status.Active {
							continue
						}
						// Skip the restore only when the locally-tracked instance is
						// ACTUALLY live — Started()+!Exited+!Paused alone only reflects
						// the in-memory bookkeeping, which can lie after a daemon
						// restart (the subprocess the TUI thought was running died with
						// the daemon; the tmux-wrapper stayed Started=true in memory).
						// Also verify executionSession.DoesSessionExist so a corpse
						// in-memory doesn't shadow a freshly-spawned daemon instance
						// under the same title.
						if existing, exists := knownInstances[title]; exists && existing != nil && existing.Started() && !existing.Exited && !existing.Paused() && existing.TmuxAlive() {
							continue
						}
						inst, err := restoreDaemonInstance(repoPath, status)
						if err != nil {
							log.WarningLog.Printf("daemon instance sync: restore %q: %v", title, err)
							continue
						}
						daemonInstances = append(daemonInstances, inst)
						knownInstances[title] = inst
					}
				}
			}

			// Scan signals from the project-local signals directory (.kasmos/signals/).
			var signals []taskfsm.Signal
			if signalsDir != "" && !daemonManagedRepo {
				signals = taskfsm.ScanSignals(signalsDir)
			}

			var taskSignals []taskfsm.TaskSignal
			if signalsDir != "" && !daemonManagedRepo {
				taskSignals = taskfsm.ScanTaskSignals(signalsDir)
			}

			var elaborationSignals []taskfsm.ElaborationSignal
			if signalsDir != "" && !daemonManagedRepo {
				elaborationSignals = taskfsm.ScanElaborationSignals(signalsDir)
			}
			var waveSignals []taskfsm.WaveSignal
			if signalsDir != "" && !daemonManagedRepo {
				waveSignals = taskfsm.ScanWaveSignals(signalsDir)
			}
			var plannerDraftSignals []taskfsm.PlannerDraftSignal
			var gatewaySignalEntries []*taskstore.SignalEntry
			if gateway != nil && project != "" && !daemonManagedRepo {
				scan, entries, err := loop.ScanGateway(gateway, project, fmt.Sprintf("tui:%s:%d", project, os.Getpid()))
				if err != nil {
					log.WarningLog.Printf("gateway signal scan failed: %v", err)
				}
				gatewaySignalEntries = entries
				signals = append(signals, scan.FSMSignals...)
				taskSignals = append(taskSignals, scan.TaskSignals...)
				waveSignals = append(waveSignals, scan.WaveSignals...)
				elaborationSignals = append(elaborationSignals, scan.ElaborationSignals...)
				plannerDraftSignals = append(plannerDraftSignals, scan.PlannerDraftSignals...)
			}

			// Also scan signals from active worktrees — agents write
			// sentinel files relative to their CWD which is the worktree,
			// not the main repo. Worktrees use .kasmos/signals/ as well.
			seen := make(map[string]bool)
			for _, sig := range signals {
				seen[sig.Key()] = true
			}
			seenTaskSignals := make(map[string]bool)
			for _, ts := range taskSignals {
				seenTaskSignals[ts.Key()] = true
			}
			if !daemonManagedRepo {
				for _, inst := range snapshots {
					wt := inst.GetWorktreePath()
					if wt == "" {
						continue
					}
					wtSignalsDir := filepath.Join(wt, ".kasmos", "signals")
					for _, sig := range taskfsm.ScanSignals(wtSignalsDir) {
						if !seen[sig.Key()] {
							seen[sig.Key()] = true
							signals = append(signals, sig)
						}
					}
					for _, ts := range taskfsm.ScanTaskSignals(wtSignalsDir) {
						if !seenTaskSignals[ts.Key()] {
							seenTaskSignals[ts.Key()] = true
							taskSignals = append(taskSignals, ts)
						}
					}
				}
			}

			tmuxCount := tmux.CountKasSessions(cmd2.MakeExecutor())

			// Periodically poll PR state for plans that have a PR URL.
			// Poll every 10th tick (~2s) to avoid hammering the GitHub API.
			var prStateUpdates []prStateUpdateMsg
			if tickCount%10 == 0 && store != nil {
				if entries, err := store.List(project); err == nil {
					for _, entry := range entries {
						if entry.PRURL == "" || entry.Branch == "" {
							continue
						}
						shared := gitpkg.NewSharedTaskWorktree(repoPath, entry.Branch)
						state, err := shared.QueryPRState()
						if err != nil {
							log.WarningLog.Printf("PR poll: QueryPRState for %q: %v", entry.Filename, err)
							continue
						}
						if state.URL == "" {
							continue
						}
						prStateUpdates = append(prStateUpdates, prStateUpdateMsg{
							planFile:       entry.Filename,
							reviewDecision: mapPRReviewDecision(state.ReviewDecision),
							checkStatus:    mapPRCheckStatus(state.CheckStatus),
						})
					}
				}
			}

			time.Sleep(200 * time.Millisecond)
			return metadataResultMsg{Results: results, PlanState: ps, PlanStateLoadedAt: planStateLoadedAt, DaemonTaskState: daemonTaskStateLoaded, Signals: signals, TaskSignals: taskSignals, WaveSignals: waveSignals, ElaborationSignals: elaborationSignals, PlannerDraftSignals: plannerDraftSignals, GatewaySignalEntries: gatewaySignalEntries, DaemonManagedRepo: daemonManagedRepo, DaemonInstances: daemonInstances, DaemonTitles: daemonTitles, TmuxSessionCount: tmuxCount, PRStateUpdates: prStateUpdates}
		}
	case metadataResultMsg:
		// Process agent sentinel signals — feed to FSM and consume sentinel files.
		// Done in Update (main goroutine) so FSM writes are never concurrent.
		// Side-effect cmds (reviewer/coder spawns) are collected and batched below.
		var signalCmds []tea.Cmd
		proc := m.ensureProcessor()
		if !msg.DaemonManagedRepo {
			// Track the specific claimed gateway rows whose originating signal
			// produced actions or reached an inline happy path. Classifying by
			// row preserves failure state for mixed batches where one signal for
			// a plan succeeds while another same-plan signal is rejected.
			gatewayProcessedEntryIDs := make(map[int64]bool)
			gatewayClaimedEntryIDs := make(map[int64]bool)
			gatewayEntriesByID := make(map[int64]*taskstore.SignalEntry, len(msg.GatewaySignalEntries))
			for _, entry := range msg.GatewaySignalEntries {
				if entry != nil {
					gatewayEntriesByID[entry.ID] = entry
				}
			}
			claimGatewayEntry := func(entryID int64, planFile, signalType string) *taskstore.SignalEntry {
				if entryID == 0 || planFile == "" {
					return nil
				}
				canonicalType, err := taskfsm.CanonicalGatewaySignalType(signalType)
				if err != nil {
					return nil
				}
				entry := gatewayEntriesByID[entryID]
				if entry == nil || gatewayClaimedEntryIDs[entry.ID] || entry.PlanFile != planFile {
					return nil
				}
				entryType, err := taskfsm.CanonicalGatewaySignalType(entry.SignalType)
				if err != nil || entryType != canonicalType {
					return nil
				}
				gatewayClaimedEntryIDs[entry.ID] = true
				return entry
			}
			markGatewayProcessedEntry := func(entry *taskstore.SignalEntry) {
				if entry != nil {
					gatewayProcessedEntryIDs[entry.ID] = true
				}
			}
			claimGatewayEventEntry := func(entryID int64, planFile string, event taskfsm.Event) *taskstore.SignalEntry {
				signalType, err := taskfsm.GatewaySignalTypeForEvent(event)
				if err != nil {
					return nil
				}
				return claimGatewayEntry(entryID, planFile, signalType)
			}

			// Process FSM signals using the Processor when available, falling back to
			// the inline test-only path when a narrow test builds home without a taskStore.
			if proc != nil {
				proc.SyncWaveOrchestrators(m.waveOrchestrators)
				for planFile := range m.waveOrchestrators {
					proc.SetWaveOrchestratorActive(planFile, true)
				}

				feedbackBeforeTick := make(map[string]bool, len(m.pendingReviewFeedback))
				for planFile := range m.pendingReviewFeedback {
					feedbackBeforeTick[planFile] = true
				}

				var actions []loop.Action
				for _, sig := range msg.Signals {
					entry := claimGatewayEventEntry(sig.GatewayEntryID, sig.TaskFile, sig.Event)
					sigActions := proc.ProcessFSMSignals([]taskfsm.Signal{sig})
					if len(sigActions) > 0 {
						markGatewayProcessedEntry(entry)
					}
					actions = append(actions, sigActions...)
					taskfsm.ConsumeSignal(sig)
				}
				for _, sig := range msg.PlannerDraftSignals {
					entry := claimGatewayEntry(sig.GatewayEntryID, sig.TaskFile, "planner_draft_finished")
					sigActions := proc.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{sig})
					if len(sigActions) > 0 {
						markGatewayProcessedEntry(entry)
					}
					actions = append(actions, sigActions...)
				}

				draftSpawnCmds := make(map[string][]tea.Cmd)
				failedDraftPlannerFanout := make(map[string]bool)
				draftSpawnGroups := make(map[string]string)
				for _, act := range actions {
					switch a := act.(type) {
					case loop.SpawnReviewerAction:
						for _, inst := range m.nav.GetInstances() {
							if inst.TaskFile == a.PlanFile && (inst.AgentType == session.AgentTypeCoder || inst.AgentType == session.AgentTypeFixer) {
								inst.ImplementationComplete = true
								_ = inst.Pause()
								break
							}
						}
						if feedbackBeforeTick[a.PlanFile] {
							delete(m.pendingReviewFeedback, a.PlanFile)
							if cmd := m.postClickUpProgress(a.PlanFile, "fixer_complete", ""); cmd != nil {
								signalCmds = append(signalCmds, cmd)
							}
						}
						if cmd := m.spawnReviewer(a.PlanFile); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
					case loop.ReviewApprovedAction:
						// Reviewer-side handoff: pause reviewer instances, clear stale
						// feedback, audit reviewing → verifying. Terminal side-effects
						// (execution state, ClickUp, success toast) fire in VerifyApprovedAction.
						m.clearLatestReviewFeedback(a.PlanFile)
						planName := taskstate.DisplayName(a.PlanFile)
						m.audit(auditlog.EventPlanTransition, "reviewing → verifying (review approved)",
							auditlog.WithPlan(a.PlanFile))
						m.toastManager.Info(fmt.Sprintf("review approved — verifying: %s", planName))
						var lastPausedReviewer *session.Instance
						for _, inst := range m.nav.GetInstances() {
							if inst.TaskFile == a.PlanFile && inst.AgentType == session.AgentTypeReviewer {
								inst.SetStatus(session.Paused)
								lastPausedReviewer = inst
							}
						}
						if lastPausedReviewer != nil {
							m.nav.SelectInstance(lastPausedReviewer)
							m.updateNavPanelStatus()
							if cmd := m.instanceChanged(); cmd != nil {
								signalCmds = append(signalCmds, cmd)
							}
						}
					case loop.VerifyApprovedAction:
						// Terminal verification approval: clear execution state, pause
						// master instances, audit verifying → done, post ClickUp progress.
						if err := m.clearExecutionState(a.PlanFile); err != nil {
							log.WarningLog.Printf("could not clear execution state for %q: %v", a.PlanFile, err)
						}
						planName := taskstate.DisplayName(a.PlanFile)
						m.audit(auditlog.EventPlanTransition, "verifying → done (verify approved)",
							auditlog.WithPlan(a.PlanFile))
						m.toastManager.Success(fmt.Sprintf("review approved: %s", planName))
						if cmd := m.postClickUpProgress(a.PlanFile, "review_approved", ""); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
						// Pause all master instances, then select one and call instanceChanged.
						var lastPausedMaster *session.Instance
						for _, inst := range m.nav.GetInstances() {
							if inst.TaskFile == a.PlanFile && inst.AgentType == session.AgentTypeMaster {
								inst.SetStatus(session.Paused)
								lastPausedMaster = inst
							}
						}
						if lastPausedMaster != nil {
							m.nav.SelectInstance(lastPausedMaster)
							m.updateNavPanelStatus()
							if cmd := m.instanceChanged(); cmd != nil {
								signalCmds = append(signalCmds, cmd)
							}
						}
					case loop.VerifyFailedAction:
						m.audit(auditlog.EventPlanTransition, "verifying → implementing (verify failed)",
							auditlog.WithPlan(a.PlanFile))
						if cmd := m.handleReviewChangesRequested(a.PlanFile, a.Feedback); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
					case loop.CreatePRAction:
						signalCmds = append(signalCmds, m.createPRAfterApproval(a.PlanFile, a.ReviewBody))
					case loop.ReviewChangesAction:
						if cmd := m.handleReviewChangesRequested(a.PlanFile, a.Feedback); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
					case loop.IncrementReviewCycleAction:
						if err := m.taskState.IncrementReviewCycle(a.PlanFile); err != nil {
							log.WarningLog.Printf("could not increment review cycle for %q: %v", a.PlanFile, err)
						}
					case loop.ClearPlannerDraftsAction:
						cacheDir := filepath.Join(m.activeRepoPath, ".kasmos", "cache")
						if err := orchestration.ClearPlannerDraftCaches(cacheDir, a.PlanFile); err != nil {
							log.WarningLog.Printf("could not clear planner draft caches for %q: %v", a.PlanFile, err)
						}
					case loop.SpawnPlannerAction:
						entry, ok := m.refreshTaskEntry(a.PlanFile)
						if !ok {
							log.WarningLog.Printf("could not spawn planner for missing task %q", a.PlanFile)
							continue
						}
						prompt := buildPlanningPrompt(a.PlanFile, taskstate.DisplayName(a.PlanFile), entry.Description, m.taskStoreProject)
						if a.DraftMode {
							if failedDraftPlannerFanout[a.PlanFile] {
								continue
							}
							if a.Primary {
								m.killExistingPlanAgent(a.PlanFile, session.AgentTypePlanner)
							}
							startGroupID := draftSpawnGroups[a.PlanFile]
							if startGroupID == "" {
								startGroupID = m.nextPlannerFanoutStartGroup(a.PlanFile)
								draftSpawnGroups[a.PlanFile] = startGroupID
							}
							cmd, err := m.spawnPlannerProfileForTask(a.PlanFile, a.PlannerProfile, a.Primary, entry.Description, "", startGroupID)
							if err != nil {
								log.WarningLog.Printf("could not spawn planner profile %q for %q: %v", a.PlannerProfile, a.PlanFile, err)
								m.markInstanceStartGroupAborted(startGroupID)
								m.killExistingPlanAgent(a.PlanFile, session.AgentTypePlanner)
								failedDraftPlannerFanout[a.PlanFile] = true
								delete(draftSpawnCmds, a.PlanFile)
								continue
							}
							if cmd != nil {
								draftSpawnCmds[a.PlanFile] = append(draftSpawnCmds[a.PlanFile], cmd)
							}
						} else {
							mdl, cmd := m.spawnPlannersForTask(a.PlanFile, prompt, entry.Description)
							m = mdl.(*home)
							if cmd != nil {
								signalCmds = append(signalCmds, cmd)
							}
						}
					case loop.SpawnCoderAction:
						// Back-compat: older callers may still emit SpawnCoderAction for
						// review-fix loops even though the processor now emits
						// SpawnFixerAction.
						if cmd := m.spawnFixerWithFeedback(a.PlanFile, a.Feedback); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
					case loop.SpawnFixerAction:
						if cmd := m.spawnFixerWithFeedback(a.PlanFile, a.Feedback); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
					case loop.SpawnMasterAction:
						if cmd := m.spawnMaster(a.PlanFile); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
					case loop.ReviewCycleLimitAction:
						planName := taskstate.DisplayName(a.PlanFile)
						m.toastManager.Error(fmt.Sprintf(
							"review-fix loop stopped: cycle limit reached (%d/%d) for %s",
							a.Cycle, a.Limit, planName))
						m.audit(auditlog.EventPlanTransition,
							fmt.Sprintf("review-fix cycle limit reached (%d/%d)", a.Cycle, a.Limit),
							auditlog.WithPlan(a.PlanFile))
					case loop.PlannerCompleteAction:
						capturedPlanFile := a.PlanFile
						{
							summary := ""
							if m.taskStore != nil {
								if content, err := m.taskStore.GetContent(m.taskStoreProject, capturedPlanFile); err == nil {
									if plan, err := taskparser.Parse(content); err == nil {
										totalTasks := 0
										for _, w := range plan.Waves {
											totalTasks += len(w.Tasks)
										}
										summary = fmt.Sprintf("%d tasks, %d waves", totalTasks, len(plan.Waves))
									}
								}
							}
							if cmd := m.postClickUpProgress(capturedPlanFile, "plan_ready", summary); cmd != nil {
								signalCmds = append(signalCmds, cmd)
							}
						}
						if m.plannerPrompted[capturedPlanFile] {
							continue
						}
						if m.appConfig != nil && m.appConfig.AutoAdvance {
							m.plannerPrompted[capturedPlanFile] = true
							signalCmds = append(signalCmds, func() tea.Msg {
								return plannerCompleteMsg{planFile: capturedPlanFile}
							})
							continue
						}
						if m.state == stateFocusAgent || m.isUserInOverlay() {
							m.deferredPlannerDialogs = append(m.deferredPlannerDialogs, capturedPlanFile)
							continue
						}
						for _, inst := range m.nav.GetInstances() {
							if inst.TaskFile == capturedPlanFile && inst.AgentType == session.AgentTypePlanner {
								if cmd := m.focusInstanceForOverlay(inst); cmd != nil {
									signalCmds = append(signalCmds, cmd)
								}
								m.pendingPlannerInstanceTitle = inst.Title
								break
							}
						}
						m.pendingPlannerTaskFile = capturedPlanFile
						m.confirmAction(
							fmt.Sprintf("task '%s' is ready. start implementation?", taskstate.DisplayName(capturedPlanFile)),
							func() tea.Msg {
								return plannerCompleteMsg{planFile: capturedPlanFile}
							},
						)
					}
				}
				for planFile, cmds := range draftSpawnCmds {
					if !failedDraftPlannerFanout[planFile] {
						signalCmds = append(signalCmds, cmds...)
					}
				}
			} else {
				for _, sig := range msg.Signals {
					if sig.Event == taskfsm.ImplementFinished {
						if _, hasOrch := m.waveOrchestrators[sig.TaskFile]; hasOrch {
							log.WarningLog.Printf("ignoring implement-finished signal for %q — wave orchestrator active", sig.TaskFile)
							taskfsm.ConsumeSignal(sig)
							continue
						}
					}

					if err := m.fsm.Transition(sig.TaskFile, sig.Event); err != nil {
						log.WarningLog.Printf("signal %s for %s rejected: %v", sig.Event, sig.TaskFile, err)
						taskfsm.ConsumeSignal(sig)
						continue
					}
					taskfsm.ConsumeSignal(sig)

					switch sig.Event {
					case taskfsm.ImplementFinished:
						for _, inst := range m.nav.GetInstances() {
							if inst.TaskFile == sig.TaskFile && (inst.AgentType == session.AgentTypeCoder || inst.AgentType == session.AgentTypeFixer) {
								inst.ImplementationComplete = true
								_ = inst.Pause()
								break
							}
						}
						if _, hasFeedback := m.pendingReviewFeedback[sig.TaskFile]; hasFeedback {
							delete(m.pendingReviewFeedback, sig.TaskFile)
							if cmd := m.postClickUpProgress(sig.TaskFile, "fixer_complete", ""); cmd != nil {
								signalCmds = append(signalCmds, cmd)
							}
						}
						if cmd := m.spawnReviewer(sig.TaskFile); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
					case taskfsm.ReviewApproved:
						// In the legacy test-only path (no processor), chain through
						// VerifyApproved immediately — there is no readiness-review gate.
						if err := m.fsm.Transition(sig.TaskFile, taskfsm.VerifyApproved); err != nil {
							log.WarningLog.Printf("chained verify-approved for %q rejected: %v", sig.TaskFile, err)
						}
						m.clearLatestReviewFeedback(sig.TaskFile)
						if err := m.clearExecutionState(sig.TaskFile); err != nil {
							log.WarningLog.Printf("could not clear execution state for %q: %v", sig.TaskFile, err)
						}
						planName := taskstate.DisplayName(sig.TaskFile)
						m.audit(auditlog.EventPlanTransition, "reviewing → done (review approved)",
							auditlog.WithPlan(sig.TaskFile))
						m.toastManager.Success(fmt.Sprintf("review approved: %s", planName))
						if cmd := m.postClickUpProgress(sig.TaskFile, "review_approved", ""); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
						// Pause all reviewer/master instances first, then select
						// one and call instanceChanged once — same rationale as
						// the processor path above.
						var lastPaused *session.Instance
						for _, inst := range m.nav.GetInstances() {
							if inst.TaskFile == sig.TaskFile &&
								(inst.AgentType == session.AgentTypeReviewer || inst.AgentType == session.AgentTypeMaster) {
								inst.SetStatus(session.Paused)
								lastPaused = inst
							}
						}
						if lastPaused != nil {
							m.nav.SelectInstance(lastPaused)
							m.updateNavPanelStatus()
							if cmd := m.instanceChanged(); cmd != nil {
								signalCmds = append(signalCmds, cmd)
							}
						}
						if m.taskStore != nil {
							if entry, err := m.taskStore.Get(m.taskStoreProject, sig.TaskFile); err == nil {
								if shouldCreatePR(entry) {
									signalCmds = append(signalCmds, m.createPRAfterApproval(sig.TaskFile, sig.Body))
								}
							}
						}
					case taskfsm.ReviewChangesRequested:
						feedback := sig.Body
						if cmd := m.handleReviewChangesRequested(sig.TaskFile, feedback); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
						if m.appConfig == nil || !m.appConfig.AutoReviewFix {
							break
						}
						if m.appConfig != nil && m.appConfig.MaxReviewFixCycles > 0 {
							if cycle, err := m.taskState.ReviewCycle(sig.TaskFile); err == nil {
								if cycle+1 > m.appConfig.MaxReviewFixCycles {
									planName := taskstate.DisplayName(sig.TaskFile)
									m.toastManager.Error(fmt.Sprintf(
										"review-fix loop stopped: cycle limit reached (%d/%d) for %s",
										cycle+1, m.appConfig.MaxReviewFixCycles, planName))
									m.audit(auditlog.EventPlanTransition,
										fmt.Sprintf("review-fix cycle limit reached (%d/%d)", cycle+1, m.appConfig.MaxReviewFixCycles),
										auditlog.WithPlan(sig.TaskFile))
									continue // skip spawning fixer
								}
							}
						}
						if err := m.taskState.IncrementReviewCycle(sig.TaskFile); err != nil {
							log.WarningLog.Printf("could not increment review cycle for %q: %v", sig.TaskFile, err)
						}
						if cmd := m.spawnFixerWithFeedback(sig.TaskFile, feedback); cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
					case taskfsm.PlannerFinished:
						capturedPlanFile := sig.TaskFile
						{
							summary := ""
							if m.taskStore != nil {
								if content, err := m.taskStore.GetContent(m.taskStoreProject, capturedPlanFile); err == nil {
									if plan, err := taskparser.Parse(content); err == nil {
										totalTasks := 0
										for _, w := range plan.Waves {
											totalTasks += len(w.Tasks)
										}
										summary = fmt.Sprintf("%d tasks, %d waves", totalTasks, len(plan.Waves))
									}
								}
							}
							if cmd := m.postClickUpProgress(capturedPlanFile, "plan_ready", summary); cmd != nil {
								signalCmds = append(signalCmds, cmd)
							}
						}
						if m.plannerPrompted[capturedPlanFile] {
							break
						}
						if m.state == stateFocusAgent || m.isUserInOverlay() {
							m.deferredPlannerDialogs = append(m.deferredPlannerDialogs, capturedPlanFile)
							break
						}
						for _, inst := range m.nav.GetInstances() {
							if inst.TaskFile == sig.TaskFile && inst.AgentType == session.AgentTypePlanner {
								if cmd := m.focusInstanceForOverlay(inst); cmd != nil {
									signalCmds = append(signalCmds, cmd)
								}
								m.pendingPlannerInstanceTitle = inst.Title
								break
							}
						}
						m.pendingPlannerTaskFile = capturedPlanFile
						m.confirmAction(
							fmt.Sprintf("task '%s' is ready. start implementation?", taskstate.DisplayName(capturedPlanFile)),
							func() tea.Msg {
								return plannerCompleteMsg{planFile: capturedPlanFile}
							},
						)
					}
				}
			}

			for _, ts := range msg.TaskSignals {
				entry := claimGatewayEntry(ts.GatewayEntryID, ts.TaskFile, "implement_task_finished")
				orch, exists := m.waveOrchestrators[ts.TaskFile]
				if !exists {
					log.WarningLog.Printf("ignoring task-finished signal for %q — no active wave orchestrator", ts.TaskFile)
					taskfsm.ConsumeTaskSignal(ts)
					continue
				}
				if ts.WaveNumber != orch.CurrentWaveNumber() {
					log.WarningLog.Printf("ignoring task-finished signal for %q wave %d — active wave is %d", ts.TaskFile, ts.WaveNumber, orch.CurrentWaveNumber())
					taskfsm.ConsumeTaskSignal(ts)
					continue
				}
				if !orch.IsTaskRunning(ts.TaskNumber) {
					taskfsm.ConsumeTaskSignal(ts)
					continue
				}

				orch.MarkTaskComplete(ts.TaskNumber)
				markGatewayProcessedEntry(entry)
				for _, inst := range m.nav.GetInstances() {
					if inst.TaskFile != ts.TaskFile || inst.TaskNumber != ts.TaskNumber || inst.WaveNumber != ts.WaveNumber {
						continue
					}
					inst.ImplementationComplete = true
					inst.SetStatus(session.Ready)
					break
				}
				// Under limited parallelism, completing a task may open capacity
				// for pending tasks. Launch any that are now eligible.
				if orch.State() == orchestration.WaveStateRunning {
					if taskEntry, ok := m.taskState.Entry(ts.TaskFile); ok {
						mdl, pendingCmd := m.startPendingWaveTasks(orch, taskEntry)
						m = mdl.(*home)
						if pendingCmd != nil {
							signalCmds = append(signalCmds, pendingCmd)
						}
					}
				}
				taskfsm.ConsumeTaskSignal(ts)
			}

			// Process wave signals — trigger implementation for specific waves.
			for _, ws := range msg.WaveSignals {
				gatewayEntry := claimGatewayEntry(ws.GatewayEntryID, ws.TaskFile, "implement_wave")
				taskfsm.ConsumeWaveSignal(ws)

				// Check if orchestrator already exists
				if _, exists := m.waveOrchestrators[ws.TaskFile]; exists {
					m.toastManager.Error(fmt.Sprintf("wave already running for '%s'", taskstate.DisplayName(ws.TaskFile)))
					continue
				}

				// Read and parse the plan from store
				content, err := m.taskStore.GetContent(m.taskStoreProject, ws.TaskFile)
				if err != nil {
					log.WarningLog.Printf("wave signal: could not read plan %s: %v", ws.TaskFile, err)
					continue
				}
				plan, err := taskparser.Parse(content)
				if err != nil {
					m.toastManager.Error(fmt.Sprintf("plan '%s' has no wave headers", taskstate.DisplayName(ws.TaskFile)))
					continue
				}

				if ws.WaveNumber > len(plan.Waves) {
					m.toastManager.Error(fmt.Sprintf("plan has %d waves, requested wave %d", len(plan.Waves), ws.WaveNumber))
					continue
				}

				entry, ok := m.taskState.Entry(ws.TaskFile)
				if !ok {
					log.WarningLog.Printf("wave signal: plan %s not found in plan state", ws.TaskFile)
					continue
				}

				orch := orchestration.NewWaveOrchestrator(ws.TaskFile, plan)
				orch.SetStore(m.taskStore, m.taskStoreProject)
				m.waveOrchestrators[ws.TaskFile] = orch
				markGatewayProcessedEntry(gatewayEntry)

				// Fast-forward to the requested wave
				for i := 1; i < ws.WaveNumber; i++ {
					tasks := orch.StartNextWave()
					for _, t := range tasks {
						orch.MarkTaskComplete(t.Number)
					}
				}

				mdl, cmd := m.startNextWave(orch, entry)
				m = mdl.(*home)
				if cmd != nil {
					signalCmds = append(signalCmds, cmd)
				}
			}

			// Process architect-pass completion signals. The gateway contract still
			// uses the elaborator_finished name for compatibility, but the app and
			// daemon both reuse the shared processor semantics for reloading the
			// enriched plan and advancing to wave 1.
			if proc != nil {
				for _, es := range msg.ElaborationSignals {
					entry := claimGatewayEntry(es.GatewayEntryID, es.TaskFile, "elaborator_finished")
					actions := proc.ProcessElaborationSignals([]taskfsm.ElaborationSignal{es})
					if len(actions) > 0 {
						markGatewayProcessedEntry(entry)
					}
					taskfsm.ConsumeElaborationSignal(es)
					for _, act := range actions {
						advance, ok := act.(loop.AdvanceWaveAction)
						if !ok {
							continue
						}
						mdl, cmd := m.applyAdvanceWaveAction(
							advance,
							fmt.Sprintf("architect pass complete — starting wave %d for '%s'", advance.Wave, taskstate.DisplayName(advance.PlanFile)),
							session.AgentTypeElaborator,
						)
						m = mdl.(*home)
						if cmd != nil {
							signalCmds = append(signalCmds, cmd)
						}
					}
				}
			}

			// Classify each claimed gateway row before acknowledging it. Rows
			// whose own signal produced an action (or hit the inline happy path
			// for task/wave signals) get SignalDone with an empty result; the
			// rest fall through to GatewayNoopOutcome which matches the daemon's
			// classification for rejected lifecycle signals.
			for _, entry := range msg.GatewaySignalEntries {
				if m.signalGateway == nil || entry == nil {
					continue
				}
				status := taskstore.SignalDone
				result := ""
				if !gatewayProcessedEntryIDs[entry.ID] {
					if proc != nil {
						status, result = proc.GatewayNoopOutcome(entry)
					} else {
						status, result = loop.GatewayNoopOutcome(entry)
					}
				}
				if err := m.signalGateway.MarkProcessed(entry.ID, status, result); err != nil {
					log.WarningLog.Printf("could not mark gateway signal %d %s: %v", entry.ID, status, err)
				}
			}

			processedSignals := len(msg.Signals) > 0 || len(msg.TaskSignals) > 0 || len(msg.WaveSignals) > 0 || len(msg.ElaborationSignals) > 0 || len(msg.PlannerDraftSignals) > 0
			if processedSignals {
				m.loadTaskState() // refresh after signal processing
			}
		}

		m.reconcileDismissedInstanceTitles(msg.DaemonTitles)
		for _, inst := range msg.DaemonInstances {
			if inst == nil {
				continue
			}
			if m.isInstanceTitleDismissed(inst.Title) {
				continue
			}
			exists := false
			selected := m.nav.GetSelectedInstance()
			selectedTitle := ""
			if selected != nil {
				selectedTitle = selected.Title
			}
			for _, existing := range m.nav.GetInstances() {
				if existing.Title == inst.Title {
					// Same title, locally-tracked instance is Started and not
					// Paused/Exited — BUT also verify the underlying session is
					// actually alive. A stale in-memory Started=true can survive
					// a daemon restart (SDK subprocess died with the daemon, or
					// tmux session was reaped externally) and silently shadow a
					// freshly-spawned replacement under the same title.
					if existing.Started() && !existing.Exited && !existing.Paused() && existing.TmuxAlive() {
						exists = true
						break
					}
					if !existing.Started() && !inst.Started() && !existing.Exited && existing.Status == inst.Status {
						exists = true
						break
					}
					m.nav.RemoveByTitle(existing.Title)
					m.removeFromAllInstances(existing.Title)
					delete(m.instanceFinalizers, existing)
					if selectedTitle == existing.Title {
						selectedTitle = inst.Title
					}
					break
				}
			}
			if exists {
				continue
			}
			m.addInstanceFinalizer(inst, m.nav.AddInstance(inst))
			m.allInstances = append(m.allInstances, inst)
			if selectedTitle == inst.Title {
				m.nav.SelectInstance(inst)
			}
		}

		// Expire loading placeholders whose daemon spawn has failed or timed out.
		// Loading placeholders are created when the daemon reports an instance as
		// loading; if the daemon later reports it inactive (spawn failed) or the
		// placeholder has been stuck for >30s, remove it so the UI doesn't freeze.
		// The inactive branch is gated by a short grace period because the daemon
		// flips active asynchronously after registering the title — without the
		// grace, slow-to-start agents (e.g. planner with a heavyweight model and
		// skill load) get evicted within ~1s of spawn, before they ever go active.
		const loadingPlaceholderInactiveGrace = 5 * time.Second
		const loadingPlaceholderMaxAge = 30 * time.Second
		if msg.DaemonManagedRepo {
			activeDaemonTitles := make(map[string]struct{}, len(msg.DaemonInstances))
			for _, inst := range msg.DaemonInstances {
				if inst != nil {
					activeDaemonTitles[inst.Title] = struct{}{}
				}
			}
			daemonTitleSet := make(map[string]struct{}, len(msg.DaemonTitles))
			for _, t := range msg.DaemonTitles {
				daemonTitleSet[t] = struct{}{}
			}
			for _, existing := range m.nav.GetInstances() {
				if existing.Started() || existing.Status != session.Loading {
					continue
				}
				_, daemonKnows := daemonTitleSet[existing.Title]
				_, daemonActive := activeDaemonTitles[existing.Title]
				age := time.Since(existing.CreatedAt)
				notReadyYet := !daemonActive && !existing.CreatedAt.IsZero() && age > loadingPlaceholderInactiveGrace
				stale := !existing.CreatedAt.IsZero() && age > loadingPlaceholderMaxAge
				if daemonKnows && (notReadyYet || stale) {
					log.WarningLog.Printf("expiring stale loading placeholder %q (daemon_known=%v daemon_active=%v age=%v)",
						existing.Title, daemonKnows, daemonActive, age.Round(time.Second))
					m.nav.RemoveByTitle(existing.Title)
					m.removeFromAllInstances(existing.Title)
					m.toastManager.Error(fmt.Sprintf("'%s' failed to start — check .kasmos/logs/ for details", existing.Title))
				}
			}
		}

		// Apply collected metadata to instances — zero I/O, just field writes.
		// All subprocess calls (TapEnter, SendPrompt) are deferred to tea.Cmds.
		instanceMap := make(map[string]*session.Instance)
		for _, inst := range m.nav.GetInstances() {
			instanceMap[inst.Title] = inst
		}

		var asyncCmds []tea.Cmd

		for _, md := range msg.Results {
			inst, ok := instanceMap[md.Title]
			if !ok {
				continue
			}

			if md.PresentationCached {
				inst.SetCachedPresentation(md.PresentationTurns)
			}
			if md.RendererStats != nil {
				inst.SetCachedRendererStats(*md.RendererStats)
			}

			if md.ContentCaptured {
				inst.CachedContent = md.Content
				inst.CachedContentSet = true

				if md.Updated {
					// Mark that the agent has produced real work only after the
					// queued task prompt has been dispatched and we observe
					// non-prompt output. This prevents startup/prologue output and
					// prompt-echo ticks from prematurely completing wave tasks.
					if inst.TaskNumber > 0 && !inst.HasWorked && inst.QueuedPrompt == "" && !md.HasPrompt {
						inst.HasWorked = true
					}
					if md.Content != "" {
						inst.LastActivity = session.ParseActivity(md.Content, inst.Program)
					}
				}
				if md.HasPrompt {
					inst.SetStatus(session.Ready)
					inst.PromptDetected = true
					// Don't nudge wave tasks that have finished work — they're done
					// and the wave monitor will mark them complete on this tick.
					// Skip TapEnter when a permission prompt is detected — the
					// permission handler below will deal with it. Firing both
					// causes the second handler to send "1" as literal text.
					if !(inst.TaskNumber > 0 && inst.HasWorked) && md.PermissionPrompt == nil {
						// Defer tmux send-keys to async Cmd (was blocking Update).
						i := inst
						asyncCmds = append(asyncCmds, func() tea.Msg {
							i.TapEnter()
							return nil
						})
					}
				} else if md.Updated {
					inst.SetStatus(session.Running)
				} else {
					inst.SetStatus(session.Ready)
				}
				if inst.Status != session.Running {
					inst.LastActivity = nil
				}
			}

			// Permission prompt detection for supported harnesses.
			if md.PermissionPrompt != nil && (m.state == stateDefault || m.state == stateFocusAgent) {
				pp := md.PermissionPrompt
				cacheKey := config.CacheKey(pp.Pattern, pp.Description)
				// Guard key: use cache key if available, else sentinel.
				// Must match what app_input.go sets on confirm.
				guardKey := cacheKey
				if guardKey == "" {
					guardKey = "__handled__"
				}

				if storedKey, handled := m.permissionHandled[inst]; handled && storedKey == guardKey {
					// Same prompt still visible — skip until cleared.
				} else if cacheKey != "" && m.permissionStore != nil && m.permissionStore.IsAllowedAlways(m.activeProject(), cacheKey) {
					// Auto-approve cached permission.
					m.permissionHandled[inst] = guardKey
					i := inst
					asyncCmds = append(asyncCmds, func() tea.Msg {
						return permissionAutoApproveMsg{instance: i}
					})
				} else if m.state == stateFocusAgent {
					// Defer permission overlay until the user leaves focus mode.
					// Set the guard so subsequent ticks skip the full detection
					// path (cacheKey computation + IsAllowedAlways query).
					m.permissionHandled[inst] = guardKey
					m.queuePermissionPrompt(inst, pp.Pattern, pp.Description)
				} else {
					// Save the current nav row id before focusing away (first-write-wins).
					// Capturing the row id — rather than the selected instance pointer —
					// means plan/history selections (where GetSelectedInstance() is nil)
					// still restore correctly when the overlay is dismissed.
					// Skip capture+focus when the prompt is on the already-selected instance —
					// no restoration needed and avoids unnecessary instanceChanged() side effects.
					if m.nav.GetSelectedInstance() != inst {
						if !m.preOverlayCaptured {
							m.preOverlayNavID = m.nav.GetSelectedID()
							m.preOverlayCaptured = true
						}
						// Focus the instance so the user can see the agent output behind the overlay.
						if cmd := m.focusInstanceForOverlay(inst); cmd != nil {
							asyncCmds = append(asyncCmds, cmd)
						}
					}
					// Show modal (statePermission blocks re-entry on subsequent ticks).
					perm := overlay.NewPermissionOverlay(inst.Title, pp.Description, pp.Pattern)
					m.pendingPermissionPattern = pp.Pattern
					m.pendingPermissionDesc = pp.Description
					m.overlays.Show(perm)
					m.pendingPermissionInstance = inst
					m.state = statePermission
					m.audit(auditlog.EventPermissionDetected,
						fmt.Sprintf("permission prompt detected for %s", inst.Title),
						auditlog.WithInstance(inst.Title),
					)
				}
			} else if md.PermissionPrompt == nil {
				// Prompt cleared — remove the in-flight guard so a future permission
				// prompt for this instance can trigger auto-approve again.
				delete(m.permissionHandled, inst)
				// Also remove any deferred queue entry for this instance — the
				// permission was resolved externally (e.g. agent auto-dismissed it)
				// so draining a stale entry would show a bogus overlay or block
				// focus restoration.
				m.clearDeferredPermissionPrompt(inst)
			}

			// Mirror the daemon's PermissionBlocked tracking so the TUI wave
			// completion monitor doesn't mistake a permission prompt for task
			// completion (the composer prompt and permission prompt both set
			// PromptDetected=true).
			inst.PermissionBlocked = (md.PermissionPrompt != nil)

			// Deliver queued prompt via async Cmd — SendPrompt contains a 100ms
			// sleep + two tmux subprocess calls that were blocking the event loop.
			if inst.QueuedPrompt != "" && (inst.Status == session.Ready || inst.PromptDetected) {
				prompt := inst.QueuedPrompt
				inst.QueuedPrompt = "" // clear immediately to prevent re-send
				inst.AwaitingWork = true
				i := inst
				asyncCmds = append(asyncCmds, func() tea.Msg {
					if err := i.SendPrompt(prompt); err != nil {
						log.WarningLog.Printf("could not send queued prompt to %q: %v", i.Title, err)
					}
					return nil
				})
			}

			if md.ResourceUsageValid {
				inst.CPUPercent = md.CPUPercent
				inst.MemMB = md.MemMB
			}
		}

		// Clear activity for non-started / paused instances
		for _, inst := range m.nav.GetInstances() {
			if !inst.Started() || inst.Paused() {
				inst.LastActivity = nil
			}
		}

		// Apply plan state loaded in the goroutine (replaces synchronous loadTaskState call).
		// Skip when any signal type was processed: loadTaskState() above already gave us
		// fresh state, and msg.PlanState was loaded before signal scanning.
		processedSignals := len(msg.Signals) > 0 || len(msg.TaskSignals) > 0 || len(msg.WaveSignals) > 0 || len(msg.ElaborationSignals) > 0 || len(msg.PlannerDraftSignals) > 0
		if msg.PlanState != nil && (msg.DaemonTaskState || !processedSignals) {
			if msg.PlanStateLoadedAt.IsZero() || !msg.PlanStateLoadedAt.Before(m.taskStateLoadedAt) {
				m.taskState = msg.PlanState
				if !msg.PlanStateLoadedAt.IsZero() {
					m.taskStateLoadedAt = msg.PlanStateLoadedAt
				}
			}
		}
		if m.taskState != nil {
			for _, inst := range m.nav.GetInstances() {
				if inst == nil || inst.TaskFile == "" || inst.TaskNumber < 1 || inst.ImplementationComplete {
					continue
				}
				if m.isSubtaskPersistedComplete(inst.TaskFile, inst.TaskNumber) {
					inst.ImplementationComplete = true
				}
			}

			// Rebuild orphaned wave orchestrators on every metadata tick so local and
			// daemon-managed repos both recover from adopted orphan sessions, exited
			// task panes, and restart gaps. The helper is idempotent and skips plans
			// that already have in-memory orchestration state.
			m.rebuildOrphanedOrchestrators()
		}

		// Store the latest tmux session count for the bottom bar.
		m.tmuxSessionCount = msg.TmuxSessionCount
		m.menu.SetTmuxSessionCount(m.tmuxSessionCount)

		if m.taskState != nil {
			tmuxAliveMap := make(map[string]bool, len(msg.Results))
			for _, md := range msg.Results {
				tmuxAliveMap[md.Title] = md.TmuxAlive
			}

			// Implementer-exit → push-prompt: when a coder or fixer session's tmux
			// pane has exited and the plan is still in StatusImplementing, prompt the
			// user to push the implementation branch before advancing to reviewing.
			// Skip when a confirmation overlay is already showing to avoid re-prompting
			// on every tick while the user is deciding.
			for _, inst := range m.nav.GetInstances() {
				if m.isUserInOverlay() {
					break
				}
				alive, collected := tmuxAliveMap[inst.Title]
				if !collected {
					continue
				}
				entry := m.taskState.Plans[inst.TaskFile]
				if !session.ShouldAutoAdvanceLifecycleImplementer(string(entry.Status), entry.ExecutionState, inst, alive) {
					continue
				}
				// Skip if the push prompt was already shown and dismissed for this plan.
				// Cleared when a new implementer is spawned for the next round.
				if m.coderPushPrompted[inst.TaskFile] {
					continue
				}
				if m.state == stateFocusAgent {
					m.queueCoderPushDialog(inst.TaskFile)
					break
				}
				// Focus the implementer instance so the user can see its output behind the overlay.
				if cmd := m.focusInstanceForOverlay(inst); cmd != nil {
					asyncCmds = append(asyncCmds, cmd)
				}
				if cmd := m.promptPushBranchThenAdvance(inst); cmd != nil {
					asyncCmds = append(asyncCmds, cmd)
				}
				// Only prompt for one instance per tick to avoid stacking overlays.
				break
			}

			// Tmux death detection: mark instances as exited when their tmux
			// session dies so the UI renders them greyed-out + strikethrough
			// and allows cleanup. Covers solo agents, reviewers, and any
			// other instance whose tmux session disappears while the TUI runs.
			for _, inst := range m.nav.GetInstances() {
				alive, collected := tmuxAliveMap[inst.Title]
				// Clear Exited when a transient TmuxAlive=false (e.g. tmux server
				// still warming up post-reboot) got latched in a prior tick but
				// the session is now responsive. Mirrors daemon.monitorRunningInstances.
				if collected && alive && inst.Exited {
					inst.Exited = false
				}
				if inst.Exited || inst.Paused() {
					continue
				}
				if collected && !alive {
					inst.Exited = true
					if inst.Status == session.Running {
						inst.SetStatus(session.Ready)
					}
					m.audit(auditlog.EventAgentFinished, fmt.Sprintf("agent finished: %s", inst.Title),
						auditlog.WithInstance(inst.Title),
						auditlog.WithAgent(inst.AgentType),
						auditlog.WithPlan(inst.TaskFile),
					)
				}
			}

			// Wave completion monitoring: check task completion and trigger wave transitions.
			// We process both orchestration.WaveStateRunning (check task statuses) and orchestration.WaveStateWaveComplete
			// (re-show confirm dialog after user cancelled, resetting the latch via ResetConfirm).
			for planFile, orch := range m.waveOrchestrators {
				orchState := orch.State()
				if orchState != orchestration.WaveStateRunning && orchState != orchestration.WaveStateWaveComplete && orchState != orchestration.WaveStateAllComplete {
					continue
				}

				if orchState == orchestration.WaveStateRunning {
					// Check task status updates only while the wave is actively running.
					planName := taskstate.DisplayName(planFile)
					for _, task := range orch.CurrentWaveTasks() {
						taskTitle := fmt.Sprintf("%s-W%d-T%d", planName, orch.CurrentWaveNumber(), task.Number)
						inst, exists := instanceMap[taskTitle]
						if !exists {
							// Instance not in metadata results — check if it exists in the
							// nav list but hasn't started yet (async spawn still in flight).
							// Only mark failed if the instance is truly missing.
							stillSpawning := false
							for _, navInst := range m.nav.GetInstances() {
								if navInst.Title == taskTitle && (!navInst.Started() || navInst.Status == session.Loading) {
									stillSpawning = true
									break
								}
							}
							if stillSpawning {
								continue // wait for async start to complete
							}
							// Instance was dismissed (e.g. via k+k+k) or never re-hydrated
							// after a restart. If the store says the subtask is already
							// done, honor that — otherwise the rebuilt orchestrator would
							// flip a previously-completed wave into the failed-wave dialog.
							if m.isSubtaskPersistedComplete(planFile, task.Number) {
								orch.MarkTaskComplete(task.Number)
								continue
							}
							orch.MarkTaskFailed(task.Number)
							continue
						}
						if inst.Paused() {
							// A paused row that did real work (or is persisted complete
							// in the store) is a completed task, not a failure — wave
							// advance pauses finished tasks by design.
							if inst.HasWorked || m.isSubtaskPersistedComplete(planFile, task.Number) {
								orch.MarkTaskComplete(task.Number)
								inst.ImplementationComplete = true
								continue
							}
							orch.MarkTaskFailed(task.Number)
							continue
						}
						// Auto-detect wave task completion: agent finished work and returned to prompt.
						// Mirrors the single-agent detection in ShouldAutoAdvanceLifecycleImplementer.
						// Guard: do NOT mark complete while a permission prompt is active — the agent
						// is waiting for approval, not finished with its task.
						if inst.HasWorked && inst.PromptDetected && !inst.AwaitingWork && !inst.PermissionBlocked {
							orch.MarkTaskComplete(task.Number)
							inst.ImplementationComplete = true
							m.snapshotPaneOnCompletion(inst, planFile, task.Number, orch.CurrentWaveNumber())
							continue
						}
						// For daemon-managed instances, HasWorked may not be set (the daemon
						// API doesn't communicate it). If the TUI detected a prompt AND the
						// daemon already marked this subtask complete in the store, trust it.
						// This avoids false positives from startup prompt noise.
						if !inst.HasWorked && inst.PromptDetected && !inst.AwaitingWork && inst.TaskNumber > 0 &&
							m.isSubtaskPersistedComplete(planFile, task.Number) {
							orch.MarkTaskComplete(task.Number)
							inst.ImplementationComplete = true
							continue
						}
						alive, collected := tmuxAliveMap[inst.Title]
						if !collected {
							continue
						}
						if !alive {
							// Tmux died after the agent did real work — treat as completion, not failure.
							if inst.HasWorked {
								orch.MarkTaskComplete(task.Number)
								inst.ImplementationComplete = true
							} else if m.isSubtaskPersistedComplete(planFile, task.Number) {
								// Daemon already marked this task complete in the store
								// but the TUI never saw it working. Trust the store.
								orch.MarkTaskComplete(task.Number)
								inst.ImplementationComplete = true
							} else {
								orch.MarkTaskFailed(task.Number)
							}
						}
					}
					orchState = orch.State() // refresh after task updates
				}

				// All waves complete — pause the last wave's tasks, prompt for review.
				if orchState == orchestration.WaveStateAllComplete {
					capturedPlanFile := planFile
					planName := taskstate.DisplayName(planFile)
					totalWaves := orch.TotalWaves()
					waveNumFinal := orch.CurrentWaveNumber()
					completedFinal := orch.CompletedTaskCount()
					totalFinal := completedFinal + orch.FailedTaskCount()
					if err := m.setExecutionState(capturedPlanFile, taskstore.ExecutionState{
						Phase:           string(taskfsm.ExecutionPhaseWaveWaiting),
						ActiveAgentType: session.AgentTypeCoder,
						ActiveWave:      waveNumFinal,
					}); err != nil {
						log.WarningLog.Printf("could not persist wave waiting state for %q: %v", capturedPlanFile, err)
					}

					// Pause all task instances (they're done, free up resources).
					for _, inst := range m.nav.GetInstances() {
						if inst.TaskFile == capturedPlanFile && inst.TaskNumber > 0 {
							inst.ImplementationComplete = true
							_ = inst.Pause()
						}
					}
					delete(m.waveOrchestrators, planFile)
					m.audit(auditlog.EventWaveCompleted, "all waves complete: "+planName,
						auditlog.WithPlan(capturedPlanFile))
					// Post wave complete comment to ClickUp for multi-wave plans.
					if orch.ShouldPostWaveCompleteComment() {
						detail := fmt.Sprintf("%d/%d: %d/%d tasks", waveNumFinal, totalWaves, completedFinal, totalFinal)
						if cmd := m.postClickUpProgress(capturedPlanFile, "wave_complete", detail); cmd != nil {
							asyncCmds = append(asyncCmds, cmd)
						}
					}

					if m.state == stateFocusAgent {
						m.queueAllCompletePrompt(capturedPlanFile)
					} else {
						if !m.isUserInOverlay() {
							// Focus a task instance so the user can see agent output behind the overlay.
							if cmd := m.focusPlanInstanceForOverlay(capturedPlanFile); cmd != nil {
								asyncCmds = append(asyncCmds, cmd)
							}
							m.showAllCompletePrompt(capturedPlanFile)
						} else {
							// Another overlay is active — defer the prompt so it fires on
							// a later tick when the overlay clears. Without this, the
							// orchestrator deletion above means we never re-enter here.
							m.queueAllCompletePrompt(capturedPlanFile)
						}
					}
					continue
				}

				// orchState must be orchestration.WaveStateWaveComplete here.
				// Show wave decision once per wave (NeedsConfirm is one-shot;
				// ResetConfirm on dismiss allows the prompt to reappear next tick).
				needsConfirm := orch.NeedsConfirm()
				if needsConfirm && m.state == stateFocusAgent {
					m.queueWaveDialog(planFile)
					orch.ResetConfirm()
					continue
				}
				if needsConfirm && m.isUserInOverlay() {
					// Overlay active but not focus mode — queue for later.
					// Do NOT ResetConfirm: the deferred drain handles showing
					// the dialog, and keeping waitingForConfirm=true prevents
					// the wave monitor from re-queuing on every tick.
					m.queueWaveDialog(planFile)
					continue
				}
				if !m.isUserInOverlay() && time.Since(m.waveConfirmDismissedAt) > 30*time.Second && needsConfirm {
					asyncCmds = append(asyncCmds, m.showWaveDialog(planFile, orch)...)
				}
			}
		}

		// Apply PR state updates from periodic polling.
		if len(msg.PRStateUpdates) > 0 && m.taskStore != nil {
			selectedPlanFile := m.nav.GetSelectedPlanFile()
			selectedPlanChanged := false
			for _, u := range msg.PRStateUpdates {
				if err := m.taskStore.SetPRState(m.taskStoreProject, u.planFile, u.reviewDecision, u.checkStatus); err != nil {
					log.WarningLog.Printf("PR state update: could not persist for %q: %v", u.planFile, err)
				}
				if u.planFile == selectedPlanFile {
					selectedPlanChanged = true
				}
			}
			if selectedPlanChanged {
				m.loadTaskState()
				m.updateInfoPane()
			}
		}

		m.updateSidebarTasks()
		m.updateInfoPane()
		m.refreshSelectedPreview()

		// Dismiss stale task-backed context menus when the FSM state has changed
		// since the menu was opened. drainDeferredDialogs is always called below
		// so a replacement overlay can appear on the same tick.
		if m.contextMenuTaskFile != "" && m.state == stateContextMenu {
			if entry, ok := m.taskState.Entry(m.contextMenuTaskFile); !ok {
				m.overlays.Dismiss()
				m.state = stateDefault
				m.clearContextMenuTracking()
				m.toastManager.Error("task removed; menu dismissed")
			} else {
				status, phase := lifecycleSnapshot(entry)
				if status != m.contextMenuTaskStatus || phase != m.contextMenuTaskPhase {
					m.overlays.Dismiss()
					m.state = stateDefault
					m.clearContextMenuTracking()
					m.toastManager.Error(fmt.Sprintf("task changed to %s; menu dismissed", lifecycleSnapshotLabel(entry)))
				}
			}
		}

		completionCmd := m.checkPlanCompletion()
		asyncCmds = append(asyncCmds, signalCmds...)
		asyncCmds = append(asyncCmds, m.drainDeferredDialogs()...)
		asyncCmds = append(asyncCmds, tickUpdateMetadataCmd, completionCmd)
		// Restart toast tick loop if any toasts were created during this tick
		// (e.g. by transitionToReview or spawnFixerWithFeedback).
		if m.toastManager.HasActiveToasts() {
			asyncCmds = append(asyncCmds, m.toastTickCmd())
		}
		return m, tea.Batch(asyncCmds...)
	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)
	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)
	case doubleTapTimeoutMsg:
		return m.handleDoubleTapTimeout(msg)
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		m.updateHandleWindowSizeEvent(msg)
		return m, nil
	case lifecycleActionRejectedMsg:
		// Stale context menu race — show as a plain toast, not an audit-log error.
		m.toastManager.Error(msg.message)
		return m, m.toastTickCmd()
	case promptSubmittedMsg:
		if msg.err != nil {
			return m, m.handleError(msg.err)
		}
		if msg.instance != nil {
			msg.instance.SetStatus(session.Running)
			m.audit(auditlog.EventPromptSent, msg.auditMsg, auditlog.WithInstance(msg.instance.Title))
		}
		return m, nil
	case shellCommandSubmittedMsg:
		if msg.err != nil {
			return m, m.handleError(msg.err)
		}
		if msg.instance != nil {
			m.audit(auditlog.EventShellRan, msg.auditMsg, auditlog.WithInstance(msg.instance.Title))
		}
		return m, nil
	case error:
		// Handle errors from confirmation actions
		return m, m.handleError(msg)
	case instanceChangedMsg:
		// Handle instance changed after confirmation action
		m.updateNavPanelStatus()
		return m, m.instanceChanged()
	case previewTerminalReadyMsg:
		// Discard stale attach if selection changed while spawning.
		selected := m.nav.GetSelectedInstance()
		if msg.err != nil || !m.shouldAttachPreviewTerminal(selected) || previewIdentityKey(selected) != msg.instanceKey {
			return m, asyncClosePreviewTerminal(msg.term)
		}
		m.previewTerminal = msg.term
		m.previewTerminalInstance = msg.instanceKey
		if msg.term != nil {
			previewWidth, previewHeight := m.tabbedWindow.GetPreviewSize()
			msg.term.Resize(previewWidth, previewHeight)
		}
		return m, tea.RequestWindowSize
	case killInstanceMsg:
		// Async pre-kill checks passed — pause instead of destroying (branch preserved).
		for _, inst := range m.allInstances {
			if inst.Title == msg.title {
				if err := inst.Pause(); err != nil {
					return m, m.handleError(err)
				}
				break
			}
		}
		m.saveAllInstances()
		m.updateNavPanelStatus()
		return m, tea.Batch(tea.RequestWindowSize, m.instanceChanged())
	case taskStageConfirmedMsg:
		// User confirmed past the topic-concurrency gate — execute the stage.
		return m.executeTaskStage(msg.planFile, msg.stage)
	case taskRefreshMsg:
		// Reload plan state and refresh sidebar after async plan mutation.
		m.loadTaskState()
		m.updateSidebarTasks()
		return m, tea.Batch(tea.RequestWindowSize, m.instanceChanged())
	case startOverCompletedMsg:
		// The reset command does branch/FSM I/O only. Keep model mutations and
		// replacement agent spawning on the Bubble Tea update path.
		for _, inst := range append([]*session.Instance(nil), m.allInstances...) {
			if inst.TaskFile != msg.planFile {
				continue
			}
			m.nav.RemoveByTitle(inst.Title)
			m.removeFromAllInstances(inst.Title)
			delete(m.instanceFinalizers, inst)
		}
		if err := m.saveAllInstances(); err != nil {
			return m, m.handleError(err)
		}
		m.loadTaskState()
		m.updateSidebarTasks()
		model, spawnCmd := m.spawnPlannersForTask(
			msg.planFile,
			buildPlanningPrompt(msg.planFile, msg.planName, msg.description, m.taskStoreProject),
			msg.description,
		)
		updated, ok := model.(*home)
		if !ok {
			return model, spawnCmd
		}
		return updated, tea.Batch(tea.RequestWindowSize, updated.instanceChanged(), spawnCmd)
	case clickUpTaskFetchedMsg:
		if msg.Err != nil {
			m.toastManager.Error("clickup fetch failed: " + msg.Err.Error())
			m.state = stateDefault
			return m, m.toastTickCmd()
		}
		m.state = stateDefault
		return m.importClickUpTask(msg.Task)
	case waveAdvanceMsg:
		orch, ok := m.waveOrchestrators[msg.planFile]
		if !ok {
			return m, nil
		}
		// Pause completed wave's instances before starting the next.
		planName := taskstate.DisplayName(msg.planFile)
		for _, task := range orch.CurrentWaveTasks() {
			taskTitle := fmt.Sprintf("%s-W%d-T%d", planName, orch.CurrentWaveNumber(), task.Number)
			for _, inst := range m.nav.GetInstances() {
				if inst.Title == taskTitle && inst.PromptDetected {
					if err := inst.Pause(); err != nil {
						log.WarningLog.Printf("could not pause task %s: %v", taskTitle, err)
					}
				}
			}
		}
		return m.startNextWave(orch, msg.entry)
	case waveRetryMsg:
		orch, ok := m.waveOrchestrators[msg.planFile]
		if !ok {
			return m, nil
		}
		return m.retryFailedWaveTasks(orch, msg.entry)
	case waveAbortMsg:
		delete(m.waveOrchestrators, msg.planFile)
		// Kill and remove all task instances that belong to the aborted plan.
		// Their tmux sessions are already dead (tasks failed), so no worktree
		// check is needed — just clean them out of the list.
		// Collect first to avoid mutating m.allInstances while iterating it.
		var taskInsts []*session.Instance
		for _, inst := range m.allInstances {
			if inst.TaskFile == msg.planFile && inst.TaskNumber > 0 {
				taskInsts = append(taskInsts, inst)
			}
		}
		for _, inst := range taskInsts {
			if m.nav.SelectInstance(inst) {
				m.nav.Kill()
			}
			m.removeFromAllInstances(inst.Title)
		}
		m.saveAllInstances()
		m.updateNavPanelStatus()
		m.toastManager.Info(fmt.Sprintf("wave orchestration aborted for %s",
			taskstate.DisplayName(msg.planFile)))
		return m, tea.Batch(tea.RequestWindowSize, m.instanceChanged(), m.toastTickCmd())
	case waveAllCompleteMsg:
		// Capture the target instance on the main goroutine to avoid a data
		// race when the returned tea.Cmd runs concurrently with future Updates
		// that may mutate m.nav.instances.
		planFile := msg.planFile
		if m.allCompleteAdvancing == nil {
			m.allCompleteAdvancing = make(map[string]bool)
		}
		m.allCompleteAdvancing[planFile] = true
		m.pendingAllCompleteTaskFile = ""
		if err := m.setExecutionState(planFile, taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseWaveWaiting),
			ActiveAgentType: session.AgentTypeCoder,
			ActiveWave:      1,
		}); err != nil {
			log.WarningLog.Printf("could not persist wave waiting state for %q: %v", planFile, err)
		}
		var pushInst *session.Instance
		for _, inst := range m.nav.GetInstances() {
			if inst.TaskFile == planFile && inst.TaskNumber > 0 {
				if inst.WaveNumber > 0 {
					_ = m.setExecutionState(planFile, taskstore.ExecutionState{
						Phase:           string(taskfsm.ExecutionPhaseWaveWaiting),
						ActiveAgentType: session.AgentTypeCoder,
						ActiveWave:      inst.WaveNumber,
					})
				}
				pushInst = inst
				break
			}
		}
		return m, func() tea.Msg {
			if pushInst != nil {
				if worktree, err := pushInst.GetGitWorktree(); err == nil && worktree != nil {
					_ = worktree.Push(false)
				}
			}
			return wavePushCompleteMsg{planFile: planFile}
		}
	case wavePushCompleteMsg:
		// After async push completes for wave flow, transition and spawn reviewer.
		planFile := msg.planFile
		planName := taskstate.DisplayName(planFile)
		delete(m.allCompleteAdvancing, planFile)

		if err := m.fsm.Transition(planFile, taskfsm.ImplementFinished); err != nil {
			log.WarningLog.Printf("wave push-complete: could not transition %q to reviewing: %v", planFile, err)
		}
		m.loadTaskState()
		m.updateSidebarTasks()

		var reviewerCmd tea.Cmd
		if cmd := m.spawnReviewer(planFile); cmd != nil {
			reviewerCmd = cmd
		}
		toastMsg := fmt.Sprintf("all waves complete for '%s'. starting review", planName)
		if m.allCompleteToastIDs != nil && m.allCompleteToastIDs[planFile] != "" {
			m.resolveAllCompleteToast(planFile, overlay.ToastSuccess, toastMsg)
		} else {
			m.toastManager.Info(toastMsg)
		}
		return m, tea.Batch(tea.RequestWindowSize, reviewerCmd, m.toastTickCmd())
	case coderCompleteMsg:
		// Single-plan implementation finished and user confirmed push.
		// Transition FSM and spawn reviewer — mirrors waveAllCompleteMsg flow.
		planFile := msg.planFile
		planName := taskstate.DisplayName(planFile)

		if err := m.fsm.Transition(planFile, taskfsm.ImplementFinished); err != nil {
			log.WarningLog.Printf("coder-complete: could not transition %q to reviewing: %v", planFile, err)
		}

		// Clear the push-prompt dedup flag — the plan is now in reviewing, so
		// if a review round sends it back to implementing the next implementer can
		// trigger the push prompt cleanly.
		delete(m.coderPushPrompted, planFile)

		// Mark the current implementer instance as implementation-complete and pause it.
		for _, inst := range m.nav.GetInstances() {
			if inst.TaskFile == planFile && (inst.AgentType == session.AgentTypeCoder || inst.AgentType == session.AgentTypeFixer) {
				inst.ImplementationComplete = true
				_ = inst.Pause()
				break
			}
		}

		m.loadTaskState()
		m.updateSidebarTasks()

		var reviewerCmd tea.Cmd
		if cmd := m.spawnReviewer(planFile); cmd != nil {
			reviewerCmd = cmd
		}
		m.toastManager.Info(fmt.Sprintf("implementation complete for '%s' — starting review", planName))
		return m, tea.Batch(tea.RequestWindowSize, reviewerCmd, m.toastTickCmd())
	case tmuxSessionsMsg:
		if msg.err != nil {
			return m, m.handleError(msg.err)
		}
		if len(msg.sessions) == 0 {
			if m.toastManager != nil {
				m.toastManager.Info("no kas tmux sessions found")
				return m, m.toastTickCmd()
			}
			return m, nil
		}
		// Build instance lookup for enrichment.
		instMap := make(map[string]*session.Instance, len(m.allInstances))
		for _, inst := range m.allInstances {
			if inst.Started() {
				instMap[tmux.ToKasTmuxNamePublic(inst.Title)] = inst
			}
		}
		items := make([]overlay.TmuxBrowserItem, len(msg.sessions))
		for i, s := range msg.sessions {
			items[i] = overlay.TmuxBrowserItem{
				Name:     s.Name,
				Title:    s.Title,
				Created:  s.Created,
				Windows:  s.Windows,
				Attached: s.Attached,
				Width:    s.Width,
				Height:   s.Height,
				Managed:  s.Managed,
			}
			if inst, ok := instMap[s.Name]; ok {
				items[i].TaskFile = inst.TaskFile
				items[i].AgentType = inst.AgentType
				items[i].Status = statusString(inst.Status)
			}
		}
		m.overlays.Show(overlay.NewTmuxBrowserOverlay(items))
		m.state = stateTmuxBrowser
		return m, nil
	case tmuxKillResultMsg:
		if msg.err != nil {
			m.toastManager.Error(fmt.Sprintf("failed to kill session: %v", msg.err))
		} else {
			m.toastManager.Success(fmt.Sprintf("killed session '%s'", msg.name))
		}
		return m, m.toastTickCmd()
	case manualSignalResultMsg:
		if msg.err != nil {
			return m, m.handleError(msg.err)
		}
		m.loadTaskState()
		m.updateSidebarTasks()
		m.updateInfoPane()
		m.audit(auditlog.EventPromptSent, fmt.Sprintf("queued %s manually", msg.signalType),
			auditlog.WithPlan(msg.planFile),
			auditlog.WithInstance(msg.instanceTitle),
			auditlog.WithAgent(msg.agentType),
		)
		if msg.successToast != "" {
			m.toastManager.Success(msg.successToast)
			return m, m.toastTickCmd()
		}
		return m, nil
	case tmuxAttachReturnMsg:
		m.toastManager.Info("detached from tmux session")
		// Reset stored dimensions so the next WindowSizeMsg always triggers
		// termResized==true in updateHandleWindowSizeEvent, forcing a full
		// layout recalculation including overlay sizing.
		m.termWidth = 0
		m.termHeight = 0
		// Use Sequence (not Batch) so the terminal is cleared before the
		// window-size query fires — prevents a stale-state render between
		// alt-screen re-entry and the first correct frame.
		return m, tea.Sequence(
			rawClearScreenCmd(),
			clearScreenCmd(),
			tea.RequestWindowSize,
			m.instanceChanged(),
			m.toastTickCmd(),
		)
	case permissionAutoApproveMsg:
		if msg.instance != nil && msg.instance.Started() {
			i := msg.instance
			return m, func() tea.Msg {
				if handled, err := m.daemonRoutePermissionResponse(i, tmux.PermissionAllowAlways); handled {
					return err
				}
				i.SendPermissionResponse(tmux.PermissionAllowAlways)
				return nil
			}
		}
		return m, nil
	case planTitleMsg:
		if m.state == stateNewPlanDeriving {
			if msg.err == nil && msg.title != "" {
				m.pendingPlanName = msg.title
			}
			topicNames := m.getTopicNames()
			topicNames = append([]string{"(No topic)"}, topicNames...)
			pickerTitle := fmt.Sprintf("assign to topic for '%s'", m.pendingPlanName)
			p := overlay.NewPickerOverlay(pickerTitle, topicNames)
			p.SetAllowCustom(true)
			m.overlays.Show(p)
			m.state = stateNewPlanTopic
			return m, nil
		}
		// Safety net: if title arrives while already in topic picker, update silently
		if msg.err == nil && msg.title != "" {
			if m.state == stateNewPlanTopic && m.pendingPlanDesc != "" {
				m.pendingPlanName = msg.title
				if po, ok := m.overlays.Current().(*overlay.PickerOverlay); ok {
					po.SetTitle(
						fmt.Sprintf("assign to topic for '%s'", msg.title),
					)
					return m, tea.RequestWindowSize
				}
			}
		}
		return m, nil
	case plannerCompleteMsg:
		// User confirmed: start implementation. Keep the planner session around so
		// the architect handoff does not discard the planner's terminal history.
		m.plannerPrompted[msg.planFile] = true
		_ = m.saveAllInstances()
		m.pendingPlannerInstanceTitle = ""
		m.pendingPlannerTaskFile = ""
		m.updateNavPanelStatus()
		return m.triggerTaskStage(msg.planFile, "implement")
	case daemonPlannerStartedMsg:
		if msg.err != nil {
			return m, m.handleError(msg.err)
		}
		if msg.instance == nil {
			return m, m.handleError(fmt.Errorf("daemon planner start returned no instance"))
		}
		m.addInstanceFinalizer(msg.instance, m.nav.AddInstance(msg.instance))
		m.allInstances = append(m.allInstances, msg.instance)
		if err := m.saveAllInstances(); err != nil {
			return m, m.handleError(err)
		}
		m.updateNavPanelStatus()
		if fn, ok := m.instanceFinalizers[msg.instance]; ok {
			fn()
			delete(m.instanceFinalizers, msg.instance)
		}
		if m.autoYes {
			msg.instance.AutoYes = true
		}
		return m, tea.Batch(tea.RequestWindowSize, m.instanceChanged())
	case instanceStartedMsg:
		if msg.instance == nil {
			return m, m.handleError(fmt.Errorf("instance start returned no instance"))
		}
		if msg.err != nil {
			if msg.startGroupID != "" && m.isDraftPlannerFanoutInstance(msg.instance) {
				m.markInstanceStartGroupAborted(msg.startGroupID)
				m.killExistingPlanAgent(msg.instance.TaskFile, session.AgentTypePlanner)
				m.updateNavPanelStatus()
				return m, m.handleError(fmt.Errorf("planner fan-out failed: %w", msg.err))
			}
			// Remove the specific instance that failed — not the currently selected one.
			_ = msg.instance.Kill()
			m.nav.RemoveByTitle(msg.instance.Title)
			m.removeFromAllInstances(msg.instance.Title)
			delete(m.instanceFinalizers, msg.instance)
			m.updateNavPanelStatus()
			return m, m.handleError(msg.err)
		}
		if msg.startGroupID != "" && m.instanceStartGroupAborted(msg.startGroupID) {
			_ = msg.instance.Kill()
			m.nav.RemoveByTitle(msg.instance.Title)
			m.removeFromAllInstances(msg.instance.Title)
			delete(m.instanceFinalizers, msg.instance)
			m.updateNavPanelStatus()
			return m, tea.Batch(tea.RequestWindowSize, m.instanceChanged())
		}
		// Instance started successfully — add to master list, save and finalize
		m.allInstances = append(m.allInstances, msg.instance)
		if err := m.saveAllInstances(); err != nil {
			return m, m.handleError(err)
		}
		m.updateNavPanelStatus()
		if fn, ok := m.instanceFinalizers[msg.instance]; ok {
			fn()
			delete(m.instanceFinalizers, msg.instance)
		}
		if m.autoYes {
			msg.instance.AutoYes = true
		}
		cmds := []tea.Cmd{tea.RequestWindowSize, m.instanceChanged()}
		if msg.instance.TaskFile == "" &&
			msg.instance.AgentType == session.AgentTypeMaster &&
			session.NormalizeExecutionMode(msg.instance.ExecutionMode) == session.ExecutionModeSDK &&
			msg.instance.Started() {
			if focusCmd := m.enterFocusMode(); focusCmd != nil {
				cmds = append(cmds, focusCmd)
			}
		}
		if syncCmd := m.quickLaunchTitleSyncCmd(msg.instance); syncCmd != nil {
			cmds = append(cmds, syncCmd)
		}
		return m, tea.Batch(cmds...)
	case instanceTitleSyncMsg:
		if msg.instance == nil {
			return m, nil
		}
		if err := m.syncInstanceDisplayTitle(msg.instance, msg.newTitle); err != nil {
			return m, m.handleError(err)
		}
		return m, nil
	case tea.ClipboardMsg:
		if m.previewClipboardPending {
			selection := m.previewClipboardTarget
			if selection == 0 {
				selection = ansi.SystemClipboard
			}
			m.previewClipboardPending = false
			m.previewClipboardTarget = 0
			if m.previewTerminal != nil {
				if err := m.previewTerminal.SendKey([]byte(ansi.SetClipboard(selection, msg.Content))); err != nil {
					return m, m.handleError(err)
				}
			}
			return m, nil
		}
	case tea.PasteMsg:
		// Route paste to any active overlay that supports it first.
		if m.overlays.IsActive() {
			m.overlays.HandlePaste(msg.Content)
			return m, nil
		}
		if m.state == stateFocusAgent {
			if selected := m.nav.GetSelectedInstance(); selected != nil && session.NormalizeExecutionMode(selected.ExecutionMode) == session.ExecutionModeSDK {
				return m.handleSDKComposerPaste(selected, msg.Content)
			}
		}
		// Forward pasted text to the embedded PTY in focus mode.
		if m.state == stateFocusAgent && m.previewTerminal != nil {
			if content := msg.Content; content != "" && !pasteContentLooksBinary(content) {
				// Wrap in bracketed paste so the program inside tmux sees it
				// as a paste event rather than typed input.
				data := []byte("\x1b[200~" + content + "\x1b[201~")
				_ = m.previewTerminal.SendKey(data)
			} else {
				// Empty paste content means the clipboard holds non-text data
				// (e.g. an image). Forward raw ctrl+v (0x16) so the embedded
				// program can request clipboard contents via OSC 52 or its own
				// native paste mechanism.
				_ = m.previewTerminal.SendKey([]byte{0x16})
			}
			return m, nil
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		// Forward unknown CSI sequences (kitty keyboard protocol, etc.) to the
		// embedded PTY when in focus/interactive mode. Bubbletea emits these as
		// an unexported []byte-based type; use reflect to extract raw bytes.
		if m.state == stateFocusAgent && m.previewTerminal != nil {
			v := reflect.ValueOf(msg)
			if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
				if data := v.Bytes(); len(data) > 0 {
					_ = m.previewTerminal.SendKey(data)
				}
				return m, nil
			}
		}
	}
	return m, nil
}

func nextPreviewTickCmd(term *session.EmbeddedTerminal) tea.Cmd {
	return func() tea.Msg {
		if term != nil {
			term.WaitForRender(16 * time.Millisecond)
		} else {
			time.Sleep(50 * time.Millisecond)
		}
		return previewTickMsg{}
	}
}

func readClipboardCmd(selection byte) tea.Cmd {
	return func() tea.Msg {
		if selection == ansi.PrimaryClipboard {
			return tea.ReadPrimaryClipboard()
		}
		return tea.ReadClipboard()
	}
}

func (m *home) handleQuit() (tea.Model, tea.Cmd) {
	// Check if any instances are actively running or loading.
	hasActive := false
	for _, inst := range m.nav.GetInstances() {
		if inst.Status == session.Running || inst.Status == session.Loading {
			hasActive = true
			break
		}
	}

	if hasActive {
		quitAction := func() tea.Msg {
			m.audit(auditlog.EventSessionStopped, "kasmos stopped")
			_ = m.saveAllInstances()
			return tea.QuitMsg{}
		}
		return m, m.confirmAction("quit kasmos? active sessions will be preserved.", quitAction)
	}

	m.audit(auditlog.EventSessionStopped, "kasmos stopped")
	if err := m.saveAllInstances(); err != nil {
		return m, m.handleError(err)
	}
	return m, tea.Quit
}

func (m *home) View() tea.View {
	// All columns use identical padding and height for uniform alignment.
	colStyle := lipgloss.NewStyle().Height(m.contentHeight)
	previewWithPadding := colStyle.Render(m.tabbedWindow.String())

	// Layout: nav | preview/tabs
	var cols []string
	if !m.sidebarHidden {
		cols = append(cols, colStyle.Render(m.nav.String()))
	}
	cols = append(cols, previewWithPadding)
	listAndPreview := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	statusBarView := ""
	if m.statusBar != nil {
		m.statusBar.SetData(m.computeStatusBarData())
		statusBarView = m.statusBar.String()
	}
	if m.menu != nil && m.nav != nil {
		m.menu.SetSidebarSpaceAction(m.nav.SelectedSpaceAction())
	}
	accentView := ""
	if m.state == stateFocusAgent {
		// No accent strip in focus mode — the rose menu bar extends to the bottom edge.
	} else if m.appConfig != nil {
		accentView = ui.NewAccentStrip(m.appConfig.AccentColor, m.termWidth)
	}

	parts := []string{statusBarView, listAndPreview, m.menu.String()}
	if accentView != "" {
		parts = append(parts, accentView)
	}

	mainView := lipgloss.JoinVertical(lipgloss.Left, parts...)

	result := m.overlays.Render(mainView)

	if toastView := m.toastManager.View(); toastView != "" {
		x, y := m.toastManager.GetPosition()
		result = overlay.PlaceOverlay(x, y, toastView, result, false, false)
	}

	// Process bubblezone markers before rendering is complete
	// (zone markers inflate lipgloss.Width if left in place).
	result = safeZoneScan(result)

	// Height-fill — ensure enough lines for bubbletea's alt-screen renderer.
	// OSC 11 handles the actual background color; this just pads vertically.
	result = ui.FillBackground(result, m.termHeight)

	v := tea.NewView(result)
	v.AltScreen = true
	// We only use click/release/wheel interactions. All-motion floods Update with
	// hover events and makes the full-screen zone scan/path render laggy.
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func safeZoneScan(s string) (scanned string) {
	defer func() {
		if recover() != nil {
			scanned = s
		}
	}()

	return zone.Scan(s)
}

// permissionAutoApproveMsg is sent when a cached "allow always" pattern is detected.
type permissionAutoApproveMsg struct {
	instance *session.Instance
}

// permissionResponseMsg is sent when the user confirms a permission choice in the modal.
type permissionResponseMsg struct {
	instance *session.Instance
	choice   overlay.PermissionChoice
	pattern  string
}

// prCreatedMsg is sent when async PR creation succeeds.
type prCreatedMsg struct {
	instanceTitle string
	prTitle       string
}

// prCreatedForPlanMsg is sent when automatic PR creation on review approval succeeds.
type prCreatedForPlanMsg struct {
	planFile string
	url      string
}

// prStateUpdateMsg carries updated PR review/check state for a single plan.
type prStateUpdateMsg struct {
	planFile       string
	reviewDecision string
	checkStatus    string
}

// prErrorMsg is sent when async PR creation fails.
type prErrorMsg struct {
	id  string
	err error
}

// previewTickMsg implements tea.Msg and triggers a preview update
type previewTickMsg struct{}

type tickUpdateMetadataMessage struct{}

// previewTerminalReadyMsg signals that the async terminal attach completed.
type previewTerminalReadyMsg struct {
	term        *session.EmbeddedTerminal
	instanceKey string
	err         error
}

type instanceChangedMsg struct{}

// killInstanceMsg is sent after async pre-kill checks pass (worktree not checked out).
// Model mutations (list.Kill, removeFromAllInstances) happen in Update, not in the goroutine.
type killInstanceMsg struct {
	title string
}

// taskStageConfirmedMsg is sent when the user confirms proceeding past the
// topic-concurrency gate. Re-enters plan stage execution skipping the
// concurrency check that was already acknowledged.
type taskStageConfirmedMsg struct {
	planFile string
	stage    string
}

// taskRefreshMsg triggers a plan state reload and sidebar refresh in Update.
type taskRefreshMsg struct{}

// startOverCompletedMsg is emitted after the async start-over reset finishes.
// Update removes old runtime rows and starts replacement planning agents.
type startOverCompletedMsg struct {
	planFile    string
	planName    string
	description string
}

// lifecycleActionRejectedMsg is sent when a lifecycle action is rejected because
// the task's FSM state changed after the context menu was opened. This is expected
// UI churn (stale menu race), not an audit-log error.
type lifecycleActionRejectedMsg struct{ message string }

// waveAdvanceMsg is sent when the user confirms advancing to the next wave.
type waveAdvanceMsg struct {
	planFile string
	entry    taskstate.TaskEntry
}

// waveRetryMsg is sent when the user chooses "retry" on the failed-wave decision prompt.
type waveRetryMsg struct {
	planFile string
	entry    taskstate.TaskEntry
}

// waveAbortMsg is sent when the user chooses "abort" on the failed-wave decision prompt.
type waveAbortMsg struct {
	planFile string
}

// waveAllCompleteMsg is sent when the user confirms advancing to review
// after all waves in a plan have finished.
type waveAllCompleteMsg struct {
	planFile string
}

// wavePushCompleteMsg is sent when the async wave-complete push path finishes.
type wavePushCompleteMsg struct {
	planFile string
}

// coderCompleteMsg is sent when a single-coder (non-wave) implementation finishes
// and the user confirms pushing. Triggers FSM transition and reviewer spawn.
type coderCompleteMsg struct {
	planFile string
}

// plannerCompleteMsg is sent when the user confirms starting implementation
// after a planner session finishes.
type plannerCompleteMsg struct {
	planFile string
}

// tmuxSessionsMsg carries discovered kas_ tmux sessions (managed + orphaned).
type tmuxSessionsMsg struct {
	sessions []tmux.SessionInfo
	err      error
}

// tmuxKillResultMsg is sent after an orphaned tmux session is killed.
type tmuxKillResultMsg struct {
	name string
	err  error
}

type manualSignalResultMsg struct {
	signalType    string
	planFile      string
	instanceTitle string
	agentType     string
	successToast  string
	err           error
}

// tmuxAttachReturnMsg is sent when the user detaches from a passively attached orphan session.
type tmuxAttachReturnMsg struct{}

func clearScreenCmd() tea.Cmd {
	return func() tea.Msg {
		return tea.ClearScreen()
	}
}

// rawClearScreenCmd emits ANSI "erase display + cursor home" directly to the
// terminal, bypassing bubbletea's differential renderer.  This guarantees the
// physical screen is blank before the renderer's next flush, preventing stale
// alt-screen content from being visible after returning from tea.Exec.
func rawClearScreenCmd() tea.Cmd {
	return tea.Raw("\033[2J\033[H")
}

// clickUpDetectedMsg is sent at startup when ClickUp MCP is detected.
type clickUpDetectedMsg struct {
	Config clickup.MCPServerConfig
}

// clickUpSearchResultMsg is sent when ClickUp search completes.
type clickUpSearchResultMsg struct {
	Results []clickup.SearchResult
	Query   string // original query, used to retry after workspace selection
	Err     error
}

// clickUpTaskFetchedMsg is sent when a full ClickUp task is fetched.
type clickUpTaskFetchedMsg struct {
	Task *clickup.Task
	Err  error
}

// addInstanceFinalizer registers a finalizer for the given instance.
// Lazily initializes the map so tests that don't pre-initialize it still work.
func (m *home) addInstanceFinalizer(inst *session.Instance, fn func()) {
	if m.instanceFinalizers == nil {
		m.instanceFinalizers = make(map[*session.Instance]func())
	}
	m.clearDismissedInstanceTitle(inst.Title)
	m.instanceFinalizers[inst] = fn
}

func (m *home) markInstanceTitleDismissed(title string) {
	if strings.TrimSpace(title) == "" {
		return
	}
	if m.dismissedInstanceTitles == nil {
		m.dismissedInstanceTitles = make(map[string]struct{})
	}
	m.dismissedInstanceTitles[title] = struct{}{}
}

func (m *home) clearDismissedInstanceTitle(title string) {
	if m.dismissedInstanceTitles == nil || strings.TrimSpace(title) == "" {
		return
	}
	delete(m.dismissedInstanceTitles, title)
}

func (m *home) isInstanceTitleDismissed(title string) bool {
	if m.dismissedInstanceTitles == nil || strings.TrimSpace(title) == "" {
		return false
	}
	_, ok := m.dismissedInstanceTitles[title]
	return ok
}

func (m *home) reconcileDismissedInstanceTitles(daemonTitles []string) {
	if len(m.dismissedInstanceTitles) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(daemonTitles))
	for _, title := range daemonTitles {
		if strings.TrimSpace(title) == "" {
			continue
		}
		seen[title] = struct{}{}
	}
	for title := range m.dismissedInstanceTitles {
		if _, ok := seen[title]; !ok {
			delete(m.dismissedInstanceTitles, title)
		}
	}
}

// instanceStartedMsg is sent when an async instance startup completes.
type instanceStartedMsg struct {
	instance     *session.Instance
	err          error
	startGroupID string
}

// promptSubmittedMsg is sent when an async prompt delivery finishes.
type promptSubmittedMsg struct {
	instance *session.Instance
	auditMsg string
	err      error
}

// shellCommandSubmittedMsg is sent when an async shell execution finishes.
type shellCommandSubmittedMsg struct {
	instance *session.Instance
	auditMsg string
	err      error
}

type keyupMsg struct{}

// doubleTapTimeoutMsg fires when the debounce window expires for a debounced
// key (s, space). If no second tap arrived in the window, the original
// single-press action is dispatched via handleDoubleTapTimeout.
type doubleTapTimeoutMsg struct {
	key string
	seq int
}

// scheduleDoubleTapTimeout returns a Cmd that delivers doubleTapTimeoutMsg after
// delay. Overridable in tests to make timing synchronous without real sleeps.
var scheduleDoubleTapTimeout = func(delay time.Duration, key string, seq int) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return doubleTapTimeoutMsg{key: key, seq: seq}
	})
}

// planRenderedMsg delivers the async glamour render result back to the Update loop.
type planRenderedMsg struct {
	planFile string
	rendered string
	err      error
}

func rendererStatsFromDaemon(rs *daemonapi.RendererStats) *sessionsdk.RendererStats {
	if rs == nil {
		return nil
	}
	return &sessionsdk.RendererStats{
		Bytes:         rs.Bytes,
		Lines:         rs.Lines,
		Turns:         rs.Turns,
		MaxBytes:      rs.MaxBytes,
		MaxTurns:      rs.MaxTurns,
		EvictedTurns:  rs.EvictedTurns,
		EvictedLines:  rs.EvictedLines,
		EvictedBytes:  rs.EvictedBytes,
		TruncatedRows: rs.TruncatedRows,
	}
}

// instanceMetadata holds the results of polling a single instance's subprocess data.
// Collected in a goroutine, applied to the model in Update.
type instanceMetadata struct {
	Title              string
	Content            string // tmux capture-pane output (reused for preview, activity, hash)
	ContentCaptured    bool
	PresentationTurns  []*sessionsdk.PresentationTurn
	PresentationCached bool
	// RendererStats carries the latest SDK renderer stats. Nil when stats are
	// not available (tmux instances or daemon not reachable).
	RendererStats      *sessionsdk.RendererStats
	Updated            bool
	HasPrompt          bool
	CPUPercent         float64
	MemMB              float64
	ResourceUsageValid bool
	TmuxAlive          bool
	PermissionPrompt   *session.PermissionPrompt // non-nil when a supported harness shows a permission dialog
}

// metadataResultMsg carries all per-instance metadata collected by the async tick.
type metadataResultMsg struct {
	Results              []instanceMetadata
	PlanState            *taskstate.TaskState         // pre-loaded plan state (nil if dir not set)
	PlanStateLoadedAt    time.Time                    // when PlanState was loaded in the background tick goroutine
	DaemonTaskState      bool                         // true when PlanState came from the daemon task-list API
	Signals              []taskfsm.Signal             // agent sentinel files found this tick
	TaskSignals          []taskfsm.TaskSignal         // task completion sentinel files found this tick
	WaveSignals          []taskfsm.WaveSignal         // implement-wave-N signal files found this tick
	ElaborationSignals   []taskfsm.ElaborationSignal  // architect completion signal files found this tick
	PlannerDraftSignals  []taskfsm.PlannerDraftSignal // planner_draft_finished gateway rows found this tick
	GatewaySignalEntries []*taskstore.SignalEntry     // claimed DB-backed signal rows to classify+mark after processing
	DaemonManagedRepo    bool                         // true when the active repo is managed by a running daemon
	DaemonInstances      []*session.Instance          // daemon-tracked instances missing from the local nav model
	DaemonTitles         []string                     // all daemon-reported instance titles for dismissal reconciliation
	TmuxSessionCount     int                          // number of kas_-prefixed tmux sessions
	PRStateUpdates       []prStateUpdateMsg           // PR review/check state refreshed this tick
}

// tickUpdateMetadataCmd is the callback to update the metadata of the instances every 200ms. We iterate
// over all instances and capture their output, but each tmux capture-pane call is <5ms so this is fine
// even at 20 instances (~100ms total). 200ms gives 5 ticks/sec for responsive signal processing.
var tickUpdateMetadataCmd = func() tea.Msg {
	time.Sleep(200 * time.Millisecond)
	return tickUpdateMetadataMessage{}
}

func (m *home) toastTickCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(50 * time.Millisecond)
		return overlay.ToastTickMsg{}
	}
}

func (m *home) searchClickUp(query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, clickUpOpTimeout)
		defer cancel()

		importer, err := m.getOrCreateImporter(ctx)
		if err != nil {
			return clickUpSearchResultMsg{Query: query, Err: normalizeClickUpError(err)}
		}

		searchDone := make(chan clickUpSearchResultMsg, 1)
		go func() {
			results, searchErr := importer.Search(query)
			searchDone <- clickUpSearchResultMsg{Query: query, Results: results, Err: searchErr}
		}()

		select {
		case msg := <-searchDone:
			msg.Err = normalizeClickUpError(msg.Err)
			if msg.Err != nil {
				// Don't nil the importer for MultipleWorkspacesError — we need
				// to call SetWorkspaceID on it after the user picks a workspace.
				var mwErr *clickup.MultipleWorkspacesError
				if errors.As(msg.Err, &mwErr) {
					// Resolve workspace IDs to names for a better picker UX.
					mwErr.WorkspaceNames = importer.FetchWorkspaceNames(mwErr.WorkspaceIDs)
				} else {
					m.clickUpImporter = nil // force re-init on next attempt
				}
			}
			return msg
		case <-ctx.Done():
			m.clickUpImporter = nil // force re-init on next attempt
			return clickUpSearchResultMsg{Query: query, Err: normalizeClickUpError(ctx.Err())}
		}
	}
}

func (m *home) fetchClickUpTaskWithTimeout(taskID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, clickUpOpTimeout)
		defer cancel()

		if m.clickUpImporter == nil {
			return clickUpTaskFetchedMsg{Err: fmt.Errorf("importer not initialized")}
		}

		fetchDone := make(chan clickUpTaskFetchedMsg, 1)
		go func() {
			task, fetchErr := m.clickUpImporter.FetchTask(taskID)
			fetchDone <- clickUpTaskFetchedMsg{Task: task, Err: fetchErr}
		}()

		select {
		case msg := <-fetchDone:
			msg.Err = normalizeClickUpError(msg.Err)
			if msg.Err != nil {
				m.clickUpImporter = nil // force re-init on next attempt
			}
			return msg
		case <-ctx.Done():
			m.clickUpImporter = nil // force re-init on next attempt
			return clickUpTaskFetchedMsg{Err: normalizeClickUpError(ctx.Err())}
		}
	}
}

func normalizeClickUpError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("operation canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("operation timed out after %s", clickUpOpTimeout)
	}
	return err
}

func (m *home) getOrCreateImporter(ctx context.Context) (*clickup.Importer, error) {
	if m.clickUpImporter != nil {
		return m.clickUpImporter, nil
	}
	if m.clickUpConfig == nil {
		return nil, fmt.Errorf("no clickup MCP server configured")
	}

	transport, err := m.createTransport(ctx, *m.clickUpConfig)
	if err != nil {
		return nil, err
	}

	client, err := mcpclient.NewClient(transport)
	if err != nil {
		_ = transport.Close()
		return nil, err
	}
	if err := client.Initialize(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("MCP initialize: %w", err)
	}
	if _, err := client.ListTools(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("MCP list tools: %w", err)
	}

	m.clickUpMCPClient = client
	m.clickUpImporter = clickup.NewImporter(client)

	// Restore saved workspace_id from per-project config.
	if projCfg := clickup.LoadProjectConfig(m.activeRepoPath); projCfg.WorkspaceID != "" {
		m.clickUpImporter.SetWorkspaceID(projCfg.WorkspaceID)
	}

	return m.clickUpImporter, nil
}

func (m *home) createTransport(ctx context.Context, cfg clickup.MCPServerConfig) (mcpclient.Transport, error) {
	switch cfg.Type {
	case "http":
		token, err := m.getClickUpToken(ctx)
		if err != nil {
			return nil, err
		}
		return mcpclient.NewHTTPTransport(cfg.URL, token), nil
	case "stdio":
		envSlice := make([]string, 0, len(cfg.Env))
		for k, v := range cfg.Env {
			envSlice = append(envSlice, k+"="+v)
		}
		return mcpclient.NewStdioTransport(cfg.Command, cfg.Args, envSlice)
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", cfg.Type)
	}
}

func (m *home) getClickUpToken(ctx context.Context) (string, error) {
	// 1. Check opencode's mcp-auth.json first (populated by `opencode mcp auth clickup`).
	ocPath := mcpclient.OpencodeMCPAuthPath()
	if tok, err := mcpclient.LoadOpencodeToken(ocPath, "clickup"); err == nil && !tok.IsExpired() {
		return tok.AccessToken, nil
	}

	// 2. Fall back to kasmos's own cached token.
	path := mcpclient.TokenPath()
	tok, err := mcpclient.LoadToken(path)
	if err == nil && !tok.IsExpired() {
		return tok.AccessToken, nil
	}

	// 3. Last resort: run our own OAuth flow.
	oauthCfg := mcpclient.OAuthConfig{
		AuthURL:  "https://app.clickup.com/api",
		TokenURL: "https://api.clickup.com/api/v2/oauth/token",
		ClientID: "kasmos", // TODO: register ClickUp OAuth app
	}
	tok, err = mcpclient.OAuthFlow(ctx, oauthCfg, nil)
	if err != nil {
		return "", fmt.Errorf("oauth: %w", err)
	}
	if err := mcpclient.SaveToken(path, tok); err != nil {
		return "", fmt.Errorf("save token: %w", err)
	}
	return tok.AccessToken, nil
}

func detectClickUpCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		claudeDir := filepath.Join(os.Getenv("HOME"), ".claude")
		cfg, found := clickup.DetectMCP(repoPath, claudeDir)
		if !found {
			return nil
		}
		return clickUpDetectedMsg{Config: cfg}
	}
}

// lifecycleSnapshot returns the (status, phase) pair that uniquely identifies
// the current FSM position of a task entry. Used to detect drift while a
// context menu is open.
func lifecycleSnapshot(entry taskstate.TaskEntry) (taskstate.Status, string) {
	return entry.Status, strings.TrimSpace(entry.ExecutionState.Phase)
}

// lifecycleSnapshotLabel formats the FSM snapshot as a human-readable label.
// Phase-aware: emits labels such as "ready (planned)" or "implementing (architecting)"
// so dismissal toasts explain which state the task moved to.
func lifecycleSnapshotLabel(entry taskstate.TaskEntry) string {
	status := string(entry.Status)
	phase := strings.TrimSpace(entry.ExecutionState.Phase)
	if phase != "" {
		return fmt.Sprintf("%s (%s)", status, phase)
	}
	return status
}

// clearContextMenuTracking zeroes the fields that track the task backing an open
// context menu. Must be called on every stateContextMenu exit path.
func (m *home) clearContextMenuTracking() {
	m.contextMenuTaskFile = ""
	m.contextMenuTaskStatus = ""
	m.contextMenuTaskPhase = ""
}

// drainDeferredDialogs drains one item from each deferred-dialog queue in order:
// planner, all-complete, coder-push, wave, permission. It is a no-op when the
// user is in focus mode or has any overlay open. Returns the async cmds produced.
func (m *home) drainDeferredDialogs() []tea.Cmd {
	if m.state == stateFocusAgent || m.isUserInOverlay() {
		return nil
	}
	var cmds []tea.Cmd

	// 1. Planner dialogs
	if len(m.deferredPlannerDialogs) > 0 {
		planFile := m.deferredPlannerDialogs[0]
		m.deferredPlannerDialogs = m.deferredPlannerDialogs[1:]
		if !m.plannerPrompted[planFile] {
			if cmd := m.showPlannerDialog(planFile); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	// 2. All-complete prompts — purge already-dismissed/advancing entries first.
	if len(m.pendingAllComplete) > 0 && (len(m.allCompleteDismissed) > 0 || len(m.allCompleteAdvancing) > 0) {
		filtered := m.pendingAllComplete[:0]
		for _, pf := range m.pendingAllComplete {
			if !m.allCompleteDismissed[pf] && !m.allCompleteAdvancing[pf] {
				filtered = append(filtered, pf)
			}
		}
		m.pendingAllComplete = filtered
	}
	if len(m.pendingAllComplete) > 0 {
		planFile := m.pendingAllComplete[0]
		m.pendingAllComplete = m.pendingAllComplete[1:]
		if cmd := m.focusPlanInstanceForOverlay(planFile); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.showAllCompletePrompt(planFile)
	}

	// 3. Coder push dialogs
	if len(m.deferredCoderPushDialogs) > 0 {
		planFile := m.deferredCoderPushDialogs[0]
		m.deferredCoderPushDialogs = m.deferredCoderPushDialogs[1:]
		if cmd := m.showCoderPushDialog(planFile); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// 4. Wave dialogs
	if len(m.deferredWaveDialogs) > 0 {
		planFile := m.deferredWaveDialogs[0]
		m.deferredWaveDialogs = m.deferredWaveDialogs[1:]
		if orch, ok := m.waveOrchestrators[planFile]; ok {
			cmds = append(cmds, m.showWaveDialog(planFile, orch)...)
		}
	}

	// 5. Permission prompts
	if len(m.deferredPermissionPrompts) > 0 {
		deferred := m.deferredPermissionPrompts[0]
		m.deferredPermissionPrompts = m.deferredPermissionPrompts[1:]
		if cmd := m.showPermissionPrompt(deferred); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return cmds
}
