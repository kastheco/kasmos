package taskstore

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// signalSlotShift allocates the top 16 bits for the repo slot and the bottom
// 48 bits for the per-repo local signal ID.  SQLite INTEGER PRIMARY KEY is a
// signed 64-bit integer and rows are auto-incremented; 2^48 ≈ 281 trillion
// rows per repo is far beyond any practical limit.
const signalSlotShift = 48

// RepoConfig describes a repository whose local taskstore should be opened.
type RepoConfig struct {
	Path string
}

// MultiStore dispatches Store calls to per-project backends.
type MultiStore struct {
	stores map[string]Store
}

// MultiSignalGateway dispatches SignalGateway calls to per-project backends.
type MultiSignalGateway struct {
	gateways      map[string]SignalGateway
	slotByProject map[string]int
	projectBySlot map[int]string
}

var _ Store = (*MultiStore)(nil)
var _ ExecutionStateWriter = (*MultiStore)(nil)
var _ SignalGateway = (*MultiSignalGateway)(nil)

// NewMultiStore opens one SQLite task store per configured repository.
func NewMultiStore(repos []RepoConfig) (*MultiStore, error) {
	stores := make(map[string]Store, len(repos))
	seenPaths := make(map[string]struct{}, len(repos))
	pathByProject := make(map[string]string, len(repos))
	opened := make([]Store, 0, len(repos))

	for _, repo := range repos {
		repoPath, err := normalizeRepoPath(repo.Path)
		if err != nil {
			closeAllStores(opened)
			return nil, err
		}
		if _, ok := seenPaths[repoPath]; ok {
			closeAllStores(opened)
			return nil, fmt.Errorf("repo already registered: %s", repoPath)
		}

		project := filepath.Base(repoPath)
		if existingPath, ok := pathByProject[project]; ok {
			closeAllStores(opened)
			return nil, fmt.Errorf("repo with basename %q already registered (path: %s); rename one of the directories or use distinct names", project, existingPath)
		}

		kasmosDir := filepath.Join(repoPath, ".kasmos")
		if err := os.MkdirAll(kasmosDir, 0o755); err != nil {
			closeAllStores(opened)
			return nil, fmt.Errorf("create kasmos dir: %w", err)
		}

		store, err := NewSQLiteStore(filepath.Join(kasmosDir, "taskstore.db"))
		if err != nil {
			closeAllStores(opened)
			return nil, err
		}

		stores[project] = store
		seenPaths[repoPath] = struct{}{}
		pathByProject[project] = repoPath
		opened = append(opened, store)
	}

	return &MultiStore{stores: stores}, nil
}

// NewMultiSignalGateway opens one SQLite signal gateway per configured repository.
func NewMultiSignalGateway(repos []RepoConfig) (*MultiSignalGateway, error) {
	gateways := make(map[string]SignalGateway, len(repos))
	slotByProject := make(map[string]int, len(repos))
	projectBySlot := make(map[int]string, len(repos))
	seenPaths := make(map[string]struct{}, len(repos))
	pathByProject := make(map[string]string, len(repos))
	opened := make([]SignalGateway, 0, len(repos))

	for slot, repo := range repos {
		repoPath, err := normalizeRepoPath(repo.Path)
		if err != nil {
			closeAllGateways(opened)
			return nil, err
		}
		if _, ok := seenPaths[repoPath]; ok {
			closeAllGateways(opened)
			return nil, fmt.Errorf("repo already registered: %s", repoPath)
		}

		project := filepath.Base(repoPath)
		if existingPath, ok := pathByProject[project]; ok {
			closeAllGateways(opened)
			return nil, fmt.Errorf("repo with basename %q already registered (path: %s); rename one of the directories or use distinct names", project, existingPath)
		}

		kasmosDir := filepath.Join(repoPath, ".kasmos")
		if err := os.MkdirAll(kasmosDir, 0o755); err != nil {
			closeAllGateways(opened)
			return nil, fmt.Errorf("create kasmos dir: %w", err)
		}

		gateway, err := NewSQLiteSignalGateway(filepath.Join(kasmosDir, "taskstore.db"))
		if err != nil {
			closeAllGateways(opened)
			return nil, err
		}

		gateways[project] = gateway
		slotByProject[project] = slot
		projectBySlot[slot] = project
		seenPaths[repoPath] = struct{}{}
		pathByProject[project] = repoPath
		opened = append(opened, gateway)
	}

	return &MultiSignalGateway{
		gateways:      gateways,
		slotByProject: slotByProject,
		projectBySlot: projectBySlot,
	}, nil
}

func normalizeRepoPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve repo path %q: %w", path, err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If the path does not exist yet, fall back to the absolute, cleaned path.
		if os.IsNotExist(err) {
			return filepath.Clean(absPath), nil
		}
		return "", fmt.Errorf("canonicalize repo path %q: %w", path, err)
	}
	return filepath.Clean(resolvedPath), nil
}

