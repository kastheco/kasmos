package taskstate

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
)

type Status string

const ManualOverridePlanned = "planned"

var manualOverrideOptions = []string{
	string(StatusReady),
	ManualOverridePlanned,
	string(StatusPlanning),
	string(StatusImplementing),
	string(StatusReviewing),
	string(StatusVerifying),
	string(StatusDone),
	string(StatusCancelled),
}

const (
	StatusReady      Status = "ready"
	StatusDone       Status = "done"
	StatusReviewing  Status = "reviewing"
	StatusVerifying  Status = "verifying"
	StatusCancelled  Status = "cancelled"

	// Lifecycle-stage statuses — canonical names used by the FSM.
	StatusPlanning     Status = "planning"
	StatusImplementing Status = "implementing"
)

// IsDraftReady returns true for a ready task that has not yet been promoted to
// an executable planned state.
func IsDraftReady(entry TaskEntry) bool {
	return entry.Status == StatusReady && strings.TrimSpace(entry.ExecutionState.Phase) == ""
}

// IsPlannedReady returns true for a ready task whose execution metadata marks
// it as planned and ready to execute.
func IsPlannedReady(entry TaskEntry) bool {
	return entry.Status == StatusReady && strings.TrimSpace(entry.ExecutionState.Phase) == "planned"
}

// ManualOverrideOptions returns the valid operator-facing lifecycle override
// targets. This includes the phase-aware planned-ready state exposed as
// "planned" even though its persisted lifecycle status remains ready.
func ManualOverrideOptions() []string {
	options := make([]string, len(manualOverrideOptions))
	copy(options, manualOverrideOptions)
	return options
}

// ResolveManualOverride maps an operator-facing override target to the
// lifecycle status plus any persisted execution metadata required to represent
// it faithfully in the task store.
func ResolveManualOverride(target string) (Status, taskstore.ExecutionState, error) {
	switch strings.TrimSpace(target) {
	case string(StatusReady):
		return StatusReady, taskstore.ExecutionState{}, nil
	case ManualOverridePlanned:
		return StatusReady, taskstore.ExecutionState{Phase: ManualOverridePlanned}, nil
	case string(StatusPlanning):
		return StatusPlanning, taskstore.ExecutionState{}, nil
	case string(StatusImplementing):
		return StatusImplementing, taskstore.ExecutionState{}, nil
	case string(StatusReviewing):
		return StatusReviewing, taskstore.ExecutionState{}, nil
	case string(StatusVerifying):
		return StatusVerifying, taskstore.ExecutionState{}, nil
	case string(StatusDone):
		return StatusDone, taskstore.ExecutionState{}, nil
	case string(StatusCancelled):
		return StatusCancelled, taskstore.ExecutionState{}, nil
	default:
		return "", taskstore.ExecutionState{}, fmt.Errorf("invalid manual override %q: must be one of %s", target, strings.Join(manualOverrideOptions, ", "))
	}
}

// IsActiveLifecycle returns true while a task is actively moving through its
// lifecycle, excluding draft/planned-ready and terminal states.
func IsActiveLifecycle(entry TaskEntry) bool {
	switch entry.Status {
	case StatusPlanning, StatusImplementing, StatusReviewing, StatusVerifying:
		return true
	default:
		return false
	}
}

// CanStartImplementation returns true when implementation can legitimately
// start or resume from the current task entry.
func CanStartImplementation(entry TaskEntry) bool {
	return entry.Status == StatusPlanning || IsPlannedReady(entry) || entry.Status == StatusImplementing
}

