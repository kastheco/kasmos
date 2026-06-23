package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/internal/cmdexec"
	"github.com/kastheco/kasmos/internal/opencodesession"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/session/resourcecontrol"
	"golang.org/x/term"
)

const ProgramClaude = "claude"

var agentFlagPattern = regexp.MustCompile(`(^|[[:space:]])--agent([[:space:]]|$)`)

const ProgramAider = "aider"
const ProgramGemini = "gemini"
const ProgramOpenCode = "opencode"
const ProgramCodex = "codex"

// codexBypassFlag disables the codex sandbox and approval prompts so kasmos
// can drive codex non-interactively. Verified against `codex --help`.
const codexBypassFlag = "--dangerously-bypass-approvals-and-sandbox"

// codexGracePeriod is the minimum time to wait before treating a codex session
// as ready when no stable startup banner is available. Exposed as a var so
// tests can shorten it without real-time delays.
var codexGracePeriod = 2 * time.Second

var (
	sessionStartWaitTimeout          = 2 * time.Second
	sessionStartPollInitialDelay     = 5 * time.Millisecond
	sessionStartPollMaxDelay         = 50 * time.Millisecond
	programReadyMaxWaitTime          = 30 * time.Second
	programReadyPollInitialDelay     = 100 * time.Millisecond
	programReadyPollMaxDelay         = time.Second
	programReadySessionCheckInterval = 3 * time.Second
)

// ansiRe strips ANSI escape sequences (SGR, cursor movement, etc.) so that
// content hashing is not affected by cursor blink, color resets, or other
// terminal control codes that change between captures of an otherwise-idle pane.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

var resolveProgramPath = config.ResolveCommandPath

// Executor aliases the canonical command executor type used across packages.
type Executor = cmdexec.Executor

// TmuxSession represents a managed tmux session.
// It implements the Session interface defined in session.go.
type TmuxSession struct {
	// Initialized by NewTmuxSession
	//
	// sanitizedName is the kas_-prefixed, whitespace-free tmux session name.
	sanitizedName string
	program       string
	// ptyFactory is used to create a PTY for the tmux session.
	ptyFactory PtyFactory
	// cmdExec is used to execute commands in the tmux session.
	cmdExec Executor
	// skipPermissions appends --permission-mode bypassPermissions to Claude commands.
	skipPermissions bool
	// agentType, when non-empty, appends --agent <type> to the program command.
	agentType string
	// initialPrompt, when non-empty, is baked into the CLI command at Start()
	// using per-program syntax (--prompt for opencode, positional for claude).
	initialPrompt string
	// taskNumber, waveNumber, peerCount are set for wave task instances.
	// When non-zero, they are prepended as KASMOS_TASK, KASMOS_WAVE, KASMOS_PEERS
	// env vars to the program command string.
	taskNumber int
	waveNumber int
	peerCount  int
	// project is the repository base name, set via SetProject. When non-empty,
	// KASMOS_PROJECT=<project> is prepended before KASMOS_MANAGED=1 at Start() time.
	project string
	// noFlicker controls whether CLAUDE_CODE_NO_FLICKER is set to 1 or 0.
	// Defaults to false (0) so prompt detection works correctly in spawned agents.
	noFlicker bool
	// resourceControls holds the resolved resource-control policy set by SetResourceControls.
	// Applied in Start() to wrap the agent command with nice/ionice and inject env vars.
	resourceControls config.ResolvedResourceControls
	// ProgressFunc is called with (stage, description) during Start() to report progress.
	ProgressFunc func(stage int, desc string)
	// promptFile is the path to a temporary file containing the initial prompt.
	// Set during Start() when the prompt exceeds maxInlinePromptLen. Cleaned up by Close().
	promptFile string
	// sessionTitle is the desired human-readable title for the opencode session.
	// When non-empty and titleFunc is set, the title is applied after the session
	// reports ready (after "Ask anything" is detected).
	sessionTitle string
	// titleFunc is a callback that performs the actual DB title update for opencode sessions.
	// It is injected by the instance layer to avoid a direct dependency from tmux → opencodesession.
	// Called as a goroutine (best-effort, non-blocking) after session startup.
	titleFunc func(workDir string, beforeStart time.Time, title string)

	// Initialized by Start or Restore
	//
	// ptmx is the PTY file for the active interactive attach. It is non-nil
	// only between a successful Attach() call and the subsequent Detach() or
	// Close(). During detached preview/monitoring, ptmx is nil.
	ptmx *os.File
	// ptmxHandle owns the PTY file and child process for the active attach.
	// It is set alongside ptmx in Attach() and cleared by closeActivePty.
	ptmxHandle PtyHandle
	// monitor monitors the tmux pane content and sends signals to the UI when its status changes.
	monitor *StatusMonitor

	// Initialized by Attach; deinitialized by Detach.
	//
	// attachCh is closed at the very end of detaching. Used to signal callers.
	attachCh chan struct{}
	// ctx, cancel, wg manage goroutines launched by Attach.
	ctx    context.Context
	cancel func()
	wg     *sync.WaitGroup
	// outerMouseWasEnabled is set when Attach() disables mouse on the outer tmux
	// session so Detach() can restore it.
	outerMouseWasEnabled bool
	stdinFD              int
	rawInputState        *term.State
}

