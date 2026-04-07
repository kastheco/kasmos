package ui

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

var (
	infoSectionStyle    = lipgloss.NewStyle().Foreground(ColorFoam).Bold(true)
	infoDividerStyle    = lipgloss.NewStyle().Foreground(ColorOverlay)
	infoLabelStyle      = lipgloss.NewStyle().Foreground(ColorMuted).Width(20)
	infoValueStyle      = lipgloss.NewStyle().Foreground(ColorText)
	infoPlanButtonStyle = lipgloss.NewStyle().Foreground(ColorFoam).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorOverlay).
				Padding(0, 1)
)

func renderViewPlanButton(label string) string {
	return zone.Mark(ZoneViewPlan, infoPlanButtonStyle.Render(label))
}

// InfoData carries display data for the info pane.
// Populated by the app layer from instance + plan + wave state.
type InfoData struct {
	// Instance fields
	Title   string
	Program string
	Branch  string
	Path    string
	Created string
	Status  string

	// Plan fields (empty when no plan is associated)
	PlanName        string
	PlanDescription string
	PlanStatus      string
	PlanGoal        string
	PlanTopic       string
	PlanBranch      string
	PlanCreated     string
	ExecutionPhase  string
	ActiveAgentType string
	ActiveWave      int
	ActiveRound     int
	PlanningAt      time.Time
	ImplementingAt  time.Time
	ReviewingAt     time.Time
	DoneAt          time.Time

	// Plan summary fields (rendered when plan header row is selected)
	PlanInstanceCount int
	PlanRunningCount  int
	PlanReadyCount    int
	PlanPausedCount   int
	PlanAddedLines    int
	PlanRemovedLines  int
	CompletedTasks    int
	TotalSubtasks     int
	AllWaveSubtasks   []WaveSubtaskGroup

	// Resource utilisation
	CPUPercent float64
	MemMB      float64

	// Wave / task context (zero values mean no wave info)
	AgentType     string
	WaveNumber    int
	TotalWaves    int
	TaskNumber    int
	TotalTasks    int
	WaveTasks     []WaveTaskInfo
	TaskTitle     string
	WaveTaskIndex int // 1-indexed position within the wave (0 = unknown / non-wave)
	WaveTaskCount int // total tasks in this wave (0 = unknown / non-wave)

	// Review outcome (populated when plan is done)
	ReviewCycle        int
	ReviewOutcome      string
	MaxReviewFixCycles int

	// Selection state flags
	HasPlan              bool
	HasInstance          bool
	IsPlanHeaderSelected bool
}

// WaveTaskInfo describes a single task slot in the current wave.
type WaveTaskInfo struct {
	Number int
	State  string // "complete", "running", "failed", or "pending"
}

// SubtaskDisplay describes a parsed subtask in all-wave progress rendering.
type SubtaskDisplay struct {
	Number int
	Title  string
	Status string
}

// WaveSubtaskGroup groups subtasks by wave number.
type WaveSubtaskGroup struct {
	WaveNumber int
	Subtasks   []SubtaskDisplay
}

// InfoPane renders instance and plan metadata inside a scrollable viewport.
type InfoPane struct {
	width, height int
	data          InfoData
	viewport      viewport.Model
}

// NewInfoPane returns a zero-sized InfoPane ready for use.
func NewInfoPane() *InfoPane {
	return &InfoPane{viewport: viewport.New()}
}

// SetSize stores dimensions and refreshes the viewport.
func (p *InfoPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.viewport.SetWidth(width)
	p.viewport.SetHeight(height)
	p.viewport.SetContent(p.render())
}

// SetData replaces the current data and re-renders from the top.
func (p *InfoPane) SetData(data InfoData) {
	p.data = data
	p.viewport.SetContent(p.render())
	p.viewport.GotoTop()
}

// ScrollUp moves the viewport one line toward the top.
func (p *InfoPane) ScrollUp() {
	p.viewport.ScrollUp(1)
}