func normalizeExecutionState(status Status, state taskstore.ExecutionState) taskstore.ExecutionState {
	state.Phase = strings.TrimSpace(state.Phase)
	state.ActiveAgentType = strings.TrimSpace(state.ActiveAgentType)
	if state.ActiveWave < 0 {
		state.ActiveWave = 0
	}

	switch status {
	case StatusReady:
		if state.Phase != "planned" {
			return taskstore.ExecutionState{}
		}
		return taskstore.ExecutionState{Phase: state.Phase}
	case StatusImplementing:
		switch state.Phase {
		case "architecting":
			if state.ActiveAgentType == "" {
				return taskstore.ExecutionState{}
			}
			return taskstore.ExecutionState{Phase: state.Phase, ActiveAgentType: state.ActiveAgentType}
		case "single_agent_implementing", "fixing":
			if state.ActiveAgentType == "" {
				return taskstore.ExecutionState{}
			}
			return taskstore.ExecutionState{Phase: state.Phase, ActiveAgentType: state.ActiveAgentType}
		case "wave_running", "wave_waiting":
			if state.ActiveAgentType == "" || state.ActiveWave <= 0 {
				return taskstore.ExecutionState{}
			}
			return taskstore.ExecutionState{Phase: state.Phase, ActiveAgentType: state.ActiveAgentType, ActiveWave: state.ActiveWave}
		default:
			return taskstore.ExecutionState{}
		}
	case StatusReviewing:
		switch state.Phase {
		case "reviewing":
			if state.ActiveAgentType == "" {
				return taskstore.ExecutionState{}
			}
			return taskstore.ExecutionState{Phase: state.Phase, ActiveAgentType: state.ActiveAgentType}
		default:
			return taskstore.ExecutionState{}
		}
	case StatusVerifying:
		if state.Phase != "" && state.Phase != "verifying" {
			return taskstore.ExecutionState{}
		}
		return taskstore.ExecutionState{ActiveAgentType: state.ActiveAgentType}
	default:
		return taskstore.ExecutionState{}
	}
}

type TaskEntry struct {
	Status               Status                   `json:"status"`
	ExecutionState       taskstore.ExecutionState `json:"execution_state,omitempty"`
	Description          string                   `json:"description,omitempty"`
	Branch               string                   `json:"branch,omitempty"`
	Topic                string                   `json:"topic,omitempty"`
	CreatedAt            time.Time                `json:"created_at,omitempty"`
	Implemented          string                   `json:"implemented,omitempty"`
	PlanningAt           time.Time                `json:"planning_at,omitempty"`
	ImplementingAt       time.Time                `json:"implementing_at,omitempty"`
	ReviewingAt          time.Time                `json:"reviewing_at,omitempty"`
	VerifyingAt          time.Time                `json:"verifying_at,omitempty"`
	DoneAt               time.Time                `json:"done_at,omitempty"`
	Goal                 string                   `json:"goal,omitempty"`
	ClickUpTaskID        string                   `json:"clickup_task_id,omitempty"`
	ReviewCycle          int                      `json:"review_cycle,omitempty"`
	LatestReviewFeedback string                   `json:"latest_review_feedback,omitempty"`
}

type TopicEntry struct {
	CreatedAt time.Time `json:"created_at"`
}

type TaskState struct {
	Dir          string
	Plans        map[string]TaskEntry
	TopicEntries map[string]TopicEntry
	store        taskstore.Store // nil for read-only snapshots loaded outside the task store
	project      string          // project name used with the store when store-backed
}

// HasStore reports whether this TaskState is backed by a persistent store.
// Returns false for read-only snapshots (e.g. daemon-synced state).
func (ps *TaskState) HasStore() bool {
	return ps.store != nil
}

var errNoStore = fmt.Errorf("task state has no backing store (read-only snapshot)")

func (ps *TaskState) requireStore() error {
	if ps.store == nil {
		return errNoStore
	}
	return nil
}

type TaskInfo struct {
	Filename    string
	Status      Status
	Description string
	Branch      string
	Topic       string
	CreatedAt   time.Time
	DoneAt      time.Time
}

type TopicInfo struct {
	Name      string
	CreatedAt time.Time
}

func taskEntryFromStoreEntry(entry taskstore.TaskEntry, goal string) TaskEntry {
	return TaskEntry{
		Status:               Status(entry.Status),
		ExecutionState:       entry.ExecutionState,
		Description:          entry.Description,
		Branch:               entry.Branch,
		Topic:                entry.Topic,
		CreatedAt:            entry.CreatedAt,
		Implemented:          entry.Implemented,
		PlanningAt:           entry.PlanningAt,
		ImplementingAt:       entry.ImplementingAt,
		ReviewingAt:          entry.ReviewingAt,
		VerifyingAt:          entry.VerifyingAt,
		DoneAt:               entry.DoneAt,
		Goal:                 goal,
		ClickUpTaskID:        entry.ClickUpTaskID,
		ReviewCycle:          entry.ReviewCycle,
		LatestReviewFeedback: entry.LatestReviewFeedback,
	}
}

