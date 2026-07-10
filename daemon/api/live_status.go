package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/livestatus"
)

func (h *Handler) handleLiveStatus(w http.ResponseWriter, r *http.Request) {
	project := r.PathValue("project")
	plans, err := h.state.ListPlans(project)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	instances := h.state.ListInstances(project)
	status := h.state.Status()
	writeJSON(w, http.StatusOK, livestatus.Assemble(buildLiveStatusInput(project, plans, instances, status)))
}

func buildLiveStatusInput(project string, plans []taskstore.TaskEntry, instances []InstanceStatus, status StatusResponse) livestatus.Input {
	tasks := make([]livestatus.TaskInput, 0, len(plans))
	for _, entry := range plans {
		tasks = append(tasks, livestatus.TaskInput{
			Filename:       entry.Filename,
			Status:         entry.Status,
			Phase:          entry.ExecutionState.Phase,
			ReviewFeedback: strings.TrimSpace(entry.LatestReviewFeedback) != "",
		})
	}

	agents := make([]livestatus.AgentInput, 0, len(instances))
	for _, instance := range instances {
		agents = append(agents, livestatus.AgentInput{
			Task:         instance.Plan,
			Role:         instance.Role,
			Wave:         instance.WaveNumber,
			Ready:        instance.Ready,
			Active:       instance.Active,
			Loading:      instance.Loading,
			HealthReason: instance.HealthReason,
		})
	}

	return livestatus.Input{
		Project: project,
		Now:     time.Now(),
		Daemon: livestatus.DaemonHeartbeat{
			Running:   status.Running,
			Uptime:    status.Uptime,
			RepoCount: status.RepoCount,
		},
		Tasks:  tasks,
		Agents: agents,
	}
}