// ScrollDown moves the viewport one line toward the bottom.
func (p *InfoPane) ScrollDown() {
	p.viewport.ScrollDown(1)
}

// String returns the visible portion of the pane, or a placeholder when
// neither an instance nor a plan header is selected.
func (p *InfoPane) String() string {
	if !p.data.HasInstance && !p.data.IsPlanHeaderSelected {
		return "no instance selected"
	}
	return p.viewport.View()
}

// statusColor maps known status strings to palette colors.
func statusColor(status string) color.Color {
	switch status {
	case "implementing":
		return ColorIris
	case "planning", "running":
		return ColorFoam
	case "reviewing", "done":
		return ColorGold
	case "ready", "cancelled", "paused":
		return ColorMuted
	case "failed", "error":
		return ColorLove
	default:
		return ColorText
	}
}

// renderRow renders a single label+value row.
func (p *InfoPane) renderRow(label, value string) string {
	valW := p.width - lipgloss.Width(infoLabelStyle.Render(label))
	if valW < 10 {
		valW = 10
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		infoLabelStyle.Render(label),
		infoValueStyle.Width(valW).Render(value),
	)
}

// renderStatusRow renders a label+value row where the value is coloured by status.
func (p *InfoPane) renderStatusRow(label, value string) string {
	valW := p.width - lipgloss.Width(infoLabelStyle.Render(label))
	if valW < 10 {
		valW = 10
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		infoLabelStyle.Render(label),
		lipgloss.NewStyle().Foreground(statusColor(value)).Width(valW).Render(value),
	)
}

func statusToGlyph(status string) (string, color.Color) {
	switch status {
	case "complete":
		return "✓", ColorFoam
	case "running":
		return "●", ColorIris
	case "failed":
		return "✗", ColorLove
	case "closed", "done":
		return "✓", ColorFoam
	default:
		return "○", ColorMuted
	}
}

func formatPhaseTime(ts time.Time) string {
	if ts.IsZero() {
		return "—"
	}
	return ts.Format("2006-01-02 15:04")
}

func infoAgentLabel(agent string) string {
	switch strings.TrimSpace(agent) {
	case "", "-":
		return ""
	case "elaborator":
		return "architect"
	default:
		return strings.TrimSpace(agent)
	}
}

func infoPhaseLabel(phase string, activeWave, activeRound int) string {
	switch strings.TrimSpace(phase) {
	case "planned":
		return "planned"
	case "architecting":
		return "architecting"
	case "wave_running":
		if activeWave > 0 {
			return fmt.Sprintf("wave %d running", activeWave)
		}
		return "wave running"
	case "wave_waiting":
		return "waiting for confirmation"
	case "single_agent_implementing":
		return "implementing"
	case "fixing":
		if activeRound > 0 {
			return fmt.Sprintf("fixing round %d", activeRound)
		}
		return "fixing"
	case "reviewing":
		if activeRound > 0 {
			return fmt.Sprintf("reviewing round %d", activeRound)
		}
		return "reviewing"
	default:
		return ""
	}
}

func (p *InfoPane) wrapText(text string) []string {
	if text == "" {
		return nil
	}
	maxWidth := p.width - lipgloss.Width(infoLabelStyle.Render("goal")) - 1
	if maxWidth < 10 {
		maxWidth = 10
	}
	var lines []string
	words := strings.Fields(text)
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
			continue
		}
		if lipgloss.Width(current)+1+lipgloss.Width(word) <= maxWidth {
			current += " " + word
		} else {
			lines = append(lines, p.renderRow("", current))
			current = word
		}
	}
	if current != "" {
		lines = append(lines, p.renderRow("", current))
	}
	return lines
}

// infoDisplayedWaveTotal returns the wave denominator clamped so it is never
// smaller than the numerator (guards against stale TotalWaves metadata).
// Returns 0 when totalWaves is unknown (<= 0); callers must render
// numerator-only (e.g. "wave 2") in that case.
func infoDisplayedWaveTotal(waveNumber, totalWaves int) int {
	if totalWaves <= 0 {
		return 0
	}
	if totalWaves < waveNumber {
		return waveNumber
	}
	return totalWaves
}