// TmuxPrefix is the prefix added to all kas-managed tmux session names.
const TmuxPrefix = "kas_"

var whiteSpaceRegex = regexp.MustCompile(`\s+`)

// cleanupSessionsRe matches current kas_ sessions and legacy klique_/hivemind_ sessions.
var cleanupSessionsRe = regexp.MustCompile(`(?:kas_|klique_|hivemind_).*:`)

// toKasTmuxName converts a human-readable name to a kas_-prefixed tmux session name.
// Whitespace is removed and dots are replaced with underscores (tmux does this natively).
func toKasTmuxName(str string) string {
	str = whiteSpaceRegex.ReplaceAllString(str, "")
	str = strings.ReplaceAll(str, ".", "_") // tmux replaces all . with _
	return fmt.Sprintf("%s%s", TmuxPrefix, str)
}

// ToKasTmuxNamePublic is the exported version of toKasTmuxName for use by the app layer.
func ToKasTmuxNamePublic(name string) string {
	return toKasTmuxName(name)
}

// NewTmuxSession creates a new TmuxSession with the given name and program.
func NewTmuxSession(name string, program string, skipPermissions bool) *TmuxSession {
	return newTmuxSession(name, program, skipPermissions, MakePtyFactory(), cmdexec.Make())
}

// NewTmuxSessionWithDeps creates a new TmuxSession with provided dependencies for testing.
func NewTmuxSessionWithDeps(name string, program string, skipPermissions bool, ptyFactory PtyFactory, cmdExec Executor) *TmuxSession {
	return newTmuxSession(name, program, skipPermissions, ptyFactory, cmdExec)
}

func newTmuxSession(name string, program string, skipPermissions bool, ptyFactory PtyFactory, cmdExec Executor) *TmuxSession {
	return &TmuxSession{
		sanitizedName:   toKasTmuxName(name),
		program:         program,
		skipPermissions: skipPermissions,
		ptyFactory:      ptyFactory,
		cmdExec:         cmdExec,
	}
}

// NewReset creates a fresh TmuxSession preserving the ptyFactory and cmdExec
// from the receiver. Used by Instance.Restart() to avoid replacing injected
// test dependencies with real implementations.
func (t *TmuxSession) NewReset(name string, program string, skipPermissions bool) *TmuxSession {
	return newTmuxSession(name, program, skipPermissions, t.ptyFactory, t.cmdExec)
}

// NewTmuxSessionFromExisting creates a TmuxSession that wraps an already-running
// tmux session identified by its raw sanitized name. Used for adopting orphans.
func NewTmuxSessionFromExisting(sanitizedName string, program string, skipPermissions bool) *TmuxSession {
	return &TmuxSession{
		sanitizedName:   sanitizedName,
		program:         program,
		skipPermissions: skipPermissions,
		ptyFactory:      MakePtyFactory(),
		cmdExec:         cmdexec.Make(),
	}
}

// SetAgentType sets the agent type flag to inject at startup (planner/coder/reviewer).
// The value is trimmed of surrounding whitespace.
func (t *TmuxSession) SetAgentType(agentType string) {
	t.agentType = strings.TrimSpace(agentType)
}

// SetInitialPrompt sets the initial prompt to bake into the CLI command at launch.
// Supported programs: opencode (--prompt), claude and codex (positional arg).
// For unsupported programs the prompt is ignored; callers should keep
// QueuedPrompt set so the send-keys fallback fires.
func (t *TmuxSession) SetInitialPrompt(prompt string) {
	t.initialPrompt = prompt
}

// SetTaskEnv sets the task identity env vars for parallel wave execution.
// When set, KASMOS_TASK, KASMOS_WAVE, and KASMOS_PEERS are prepended to the
// program command string at Start() time.
func (t *TmuxSession) SetTaskEnv(taskNumber, waveNumber, peerCount int) {
	t.taskNumber = taskNumber
	t.waveNumber = waveNumber
	t.peerCount = peerCount
}

// SetSessionTitle sets the desired title for the opencode session.
// When non-empty and a titleFunc is also set, the title is applied after
// the session reports ready. The "kas: " prefix is expected to already be
// included by the caller (e.g. from BuildTitle in the opencodesession package).
func (t *TmuxSession) SetSessionTitle(title string) {
	t.sessionTitle = title
}

