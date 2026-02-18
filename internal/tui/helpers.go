package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/user/kasmos/internal/worker"
)

func analyzeCmd(backend worker.WorkerBackend, workerID, role string, exitCode int, duration, outputTail string) tea.Cmd {
	return func() tea.Msg {
		if backend == nil {
			return analyzeCompletedMsg{WorkerID: workerID, Err: fmt.Errorf("no backend")}
		}

		prompt := fmt.Sprintf(`Analyze this failed agent output and identify the root cause.

Worker: %s (%s)
Exit code: %d
Duration: %s

Output (last 200 lines):
%s

Respond in this exact format:
ROOT_CAUSE: <one paragraph explaining what went wrong>
SUGGESTED_PROMPT: <a revised prompt that would fix the issue>`, workerID, role, exitCode, duration, outputTail)

		cfg := worker.SpawnConfig{
			ID:     fmt.Sprintf("analyze-%s", workerID),
			Role:   "reviewer",
			Prompt: prompt,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		handle, err := backend.Spawn(ctx, cfg)
		if err != nil {
			return analyzeCompletedMsg{WorkerID: workerID, Err: err}
		}

		output := worker.NewOutputBuffer(1000)
		stdout := handle.Stdout()
		done := make(chan struct{})
		go func() {
			defer close(done)
			buf := make([]byte, 4096)
			for {
				n, readErr := stdout.Read(buf)
				if n > 0 {
					output.Append(string(buf[:n]))
				}
				if readErr != nil {
					return
				}
			}
		}()

		result := handle.Wait()
		<-done

		content := strings.TrimSpace(output.Content())
		if result.Error != nil && content == "" {
			return analyzeCompletedMsg{WorkerID: workerID, Err: result.Error}
		}

		rootCause, suggestedPrompt := parseAnalysisOutput(content)
		return analyzeCompletedMsg{
			WorkerID:        workerID,
			RootCause:       rootCause,
			SuggestedPrompt: suggestedPrompt,
		}
	}
}

func genPromptCmd(backend worker.WorkerBackend, taskID, title, description, suggestedRole string, deps []string) tea.Cmd {
	return func() tea.Msg {
		if backend == nil {
			return genPromptCompletedMsg{TaskID: taskID, Err: fmt.Errorf("no backend")}
		}

		prompt := fmt.Sprintf(`Generate an implementation prompt for this task.

Task: %s - %s
Description: %s
Dependencies: %s
Suggested role: %s

Generate a detailed, actionable prompt suitable for an AI coding agent.
The prompt should be specific enough to implement without further clarification.`, taskID, title, description, strings.Join(deps, ", "), suggestedRole)

		cfg := worker.SpawnConfig{
			ID:     fmt.Sprintf("genprompt-%s", taskID),
			Role:   "planner",
			Prompt: prompt,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		handle, err := backend.Spawn(ctx, cfg)
		if err != nil {
			return genPromptCompletedMsg{TaskID: taskID, Err: err}
		}

		output := worker.NewOutputBuffer(1000)
		stdout := handle.Stdout()
		done := make(chan struct{})
		go func() {
			defer close(done)
			buf := make([]byte, 4096)
			for {
				n, readErr := stdout.Read(buf)
				if n > 0 {
					output.Append(string(buf[:n]))
				}
				if readErr != nil {
					return
				}
			}
		}()

		result := handle.Wait()
		<-done

		generated := strings.TrimSpace(output.Content())
		if result.Error != nil && generated == "" {
			return genPromptCompletedMsg{TaskID: taskID, Err: result.Error}
		}

		return genPromptCompletedMsg{TaskID: taskID, Prompt: generated}
	}
}

func parseAnalysisOutput(output string) (rootCause, suggestedPrompt string) {
	if idx := strings.Index(output, "ROOT_CAUSE:"); idx >= 0 {
		rest := output[idx+len("ROOT_CAUSE:"):]
		if sugIdx := strings.Index(rest, "SUGGESTED_PROMPT:"); sugIdx >= 0 {
			rootCause = strings.TrimSpace(rest[:sugIdx])
			suggestedPrompt = strings.TrimSpace(rest[sugIdx+len("SUGGESTED_PROMPT:"):])
		} else {
			rootCause = strings.TrimSpace(rest)
		}
	} else {
		rootCause = strings.TrimSpace(output)
	}
	return rootCause, suggestedPrompt
}
