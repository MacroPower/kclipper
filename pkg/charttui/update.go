package charttui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kclipper/pkg/chartcmd"
)

// UpdateModel displays the progress of updating one or more charts, including
// per-chart spinners, a progress bar, and a final summary.
// Create instances with [NewUpdateModel].
type UpdateModel struct {
	progress        *progress.Model
	startedCharts   []string
	completedCharts []string
	failedCharts    map[string]bool
	baseModel
	totalCharts int
}

// NewUpdateModel creates a new [UpdateModel] that displays the progress of
// updating charts.
func NewUpdateModel() *UpdateModel {
	p := progress.New(
		progress.WithDefaultBlend(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	return &UpdateModel{
		baseModel:       newBaseModel(),
		startedCharts:   []string{},
		completedCharts: []string{},
		failedCharts:    map[string]bool{},
		progress:        &p,
	}
}

func (m *UpdateModel) Init() tea.Cmd {
	return m.spinner.Tick
}

//nolint:ireturn // Third-party.
func (m *UpdateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chartcmd.EventSetChartTotal:
		m.totalCharts = int(msg)

	case chartcmd.EventUpdatingChart:
		chart := string(msg)
		m.state = stateWorking
		m.startedCharts = append(m.startedCharts, chart)

	case chartcmd.EventUpdatedChart:
		m.completedCharts = append(m.completedCharts, msg.Chart)
		if msg.Err != nil {
			m.failedCharts[msg.Chart] = true
		}

		completedCount := len(m.completedCharts)

		p := *m.progress
		progressCmd := p.SetPercent(float64(completedCount) / float64(m.totalCharts))
		m.progress = &p

		return m, progressCmd

	case progress.FrameMsg:
		if m.state == stateWorking {
			p, cmd := m.progress.Update(msg)
			m.progress = &p

			return m, cmd
		}

	default:
		if cmd, handled := m.handleCommon(msg); handled {
			return m, cmd
		}
	}

	return m, nil
}

func (m *UpdateModel) View() tea.View {
	switch m.state {
	case stateError:
		var out strings.Builder

		m.writeChartStatuses(&out)
		out.WriteString(GetErrorMessage(m.err, m.width, m.totalCharts))

		return tea.NewView(out.String())

	case stateDone:
		var out strings.Builder

		m.writeChartStatuses(&out)

		completedCount := len(m.completedCharts)
		out.WriteString(defaultStyles.done.Render(fmt.Sprintf("Done! Updated %d charts.\n", completedCount)))

		return tea.NewView(out.String())

	case stateWorking:
		var out strings.Builder

		m.writeChartStatuses(&out)

		completedCount := len(m.completedCharts)
		w := lipgloss.Width(strconv.Itoa(m.totalCharts))
		chartCount := fmt.Sprintf(" %*d/%*d", w, completedCount, w, m.totalCharts)

		prog := m.progress.View()
		progRendered := defaultStyles.progress.Render(prog + chartCount)
		progCellsRemaining := max(0, m.width-lipgloss.Width(progRendered))
		gap := strings.Repeat(" ", progCellsRemaining)
		progOut := progRendered + gap + "\n"

		inProgressCharts := differenceStringSlices(m.startedCharts, m.completedCharts)

		for _, chart := range inProgressCharts {
			spin := m.spinner.View() + " "
			cellsAvail := max(0, m.width-lipgloss.Width(spin))

			chartName := defaultStyles.itemName.Render(chart)
			info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render("Updating " + chartName)

			cellsRemaining := max(0, m.width-lipgloss.Width(spin+info))
			gap := strings.Repeat(" ", cellsRemaining) + "\n"

			out.WriteString(spin + info + gap)
		}

		out.WriteString(progOut)

		return tea.NewView(out.String())

	case stateIdle:
		return tea.NewView("")
	}

	return tea.NewView("")
}

// writeChartStatuses renders completed chart status lines (check or cross
// marks) into the given builder.
func (m *UpdateModel) writeChartStatuses(out *strings.Builder) {
	for _, chart := range m.completedCharts {
		icon := defaultStyles.check
		if m.failedCharts[chart] {
			icon = defaultStyles.cross
		}

		fmt.Fprintf(out, "%s %s\n", icon, chart)
	}
}

func differenceStringSlices(a, b []string) []string {
	difference := []string{}

	for _, x := range a {
		if !slices.Contains(b, x) {
			difference = append(difference, x)
		}
	}

	return difference
}