func closeAllStores(stores []Store) {
	for _, store := range stores {
		if store != nil {
			_ = store.Close()
		}
	}
}

func closeAllGateways(gateways []SignalGateway) {
	for _, gateway := range gateways {
		if gateway != nil {
			_ = gateway.Close()
		}
	}
}

func (m *MultiStore) storeFor(project string) (Store, error) {
	if m == nil {
		return nil, fmt.Errorf("project not found: %s", project)
	}
	store, ok := m.stores[project]
	if !ok {
		return nil, fmt.Errorf("project not found: %s", project)
	}
	return store, nil
}

func (m *MultiSignalGateway) gatewayFor(project string) (SignalGateway, error) {
	if m == nil {
		return nil, fmt.Errorf("project not found: %s", project)
	}
	gateway, ok := m.gateways[project]
	if !ok {
		return nil, fmt.Errorf("project not found: %s", project)
	}
	return gateway, nil
}

func (m *MultiSignalGateway) gatewayForID(id int64) (SignalGateway, int64, error) {
	if m == nil {
		return nil, 0, fmt.Errorf("signal not found: %d", id)
	}
	slot, localID, err := decodeSignalID(id)
	if err != nil {
		return nil, 0, err
	}
	if localID == 0 {
		return nil, 0, fmt.Errorf("signal not found: %d", id)
	}
	project, ok := m.projectBySlot[slot]
	if !ok {
		return nil, 0, fmt.Errorf("signal not found: %d", id)
	}
	gateway, ok := m.gateways[project]
	if !ok {
		return nil, 0, fmt.Errorf("signal not found: %d", id)
	}
	return gateway, localID, nil
}

// encodeSignalID encodes a repo slot and per-repo local signal ID into a single
// int64.  slot must be in [0, 2^15-1] (top 15 usable bits, with bit 63 reserved
// for the sign) and localID must be in [0, 2^48-1].
func encodeSignalID(slot int, localID int64) (int64, error) {
	const localIDMask = (int64(1) << signalSlotShift) - 1
	const maxSlot = int64(1) << (63 - signalSlotShift) // ensures encoded ID stays positive
	if int64(slot) < 0 || int64(slot) >= maxSlot {
		return 0, fmt.Errorf("slot %d exceeds encodable range (max %d)", slot, maxSlot-1)
	}
	if localID < 0 || localID > localIDMask {
		return 0, fmt.Errorf("signal local ID %d exceeds encodable range (max %d)", localID, localIDMask)
	}
	return int64(slot)<<signalSlotShift | localID, nil
}

func decodeSignalID(id int64) (slot int, localID int64, err error) {
	if id <= 0 {
		return 0, 0, fmt.Errorf("signal not found: %d", id)
	}
	return int(id >> signalSlotShift), id & ((int64(1) << signalSlotShift) - 1), nil
}

func (m *MultiStore) Create(project string, entry TaskEntry) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.Create(project, entry)
}

func (m *MultiStore) Get(project, filename string) (TaskEntry, error) {
	store, err := m.storeFor(project)
	if err != nil {
		return TaskEntry{}, err
	}
	return store.Get(project, filename)
}

func (m *MultiStore) Update(project, filename string, entry TaskEntry) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.Update(project, filename, entry)
}

func (m *MultiStore) Rename(project, oldFilename, newFilename string) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.Rename(project, oldFilename, newFilename)
}

func (m *MultiStore) GetContent(project, filename string) (string, error) {
	store, err := m.storeFor(project)
	if err != nil {
		return "", err
	}
	return store.GetContent(project, filename)
}

func (m *MultiStore) SetContent(project, filename, content string) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.SetContent(project, filename, content)
}

func (m *MultiStore) SetSubtasks(project, filename string, subtasks []SubtaskEntry) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.SetSubtasks(project, filename, subtasks)
}

func (m *MultiStore) GetSubtasks(project, filename string) ([]SubtaskEntry, error) {
	store, err := m.storeFor(project)
	if err != nil {
		return nil, err
	}
	return store.GetSubtasks(project, filename)
}

func (m *MultiStore) UpdateSubtaskStatus(project, filename string, taskNumber int, status SubtaskStatus) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.UpdateSubtaskStatus(project, filename, taskNumber, status)
}

func (m *MultiStore) SetPhaseTimestamp(project, filename, phase string, ts time.Time) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.SetPhaseTimestamp(project, filename, phase, ts)
}

func (m *MultiStore) SetClickUpTaskID(project, filename, taskID string) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.SetClickUpTaskID(project, filename, taskID)
}

func (m *MultiStore) IncrementReviewCycle(project, filename string) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.IncrementReviewCycle(project, filename)
}

