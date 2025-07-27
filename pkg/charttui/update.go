package charttui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/macropower/kclipper/pkg/chartcmd"
)

type UpdateModel struct {
	err             error
	startedCharts   []string
	completedCharts []string
	spinner         spinner.Model
	progress        progress.Model
	totalCharts     int
	width           int
	mu              sync.RWMutex
	working         bool
	done            bool
}

func NewUpdateModel() *UpdateModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	s := spinner.New()
	s.Style = spinnerStyle

	return &UpdateModel{
		startedCharts:   []string{},
		completedCharts: []string{},
		spinner:         s,
		progress:        p,
		mu:              sync.RWMutex{},
	}
}

func (m *UpdateModel) Init() tea.Cmd {
	return m.spinner.Tick
}

//nolint:ireturn // Third-party.
func (m *UpdateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		if keyExits(msg) {
			return m, tea.Quit
		}

	case teaMsgWriteLog:
		return m, writeLog(msg, m.width)

	case chartcmd.EventSetChartTotal:
		m.totalCharts = int(msg)

	case chartcmd.EventUpdatingChart:
		chart := string(msg)
		m.working = true
		m.startedCharts = append(m.startedCharts, chart)

	case chartcmd.EventUpdatedChart:
		m.completedCharts = append(m.completedCharts, msg.Chart)
		completedCount := len(m.completedCharts)
		progressCmd := m.progress.SetPercent(float64(completedCount) / float64(m.totalCharts))

		if m.totalCharts <= completedCount {
			m.working = false
		}

		icon := checkMark
		if msg.Err != nil {
			icon = errorMark
		}

		return m, tea.Batch(
			progressCmd,
			tea.Printf("%s %s", icon, msg.Chart),
		)

	case chartcmd.EventDone:
		// Allow previously sent messages to be drawn.
		preQuitCmd := tea.Tick(time.Millisecond*100, func(_ time.Time) tea.Msg {
			m.mu.Lock()
			defer m.mu.Unlock()

			m.err = msg.Err
			m.done = true

			return nil
		})

		return m, tea.Sequence(preQuitCmd, teaQuit())

	case spinner.TickMsg:
		var cmd tea.Cmd

		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd

	case progress.FrameMsg:
		if m.working {
			newModel, cmd := m.progress.Update(msg)
			if newModel, ok := newModel.(progress.Model); ok {
				m.progress = newModel
			}

			return m, cmd
		}
	}

	return m, nil
}

func (m *UpdateModel) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return getErrorMessage(m.err, m.width)
	}

	completedCount := len(m.completedCharts)

	if m.done {
		return doneStyle.Render(fmt.Sprintf("Done! Updated %d charts.\n", completedCount))
	}

	if m.working {
		w := lipgloss.Width(strconv.Itoa(m.totalCharts))
		chartCount := fmt.Sprintf(" %*d/%*d", w, completedCount, w, m.totalCharts)

		prog := m.progress.View()
		progRendered := progressStyle.Render(prog + chartCount)
		progCellsRemaining := max(0, m.width-lipgloss.Width(progRendered))
		gap := strings.Repeat(" ", progCellsRemaining)
		progOut := progRendered + gap + "\n"

		inProgressCharts := differenceStringSlices(m.startedCharts, m.completedCharts)

		spinners := []string{}
		for _, chart := range inProgressCharts {
			spin := m.spinner.View() + " "
			cellsAvail := max(0, m.width-lipgloss.Width(spin))

			chartName := currentNameStyle.Render(chart)
			info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render("Updating " + chartName)

			cellsRemaining := max(0, m.width-lipgloss.Width(spin+info))
			gap := strings.Repeat(" ", cellsRemaining) + "\n"

			spinners = append(spinners, spin+info+gap)
		}

		return strings.Join(spinners, "") + progOut
	}

	return ""
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