// SetTitleFunc sets the callback used to apply the session title after startup.
// The callback receives the work directory, a timestamp taken just before the
// tmux session was created, and the desired title string. It runs in a goroutine
// (fire-and-forget) so title setting never blocks or delays session startup.
func (t *TmuxSession) SetTitleFunc(fn func(workDir string, beforeStart time.Time, title string)) {
	t.titleFunc = fn
}

// SetProgressFunc sets the callback used to report progress during startup.
func (t *TmuxSession) SetProgressFunc(fn func(stage int, desc string)) {
	t.ProgressFunc = fn
}

// SetNoFlicker controls whether CLAUDE_CODE_NO_FLICKER is set to 1 (true) or 0 (false).
// Must be called before Start().
func (t *TmuxSession) SetNoFlicker(enabled bool) {
	t.noFlicker = enabled
}

// SetProject sets the repository project name injected as KASMOS_PROJECT at Start() time.
// Must be called before Start(). An empty string disables the injection.
func (t *TmuxSession) SetProject(project string) {
	t.project = project
}

// SetResourceControls stores the resolved resource policy applied during Start().
// The wrapper wraps the agent command with nice/ionice and prepends build-concurrency
// env vars. Must be called before Start().
func (t *TmuxSession) SetResourceControls(rc config.ResolvedResourceControls) {
	t.resourceControls = rc
}

// reportProgress calls ProgressFunc if set.
func (t *TmuxSession) reportProgress(stage int, desc string) {
	if t.ProgressFunc != nil {
		t.ProgressFunc(stage, desc)
	}
}

// programBase returns the base executable name from the first whitespace-delimited
// token in program. This correctly handles command strings that include flags
// (e.g. "claude --model opus" → "claude", "/usr/local/bin/opencode" → "opencode").
func programBase(program string) string {
	trimmed := strings.TrimSpace(program)
	exe := trimmed
	if i := strings.IndexAny(trimmed, " \t"); i >= 0 {
		exe = trimmed[:i]
	}
	return filepath.Base(exe)
}

// isClaudeProgram returns true if the program string refers to Claude Code.
func isClaudeProgram(program string) bool {
	return programBase(program) == ProgramClaude
}

// isAiderProgram returns true if the program string refers to Aider.
// Kept for compatibility with existing tmux_io.go references during the clean-room rewrite.
// Will be removed when aider support is fully dropped.
func isAiderProgram(program string) bool {
	return strings.HasPrefix(program, ProgramAider)
}

// isGeminiProgram returns true if the program string refers to Gemini.
// Kept for compatibility with existing tmux_io.go references during the clean-room rewrite.
// Will be removed when gemini support is fully dropped.
func isGeminiProgram(program string) bool {
	return strings.HasPrefix(program, ProgramGemini)
}

// isOpenCodeProgram returns true if the program string refers to OpenCode.
func isOpenCodeProgram(program string) bool {
	return programBase(program) == ProgramOpenCode
}

// isCodexProgram returns true if the program string refers to the Codex CLI.
func isCodexProgram(program string) bool {
	return programBase(program) == ProgramCodex
}

// programSupportsAgentFlag returns true if the program accepts a --agent <type>
// flag injected by kasmos. Only Claude and OpenCode are allowlisted; codex and
// legacy programs do not support this flag.
func programSupportsAgentFlag(program string) bool {
	switch programBase(program) {
	case ProgramClaude, ProgramOpenCode:
		return true
	default:
		return false
	}
}

func resolveShellProgram(program string) string {
	trimmed := strings.TrimSpace(program)
	if trimmed == "" {
		return program
	}
	end := len(trimmed)
	for i, r := range trimmed {
		if r == ' ' || r == '\t' || r == '\n' {
			end = i
			break
		}
	}
	cmdName := trimmed[:end]
	if cmdName == "" || strings.Contains(cmdName, "/") {
		return trimmed
	}
	resolved, err := resolveProgramPath(cmdName)
	if err != nil || resolved == "" || resolved == cmdName {
		return trimmed
	}
	remainder := ""
	if end < len(trimmed) {
		remainder = trimmed[end:]
	}
	return shellEscapeSingleQuote(resolved) + remainder
}