func setTopicEntry(topicEntries map[string]TopicEntry, topic string, createdAt time.Time) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return
	}
	existing, ok := topicEntries[topic]
	if !ok || existing.CreatedAt.IsZero() || (!createdAt.IsZero() && createdAt.Before(existing.CreatedAt)) {
		topicEntries[topic] = TopicEntry{CreatedAt: createdAt}
	}
}

// LoadFromEntries creates a read-only TaskState snapshot from task-store-style
// entries. Topic entries are synthesized from non-empty plan topics so grouped
// sidebar rendering keeps working when metadata comes from the daemon API.
func LoadFromEntries(dir string, entries []taskstore.TaskEntry) *TaskState {
	ps := &TaskState{
		Dir:          dir,
		Plans:        make(map[string]TaskEntry, len(entries)),
		TopicEntries: make(map[string]TopicEntry),
	}

	for _, entry := range entries {
		ps.Plans[entry.Filename] = taskEntryFromStoreEntry(entry, entry.Goal)
		setTopicEntry(ps.TopicEntries, entry.Topic, entry.CreatedAt)
	}

	return ps
}

// Load creates a TaskState backed by the given store. Plans and TopicEntries are
// populated from the store. dir is retained for compatibility and auxiliary path
// resolution, not for filename migration. The store is always required — there is
// no JSON fallback.
func Load(store taskstore.Store, project, dir string) (*TaskState, error) {
	plans, err := store.List(project)
	if err != nil {
		return nil, fmt.Errorf("task store: %w", err)
	}

	topics, err := store.ListTopics(project)
	if err != nil {
		return nil, fmt.Errorf("task store: %w", err)
	}

	ps := LoadFromEntries(dir, plans)
	ps.store = store
	ps.project = project

	for _, e := range plans {
		goal := e.Goal
		// Backfill: if content exists but goal is empty, parse it now and persist.
		if goal == "" && e.Content != "" {
			if plan, parseErr := taskparser.Parse(e.Content); parseErr == nil && plan.Goal != "" {
				goal = plan.Goal
				_ = store.SetPlanGoal(project, e.Filename, goal)
			}
		}
		ps.Plans[e.Filename] = taskEntryFromStoreEntry(e, goal)
	}

	for _, t := range topics {
		if ps.TopicEntries == nil {
			ps.TopicEntries = make(map[string]TopicEntry, len(topics))
		}
		ps.TopicEntries[t.Name] = TopicEntry{CreatedAt: t.CreatedAt}
	}

	return ps, nil
}