// infoDisplayedTaskCounter returns the (current, total, ok) counter to show
// for a task assignment. Wave-local counters (WaveTaskIndex / WaveTaskCount)
// are preferred when both are non-zero; falls back to global TaskNumber /
// TotalTasks for legacy or non-wave instances.
// When the total is unknown (zero), total is returned as 0 and callers must
// render numerator-only (e.g. "task 3") to avoid misleading "N/0" displays.
// Note: the wave-local path guarantees total > 0 via the waveTaskCount > 0 guard.
func infoDisplayedTaskCounter(taskNumber, totalTasks, waveTaskIndex, waveTaskCount int) (current, total int, ok bool) {
	if waveTaskIndex > 0 && waveTaskCount > 0 {
		// waveTaskCount > 0 guaranteed by the guard above; total is always known here.
		return waveTaskIndex, waveTaskCount, true
	}
	if taskNumber > 0 {
		// totalTasks may be 0 (unknown); callers handle tot==0 as numerator-only.
		return taskNumber, totalTasks, true
	}
	return 0, 0, false
}

// renderDivider renders a horizontal rule sized to the pane width.
func (p *InfoPane) renderDivider() string {
	w := p.width - 4
	if w < 10 {
		w = 10
	}
	return infoDividerStyle.Render(strings.Repeat("-", w))
}

func (p *InfoPane) renderGoalSection() string {
	rows := []string{
		infoSectionStyle.Render("goal"),
		p.renderDivider(),
	}
	if p.data.PlanGoal == "" {
		return strings.Join(rows, "\n")
	}
	rows = append(rows, p.wrapText(p.data.PlanGoal)...)
	return strings.Join(rows, "\n")
}

func (p *InfoPane) renderLifecycleSection() string {
	rows := []string{
		infoSectionStyle.Render("lifecycle"),
		p.renderDivider(),
	}
	if phase := infoPhaseLabel(p.data.ExecutionPhase, p.data.ActiveWave, p.data.ActiveRound); phase != "" {
		rows = append(rows, p.renderRow("current", phase))
	}
	if agent := infoAgentLabel(p.data.ActiveAgentType); agent != "" {
		rows = append(rows, p.renderRow("active agent", agent))
	}
	if p.data.ActiveWave > 0 {
		rows = append(rows, p.renderRow("active wave", fmt.Sprintf("%d", p.data.ActiveWave)))
	}
	if p.data.ActiveRound > 0 {
		rows = append(rows, p.renderRow("round", fmt.Sprintf("%d", p.data.ActiveRound)))
	}
	phases := []struct {
		label   string
		time    time.Time
		reached bool
	}{
		{"planning", p.data.PlanningAt, !p.data.PlanningAt.IsZero()},
		{"implementing", p.data.ImplementingAt, !p.data.ImplementingAt.IsZero()},
		{"reviewing", p.data.ReviewingAt, !p.data.ReviewingAt.IsZero()},
		{"done", p.data.DoneAt, !p.data.DoneAt.IsZero()},
	}
	for _, phase := range phases {
		glyph := "○"
		if phase.reached {
			glyph = "●"
		}
		rows = append(rows, p.renderRow(phase.label, fmt.Sprintf("%s %s", glyph, formatPhaseTime(phase.time))))
	}

	return strings.Join(rows, "\n")
}

