package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/linearreceipt"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/clickup"
	"github.com/kastheco/kasmos/session/git"
	"github.com/spf13/cobra"
)

func normalizeTaskFilename(filename string) string {
	return strings.TrimSuffix(strings.TrimSpace(filename), ".md")
}

func resolveExistingTaskFilename(ps *taskstate.TaskState, filename string) string {
	raw := strings.TrimSpace(filename)
	if raw == "" {
		return ""
	}
	if _, ok := ps.Entry(raw); ok {
		return raw
	}
	trimmed := normalizeTaskFilename(raw)
	if _, ok := ps.Entry(trimmed); ok {
		return trimmed
	}
	// Preserve the original user input when no stored entry matches so callers
	// can report the exact filename the user asked for.
	return raw
}

// executeTaskRegister registers a plan file into the task store. The filePath
// is resolved relative to the caller's working directory.
func executeTaskRegister(project, filePath, branch, topic, description string, store taskstore.Store) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("task file not found: %s", filePath)
	}
	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return err
	}
	planFile := normalizeTaskFilename(filepath.Base(filePath))
	if description == "" {
		description = planFile
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "# ") {
				description = strings.TrimPrefix(line, "# ")
				break
			}
		}
	}
	if branch == "" {
		branch = "plan/" + planFile
	}
	info, _ := os.Stat(filePath)
	createdAt := info.ModTime()
	return ps.CreateWithContent(planFile, description, branch, topic, createdAt, string(data))
}

// executeTaskList returns a formatted string listing all plans, optionally
// filtered by status. Exported for testing without cobra plumbing.
// When statusFilter is empty, cancelled tasks are hidden from the output.
func executeTaskList(project, statusFilter string, store taskstore.Store) string {
	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return formatTaskList(ps.List(), statusFilter)
}

func filteredTaskList(entries []taskstate.TaskInfo, statusFilter string) []taskstate.TaskInfo {
	result := make([]taskstate.TaskInfo, 0, len(entries))
	for _, info := range entries {
		if statusFilter != "" && string(info.Status) != statusFilter {
			continue
		}
		if statusFilter == "" && string(info.Status) == string(taskstore.StatusCancelled) {
			continue
		}
		result = append(result, info)
	}
	return result
}

func taskListIncludesLinear(entries []taskstate.TaskInfo) bool {
	for _, info := range entries {
		if taskListLinearDisplayID(info) != "" {
			return true
		}
	}
	return false
}

func taskListLinearDisplayID(info taskstate.TaskInfo) string {
	if info.LinearIdentifier != "" {
		return info.LinearIdentifier
	}
	return info.LinearIssueID
}

func formatTaskListRow(info taskstate.TaskInfo, includeLinear bool) string {
	if includeLinear {
		line := fmt.Sprintf("%-14s %-50s %-40s %s", info.Status, info.Filename, info.Branch, taskListLinearDisplayID(info))
		return strings.TrimRight(line, " ")
	}
	line := fmt.Sprintf("%-14s %-50s %s", info.Status, info.Filename, info.Branch)
	return strings.TrimRight(line, " ")
}

func formatTaskList(entries []taskstate.TaskInfo, statusFilter string) string {
	entries = filteredTaskList(entries, statusFilter)
	includeLinear := taskListIncludesLinear(entries)

	var sb strings.Builder
	for _, info := range entries {
		sb.WriteString(formatTaskListRow(info, includeLinear) + "\n")
	}
	return sb.String()
}

func linearLinkDisplayID(link taskstore.LinearLink) string {
	if link.LinearIdentifier != "" {
		return link.LinearIdentifier
	}
	return link.LinearIssueID
}

// executeTaskListWithStore returns a formatted string listing all plans from a
// remote store backend. storeURL is the base URL of the task store server
// (e.g. "http://athena:7433") and project is the project name to query.
// When statusFilter is empty, cancelled tasks are hidden from the output.
func executeTaskListWithStore(storeURL, project, statusFilter string) string {
	store := taskstore.NewHTTPStore(storeURL, project)
	ps, err := taskstate.Load(store, project, "")
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return formatTaskList(ps.List(), statusFilter)
}

// executeTaskSetStatus force-overrides a plan's status, bypassing the FSM.
// Requires force=true to prevent accidental misuse.
func executeTaskSetStatus(project, planFile, status string, force bool, store taskstore.Store) error {
	if !force {
		return fmt.Errorf("--force required to override task status (this bypasses the FSM)")
	}
	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return err
	}
	planFile = resolveExistingTaskFilename(ps, planFile)
	resolvedStatus, state, err := taskstate.ResolveManualOverride(status)
	if err != nil {
		return err
	}
	return ps.ForceSetLifecycle(planFile, resolvedStatus, state)
}

// executeTaskTransition applies a named FSM event to a plan and returns the new
// status. It intentionally omits gateway emission for callers that only need
// the in-process FSM primitive. CLI callers use
// executeTaskTransitionWithGateway so daemon side effects are not skipped.
func executeTaskTransition(project, planFile, event string, store taskstore.Store, hooks ...*taskfsm.HookRegistry) (string, error) {
	return executeTaskTransitionWithGateway(project, planFile, event, store, nil, hooks...)
}

