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
type UpdateModel struct {
	progress        progress.Model
	startedCharts   []string
	completedCharts []string
	baseModel
	totalCharts int
}

// NewUpdateModel creates an [UpdateModel] used to display the progress of
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
		progress:        p,
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
		completedCount := len(m.completedCharts)
		progressCmd := m.progress.SetPercent(float64(completedCount) / float64(m.totalCharts))

		icon := defaultStyles.check
		if msg.Err != nil {
			icon = defaultStyles.cross
		}

		return m, tea.Batch(
			progressCmd,
			tea.Printf("%s %s", icon, msg.Chart),
		)

	case progress.FrameMsg:
		if m.state == stateWorking {
			var cmd tea.Cmd

			m.progress, cmd = m.progress.Update(msg)

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
		return tea.NewView(getErrorMessage(m.err, m.width, m.totalCharts))

	case stateDone:
		completedCount := len(m.completedCharts)

		return tea.NewView(defaultStyles.done.Render(fmt.Sprintf("Done! Updated %d charts.\n", completedCount)))

	case stateWorking:
		completedCount := len(m.completedCharts)
		w := lipgloss.Width(strconv.Itoa(m.totalCharts))
		chartCount := fmt.Sprintf(" %*d/%*d", w, completedCount, w, m.totalCharts)

		prog := m.progress.View()
		progRendered := defaultStyles.progress.Render(prog + chartCount)
		progCellsRemaining := max(0, m.width-lipgloss.Width(progRendered))
		gap := strings.Repeat(" ", progCellsRemaining)
		progOut := progRendered + gap + "\n"

		inProgressCharts := differenceStringSlices(m.startedCharts, m.completedCharts)

		spinners := []string{}
		for _, chart := range inProgressCharts {
			spin := m.spinner.View() + " "
			cellsAvail := max(0, m.width-lipgloss.Width(spin))

			chartName := defaultStyles.itemName.Render(chart)
			info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render("Updating " + chartName)

			cellsRemaining := max(0, m.width-lipgloss.Width(spin+info))
			gap := strings.Repeat(" ", cellsRemaining) + "\n"

			spinners = append(spinners, spin+info+gap)
		}

		return tea.NewView(strings.Join(spinners, "") + progOut)

	case stateIdle:
		return tea.NewView("")
	}

	return tea.NewView("")
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