// Topics returns all topic entries sorted by name.
func (ps *TaskState) Topics() []TopicInfo {
	// Discover topics from both TopicEntries and plan topic fields.
	seen := make(map[string]TopicInfo)
	for name, entry := range ps.TopicEntries {
		seen[name] = TopicInfo{Name: name, CreatedAt: entry.CreatedAt}
	}
	for _, entry := range ps.Plans {
		if entry.Topic != "" {
			if _, ok := seen[entry.Topic]; !ok {
				seen[entry.Topic] = TopicInfo{Name: entry.Topic}
			}
		}
	}
	result := make([]TopicInfo, 0, len(seen))
	for _, info := range seen {
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// TasksByTopic returns all plans in the given topic, sorted by filename.
func (ps *TaskState) TasksByTopic(topic string) []TaskInfo {
	result := make([]TaskInfo, 0)
	for filename, entry := range ps.Plans {
		if entry.Topic == topic {
			result = append(result, TaskInfo{
				Filename: filename, Status: entry.Status,
				Description: entry.Description, Branch: entry.Branch,
				Topic: entry.Topic, CreatedAt: entry.CreatedAt,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Filename < result[j].Filename
	})
	return result
}

// UngroupedTasks returns all active plans with no topic, sorted by filename.
func (ps *TaskState) UngroupedTasks() []TaskInfo {
	result := make([]TaskInfo, 0)
	for filename, entry := range ps.Plans {
		if entry.Status == StatusDone || entry.Status == StatusCancelled {
			continue
		}
		if entry.Topic == "" {
			result = append(result, TaskInfo{
				Filename: filename, Status: entry.Status,
				Description: entry.Description, Branch: entry.Branch,
				CreatedAt: entry.CreatedAt,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Filename < result[j].Filename
	})
	return result
}

// hasTopicEntry returns true if the given topic name exists in TopicEntries.
func (ps *TaskState) hasTopicEntry(topic string) bool {
	if ps.TopicEntries == nil {
		return false
	}
	_, exists := ps.TopicEntries[topic]
	return exists
}

// HasRunningCoderInTopic checks if any plan in the given topic (other than
// excludePlan) has status StatusImplementing. Returns the conflicting plan filename.
func (ps *TaskState) HasRunningCoderInTopic(topic, excludePlan string) (bool, string) {
	if topic == "" {
		return false, ""
	}
	for filename, entry := range ps.Plans {
		if filename == excludePlan {
			continue
		}
		if entry.Topic == topic && entry.Status == StatusImplementing {
			return true, filename
		}
	}
	return false, ""
}

// Unfinished returns plans that are not done or cancelled, sorted by filename.
func (ps *TaskState) Unfinished() []TaskInfo {
	result := make([]TaskInfo, 0, len(ps.Plans))
	for filename, entry := range ps.Plans {
		if entry.Status == StatusDone || entry.Status == StatusCancelled {
			continue
		}
		result = append(result, TaskInfo{
			Filename: filename, Status: entry.Status,
			Description: entry.Description, Branch: entry.Branch,
			Topic: entry.Topic, CreatedAt: entry.CreatedAt,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Filename < result[j].Filename
	})
	return result
}

// Finished returns plans that are done, sorted by done time (newest first).
func (ps *TaskState) Finished() []TaskInfo {
	result := make([]TaskInfo, 0)
	for filename, entry := range ps.Plans {
		if entry.Status != StatusDone {
			continue
		}
		result = append(result, TaskInfo{
			Filename: filename, Status: entry.Status,
			Description: entry.Description, Branch: entry.Branch,
			Topic: entry.Topic, CreatedAt: entry.CreatedAt,
			DoneAt: entry.DoneAt,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].DoneAt.Equal(result[j].DoneAt) {
			return result[i].DoneAt.After(result[j].DoneAt)
		}
		return result[i].Filename > result[j].Filename
	})
	return result
}

// Cancelled returns all cancelled plans, sorted by filename.
func (ps *TaskState) Cancelled() []TaskInfo {
	result := make([]TaskInfo, 0)
	for filename, entry := range ps.Plans {
		if entry.Status != StatusCancelled {
			continue
		}
		result = append(result, TaskInfo{
			Filename: filename, Status: entry.Status,
			Description: entry.Description, Branch: entry.Branch,
			Topic: entry.Topic, CreatedAt: entry.CreatedAt,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Filename < result[j].Filename
	})
	return result
}

// List returns all plans (including done and cancelled), sorted by filename.
func (ps *TaskState) List() []TaskInfo {
	result := make([]TaskInfo, 0, len(ps.Plans))
	for filename, entry := range ps.Plans {
		result = append(result, TaskInfo{
			Filename:    filename,
			Status:      entry.Status,
			Description: entry.Description,
			Branch:      entry.Branch,
			Topic:       entry.Topic,
			CreatedAt:   entry.CreatedAt,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Filename < result[j].Filename
	})
	return result
}

// IsDone returns true only if the given plan has status StatusDone.
func (ps *TaskState) IsDone(filename string) bool {
	entry, ok := ps.Plans[filename]
	if !ok {
		return false
	}
	return entry.Status == StatusDone
}

// ForceSetStatus overrides a plan's status regardless of FSM rules.
// Validates the status is a known value. Use only for manual overrides (e.g. kq plan set-status --force).
func (ps *TaskState) ForceSetStatus(filename string, status Status) error {
	return ps.ForceSetLifecycle(filename, status, taskstore.ExecutionState{})
}

// ForceSetLifecycle overrides a plan's lifecycle status plus any persisted
// execution metadata used to distinguish draft-ready from planned-ready tasks.
func (ps *TaskState) ForceSetLifecycle(filename string, status Status, state taskstore.ExecutionState) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	if !isValidStatus(status) {
		return fmt.Errorf("invalid status %q: must be one of ready, planning, implementing, reviewing, done, cancelled", status)
	}
	if _, ok := ps.Plans[filename]; !ok {
		return fmt.Errorf("plan not found: %s", filename)
	}
	entry := ps.Plans[filename]
	entry.Status = status
	entry.ExecutionState = normalizeExecutionState(status, state)
	ps.Plans[filename] = entry
	if err := ps.store.Update(ps.project, filename, ps.toTaskstoreEntry(filename, entry)); err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	return nil
}

// isValidStatus returns true if s is a recognised lifecycle status.
func isValidStatus(s Status) bool {
	switch s {
	case StatusReady, StatusPlanning, StatusImplementing, StatusReviewing, StatusVerifying, StatusDone, StatusCancelled:
		return true
	}
	return false
}

// setStatus updates a plan's status and persists to the store.
// Unexported: only for use within this package (tests). Production code must use taskfsm.Transition.
func (ps *TaskState) setStatus(filename string, status Status) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	if ps.Plans == nil {
		ps.Plans = make(map[string]TaskEntry)
	}
	entry := ps.Plans[filename]
	entry.Status = status
	entry.ExecutionState = taskstore.ExecutionState{}
	ps.Plans[filename] = entry
	if err := ps.store.Update(ps.project, filename, ps.toTaskstoreEntry(filename, entry)); err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	return nil
}

// CreateWithContent adds a new plan entry with markdown content stored in the backend.
// The plan entry is created with StatusReady, and the content is persisted via store.SetContent.
// Returns an error if the plan already exists.
func (ps *TaskState) CreateWithContent(filename, description, branch, topic string, createdAt time.Time, content string) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	if err := ps.Create(filename, description, branch, topic, createdAt); err != nil {
		return err
	}
	if err := ps.store.SetContent(ps.project, filename, content); err != nil {
		return fmt.Errorf("task store set content: %w", err)
	}
	return nil
}

// GetContent retrieves the markdown content for the given plan filename from the store.
func (ps *TaskState) GetContent(filename string) (string, error) {
	if err := ps.requireStore(); err != nil {
		return "", err
	}
	return ps.store.GetContent(ps.project, filename)
}

// SetContent updates the markdown content for the given plan filename in the store.
func (ps *TaskState) SetContent(filename, content string) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	return ps.store.SetContent(ps.project, filename, content)
}

// IngestContent stores task markdown and updates derived metadata.
// Draft content without wave headers is valid persisted state: metadata is
// extracted, stale subtasks are cleared, and no warning is returned. Once wave
// headers are present, executable subtask rows are rebuilt from the parsed plan.
func (ps *TaskState) IngestContent(filename, content string) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	if _, ok := ps.Plans[filename]; !ok {
		return fmt.Errorf("plan not found: %s", filename)
	}

	if err := ps.store.SetContent(ps.project, filename, content); err != nil {
		return fmt.Errorf("task store set content: %w", err)
	}

	meta := taskparser.ExtractMetadata(content)
	if err := ps.store.SetPlanGoal(ps.project, filename, meta.Goal); err != nil {
		return fmt.Errorf("task store set plan goal: %w", err)
	}

	entry := ps.Plans[filename]
	entry.Goal = meta.Goal
	ps.Plans[filename] = entry

	if !taskparser.HasWaveHeaders(content) {
		if err := ps.store.SetSubtasks(ps.project, filename, nil); err != nil {
			return fmt.Errorf("task store clear subtasks: %w", err)
		}
		return nil
	}

	plan, err := taskparser.Parse(content)
	if err != nil {
		return fmt.Errorf("parse plan content: %w", err)
	}

	subtasks := make([]taskstore.SubtaskEntry, 0)
	for _, wave := range plan.Waves {
		for _, task := range wave.Tasks {
			subtasks = append(subtasks, taskstore.SubtaskEntry{
				TaskNumber: task.Number,
				Title:      task.Title,
				Status:     taskstore.SubtaskStatusPending,
			})
		}
	}
	if err := ps.store.SetSubtasks(ps.project, filename, subtasks); err != nil {
		return fmt.Errorf("task store set subtasks: %w", err)
	}

	return nil
}

// GetSubtasks returns persisted subtasks for the given plan.
func (ps *TaskState) GetSubtasks(filename string) ([]taskstore.SubtaskEntry, error) {
	if err := ps.requireStore(); err != nil {
		return nil, err
	}
	return ps.store.GetSubtasks(ps.project, filename)
}

// UpdateSubtaskStatus updates one persisted subtask status.
func (ps *TaskState) UpdateSubtaskStatus(filename string, taskNumber int, status taskstore.SubtaskStatus) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	return ps.store.UpdateSubtaskStatus(ps.project, filename, taskNumber, status)
}

// Create adds a new plan entry to the state and auto-creates the topic if needed.
func (ps *TaskState) Create(filename, description, branch, topic string, createdAt time.Time) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	if ps.Plans == nil {
		ps.Plans = make(map[string]TaskEntry)
	}
	if _, exists := ps.Plans[filename]; exists {
		return fmt.Errorf("plan already exists: %s", filename)
	}
	entry := TaskEntry{
		Status:      StatusReady,
		Description: description,
		Branch:      branch,
		Topic:       topic,
		CreatedAt:   createdAt.UTC(),
	}
	ps.Plans[filename] = entry
	// Auto-create topic entry if it doesn't exist
	if topic != "" {
		if ps.TopicEntries == nil {
			ps.TopicEntries = make(map[string]TopicEntry)
		}
		if _, exists := ps.TopicEntries[topic]; !exists {
			ps.TopicEntries[topic] = TopicEntry{CreatedAt: createdAt.UTC()}
		}
	}
	if err := ps.store.Create(ps.project, ps.toTaskstoreEntry(filename, entry)); err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	// Auto-create topic in store if needed
	if topic != "" {
		topicEntry := taskstore.TopicEntry{Name: topic, CreatedAt: createdAt.UTC()}
		if err := ps.store.CreateTopic(ps.project, topicEntry); err != nil {
			// Ignore "already exists" errors for topics
			if !isAlreadyExistsError(err) {
				return fmt.Errorf("task store: %w", err)
			}
		}
	}
	return nil
}

// Register adds a new plan entry with metadata and persists to the store.
// Returns an error if the plan already exists.
func (ps *TaskState) Register(filename, description, branch string, createdAt time.Time) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	if ps.Plans == nil {
		ps.Plans = make(map[string]TaskEntry)
	}
	if _, exists := ps.Plans[filename]; exists {
		return fmt.Errorf("plan already exists: %s", filename)
	}
	entry := TaskEntry{
		Status:      StatusReady,
		Description: description,
		Branch:      branch,
		CreatedAt:   createdAt.UTC(),
	}
	ps.Plans[filename] = entry
	if err := ps.store.Create(ps.project, ps.toTaskstoreEntry(filename, entry)); err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	return nil
}

// Entry returns the TaskEntry for the given filename, and whether it exists.
func (ps *TaskState) Entry(filename string) (TaskEntry, bool) {
	entry, ok := ps.Plans[filename]
	return entry, ok
}

// SetTopic assigns a topic to an existing plan entry and persists to the store.
// If topic is non-empty and does not yet exist in TopicEntries, it is auto-created.
// Pass an empty string to remove the plan from any topic.
func (ps *TaskState) SetTopic(filename, topic string) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	entry, ok := ps.Plans[filename]
	if !ok {
		return fmt.Errorf("plan not found: %s", filename)
	}
	entry.Topic = topic
	ps.Plans[filename] = entry
	// Auto-create topic entry if it doesn't exist
	if topic != "" {
		if ps.TopicEntries == nil {
			ps.TopicEntries = make(map[string]TopicEntry)
		}
		if _, exists := ps.TopicEntries[topic]; !exists {
			ps.TopicEntries[topic] = TopicEntry{CreatedAt: time.Now().UTC()}
		}
	}
	if err := ps.store.Update(ps.project, filename, ps.toTaskstoreEntry(filename, entry)); err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	// Auto-create topic in store if needed
	if topic != "" {
		topicEntry := taskstore.TopicEntry{Name: topic, CreatedAt: ps.TopicEntries[topic].CreatedAt}
		if err := ps.store.CreateTopic(ps.project, topicEntry); err != nil {
			if !isAlreadyExistsError(err) {
				return fmt.Errorf("task store: %w", err)
			}
		}
	}
	return nil
}