// Start creates and starts a new tmux session, then attaches to it.
// program is the command to run in the session. workDir is the git worktree directory.
func (t *TmuxSession) Start(workDir string) error {
	// Reattach to a surviving session from a previous crash/interrupt
	// instead of killing it — the agent may still be running.
	if t.DoesSessionExist() {
		return t.Restore()
	}

	program := resolveShellProgram(t.program)

	// Resolve the harness-specific adapter once; nil for legacy programs (aider/gemini).
	adapter := AdapterFor(t.program)

	// Append permission-bypass flags. Skip if the profile-level flags already
	// include the equivalent — otherwise daemon-spawned claude/codex get the
	// flag twice when the user also pins it in .kasmos/config.toml.
	if t.skipPermissions && isClaudeProgram(t.program) && !strings.Contains(program, "--permission-mode bypassPermissions") {
		program = program + " --permission-mode bypassPermissions"
	}
	if t.skipPermissions && isCodexProgram(t.program) && !strings.Contains(program, codexBypassFlag) {
		program = program + " " + codexBypassFlag
	}

	// Inject --agent only for programs that recognise the flag (Claude, OpenCode).
	if t.agentType != "" && programSupportsAgentFlag(t.program) && !agentFlagPattern.MatchString(program) {
		program = program + " --agent " + t.agentType
	}

	// Bake the initial prompt into the CLI command using adapter-provided syntax.
	// OpenCode: --prompt <arg>. Claude and Codex: positional argument.
	// Aider/gemini: no CLI prompt support — callers keep QueuedPrompt set so
	// the send-keys fallback fires from the app tick handler.
	if t.initialPrompt != "" && adapter != nil && adapter.SupportsCliPrompt() {
		writeFileFunc := func(_ string) string {
			return t.writePromptFile(workDir)
		}
		promptArg := adapter.BuildPromptArg(t.initialPrompt, workDir, writeFileFunc)
		if isOpenCodeProgram(t.program) {
			program = program + " --prompt " + promptArg
		} else {
			// claude, codex: prompt is a positional argument.
			program = program + " " + promptArg
		}
	}
	if isOpenCodeProgram(t.program) {
		if configPath := opencodesession.ProjectConfigPath(workDir); configPath != "" {
			program = "OPENCODE_CONFIG=" + shellEscapeSingleQuote(configPath) + " " + program
		}
	}
	if isClaudeProgram(t.program) {
		flickerVal := "0"
		if t.noFlicker {
			flickerVal = "1"
		}
		program = "CLAUDE_CODE_NO_FLICKER=" + flickerVal + " " + program
	}
	if isCodexProgram(t.program) {
		// Quiet codex's tracing — INFO-level traces flood ~/.codex/log/codex-tui.log
		// and the shared ~/.codex/logs_2.sqlite, producing multi-MB/s write
		// amplification across concurrent agents.
		program = "RUST_LOG=warn OTEL_SDK_DISABLED=true " + program
	}

	// Redirect stderr to a per-session log file so kasmos-spawned agents
	// always have debug logs available for crash diagnosis.
	logDir := filepath.Join(workDir, promptDir, "logs")
	logFile := filepath.Join(logDir, t.sanitizedName+".log")
	if err := os.MkdirAll(logDir, 0o755); err == nil {
		if isOpenCodeProgram(t.program) {
			program = program + " --print-logs 2>>" + shellEscapeSingleQuote(logFile)
		} else {
			program = program + " 2>>" + shellEscapeSingleQuote(logFile)
		}
	}

	// Apply resource-control wrapper: wrap the program command with nice/ionice
	// and collect build-concurrency env assignments to prepend at the left.
	rcWrapper := resourcecontrol.New(t.resourceControls)
	program = rcWrapper.WrapShellCommand(program)
	if envAssignments := rcWrapper.InlineEnvAssignmentsFrom(os.Environ()); len(envAssignments) > 0 {
		program = strings.Join(envAssignments, " ") + " " + program
	}

	// Prepend KASMOS_MANAGED=1 so the agent process sees it from startup.
	program = "KASMOS_MANAGED=1 " + program

	// Prepend KASMOS_PROJECT before KASMOS_MANAGED so agents know which repo they are in.
	if t.project != "" {
		program = "KASMOS_PROJECT=" + shellEscapeSingleQuote(t.project) + " " + program
	}

	// Prepend task identity env vars for parallel wave execution.
	if t.taskNumber > 0 {
		program = fmt.Sprintf("KASMOS_TASK=%d KASMOS_WAVE=%d KASMOS_PEERS=%d %s",
			t.taskNumber, t.waveNumber, t.peerCount, program)
	}

	t.reportProgress(1, "Creating tmux session...")

	// Record the timestamp immediately before launching the tmux session.
	// This is passed to titleFunc so it can find the opencode DB session
	// that was created at or after this moment.
	beforeStart := time.Now()

	// Create a new detached tmux session and start the program in it.
	cmd := exec.Command("tmux", "new-session", "-d", "-s", t.sanitizedName, "-c", workDir, program)

	handle, err := t.ptyFactory.Start(cmd)
	if err != nil {
		// Cleanup any partially created session if any exists.
		if t.DoesSessionExist() {
			cleanupCmd := exec.Command("tmux", "kill-session", "-t", t.sanitizedName)
			if cleanupErr := t.cmdExec.Run(cleanupCmd); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			}
		}
		return fmt.Errorf("error starting tmux session: %w", err)
	}
	if handle == nil {
		// Test stub returned nil handle with nil error — treat as a start failure.
		return fmt.Errorf("error starting tmux session: PTY factory returned nil handle")
	}

	t.reportProgress(2, "Waiting for session to start...")

	// Poll for session existence with exponential backoff.
	timeout := time.After(sessionStartWaitTimeout)
	sleepDuration := sessionStartPollInitialDelay
	for !t.DoesSessionExist() {
		select {
		case <-timeout:
			// Reap the short-lived new-session handle before cleanup.
			_ = handle.Close()
			if cleanupErr := t.Close(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			}
			return fmt.Errorf("timed out waiting for tmux session %s: %v", t.sanitizedName, err)
		default:
			time.Sleep(sleepDuration)
			// Exponential backoff up to max.
			if sleepDuration < sessionStartPollMaxDelay {
				sleepDuration *= 2
			}
		}
	}
	// Close and reap the short-lived new-session PTY now that the tmux session exists.
	_ = handle.Close()

	// Set history limit to enable scrollback (default is 2000, we use 10000 for more history).
	historyCmd := exec.Command("tmux", "set-option", "-t", t.sanitizedName, "history-limit", "10000")
	if err := t.cmdExec.Run(historyCmd); err != nil {
		log.InfoLog.Printf("Warning: failed to set history-limit for session %s: %v", t.sanitizedName, err)
	}

	// Hide the tmux status bar — the kasmos TUI provides its own chrome.
	statusCmd := exec.Command("tmux", "set-option", "-t", t.sanitizedName, "status", "off")
	if err := t.cmdExec.Run(statusCmd); err != nil {
		log.InfoLog.Printf("Warning: failed to hide status bar for session %s: %v", t.sanitizedName, err)
	}

	// Set escape-time to 0 so ESC is forwarded immediately to the inner program.
	// Without this, tmux waits its default 500ms to disambiguate ESC from escape
	// sequences, making ESC feel broken in focus/interactive mode.
	escapeTimeCmd := exec.Command("tmux", "set-option", "-t", t.sanitizedName, "escape-time", "0")
	if err := t.cmdExec.Run(escapeTimeCmd); err != nil {
		log.InfoLog.Printf("Warning: failed to set escape-time for session %s: %v", t.sanitizedName, err)
	}

	// Disable mouse so tmux does not intercept scroll events or enter copy-mode.
	// Kasmos handles scrolling in its own preview viewport; inner-session mouse
	// mode (e.g. from OpenCode's bubbletea) would otherwise swallow wheel events.
	mouseCmd := exec.Command("tmux", "set-option", "-t", t.sanitizedName, "mouse", "off")
	if err := t.cmdExec.Run(mouseCmd); err != nil {
		log.InfoLog.Printf("Warning: failed to disable mouse for session %s: %v", t.sanitizedName, err)
	}

	// Inject KASMOS_MANAGED=1 so agents can detect they're running under kasmos orchestration.
	envCmd := exec.Command("tmux", "set-environment", "-t", t.sanitizedName, "KASMOS_MANAGED", "1")
	if err := t.cmdExec.Run(envCmd); err != nil {
		log.InfoLog.Printf("Warning: failed to set KASMOS_MANAGED env for session %s: %v", t.sanitizedName, err)
	}
	// Inject KASMOS_PROJECT so attached shells and any subprocess inherit the repo name.
	if t.project != "" {
		projectEnvCmd := exec.Command("tmux", "set-environment", "-t", t.sanitizedName, "KASMOS_PROJECT", t.project)
		if err := t.cmdExec.Run(projectEnvCmd); err != nil {
			log.InfoLog.Printf("Warning: failed to set KASMOS_PROJECT env for session %s: %v", t.sanitizedName, err)
		}
	}
	// Inject KASMOS_RESOURCE_PROFILE for non-normal profiles so attached interactive
	// shells inherit the profile name alongside the process-level env assignments.
	if t.resourceControls.Enabled {
		profileEnvCmd := exec.Command("tmux", "set-environment", "-t", t.sanitizedName, "KASMOS_RESOURCE_PROFILE", t.resourceControls.Profile)
		if err := t.cmdExec.Run(profileEnvCmd); err != nil {
			log.InfoLog.Printf("Warning: failed to set KASMOS_RESOURCE_PROFILE env for session %s: %v", t.sanitizedName, err)
		}
	}

	t.reportProgress(3, "Configuring session...")

	err = t.Restore()
	if err != nil {
		if cleanupErr := t.Close(); cleanupErr != nil {
			err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
		}
		return fmt.Errorf("error restoring tmux session: %w", err)
	}

	// Wait for the program to reach its ready state. Adapter-backed programs
	// (claude, opencode, codex) use the adapter's ReadyString / MaxWaitTime.
	// Legacy aider/gemini use a hardcoded fallback path.
	if adapter != nil || isAiderProgram(t.program) || isGeminiProgram(t.program) {
		t.reportProgress(4, "Waiting for program to start...")

		var searchString string
		var tapFunc func() error // nil means no key tap needed
		maxWaitTime := programReadyMaxWaitTime

		if adapter != nil {
			searchString = adapter.ReadyString()
			if adapterMax := adapter.MaxWaitTime(); adapterMax < maxWaitTime {
				maxWaitTime = adapterMax
			}
			if adapter.NeedsTrustTap() {
				tapFunc = t.TapEnter
			}
		} else {
			// aider / gemini: hardcoded startup banner + D-Enter tap.
			searchString = "Open documentation url for more info"
			tapFunc = t.TapDAndEnter
			maxWaitTime = 45 * time.Second
		}

		// Poll with exponential backoff until the ready string appears or we time out.
		startTime := time.Now()
		sleepDuration := programReadyPollInitialDelay
		sessionCheckInterval := programReadySessionCheckInterval
		lastSessionCheck := startTime

		for time.Since(startTime) < maxWaitTime {
			time.Sleep(sleepDuration)

			// Periodically verify the tmux session is still alive so we
			// don't spend 30s polling a dead session.
			if time.Since(lastSessionCheck) >= sessionCheckInterval {
				lastSessionCheck = time.Now()
				if !t.DoesSessionExist() {
					if cleanupErr := t.Close(); cleanupErr != nil {
						log.ErrorLog.Printf("cleanup after dead session %s: %v", t.sanitizedName, cleanupErr)
					}
					return t.startupExitError(workDir, logFile)
				}
			}

			content, err := t.CapturePaneContent()
			if err == nil {
				plain := ansiRe.ReplaceAllString(content, "")
				if searchString == "" {
					// No stable printable startup banner (e.g. codex): wait a short
					// grace period, verify the session is still alive, then continue.
					if time.Since(startTime) >= codexGracePeriod && t.DoesSessionExist() {
						break
					}
				} else if strings.Contains(plain, searchString) {
					if tapFunc != nil {
						if err := tapFunc(); err != nil {
							log.ErrorLog.Printf("could not tap enter on trust screen: %v", err)
						}
					}
					break
				} else if adapter != nil && adapter.DetectPrompt(plain) {
					break
				} else if isClaudeProgram(t.program) && claudeHasStarted(plain) {
					break
				}
			}

			// Exponential backoff with cap.
			sleepDuration = time.Duration(float64(sleepDuration) * 1.2)
			if sleepDuration > programReadyPollMaxDelay {
				sleepDuration = programReadyPollMaxDelay
			}
		}

		// After the session is ready, set the opencode session title (best-effort, non-blocking).
		// Only fires for opencode programs with a title and callback configured.
		if isOpenCodeProgram(t.program) && t.sessionTitle != "" && t.titleFunc != nil {
			go t.titleFunc(workDir, beforeStart, t.sessionTitle)
		}
	}
	return nil
}