func (m *MultiStore) SetPlanGoal(project, filename, goal string) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.SetPlanGoal(project, filename, goal)
}

func (m *MultiStore) SetPRURL(project, filename, url string) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.SetPRURL(project, filename, url)
}

func (m *MultiStore) SetPRState(project, filename, reviewDecision, checkStatus string) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.SetPRState(project, filename, reviewDecision, checkStatus)
}

func (m *MultiStore) RecordPRReview(project, filename string, reviewID int, state, body, reviewer string) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.RecordPRReview(project, filename, reviewID, state, body, reviewer)
}

func (m *MultiStore) IsReviewProcessed(project, filename string, reviewID int) bool {
	store, err := m.storeFor(project)
	if err != nil {
		return false
	}
	return store.IsReviewProcessed(project, filename, reviewID)
}

func (m *MultiStore) MarkReviewReacted(project, filename string, reviewID int) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.MarkReviewReacted(project, filename, reviewID)
}

func (m *MultiStore) MarkReviewFixerDispatched(project, filename string, reviewID int) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.MarkReviewFixerDispatched(project, filename, reviewID)
}

func (m *MultiStore) ListPendingReviews(project, filename string) ([]PRReviewEntry, error) {
	store, err := m.storeFor(project)
	if err != nil {
		return nil, err
	}
	return store.ListPendingReviews(project, filename)
}

func (m *MultiStore) List(project string) ([]TaskEntry, error) {
	store, err := m.storeFor(project)
	if err != nil {
		return nil, err
	}
	return store.List(project)
}

func (m *MultiStore) ListByStatus(project string, statuses ...Status) ([]TaskEntry, error) {
	store, err := m.storeFor(project)
	if err != nil {
		return nil, err
	}
	return store.ListByStatus(project, statuses...)
}

func (m *MultiStore) ListByTopic(project, topic string) ([]TaskEntry, error) {
	store, err := m.storeFor(project)
	if err != nil {
		return nil, err
	}
	return store.ListByTopic(project, topic)
}

func (m *MultiStore) ListTopics(project string) ([]TopicEntry, error) {
	store, err := m.storeFor(project)
	if err != nil {
		return nil, err
	}
	return store.ListTopics(project)
}

func (m *MultiStore) CreateTopic(project string, entry TopicEntry) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	return store.CreateTopic(project, entry)
}

func (m *MultiStore) SetExecutionState(project, filename string, state ExecutionState) error {
	store, err := m.storeFor(project)
	if err != nil {
		return err
	}
	writer, ok := store.(ExecutionStateWriter)
	if !ok {
		entry, err := store.Get(project, filename)
		if err != nil {
			return err
		}
		entry.ExecutionState = state
		return store.Update(project, filename, entry)
	}
	return writer.SetExecutionState(project, filename, state)
}

func (m *MultiStore) Ping() error {
	for _, store := range m.stores {
		if err := store.Ping(); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiStore) Close() error {
	var firstErr error
	for _, store := range m.stores {
		if err := store.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *MultiSignalGateway) Create(project string, entry SignalEntry) error {
	gateway, err := m.gatewayFor(project)
	if err != nil {
		return err
	}
	return gateway.Create(project, entry)
}

func (m *MultiSignalGateway) List(project string, statuses ...SignalStatus) ([]SignalEntry, error) {
	gateway, err := m.gatewayFor(project)
	if err != nil {
		return nil, err
	}
	entries, err := gateway.List(project, statuses...)
	if err != nil {
		return nil, err
	}
	slot := m.slotByProject[project]
	for i := range entries {
		encodedID, err := encodeSignalID(slot, entries[i].ID)
		if err != nil {
			return nil, err
		}
		entries[i].ID = encodedID
	}
	return entries, nil
}

func (m *MultiSignalGateway) Claim(project, claimedBy string) (*SignalEntry, error) {
	gateway, err := m.gatewayFor(project)
	if err != nil {
		return nil, err
	}
	entry, err := gateway.Claim(project, claimedBy)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	rewritten := *entry
	encodedID, err := encodeSignalID(m.slotByProject[project], entry.ID)
	if err != nil {
		return nil, err
	}
	rewritten.ID = encodedID
	return &rewritten, nil
}

func (m *MultiSignalGateway) MarkProcessed(id int64, status SignalStatus, result string) error {
	gateway, localID, err := m.gatewayForID(id)
	if err != nil {
		return err
	}
	return gateway.MarkProcessed(localID, status, result)
}

func (m *MultiSignalGateway) ResetStuck(olderThan time.Duration) (int, error) {
	total := 0
	for _, gateway := range m.gateways {
		count, err := gateway.ResetStuck(olderThan)
		if err != nil {
			return total, err
		}
		total += count
	}
	return total, nil
}

func (m *MultiSignalGateway) Close() error {
	var firstErr error
	for _, gateway := range m.gateways {
		if err := gateway.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