// SetBranch assigns a branch name to an existing plan entry and persists to the store.
func (ps *TaskState) SetBranch(filename, branch string) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	entry, ok := ps.Plans[filename]
	if !ok {
		return fmt.Errorf("plan not found: %s", filename)
	}
	entry.Branch = branch
	ps.Plans[filename] = entry
	if err := ps.store.Update(ps.project, filename, ps.toTaskstoreEntry(filename, entry)); err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	return nil
}

// Save is a no-op — all mutations write through to the store immediately.
// Retained for API compatibility.
func (ps *TaskState) Save() error {
	return nil
}

// DisplayName returns a plan display name. For bare slugs it is a pass-through,
// e.g. "auth-refactor" -> "auth-refactor".
func DisplayName(filename string) string {
	return filename
}

// Rename renames a plan by giving it a new display name slug.
// It rekeys the taskstate entry and persists the updated task entry in the store.
// newName should be a human-readable name (e.g., "auth refactor") which will be
// slugified automatically. Returns the new filename on success.
func (ps *TaskState) Rename(oldFilename, newName string) (string, error) {
	if err := ps.requireStore(); err != nil {
		return "", err
	}
	entry, ok := ps.Plans[oldFilename]
	if !ok {
		return "", fmt.Errorf("plan not found: %s", oldFilename)
	}
	if newName == "" {
		return "", fmt.Errorf("new name cannot be empty")
	}

	// Build new filename from the slug alone.
	newSlug := slugify(newName)
	if newSlug == "" {
		return "", fmt.Errorf("new name produced an empty slug")
	}
	newFilename := newSlug

	if newFilename == oldFilename {
		return oldFilename, nil // nothing to do
	}
	if _, exists := ps.Plans[newFilename]; exists {
		return "", fmt.Errorf("a plan named %q already exists", newFilename)
	}

	// Rekey the planstate entry.
	ps.Plans[newFilename] = entry
	delete(ps.Plans, oldFilename)

	if err := ps.store.Rename(ps.project, oldFilename, newFilename); err != nil {
		return "", fmt.Errorf("task store: %w", err)
	}
	return newFilename, nil
}