func (t *TmuxSession) startupExitError(workDir, logFile string) error {
	if logFile == "" {
		logFile = filepath.Join(workDir, promptDir, "logs", t.sanitizedName+".log")
	}
	msg := fmt.Sprintf("session %s died during startup (program %q exited immediately; tmux target %s; log %s)",
		t.sanitizedName, programBase(t.program), t.sanitizedName, logFile)
	if tail := readStartupLogTail(logFile, 2048); tail != "" {
		msg += "\nlast log output:\n" + tail
	}
	return errors.New(msg)
}

func readStartupLogTail(path string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	if len(data) > maxBytes {
		data = data[len(data)-maxBytes:]
	}
	return strings.TrimSpace(string(data))
}

// Restore reattaches monitoring to an existing tmux session without spawning a
// background PTY client. It is monitor-only: ptmx and ptmxHandle remain nil.
// An interactive PTY is created only by Attach().
func (t *TmuxSession) Restore() error {
	t.monitor = NewStatusMonitor()
	// Idempotently hide the status bar — also covers sessions restored from crash
	// that were created before this option was set.
	statusCmd := exec.Command("tmux", "set-option", "-t", t.sanitizedName, "status", "off")
	if err := t.cmdExec.Run(statusCmd); err != nil {
		log.InfoLog.Printf("Warning: failed to hide status bar for restored session %s: %v", t.sanitizedName, err)
	}
	// Idempotently set escape-time to 0 for immediate ESC forwarding.
	escapeTimeCmd := exec.Command("tmux", "set-option", "-t", t.sanitizedName, "escape-time", "0")
	if err := t.cmdExec.Run(escapeTimeCmd); err != nil {
		log.InfoLog.Printf("Warning: failed to set escape-time for restored session %s: %v", t.sanitizedName, err)
	}
	// Idempotently disable mouse — covers sessions restored from crash.
	mouseCmd := exec.Command("tmux", "set-option", "-t", t.sanitizedName, "mouse", "off")
	if err := t.cmdExec.Run(mouseCmd); err != nil {
		log.InfoLog.Printf("Warning: failed to disable mouse for restored session %s: %v", t.sanitizedName, err)
	}
	return nil
}