func asciiProgressBar(total, done int) string {
	barWidth := 10
	if total <= 0 {
		return "[          ]"
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	filled := int(float64(done) / float64(total) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", barWidth-filled) + "]"
}

func (p *InfoPane) renderProgressSection() string {
	rows := []string{
		infoSectionStyle.Render("progress"),
		p.renderDivider(),
		p.renderRow("subtasks", fmt.Sprintf("%d/%d %s", p.data.CompletedTasks, p.data.TotalSubtasks, asciiProgressBar(p.data.TotalSubtasks, p.data.CompletedTasks))),
	}
	if p.data.ActiveWave > 0 {
		activeWave := fmt.Sprintf("%d", p.data.ActiveWave)
		if p.data.TotalWaves > 0 {
			waveTotal := infoDisplayedWaveTotal(p.data.ActiveWave, p.data.TotalWaves)
			activeWave = fmt.Sprintf("%d/%d", p.data.ActiveWave, waveTotal)
		}
		rows = append(rows, p.renderRow("active wave", activeWave))
	}

	groups := append([]WaveSubtaskGroup{}, p.data.AllWaveSubtasks...)
	sort.Slice(groups, func(i, j int) bool { return groups[i].WaveNumber < groups[j].WaveNumber })
	for _, group := range groups {
		rows = append(rows, p.renderRow(fmt.Sprintf("wave %d", group.WaveNumber), ""))
		for _, task := range group.Subtasks {
			glyph, col := statusToGlyph(task.Status)
			icon := lipgloss.NewStyle().Foreground(col).Render(glyph)
			taskLine := fmt.Sprintf("%s task %d: %s", icon, task.Number, task.Title)
			rows = append(rows, infoValueStyle.Render(taskLine))
		}
	}
	return strings.Join(rows, "\n")
}

// renderPlanSection renders the plan metadata block for instance-bound views.
func (p *InfoPane) renderPlanSection() string {
	rows := []string{
		infoSectionStyle.Render("plan"),
		p.renderDivider(),
	}
	if p.data.PlanName != "" {
		rows = append(rows, p.renderRow("name", p.data.PlanName))
	}
	if p.data.PlanDescription != "" {
		rows = append(rows, p.renderRow("description", p.data.PlanDescription))
	}
	if p.data.PlanStatus != "" {
		rows = append(rows, p.renderStatusRow("status", p.data.PlanStatus))
	}
	if phase := infoPhaseLabel(p.data.ExecutionPhase, p.data.ActiveWave, p.data.ActiveRound); phase != "" {
		rows = append(rows, p.renderRow("phase", phase))
	}
	if agent := infoAgentLabel(p.data.ActiveAgentType); agent != "" {
		rows = append(rows, p.renderRow("active agent", agent))
	}
	if p.data.PlanTopic != "" {
		rows = append(rows, p.renderRow("topic", p.data.PlanTopic))
	}
	if p.data.PlanBranch != "" {
		rows = append(rows, p.renderRow("branch", p.data.PlanBranch))
	}
	if p.data.PlanCreated != "" {
		rows = append(rows, p.renderRow("created", p.data.PlanCreated))
	}
	return strings.Join(rows, "\n")
}

// renderInstanceSection renders the instance metadata block.
func (p *InfoPane) renderInstanceSection() string {
	rows := []string{
		infoSectionStyle.Render("instance"),
		p.renderDivider(),
	}
	if p.data.Title != "" {
		rows = append(rows, p.renderRow("title", p.data.Title))
	}
	if p.data.AgentType != "" {
		rows = append(rows, p.renderRow("role", p.data.AgentType))
	}
	if p.data.Program != "" {
		rows = append(rows, p.renderRow("program", p.data.Program))
	}
	if p.data.Status != "" {
		rows = append(rows, p.renderStatusRow("status", p.data.Status))
	}
	if p.data.Branch != "" {
		rows = append(rows, p.renderRow("branch", p.data.Branch))
	}
	if p.data.Path != "" {
		rows = append(rows, p.renderRow("path", p.data.Path))
	}
	if p.data.Created != "" {
		rows = append(rows, p.renderRow("created", p.data.Created))
	}
	if p.data.PlanGoal != "" {
		rows = append(rows, p.renderRow("goal", p.data.PlanGoal))
	}
	if phase := infoPhaseLabel(p.data.ExecutionPhase, p.data.ActiveWave, p.data.ActiveRound); phase != "" {
		rows = append(rows, p.renderRow("phase", phase))
	}
	if agent := infoAgentLabel(p.data.ActiveAgentType); agent != "" {
		rows = append(rows, p.renderRow("active agent", agent))
	}
	if p.data.WaveNumber > 0 {
		waveTotal := infoDisplayedWaveTotal(p.data.WaveNumber, p.data.TotalWaves)
		var waveText string
		if waveTotal > 0 {
			waveText = fmt.Sprintf("%d/%d", p.data.WaveNumber, waveTotal)
		} else {
			waveText = fmt.Sprintf("%d", p.data.WaveNumber)
		}
		rows = append(rows, p.renderRow("wave", waveText))
	}
	if cur, tot, ok := infoDisplayedTaskCounter(p.data.TaskNumber, p.data.TotalTasks, p.data.WaveTaskIndex, p.data.WaveTaskCount); ok {
		var taskText string
		if tot > 0 {
			taskText = fmt.Sprintf("%d of %d", cur, tot)
			if p.data.TaskTitle != "" {
				taskText = fmt.Sprintf("%d of %d: %s", cur, tot, p.data.TaskTitle)
			}
		} else {
			taskText = fmt.Sprintf("%d", cur)
			if p.data.TaskTitle != "" {
				taskText = fmt.Sprintf("%d: %s", cur, p.data.TaskTitle)
			}
		}
		rows = append(rows, p.renderRow("task", taskText))
	}
	if p.data.CPUPercent > 0 || p.data.MemMB > 0 {
		rows = append(rows, p.renderRow("cpu", fmt.Sprintf("%.0f%%", math.Round(p.data.CPUPercent))))
		rows = append(rows, p.renderRow("memory", fmt.Sprintf("%.0fM", p.data.MemMB)))
	}
	if p.data.HasPlan {
		rows = append(rows, "", renderViewPlanButton("view plan doc"))
	}
	return strings.Join(rows, "\n")
}

// renderReviewSection renders the review outcome block (cycle and outcome)
// for plans that have been approved. Only appended to the plan summary when
// ReviewOutcome is non-empty.
func (p *InfoPane) renderReviewSection() string {
	rows := []string{
		infoSectionStyle.Render("review"),
		p.renderDivider(),
	}
	if p.data.ReviewCycle > 0 {
		cycleLabel := fmt.Sprintf("%d", p.data.ReviewCycle)
		if p.data.MaxReviewFixCycles > 0 {
			cycleLabel = fmt.Sprintf("%d / %d", p.data.ReviewCycle, p.data.MaxReviewFixCycles)
		}
		rows = append(rows, p.renderRow("cycle", cycleLabel))
	}
	rows = append(rows, p.renderStatusRow("outcome", p.data.ReviewOutcome))
	return strings.Join(rows, "\n")
}

// renderPlanSummary renders the plan header view: metadata, instance counts,
// line-change totals, and a "view plan doc" action button.
func (p *InfoPane) renderPlanSummary() string {
	rows := []string{p.renderPlanSection()}
	rows = append(rows, p.renderGoalSection())
	rows = append(rows, p.renderLifecycleSection())
	rows = append(rows, p.renderProgressSection())
	if p.data.ReviewOutcome != "" {
		rows = append(rows, p.renderReviewSection())
	}
	if p.data.PlanInstanceCount > 0 {
		instanceSummary := fmt.Sprintf("%d", p.data.PlanInstanceCount)
		var parts []string
		if p.data.PlanRunningCount > 0 {
			parts = append(parts, fmt.Sprintf("%d running", p.data.PlanRunningCount))
		}
		if p.data.PlanReadyCount > 0 {
			parts = append(parts, fmt.Sprintf("%d ready", p.data.PlanReadyCount))
		}
		if p.data.PlanPausedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d paused", p.data.PlanPausedCount))
		}
		if len(parts) > 0 {
			instanceSummary += " (" + strings.Join(parts, ", ") + ")"
		}
		rows = append(rows, infoSectionStyle.Render("instances"), p.renderDivider(), p.renderRow("instances", instanceSummary))
	}
	rows = append(rows, "")
	rows = append(rows, renderViewPlanButton("view plan doc"))

	return strings.Join(rows, "\n\n")
}