// executeTaskTransitionWithGateway applies a transition and emits the matching
// pre-applied gateway signal when the event drives daemon side effects.
func executeTaskTransitionWithGateway(project, planFile, event string, store taskstore.Store, gateway taskstore.SignalGateway, hooks ...*taskfsm.HookRegistry) (string, error) {
	fsmEvent, ok := taskfsm.EventByName(event)
	if !ok {
		return "", fmt.Errorf("unknown event %q; valid events: %s", event, strings.Join(taskfsm.EventNames(), ", "))
	}
	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return "", err
	}
	planFile = resolveExistingTaskFilename(ps, planFile)
	var hookRegistry *taskfsm.HookRegistry
	if len(hooks) > 0 {
		hookRegistry = hooks[0]
	}
	fsm := newFSMByProjectWithHooks(project, store, hookRegistry)
	if err := fsm.Transition(planFile, fsmEvent); err != nil {
		return "", err
	}
	if hookRegistry != nil {
		hookRegistry.Wait()
	}
	if gateway != nil {
		if signalType, mapErr := taskfsm.GatewaySignalTypeForEvent(fsmEvent); mapErr == nil {
			if emitErr := taskfsm.EmitGatewaySignal(gateway, project, signalType, planFile, taskfsm.PreAppliedGatewayPayload); emitErr != nil {
				return "", fmt.Errorf("transition applied but emit %s signal: %w", signalType, emitErr)
			}
		}
	}
	ps, err = loadTaskStateByProject(project, store)
	if err != nil {
		return "", err
	}
	entry, _ := ps.Entry(planFile)
	return string(entry.Status), nil
}

type taskRecoverAction struct {
	name       string
	signalType string
}

func canonicalTaskRecoverAction(raw string) (taskRecoverAction, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "_", "-")
	if normalized == "" {
		return taskRecoverAction{}, fmt.Errorf("recovery action is required; valid actions: planner-finished, architect-finished, implement-finished, review-approved, review-changes, verify-approved, verify-failed, advance-review-cycle, advance-wave, retry-wave")
	}

	signalAction := func(name, signalType string) (taskRecoverAction, error) {
		canonical, err := taskfsm.CanonicalGatewaySignalType(signalType)
		if err != nil {
			return taskRecoverAction{}, err
		}
		return taskRecoverAction{name: name, signalType: canonical}, nil
	}

	switch normalized {
	case "planner-finished":
		return signalAction("planner-finished", normalized)
	case "architect-finished", "elaborator-finished":
		return signalAction("architect-finished", normalized)
	case "implement-finished":
		return signalAction("implement-finished", normalized)
	case "review-approved":
		return signalAction("review-approved", normalized)
	case "review-changes", "review-changes-requested":
		return signalAction("review-changes", normalized)
	case "verify-approved":
		return signalAction("verify-approved", "verify_approved")
	case "verify-failed":
		return signalAction("verify-failed", "verify_failed")
	// Deprecated aliases: readiness-approved → verify-approved, readiness-changes → verify-failed.
	case "readiness-approved":
		return signalAction("readiness-approved", "readiness_approved")
	case "readiness-changes", "readiness-changes-requested":
		return signalAction("readiness-changes", "readiness_changes")
	case "advance-review-cycle":
		return taskRecoverAction{name: "advance-review-cycle"}, nil
	case "advance-wave":
		return signalAction("advance-wave", normalized)
	case "retry-wave":
		return signalAction("retry-wave", normalized)
	default:
		return taskRecoverAction{}, fmt.Errorf("unknown recovery action %q; valid actions: planner-finished, architect-finished, implement-finished, review-approved, review-changes, verify-approved, verify-failed, advance-review-cycle, advance-wave, retry-wave", raw)
	}
}

func taskRecoverRequiresSignalGateway(action taskRecoverAction) bool {
	return action.signalType != ""
}

func executeTaskRecover(project, planFile, action, feedback string, store taskstore.Store, gateway taskstore.SignalGateway) error {
	recoverAction, err := canonicalTaskRecoverAction(action)
	if err != nil {
		return err
	}

	closeStore := false
	if store == nil {
		store, err = taskstore.OpenAuthoritativeStore(project)
		if err != nil {
			return err
		}
		closeStore = true
	}
	if closeStore {
		defer store.Close()
	}

	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return err
	}
	planFile = resolveExistingTaskFilename(ps, planFile)
	if _, ok := ps.Entry(planFile); !ok {
		return fmt.Errorf("task not found: %s", planFile)
	}

	if recoverAction.name == "advance-review-cycle" {
		if trimmed := strings.TrimSpace(feedback); trimmed != "" {
			if err := ps.SetLatestReviewFeedback(planFile, trimmed); err != nil {
				return fmt.Errorf("persist latest review feedback for %s: %w", planFile, err)
			}
		}
		if err := ps.IncrementReviewCycle(planFile); err != nil {
			return fmt.Errorf("increment review cycle for %s: %w", planFile, err)
		}
		return nil
	}

	closeGateway := false
	if gateway == nil && taskRecoverRequiresSignalGateway(recoverAction) {
		gateway, err = taskstore.OpenAuthoritativeSignalGateway(project)
		if err != nil {
			return err
		}
		closeGateway = true
	}
	if closeGateway {
		defer gateway.Close() //nolint:errcheck
	}

	if gateway == nil {
		return fmt.Errorf("signal gateway unavailable for recovery action %q", recoverAction.name)
	}
	if err := executeSignalEmit(gateway, project, recoverAction.signalType, planFile, feedback); err != nil {
		return fmt.Errorf("queue recovery action %q for %s: %w", recoverAction.name, planFile, err)
	}
	return nil
}