// outerTmuxSession returns the name of the enclosing tmux session (the one
// running kasmos), or "" if we are not inside tmux.
func outerTmuxSession() string {
	if os.Getenv("TMUX") == "" {
		return ""
	}
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// outerMouseEnabled reports whether mouse is currently on in the given tmux session.
func outerMouseEnabled(session string) bool {
	out, err := exec.Command("tmux", "show-options", "-t", session, "mouse").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "on")
}

// closeActivePty closes the active PTY handle (set by Attach) and nils both
// ptmxHandle and ptmx. If only ptmx is set (e.g. by a legacy test helper),
// it falls back to closing the file directly. The label is included in any
// returned error for context.
func (t *TmuxSession) closeActivePty(label string) error {
	if t.ptmxHandle != nil {
		err := t.ptmxHandle.Close()
		t.ptmxHandle = nil
		t.ptmx = nil
		if err != nil {
			return fmt.Errorf("%s: close active PTY: %w", label, err)
		}
		return nil
	}
	if t.ptmx != nil {
		err := t.ptmx.Close()
		t.ptmx = nil
		if err != nil {
			return fmt.Errorf("%s: close active PTY: %w", label, err)
		}
	}
	return nil
}

// Close terminates the tmux session and cleans up resources.
func (t *TmuxSession) Close() error {
	var errs []error

	if err := t.closeActivePty("close"); err != nil {
		errs = append(errs, err)
	}

	existsCmd := exec.Command("tmux", "has-session", fmt.Sprintf("-t=%s", t.sanitizedName))
	if err := t.cmdExec.Run(existsCmd); err == nil {
		cmd := exec.Command("tmux", "kill-session", "-t", t.sanitizedName)
		if err := t.cmdExec.Run(cmd); err != nil && !isMissingTmuxSessionError(err) {
			errs = append(errs, fmt.Errorf("error killing tmux session: %w", err))
		}
	} else if !isMissingTmuxSessionError(err) {
		errs = append(errs, fmt.Errorf("error checking tmux session: %w", err))
	}

	if t.promptFile != "" {
		os.Remove(t.promptFile)
		t.promptFile = ""
	}

	return errors.Join(errs...)
}