// renderWaveSection renders the task-state grid for the current wave.
func (p *InfoPane) renderWaveSection() string {
	rows := []string{
		infoSectionStyle.Render("wave progress"),
		p.renderDivider(),
	}
	for _, t := range p.data.WaveTasks {
		var glyph string
		var col color.Color
		switch t.State {
		case "complete":
			glyph, col = "✓", ColorFoam
		case "running":
			glyph, col = "●", ColorIris
		case "failed":
			glyph, col = "✗", ColorLove
		default:
			glyph, col = "○", ColorMuted
		}
		icon := lipgloss.NewStyle().Foreground(col).Render(glyph)
		rows = append(rows, infoLabelStyle.Render(fmt.Sprintf("task %d", t.Number))+icon+" "+t.State)
	}
	return strings.Join(rows, "\n")
}

// RenderCompact returns a 1-2 line summary suitable for display above the tab
// bar. Returns empty string when no meaningful data is present.
func (p *InfoPane) RenderCompact(width int, minimal ...bool) string {
	if width <= 0 {
		return ""
	}
	d := p.data
	min := len(minimal) > 0 && minimal[0]
	var lines []string

	// Choose plan summary first, else instance summary.
	if d.PlanName != "" || d.IsPlanHeaderSelected {
		if d.PlanName != "" {
			nameStyle := lipgloss.NewStyle().Foreground(ColorFoam).Bold(true)
			statusStyle := lipgloss.NewStyle().Foreground(statusColor(d.PlanStatus))
			line1 := nameStyle.Render(d.PlanName)
			phase := infoPhaseLabel(d.ExecutionPhase, d.ActiveWave, d.ActiveRound)
			phaseStyle := lipgloss.NewStyle().Foreground(ColorGold)
			var states []string
			if d.PlanStatus != "" {
				states = append(states, statusStyle.Render(d.PlanStatus))
			}
			if phase != "" && phase != d.PlanStatus {
				states = append(states, phaseStyle.Render(phase))
			}
			if len(states) > 0 {
				line1 += "  " + strings.Join(states, " · ")
			}
			lines = append(lines, line1)

			if !min {
				// Build line 2 from branch + wave/task counters.
				var parts []string
				branch := d.PlanBranch
				if branch == "" {
					branch = d.Branch
				}
				if branch != "" {
					parts = append(parts, lipgloss.NewStyle().Foreground(ColorMuted).Render(branch))
				}
				if agent := infoAgentLabel(d.ActiveAgentType); agent != "" {
					parts = append(parts, agent)
				}
				// Show "active wave N" only when the selected instance has no concrete
				// wave counter of its own (which would make the label redundant).
				hasInstanceWave := d.WaveNumber > 0
				if d.ActiveWave > 0 && !hasInstanceWave {
					parts = append(parts, fmt.Sprintf("active wave %d", d.ActiveWave))
				}
				if d.ActiveRound > 0 {
					parts = append(parts, fmt.Sprintf("round %d", d.ActiveRound))
				}
				if d.WaveNumber > 0 {
					waveTotal := infoDisplayedWaveTotal(d.WaveNumber, d.TotalWaves)
					if waveTotal > 0 {
						parts = append(parts, fmt.Sprintf("wave %d/%d", d.WaveNumber, waveTotal))
					} else {
						parts = append(parts, fmt.Sprintf("wave %d", d.WaveNumber))
					}
				}
				if cur, tot, ok := infoDisplayedTaskCounter(d.TaskNumber, d.TotalTasks, d.WaveTaskIndex, d.WaveTaskCount); ok {
					if tot > 0 {
						parts = append(parts, fmt.Sprintf("task %d/%d", cur, tot))
					} else {
						parts = append(parts, fmt.Sprintf("task %d", cur))
					}
				}
				if len(parts) > 0 {
					lines = append(lines, strings.Join(parts, "  "))
				}
			}
		}
		if !min && (d.HasPlan || d.IsPlanHeaderSelected) {
			lines = append(lines, renderViewPlanButton("view plan [p]"))
		}
	} else if d.HasInstance && d.Title != "" {
		nameStyle := lipgloss.NewStyle().Foreground(ColorFoam).Bold(true)
		statusStyle := lipgloss.NewStyle().Foreground(statusColor(d.Status))
		line1 := nameStyle.Render(d.Title)
		phase := infoPhaseLabel(d.ExecutionPhase, d.ActiveWave, d.ActiveRound)
		phaseStyle := lipgloss.NewStyle().Foreground(ColorGold)
		var states []string
		if d.Status != "" {
			states = append(states, statusStyle.Render(d.Status))
		}
		if phase != "" {
			states = append(states, phaseStyle.Render(phase))
		}
		if len(states) > 0 {
			line1 += "  " + strings.Join(states, " · ")
		}
		lines = append(lines, line1)

		if !min {
			// Build line 2 from branch + wave/task counters.
			var parts []string
			if d.Branch != "" {
				parts = append(parts, lipgloss.NewStyle().Foreground(ColorMuted).Render(d.Branch))
			}
			if agent := infoAgentLabel(d.ActiveAgentType); agent != "" {
				parts = append(parts, agent)
			}
			// Suppress "active wave N" when a concrete wave counter is already shown.
			hasInstanceWave := d.WaveNumber > 0
			if d.ActiveWave > 0 && !hasInstanceWave {
				parts = append(parts, fmt.Sprintf("active wave %d", d.ActiveWave))
			}
			if d.ActiveRound > 0 {
				parts = append(parts, fmt.Sprintf("round %d", d.ActiveRound))
			}
			if d.WaveNumber > 0 {
				waveTotal := infoDisplayedWaveTotal(d.WaveNumber, d.TotalWaves)
				if waveTotal > 0 {
					parts = append(parts, fmt.Sprintf("wave %d/%d", d.WaveNumber, waveTotal))
				} else {
					parts = append(parts, fmt.Sprintf("wave %d", d.WaveNumber))
				}
			}
			if cur, tot, ok := infoDisplayedTaskCounter(d.TaskNumber, d.TotalTasks, d.WaveTaskIndex, d.WaveTaskCount); ok {
				if tot > 0 {
					parts = append(parts, fmt.Sprintf("task %d/%d", cur, tot))
				} else {
					parts = append(parts, fmt.Sprintf("task %d", cur))
				}
			}
			if len(parts) > 0 {
				lines = append(lines, strings.Join(parts, "  "))
			}
			if d.HasPlan {
				lines = append(lines, renderViewPlanButton("view plan [p]"))
			}
		}
	}

	if len(lines) == 0 {
		return ""
	}
	return lipgloss.NewStyle().Width(width).Padding(0, 1).Render(strings.Join(lines, "\n"))
}

// render assembles the full content string placed into the viewport.
func (p *InfoPane) render() string {
	if !p.data.HasInstance && !p.data.IsPlanHeaderSelected {
		return "no instance selected"
	}

	var sections []string
	switch {
	case p.data.IsPlanHeaderSelected:
		sections = append(sections, p.renderPlanSummary())
		if len(p.data.WaveTasks) > 0 {
			sections = append(sections, p.renderWaveSection())
		}
	default:
		if p.data.HasPlan {
			sections = append(sections, p.renderPlanSection())
		}
		sections = append(sections, p.renderInstanceSection())
		if len(p.data.WaveTasks) > 0 {
			sections = append(sections, p.renderWaveSection())
		}
	}

	return strings.Join(sections, "\n\n")
}