// executeTaskImplement transitions a plan into implementing state and writes
// a wave signal file so the TUI metadata tick can pick it up.
func executeTaskImplement(repoRoot, project, planFile string, wave int, store taskstore.Store) error {
	if wave < 1 {
		return fmt.Errorf("wave number must be >= 1, got %d", wave)
	}
	fsm := newFSMByProject(project, store)
	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return err
	}
	planFile = resolveExistingTaskFilename(ps, planFile)
	entry, ok := ps.Entry(planFile)
	if !ok {
		return fmt.Errorf("task not found: %s", planFile)
	}
	if taskstate.IsDraftReady(entry) {
		return fmt.Errorf("task is ready but not yet planned: %s", planFile)
	}
	current := taskfsm.Status(entry.Status)
	// If still in planning, finish that phase first (→ ready).
	if current == taskfsm.StatusPlanning {
		if err := fsm.Transition(planFile, taskfsm.PlannerFinished); err != nil {
			return err
		}
		current = taskfsm.StatusReady
	}
	// Advance to implementing unless already there.
	if current != taskfsm.StatusImplementing {
		if err := fsm.Transition(planFile, taskfsm.ImplementStart); err != nil {
			return err
		}
	}

	// Write the wave signal file consumed by the TUI metadata tick.
	signalsDir := filepath.Join(repoRoot, ".kasmos", "signals")
	if err := os.MkdirAll(signalsDir, 0o755); err != nil {
		return err
	}
	signalName := fmt.Sprintf("implement-wave-%d-%s", wave, planFile)
	return os.WriteFile(filepath.Join(signalsDir, signalName), nil, 0o644)
}

// executeTaskShow retrieves plan content from the task store and returns it
// as raw markdown. Returns an error if the plan doesn't exist or has no content.
func executeTaskShow(project, planFile string, store taskstore.Store) (string, error) {
	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return "", err
	}
	planFile = resolveExistingTaskFilename(ps, planFile)
	if _, ok := ps.Entry(planFile); !ok {
		return "", fmt.Errorf("task not found: %s", planFile)
	}
	content, err := ps.GetContent(planFile)
	if err != nil {
		return "", fmt.Errorf("get content for %s: %w", planFile, err)
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("no content stored for %s", planFile)
	}
	return content, nil
}

var errRefusingDeleteWithoutYes = errors.New("refusing to delete without --yes")

func promptForDelete(r io.Reader, out io.Writer, filename, status string) (bool, error) {
	scanner := bufio.NewScanner(r)
	fmt.Fprintf(out, "delete %s (%s)? [y/N]: ", filename, status)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("read confirmation: %w", err)
		}
		return false, nil
	}
	return strings.TrimSpace(strings.ToLower(scanner.Text())) == "y", nil
}

func executeTaskUpdateContent(project, filename string, reader io.Reader, store taskstore.Store) error {
	contentBytes, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read content: %w", err)
	}
	content := string(contentBytes)
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("no content provided; pipe plan content via stdin: cat plan.md | kas task update-content <plan>")
	}

	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return err
	}
	filename = resolveExistingTaskFilename(ps, filename)
	if err := ps.IngestContent(filename, content); err != nil {
		return fmt.Errorf("update content for %s: %w", filename, err)
	}
	return nil
}

func validateUpdateContentStdin(stdin *os.File) error {
	info, err := stdin.Stat()
	if err != nil {
		return fmt.Errorf("stat stdin: %w", err)
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return fmt.Errorf("stdin is a tty; pipe plan content via stdin: cat plan.md | kas task update-content <plan>")
	}
	return nil
}

func openUpdateContentReader(stdin *os.File, filePath string) (io.ReadCloser, error) {
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("open --file %q: %w", filePath, err)
		}
		return f, nil
	}
	if err := validateUpdateContentStdin(stdin); err != nil {
		return nil, err
	}
	return stdin, nil
}

// executeTaskLinkClickUp iterates all plans in the given project, reads their
// content, parses the ClickUp task ID from the "**Source:** ClickUp <ID>" line,
// and stores it in the clickup_task_id field for any plan that has an ID in its
// content but not yet in the store. Returns the count of plans updated.
func executeTaskLinkClickUp(project string, store taskstore.Store) (int, error) {
	plans, err := store.List(project)
	if err != nil {
		return 0, fmt.Errorf("list tasks: %w", err)
	}

	updated := 0
	for _, plan := range plans {
		// Skip plans that already have a ClickUp task ID.
		if plan.ClickUpTaskID != "" {
			continue
		}

		content, err := store.GetContent(project, plan.Filename)
		if err != nil {
			// Non-fatal: skip plans whose content can't be read.
			continue
		}

		taskID := clickup.ParseClickUpTaskID(content)
		if taskID == "" {
			continue
		}

		if err := store.SetClickUpTaskID(project, plan.Filename, taskID); err != nil {
			return updated, fmt.Errorf("set clickup task id for %s: %w", plan.Filename, err)
		}
		updated++
	}

	return updated, nil
}

// resolveTaskEntry loads task state for the given project, looks up the entry
// by filename, and backfills the branch name if it is empty (using
// git.TaskBranchFromFile). Returns an error if the entry is not found.
func resolveTaskEntry(project, filename string, store taskstore.Store) (taskstate.TaskEntry, error) {
	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return taskstate.TaskEntry{}, err
	}
	filename = resolveExistingTaskFilename(ps, filename)
	entry, ok := ps.Entry(filename)
	if !ok {
		return taskstate.TaskEntry{}, fmt.Errorf("task not found: %s", filename)
	}
	if entry.Branch == "" {
		entry.Branch = git.TaskBranchFromFile(filename)
	}
	return entry, nil
}