func isMissingTmuxSessionError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

// DoesSessionExist returns true if the tmux session is currently running.
func (t *TmuxSession) DoesSessionExist() bool {
	// Using "-t name" does a prefix match, which is wrong. `-t=` does an exact match.
	existsCmd := exec.Command("tmux", "has-session", fmt.Sprintf("-t=%s", t.sanitizedName))
	return t.cmdExec.Run(existsCmd) == nil
}

// CleanupSessions kills all tmux sessions that start with the kas prefix.
// Also cleans up legacy "hivemind_" and "klique_" sessions from before the rename.
func CleanupSessions(cmdExec Executor) error {
	// First try to list sessions.
	cmd := exec.Command("tmux", "ls")
	output, err := cmdExec.Output(cmd)

	// If there's an error and it's because no server is running, that's fine.
	// Exit code 1 typically means no sessions exist.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil // No sessions to clean up.
		}
		return fmt.Errorf("failed to list tmux sessions: %v", err)
	}

	matches := cleanupSessionsRe.FindAllString(string(output), -1)
	for i, match := range matches {
		matches[i] = match[:strings.Index(match, ":")]
	}

	for _, match := range matches {
		log.InfoLog.Printf("cleaning up session: %s", match)
		if err := cmdExec.Run(exec.Command("tmux", "kill-session", "-t", match)); err != nil {
			return fmt.Errorf("failed to kill tmux session %s: %v", match, err)
		}
	}
	return nil
}

// OrphanSession represents a kas_ tmux session not tracked by any kasmos Instance.
type OrphanSession struct {
	Name     string    // raw tmux session name, e.g. "kas_auth-refactor-implement"
	Title    string    // human name with "kas_" prefix stripped
	Created  time.Time // session creation time
	Windows  int       // window count
	Attached bool      // whether another client is attached
	Width    int       // pane columns
	Height   int       // pane rows
}

