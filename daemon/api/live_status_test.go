package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/livestatus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type liveStatusState struct {
	*DaemonState
	plans map[string][]taskstore.TaskEntry
}

func (s *liveStatusState) ListPlans(project string) ([]taskstore.TaskEntry, error) {
	plans, ok := s.plans[project]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, project)
	}
	return plans, nil
}

func (s *liveStatusState) ListInstances(project string) []InstanceStatus {
	var instances []InstanceStatus
	for _, instance := range s.Instances {
		if instance.Project == project {
			instances = append(instances, instance)
		}
	}
	return instances
}

func TestLiveStatusSurfaceAdapter(t *testing.T) {
	state := &liveStatusState{
		DaemonState: &DaemonState{
			Running: true,
			Repos:   []RepoStatus{{Project: "kasmos"}},
			Instances: []InstanceStatus{
				{Project: "kasmos", Plan: "stale", Role: "coder", WaveNumber: 2, Active: true, HealthReason: "no heartbeat"},
			},
		},
		plans: map[string][]taskstore.TaskEntry{
			"kasmos": {
				{Filename: "decision", Status: taskstore.StatusImplementing, ExecutionState: taskstore.ExecutionState{Phase: "wave_waiting"}},
				{Filename: "review", Status: taskstore.StatusReviewing, LatestReviewFeedback: " changes requested "},
				{Filename: "ready", Status: taskstore.StatusReady},
			},
		},
	}

	recorder := httptest.NewRecorder()
	NewHandler(state).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/repos/kasmos/live-status", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var got livestatus.LiveStatus
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&got))
	assert.Equal(t, livestatus.SchemaVersion, got.SchemaVersion)
	assert.Equal(t, "kasmos", got.Project)
	assert.True(t, got.DaemonRunning)
	assert.Equal(t, 1, got.RepoCount)
	assert.Equal(t, livestatus.LifecycleCounts{Ready: 1, Implementing: 1, Reviewing: 1, Total: 3}, got.Lifecycle)
	assert.Contains(t, got.Attention, livestatus.AttentionItem{Task: "decision", Kind: livestatus.KindNeedsDecision})
	assert.Contains(t, got.Attention, livestatus.AttentionItem{Task: "review", Kind: livestatus.KindReviewFeedback})
	assert.Contains(t, got.Attention, livestatus.AttentionItem{Task: "stale", Kind: livestatus.KindStaleInstance, Detail: "no heartbeat"})
}

func TestLiveStatusUnknownProject(t *testing.T) {
	state := &liveStatusState{DaemonState: &DaemonState{}, plans: map[string][]taskstore.TaskEntry{}}
	recorder := httptest.NewRecorder()
	NewHandler(state).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/repos/missing/live-status", nil))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestLiveStatusCapQuery(t *testing.T) {
	tests := []struct {
		name        string
		cap         string
		agentCount  int
		wantCount   int
		wantOmitted int
	}{
		{name: "explicit cap", cap: "2", agentCount: 5, wantCount: 2, wantOmitted: 3},
		{name: "hard maximum", cap: "1000", agentCount: 105, wantCount: 100, wantOmitted: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instances := make([]InstanceStatus, tt.agentCount)
			for i := range instances {
				instances[i] = InstanceStatus{Project: "kasmos", Plan: fmt.Sprintf("task-%d", i), Active: true}
			}
			state := &liveStatusState{
				DaemonState: &DaemonState{Instances: instances},
				plans:       map[string][]taskstore.TaskEntry{"kasmos": {}},
			}

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/v1/repos/kasmos/live-status?cap="+tt.cap, nil)
			NewHandler(state).ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code)

			var got livestatus.LiveStatus
			require.NoError(t, json.NewDecoder(recorder.Body).Decode(&got))
			assert.Len(t, got.ActiveAgents, tt.wantCount)
			assert.Equal(t, tt.wantOmitted, got.Truncated.ActiveAgents)
		})
	}
}

func TestLiveStatusRejectsInvalidCap(t *testing.T) {
	state := &liveStatusState{DaemonState: &DaemonState{}, plans: map[string][]taskstore.TaskEntry{"kasmos": {}}}
	recorder := httptest.NewRecorder()
	NewHandler(state).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/repos/kasmos/live-status?cap=invalid", nil))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