// executeTaskCreate creates a new task entry in the store. name is the plan slug.
// branch defaults to "plan/<name>" when empty. If content is non-empty, it is
// stored alongside the metadata.
func executeTaskCreate(project, name, description, branch, topic, content string, store taskstore.Store) error {
	filename := normalizeTaskFilename(name)
	if branch == "" {
		branch = "plan/" + filename
	}
	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return err
	}
	createdAt := time.Now()
	if content != "" {
		return ps.CreateWithContent(filename, description, branch, topic, createdAt, content)
	}
	return ps.Create(filename, description, branch, topic, createdAt)
}

// executeTaskStart transitions a plan to implementing status and sets up the
// git branch + worktree. It walks planning → ready → implementing via the FSM
// if the plan is currently in planning state. Returns the worktree path.
func executeTaskStart(repoRoot, project, planFile string, store taskstore.Store) (string, error) {
	fsm := newFSMByProject(project, store)

	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return "", err
	}
	planFile = resolveExistingTaskFilename(ps, planFile)
	entry, ok := ps.Entry(planFile)
	if !ok {
		return "", fmt.Errorf("task not found: %s", planFile)
	}
	if taskstate.IsDraftReady(entry) {
		return "", fmt.Errorf("task is ready but not yet planned: %s", planFile)
	}

	current := taskfsm.Status(entry.Status)
	// Walk planning → ready first if needed.
	if current == taskfsm.StatusPlanning {
		if err := fsm.Transition(planFile, taskfsm.PlannerFinished); err != nil {
			return "", err
		}
		current = taskfsm.StatusReady
	}
	// Advance to implementing.
	if current != taskfsm.StatusImplementing {
		if err := fsm.Transition(planFile, taskfsm.ImplementStart); err != nil {
			return "", err
		}
	}

	// Resolve branch — backfill if not set.
	branch := entry.Branch
	if branch == "" {
		branch = git.TaskBranchFromFile(planFile)
	}

	// Set up the git branch and worktree.
	if err := git.EnsureTaskBranch(repoRoot, branch); err != nil {
		return "", fmt.Errorf("ensure task branch: %w", err)
	}
	wt := git.NewSharedTaskWorktree(repoRoot, branch)
	if err := wt.Setup(); err != nil {
		return "", fmt.Errorf("setup worktree: %w", err)
	}
	return wt.GetWorktreePath(), nil
}

// executeTaskPush resolves the task entry and its branch, constructs a
// GitWorktree from stored state, commits any dirty changes, and pushes to
// origin. The commit message defaults to "update from kas" when empty.
func executeTaskPush(repoRoot, project, planFile, message string, store taskstore.Store) error {
	if message == "" {
		message = "update from kas"
	}
	entry, err := resolveTaskEntry(project, planFile, store)
	if err != nil {
		return err
	}
	branch := entry.Branch
	worktreePath := git.TaskWorktreePath(repoRoot, branch)
	wt := git.NewGitWorktreeFromStorage(repoRoot, worktreePath, "push", branch, "")
	return wt.PushChanges(message, false)
}

// executeTaskMerge merges the plan branch into the current branch (typically
// main), then walks the FSM to done. If the task is not yet in reviewing state,
// it transitions through ImplementFinished first. Returns an error if the git
// merge fails.
func executeTaskMerge(repoRoot, project, planFile string, store taskstore.Store) error {
	entry, err := resolveTaskEntry(project, planFile, store)
	if err != nil {
		return err
	}
	branch := entry.Branch

	// Validate repoRoot before attempting git operations.
	if _, serr := os.Stat(repoRoot); serr != nil {
		return fmt.Errorf("invalid repo root %q: %w", repoRoot, serr)
	}

	// Git merge first — only advance the FSM on success.
	if err := git.MergeTaskBranch(repoRoot, branch); err != nil {
		return err
	}

	// Walk FSM to done.
	fsm := newFSMByProject(project, store)
	current := taskfsm.Status(entry.Status)
	switch current {
	case taskfsm.StatusDone:
		// Already done — no FSM transitions needed after the git merge.
		return nil
	case taskfsm.StatusVerifying:
		return fsm.Transition(planFile, taskfsm.VerifyApproved)
	case taskfsm.StatusReviewing:
		if err := fsm.Transition(planFile, taskfsm.ReviewApproved); err != nil {
			return err
		}
		return fsm.Transition(planFile, taskfsm.VerifyApproved)
	default:
		// implementing or earlier: walk to reviewing first.
		if current == taskfsm.StatusImplementing {
			if err := fsm.Transition(planFile, taskfsm.ImplementFinished); err != nil {
				return err
			}
		} else {
			// Force to reviewing so ReviewApproved can proceed.
			ps, lerr := loadTaskStateByProject(project, store)
			if lerr != nil {
				return lerr
			}
			if ferr := ps.ForceSetStatus(planFile, taskstate.StatusReviewing); ferr != nil {
				return ferr
			}
		}
		if err := fsm.Transition(planFile, taskfsm.ReviewApproved); err != nil {
			return err
		}
		return fsm.Transition(planFile, taskfsm.VerifyApproved)
	}
}