// slugify converts a human name to a lowercase, hyphen-separated slug.
// "My Cool Feature!" → "my-cool-feature"
func slugify(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	// Replace any sequence of non-alphanumeric characters with a hyphen.
	result := make([]rune, 0, len(name))
	inHyphen := false
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			result = append(result, r)
			inHyphen = false
		} else if !inHyphen && len(result) > 0 {
			result = append(result, '-')
			inHyphen = true
		}
	}
	// Trim trailing hyphen.
	for len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}
	return string(result)
}

// isAlreadyExistsError returns true if the error indicates a duplicate resource.
func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "already exists")
}

// toTaskstoreEntry converts a local TaskEntry to a taskstore.TaskEntry for
// writing to the store.
func (ps *TaskState) toTaskstoreEntry(filename string, e TaskEntry) taskstore.TaskEntry {
	return taskstore.TaskEntry{
		ExecutionState:       e.ExecutionState,
		Filename:             filename,
		Status:               taskstore.Status(e.Status),
		Description:          e.Description,
		Branch:               e.Branch,
		Topic:                e.Topic,
		CreatedAt:            e.CreatedAt,
		Implemented:          e.Implemented,
		PlanningAt:           e.PlanningAt,
		ImplementingAt:       e.ImplementingAt,
		ReviewingAt:          e.ReviewingAt,
		VerifyingAt:          e.VerifyingAt,
		DoneAt:               e.DoneAt,
		Goal:                 e.Goal,
		ClickUpTaskID:        e.ClickUpTaskID,
		ReviewCycle:          e.ReviewCycle,
		LatestReviewFeedback: e.LatestReviewFeedback,
	}
}