// DiscoverOrphans lists kas_-prefixed tmux sessions that are NOT in knownNames.
// knownNames should contain the sanitized tmux names of all current Instances.
func DiscoverOrphans(cmdExec Executor, knownNames []string) ([]OrphanSession, error) {
	lsCmd := exec.Command("tmux", "ls", "-F",
		"#{session_name}|#{session_created}|#{session_windows}|#{session_attached}|#{window_width}|#{window_height}")
	output, err := cmdExec.Output(lsCmd)
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil, nil // no tmux server running or no sessions
		}
		return nil, fmt.Errorf("failed to list tmux sessions: %w", err)
	}

	known := make(map[string]bool, len(knownNames))
	for _, n := range knownNames {
		known[n] = true
	}

	var orphans []OrphanSession
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 6 {
			continue
		}
		name := parts[0]
		if !strings.HasPrefix(name, TmuxPrefix) {
			continue
		}
		if known[name] {
			continue
		}

		var created time.Time
		if epoch, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			created = time.Unix(epoch, 0)
		}
		windows, _ := strconv.Atoi(parts[2])
		attached := parts[3] != "0"
		width, _ := strconv.Atoi(parts[4])
		height, _ := strconv.Atoi(parts[5])

		title := strings.TrimPrefix(name, TmuxPrefix)
		orphans = append(orphans, OrphanSession{
			Name:     name,
			Title:    title,
			Created:  created,
			Windows:  windows,
			Attached: attached,
			Width:    width,
			Height:   height,
		})
	}
	return orphans, nil
}

// SessionInfo represents any kas_ tmux session (managed or orphaned).
type SessionInfo struct {
	Name     string    // raw tmux session name, e.g. "kas_auth-refactor-implement"
	Title    string    // human name with "kas_" prefix stripped
	Created  time.Time // session creation time
	Windows  int       // window count
	Attached bool      // whether another client is attached
	Width    int       // pane columns
	Height   int       // pane rows
	Managed  bool      // true if matched a known instance name
}

// DiscoverAll lists all kas_-prefixed tmux sessions, marking each as Managed
// if its name appears in knownNames. knownNames should contain the sanitized
// tmux names of all current Instances (e.g. from ToKasTmuxNamePublic).
func DiscoverAll(cmdExec Executor, knownNames []string) ([]SessionInfo, error) {
	lsCmd := exec.Command("tmux", "ls", "-F",
		"#{session_name}|#{session_created}|#{session_windows}|#{session_attached}|#{window_width}|#{window_height}")
	output, err := cmdExec.Output(lsCmd)
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list tmux sessions: %w", err)
	}

	known := make(map[string]bool, len(knownNames))
	for _, n := range knownNames {
		known[n] = true
	}

	var sessions []SessionInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 6 {
			continue
		}
		name := parts[0]
		if !strings.HasPrefix(name, TmuxPrefix) {
			continue
		}

		var created time.Time
		if epoch, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			created = time.Unix(epoch, 0)
		}
		windows, _ := strconv.Atoi(parts[2])
		attached := parts[3] != "0"
		width, _ := strconv.Atoi(parts[4])
		height, _ := strconv.Atoi(parts[5])

		sessions = append(sessions, SessionInfo{
			Name:     name,
			Title:    strings.TrimPrefix(name, TmuxPrefix),
			Created:  created,
			Windows:  windows,
			Attached: attached,
			Width:    width,
			Height:   height,
			Managed:  known[name],
		})
	}
	return sessions, nil
}

// HasAttachedClients returns true if the given tmux session currently has one
// or more attached clients. On any error (executor failure, tmux hiccup,
// non-zero exit) it returns true so that callers defer cleanup rather than
// risk terminating a session that may have active attached users.
func HasAttachedClients(cmdExec Executor, sessionName string) bool {
	dmCmd := exec.Command("tmux", "display-message", "-t", sessionName, "-p", "#{session_attached}")
	output, err := cmdExec.Output(dmCmd)
	if err != nil {
		// Treat probe failures as "attached" to preserve the grace-period
		// safety guarantee — it is safer to defer cleanup than to kill a
		// session whose client state cannot be determined.
		return true
	}
	return parseClientCount(strings.TrimSpace(string(output))) > 0
}

// parseClientCount parses the integer client count from tmux display-message
// output. Returns 0 for any parse error or negative value.
func parseClientCount(s string) int {
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// CountKasSessions returns the number of kas_-prefixed tmux sessions.
// Returns 0 if no tmux server is running or on any error.
func CountKasSessions(cmdExec Executor) int {
	lsCmd := exec.Command("tmux", "ls")
	output, err := cmdExec.Output(lsCmd)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.HasPrefix(line, TmuxPrefix) {
			count++
		}
	}
	return count
}