// executeTaskStartOver removes the plan worktree, deletes and recreates the
// branch from HEAD (via git.ResetTaskBranch), then transitions the FSM to
// planning. Uses StartOver FSM event for states where it is valid; falls back
// to ForceSetStatus for all other states.
func executeTaskStartOver(repoRoot, project, planFile string, store taskstore.Store) error {
	entry, err := resolveTaskEntry(project, planFile, store)
	if err != nil {
		return err
	}
	branch := entry.Branch

	// Validate repoRoot before attempting git operations.
	if _, serr := os.Stat(repoRoot); serr != nil {
		return fmt.Errorf("invalid repo root %q: %w", repoRoot, serr)
	}

	// Git reset first — only touch FSM on success.
	if err := git.ResetTaskBranch(repoRoot, branch); err != nil {
		return err
	}

	// Try the FSM StartOver event; fall back to ForceSetStatus if not valid
	// from the current state (StartOver is only defined from done in the FSM).
	fsm := newFSMByProject(project, store)
	if ferr := fsm.Transition(planFile, taskfsm.StartOver); ferr != nil {
		ps, lerr := loadTaskStateByProject(project, store)
		if lerr != nil {
			return lerr
		}
		return ps.ForceSetStatus(planFile, taskstate.StatusPlanning)
	}
	return nil
}

// executeTaskPR resolves the task entry, derives the PR title from the task
// description when title is empty, generates a PR body from the git log, and
// creates (or reopens) the PR via the GitHub CLI. The PR URL is printed to
// stdout by the gh CLI; the returned string is currently always empty.
func executeTaskPR(repoRoot, project, planFile, title string, store taskstore.Store) (string, error) {
	if store == nil {
		var err error
		store, err = taskstore.OpenAuthoritativeStore(project)
		if err != nil {
			return "", err
		}
	}

	ps, err := loadTaskStateByProject(project, store)
	if err != nil {
		return "", err
	}
	planFile = resolveExistingTaskFilename(ps, planFile)

	entry, err := store.Get(project, planFile)
	if err != nil {
		return "", fmt.Errorf("task not found: %s (%w)", planFile, err)
	}
	branch := entry.Branch
	if branch == "" {
		branch = git.TaskBranchFromFile(planFile)
	}
	defaultTitle := git.BuildPRTitle(entry.Description, strings.TrimSuffix(planFile, ".md"))
	if title == "" {
		title = defaultTitle
	}
	worktreePath := git.TaskWorktreePath(repoRoot, branch)
	wt := git.NewGitWorktreeFromStorage(repoRoot, worktreePath, "pr", branch, "")

	gitChanges := ""
	gitCommits := ""
	gitStats := ""
	// If the branch cannot be resolved against main, keep PR metadata text-only.
	if baseOut, err := exec.Command("git", "-C", repoRoot, "merge-base", "HEAD", branch).CombinedOutput(); err == nil {
		base := strings.TrimSpace(string(baseOut))
		if base != "" {
			if files, err := exec.Command("git", "-C", worktreePath, "diff", "--name-only", base).CombinedOutput(); err == nil {
				gitChanges = strings.TrimSpace(string(files))
			}
			if commits, err := exec.Command("git", "-C", worktreePath, "log", "--oneline", base+"..HEAD").CombinedOutput(); err == nil {
				gitCommits = strings.TrimSpace(string(commits))
			}
			if stats, err := exec.Command("git", "-C", worktreePath, "diff", "--stat", base).CombinedOutput(); err == nil {
				gitStats = strings.TrimSpace(string(stats))
			}
		}
	}

	if content, err := store.GetContent(project, planFile); err == nil && content != "" {
		entry.Content = content
	}
	subtasks, _ := store.GetSubtasks(project, planFile)
	body := git.BuildPRBody(buildCLIPRMetadata(entry, subtasks, gitChanges, gitCommits, gitStats))
	if err := wt.CreatePR(title, body, "update from kas"); err != nil {
		return "", err
	}
	return "", nil
}

func buildCLIPRMetadata(
	entry taskstore.TaskEntry,
	subtasks []taskstore.SubtaskEntry,
	gitChanges, gitCommits, gitStats string,
) git.PRMetadata {
	meta := git.PRMetadata{
		Description: strings.TrimSpace(entry.Description),
		Goal:        strings.TrimSpace(entry.Goal),
		GitChanges:  strings.TrimSpace(gitChanges),
		GitCommits:  strings.TrimSpace(gitCommits),
		GitStats:    strings.TrimSpace(gitStats),
	}

	if entry.Content != "" {
		if parsed, err := taskparser.Parse(entry.Content); err == nil {
			if meta.Goal == "" && strings.TrimSpace(parsed.Goal) != "" {
				meta.Goal = strings.TrimSpace(parsed.Goal)
			}
			meta.Architecture = strings.TrimSpace(parsed.Architecture)
			meta.TechStack = strings.TrimSpace(parsed.TechStack)
		}
	}

	for _, subtask := range subtasks {
		meta.Subtasks = append(meta.Subtasks, git.PRSubtask{
			Number: subtask.TaskNumber,
			Title:  strings.TrimSpace(subtask.Title),
			Status: string(subtask.Status),
		})
	}

	return meta
}