// SetExecutionState updates a plan's fine-grained execution metadata and
// persists it to the store.
func (ps *TaskState) SetExecutionState(filename string, state taskstore.ExecutionState) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	entry, ok := ps.Plans[filename]
	if !ok {
		return fmt.Errorf("plan not found: %s", filename)
	}
	entry.ExecutionState = normalizeExecutionState(entry.Status, state)
	ps.Plans[filename] = entry
	if writer, ok := ps.store.(taskstore.ExecutionStateWriter); ok {
		if err := writer.SetExecutionState(ps.project, filename, entry.ExecutionState); err != nil {
			return fmt.Errorf("task store: %w", err)
		}
		return nil
	}
	if err := ps.store.Update(ps.project, filename, ps.toTaskstoreEntry(filename, entry)); err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	return nil
}

// ClearExecutionState removes any persisted fine-grained execution metadata for a plan.
func (ps *TaskState) ClearExecutionState(filename string) error {
	return ps.SetExecutionState(filename, taskstore.ExecutionState{})
}

// SetClickUpTaskID assigns a ClickUp task ID to an existing plan entry and
// persists to the store.
func (ps *TaskState) SetClickUpTaskID(filename, taskID string) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	entry, ok := ps.Plans[filename]
	if !ok {
		return fmt.Errorf("plan not found: %s", filename)
	}
	entry.ClickUpTaskID = taskID
	ps.Plans[filename] = entry
	if err := ps.store.SetClickUpTaskID(ps.project, filename, taskID); err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	return nil
}

