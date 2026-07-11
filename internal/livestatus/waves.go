package livestatus

import (
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
)

func DeriveWaves(content string, subtasks []taskstore.SubtaskEntry, activeWave int) []WaveProgress {
	plan, err := taskparser.Parse(content)
	if err != nil {
		return nil
	}
	byNumber := make(map[int]taskstore.SubtaskEntry, len(subtasks))
	for _, subtask := range subtasks {
		byNumber[subtask.TaskNumber] = subtask
	}
	waves := make([]WaveProgress, 0, len(plan.Waves))
	for _, wave := range plan.Waves {
		progress := WaveProgress{Wave: wave.Number, Active: wave.Number == activeWave, Tasks: make([]WaveTask, 0, len(wave.Tasks))}
		for _, task := range wave.Tasks {
			status := "pending"
			if subtask, ok := byNumber[task.Number]; ok {
				status = string(subtask.Status)
			}
			progress.Tasks = append(progress.Tasks, WaveTask{Number: task.Number, Title: task.Title, Status: status})
		}
		waves = append(waves, progress)
	}
	return waves
}