// NewTaskCmd builds the `kq plan` cobra command tree.
func NewTaskCmd() *cobra.Command {
	planCmd := &cobra.Command{
		Use:     "task",
		Aliases: []string{"t"},
		Short:   "manage task lifecycle (list, recover, set-status, transition, implement)",
	}

	withAuthoritativeStore := func(project string, fn func(taskstore.Store) error) error {
		store, err := taskstore.OpenAuthoritativeStore(project)
		if err != nil {
			return err
		}
		defer store.Close()
		return fn(store)
	}

	var statusFilter string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "list all tasks with status",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				fmt.Print(executeTaskList(project, statusFilter, store))
				return nil
			})
		},
	}
	listCmd.Flags().StringVar(&statusFilter, "status", "", "filter by status (ready, planning, implementing, reviewing, verifying, done, cancelled)")
	planCmd.AddCommand(listCmd)

	var branchFlag, topicFlag, descriptionFlag string
	registerCmd := &cobra.Command{
		Use:   "register <plan-file>",
		Short: "register an untracked task file (sets status to ready)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				if err := executeTaskRegister(project, args[0], branchFlag, topicFlag, descriptionFlag, store); err != nil {
					return err
				}
				fmt.Printf("registered: %s → ready\n", filepath.Base(args[0]))
				return nil
			})
		},
	}
	registerCmd.Flags().StringVar(&branchFlag, "branch", "", "override branch name (default: plan/<slug>)")
	registerCmd.Flags().StringVar(&topicFlag, "topic", "", "assign plan to a topic group (auto-creates topic if needed)")
	registerCmd.Flags().StringVar(&descriptionFlag, "description", "", "override description (default: extracted from first # heading)")
	planCmd.AddCommand(registerCmd)

	var forceFlag bool
	setStatusCmd := &cobra.Command{
		Use:   "set-status <plan-file> <status>",
		Short: "force-override a task's status (bypasses FSM)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				if err := executeTaskSetStatus(project, args[0], args[1], forceFlag, store); err != nil {
					return err
				}
				fmt.Printf("%s → %s\n", args[0], args[1])
				return nil
			})
		},
	}
	setStatusCmd.Flags().BoolVar(&forceFlag, "force", false, "confirm intent to bypass FSM transition rules")
	planCmd.AddCommand(setStatusCmd)

	transitionCmd := &cobra.Command{
		Use:   "transition <plan-file> <event>",
		Short: "apply an FSM event to a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				var gateway taskstore.SignalGateway
				if fsmEvent, ok := taskfsm.EventByName(args[1]); ok {
					if _, mapErr := taskfsm.GatewaySignalTypeForEvent(fsmEvent); mapErr == nil {
						gateway, err = taskstore.OpenAuthoritativeSignalGateway(project)
						if err != nil {
							return fmt.Errorf("open authoritative signal gateway: %w", err)
						}
						defer gateway.Close()
					}
				}
				hooks := buildTaskTransitionHooks(project, store)
				newStatus, err := executeTaskTransitionWithGateway(project, args[0], args[1], store, gateway, hooks)
				if err != nil {
					return err
				}
				fmt.Printf("%s → %s\n", args[0], newStatus)
				return nil
			})
		},
	}
	planCmd.AddCommand(transitionCmd)

	var (
		recoverActionName string
		recoverFeedback   string
	)
	recoverCmd := &cobra.Command{
		Use:   "recover <plan-file>",
		Short: "queue or apply a lifecycle recovery action for a task",
		Long: `recover exposes the same manual lifecycle recovery surface used by the tui.

Supported actions:
  - planner-finished
  - architect-finished (queued via the retained elaborator_finished signal contract)
  - implement-finished
  - review-approved
  - review-changes
  - verify-approved    (approve verification and mark done)
  - verify-failed      (send back to implementation; use --feedback to attach notes)
  - advance-review-cycle

Deprecated aliases: readiness-approved (→ verify-approved), readiness-changes (→ verify-failed)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			recoverAction, err := canonicalTaskRecoverAction(recoverActionName)
			if err != nil {
				return err
			}
			if err := executeTaskRecover(project, args[0], recoverActionName, recoverFeedback, nil, nil); err != nil {
				return err
			}
			if recoverAction.name == "advance-review-cycle" {
				fmt.Printf("recovery applied: action=%s plan=%s\n", recoverAction.name, args[0])
				return nil
			}
			fmt.Printf("recovery queued: action=%s signal=%s plan=%s\n", recoverAction.name, recoverAction.signalType, args[0])
			return nil
		},
	}
	recoverCmd.Flags().StringVar(&recoverActionName, "action", "", "recovery action (planner-finished, architect-finished, implement-finished, review-approved, review-changes, advance-review-cycle, advance-wave, retry-wave)")
	recoverCmd.Flags().StringVar(&recoverFeedback, "feedback", "", "optional reviewer feedback to persist or attach to the queued recovery signal")
	_ = recoverCmd.MarkFlagRequired("action")
	planCmd.AddCommand(recoverCmd)

	var waveNum int
	implementCmd := &cobra.Command{
		Use:   "implement <plan-file>",
		Short: "trigger implementation of a specific wave",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				if err := executeTaskImplement(repoRoot, project, args[0], waveNum, store); err != nil {
					return err
				}
				fmt.Printf("implementation triggered: %s wave %d\n", args[0], waveNum)
				return nil
			})
		},
	}
	implementCmd.Flags().IntVar(&waveNum, "wave", 1, "wave number to trigger (default: 1)")
	planCmd.AddCommand(implementCmd)

	var showProject string
	showCmd := &cobra.Command{
		Use:   "show <plan-file>",
		Short: "print plan content from the task store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, repoProject, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			project := repoProject
			if showProject != "" {
				project = showProject
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				content, err := executeTaskShow(project, args[0], store)
				if err != nil {
					return err
				}
				fmt.Print(content)
				return nil
			})
		},
	}
	showCmd.Flags().StringVar(&showProject, "project", "", "project name (default: derived from current directory)")
	planCmd.AddCommand(showCmd)

	var deleteYes bool
	deleteCmd := &cobra.Command{
		Use:   "delete <plan-file>",
		Short: "permanently delete a task from the store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				ps, err := loadTaskStateByProject(project, store)
				if err != nil {
					return err
				}
				resolvedFilename := resolveExistingTaskFilename(ps, args[0])
				entry, ok := ps.Entry(resolvedFilename)
				if !ok {
					return fmt.Errorf("task not found: %s", args[0])
				}
				if !deleteYes {
					stdin := cmd.InOrStdin()
					if !stdinIsTerminal(stdin) {
						return errRefusingDeleteWithoutYes
					}
					ok, err := promptForDelete(stdin, cmd.OutOrStdout(), resolvedFilename, string(entry.Status))
					if err != nil {
						return err
					}
					if !ok {
						fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
						return nil
					}
				}
				if err := store.Delete(project, resolvedFilename); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", resolvedFilename)
				return nil
			})
		},
	}
	deleteCmd.Flags().BoolVar(&deleteYes, "yes", false, "confirm permanent deletion")
	planCmd.AddCommand(deleteCmd)

	var updateContentFile string
	updateContentCmd := &cobra.Command{
		Use:   "update-content <plan-file>",
		Short: "replace plan content in the task store (reads from stdin or --file)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				filename := args[0]
				trimmedFilename := strings.TrimSuffix(filename, ".md")
				reader, err := openUpdateContentReader(os.Stdin, updateContentFile)
				if err != nil {
					return err
				}
				if reader != os.Stdin {
					defer reader.Close()
				}
				if err := executeTaskUpdateContent(project, filename, reader, store); err != nil {
					return err
				}
				fmt.Printf("updated content for %s\n", trimmedFilename)
				return nil
			})
		},
	}
	updateContentCmd.Flags().StringVar(&updateContentFile, "file", "", "read updated plan content from a file instead of stdin")
	planCmd.AddCommand(updateContentCmd)

	var (
		createDescription string
		createBranch      string
		createTopic       string
		createContent     string
	)
	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "create a new task entry in the store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			name := args[0]
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				if err := executeTaskCreate(project, name, createDescription, createBranch, createTopic, createContent, store); err != nil {
					return err
				}
				fmt.Printf("created: %s → ready\n", name)
				return nil
			})
		},
	}
	createCmd.Flags().StringVar(&createDescription, "description", "", "task description")
	createCmd.Flags().StringVar(&createBranch, "branch", "", "git branch name (default: plan/<name>)")
	createCmd.Flags().StringVar(&createTopic, "topic", "", "topic group")
	createCmd.Flags().StringVar(&createContent, "content", "", "initial plan content (markdown)")
	planCmd.AddCommand(createCmd)

	startCmd := &cobra.Command{
		Use:   "start <plan-file>",
		Short: "transition a task to implementing and set up the git worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				worktreePath, err := executeTaskStart(repoRoot, project, args[0], store)
				if err != nil {
					return err
				}
				fmt.Printf("started: %s → implementing\nworktree: %s\n", args[0], worktreePath)
				return nil
			})
		},
	}
	planCmd.AddCommand(startCmd)

	var pushMessage string
	pushCmd := &cobra.Command{
		Use:   "push <plan-file>",
		Short: "commit dirty changes and push the task branch to origin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				if err := executeTaskPush(repoRoot, project, args[0], pushMessage, store); err != nil {
					return err
				}
				fmt.Printf("pushed: %s\n", args[0])
				return nil
			})
		},
	}
	pushCmd.Flags().StringVar(&pushMessage, "message", "update from kas", "commit message for dirty changes")
	planCmd.AddCommand(pushCmd)

	var prTitle string
	prCmd := &cobra.Command{
		Use:   "pr <plan-file>",
		Short: "push and open a pull request for the task branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				url, err := executeTaskPR(repoRoot, project, args[0], prTitle, store)
				if err != nil {
					return err
				}
				if url != "" {
					fmt.Println(url)
				}
				return nil
			})
		},
	}
	prCmd.Flags().StringVar(&prTitle, "title", "", "PR title (default: task description)")
	planCmd.AddCommand(prCmd)

	mergeCmd := &cobra.Command{
		Use:   "merge <plan-file>",
		Short: "merge the task branch into main and transition to done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				if err := executeTaskMerge(repoRoot, project, args[0], store); err != nil {
					return err
				}
				fmt.Printf("merged: %s → done\n", args[0])
				return nil
			})
		},
	}
	planCmd.AddCommand(mergeCmd)

	startOverCmd := &cobra.Command{
		Use:   "start-over <plan-file>",
		Short: "reset the task branch and transition back to planning",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, project, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(project, func(store taskstore.Store) error {
				if err := executeTaskStartOver(repoRoot, project, args[0], store); err != nil {
					return err
				}
				fmt.Printf("reset: %s → planning\n", args[0])
				return nil
			})
		},
	}
	planCmd.AddCommand(startOverCmd)

	var linkProject string
	linkClickUpCmd := &cobra.Command{
		Use:   "link-clickup",
		Short: "backfill ClickUp task IDs from task content (parses **Source:** ClickUp <ID> lines)",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, repoProject, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(repoProject, func(store taskstore.Store) error {
				project := repoProject
				if linkProject != "" {
					project = linkProject
				}
				n, err := executeTaskLinkClickUp(project, store)
				if err != nil {
					return err
				}
				fmt.Printf("linked %d plan(s) to ClickUp tasks\n", n)
				return nil
			})
		},
	}
	linkClickUpCmd.Flags().StringVar(&linkProject, "project", "", "project name (default: derived from current directory)")
	planCmd.AddCommand(linkClickUpCmd)

	var linkLinearForce bool
	var linkLinearComment bool
	var linkLinearMessage string
	var linkLinearReason string
	var linkLinearProject string
	linkLinearCmd := &cobra.Command{
		Use:   "link-linear <plan-file> <issue>",
		Short: "link a task to a Linear issue",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, repoProject, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(repoProject, func(store taskstore.Store) error {
				project := repoProject
				if linkLinearProject != "" {
					project = linkLinearProject
				}
				filename := normalizeTaskFilename(args[0])
				result, err := executeTaskLinkLinear(cmd.Context(), project, linearlink.LinkInput{
					Filename:    filename,
					IssueArg:    args[1],
					Reason:      linkLinearReason,
					CommentBody: linkLinearMessage,
					Force:       linkLinearForce,
					PostComment: linkLinearComment,
				}, store)
				if err != nil {
					if errors.Is(err, linearlink.ErrAlreadyLinked) {
						return fmt.Errorf("task already linked to a linear issue; use --force to replace it: %w", err)
					}
					if errors.Is(err, linearlink.ErrDuplicateLink) {
						return fmt.Errorf("another active task is already linked to that linear issue: %w", err)
					}
					return err
				}
				fmt.Printf("linked %s → %s (%s)\n", filename, result.Link.LinearIdentifier, result.Link.LinearURL)
				if result.CommentWarning != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "comment warning: %s\n", result.CommentWarning)
				}
				return nil
			})
		},
	}
	linkLinearCmd.Flags().BoolVar(&linkLinearForce, "force", false, "replace an existing Linear link")
	linkLinearCmd.Flags().BoolVar(&linkLinearComment, "comment", false, "post a backlink comment to Linear")
	linkLinearCmd.Flags().StringVar(&linkLinearMessage, "message", "", "Linear comment body (default: generated backlink)")
	linkLinearCmd.Flags().StringVar(&linkLinearReason, "reason", "", "reason recorded in audit metadata")
	linkLinearCmd.Flags().StringVar(&linkLinearProject, "project", "", "project name (default: derived from current directory)")
	planCmd.AddCommand(linkLinearCmd)

	var unlinkLinearReason string
	var unlinkLinearProject string
	unlinkLinearCmd := &cobra.Command{
		Use:   "unlink-linear <plan-file>",
		Short: "clear a task's Linear issue link",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, repoProject, err := resolveRepoInfo()
			if err != nil {
				return err
			}
			return withAuthoritativeStore(repoProject, func(store taskstore.Store) error {
				project := repoProject
				if unlinkLinearProject != "" {
					project = unlinkLinearProject
				}
				filename := normalizeTaskFilename(args[0])
				result, err := executeTaskUnlinkLinear(cmd.Context(), project, filename, unlinkLinearReason, store)
				if err != nil {
					return err
				}
				if result.Link.LinearIssueID == "" {
					fmt.Printf("no link to clear for %s\n", filename)
					return nil
				}
				fmt.Printf("unlinked %s from %s\n", filename, linearLinkDisplayID(result.Link))
				return nil
			})
		},
	}
	unlinkLinearCmd.Flags().StringVar(&unlinkLinearReason, "reason", "", "reason recorded in audit metadata")
	unlinkLinearCmd.Flags().StringVar(&unlinkLinearProject, "project", "", "project name (default: derived from current directory)")
	planCmd.AddCommand(unlinkLinearCmd)

	return planCmd
}

// loadTaskStateByProject loads task state using the authoritative store for the
// current project when the caller does not provide one explicitly.
func loadTaskStateByProject(project string, store taskstore.Store) (*taskstate.TaskState, error) {
	if store == nil {
		var err error
		store, err = taskstore.OpenAuthoritativeStore(project)
		if err != nil {
			return nil, err
		}
	}
	return taskstate.Load(store, project, "")
}

// newFSMByProject creates a TaskStateMachine backed by the authoritative store
// for the given project when the caller does not provide one explicitly.
func newFSMByProject(project string, store taskstore.Store) *taskfsm.TaskStateMachine {
	return newFSMByProjectWithHooks(project, store, nil)
}

func newFSMByProjectWithHooks(project string, store taskstore.Store, hooks *taskfsm.HookRegistry) *taskfsm.TaskStateMachine {
	if store == nil {
		var err error
		store, err = taskstore.OpenAuthoritativeStore(project)
		if err != nil {
			panic("newFSMByProject: open authoritative task store: " + err.Error())
		}
	}
	fsm := taskfsm.New(store, project, "")
	if hooks != nil {
		fsm.SetHooks(hooks)
	}
	return fsm
}

func buildTaskTransitionHooks(project string, store taskstore.Store) *taskfsm.HookRegistry {
	result, err := config.LoadTOMLConfig()
	if err != nil || result == nil || !result.LinearReceipts.Enabled {
		return nil
	}
	client, err := linearreceipt.NewClientFromEnv()
	if err != nil {
		return nil
	}
	return linearreceipt.BuildRegistryWithReceipts(nil, result.LinearReceipts, store, client, nil, project)
}

// resolveRepoInfo resolves the main repository root and derives the project
// name from it. Handles both regular repos and git worktrees. The project name
// is the basename of the repo root directory (e.g. "kasmos" for /home/kas/dev/kasmos).
func resolveRepoInfo() (repoRoot, project string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("get cwd: %w", err)
	}

	root, err := resolveRepoRoot(cwd)
	if err != nil {
		return "", "", fmt.Errorf("cannot resolve repo root: %w", err)
	}
	return root, resolveTaskProject(root), nil
}

// resolveRepoRoot delegates to the shared config-level resolver so task
// commands and config/state paths stay aligned on the same repository root.
func resolveRepoRoot(dir string) (string, error) {
	return config.ResolveRepoRoot(dir)
}