// ReviewCycle returns the current review cycle counter for the given plan.
// Returns an error if the plan is not found.
func (ps *TaskState) ReviewCycle(filename string) (int, error) {
	entry, ok := ps.Plans[filename]
	if !ok {
		return 0, fmt.Errorf("plan not found: %s", filename)
	}
	return entry.ReviewCycle, nil
}

// IncrementReviewCycle increments the review cycle counter for the given plan
// and persists to the store. Updates the in-memory state to reflect the new value.
// Returns an error if the plan is not found.
func (ps *TaskState) IncrementReviewCycle(filename string) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	entry, ok := ps.Plans[filename]
	if !ok {
		return fmt.Errorf("plan not found: %s", filename)
	}
	if err := ps.store.IncrementReviewCycle(ps.project, filename); err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	entry.ReviewCycle++
	ps.Plans[filename] = entry
	return nil
}

// SetLatestReviewFeedback stores the latest structured reviewer feedback for a plan
// so re-review and manual fixer restarts can recover the previous round context.
func (ps *TaskState) SetLatestReviewFeedback(filename, feedback string) error {
	if err := ps.requireStore(); err != nil {
		return err
	}
	entry, ok := ps.Plans[filename]
	if !ok {
		return fmt.Errorf("plan not found: %s", filename)
	}
	trimmed := strings.TrimSpace(feedback)
	stored, err := ps.store.Get(ps.project, filename)
	if err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	stored.LatestReviewFeedback = trimmed
	if err := ps.store.Update(ps.project, filename, stored); err != nil {
		return fmt.Errorf("task store: %w", err)
	}
	entry.ReviewCycle = stored.ReviewCycle
	entry.LatestReviewFeedback = trimmed
	ps.Plans[filename] = entry
	return nil
}

// ClearLatestReviewFeedback removes any persisted reviewer feedback for a plan.
func (ps *TaskState) ClearLatestReviewFeedback(filename string) error {
	return ps.SetLatestReviewFeedback(filename, "")
}
